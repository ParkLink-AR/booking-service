package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func setupDB() *sql.DB {
	dsn := envOrDefault("DB_DSN", "postgres://appuser:apppassword@localhost:5432/cleanapi?sslmode=disable")
	database, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err := database.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}
	if err := runMigrations(database); err != nil {
		log.Fatalf("migrations: %v", err)
	}
	log.Println("database ready")
	return database
}

// runMigrations creates the booking-service schema (bookings, booking_status_history,
// reviews). Mirrors migrations/001-006_*.sql for environments that run migrations inline
// at startup.
//
// There is no EXCLUDE constraint on bookings: a capacity-N parking spot allows up to N
// overlapping active bookings, so overlap is enforced in the repository under a per-spot
// advisory lock (see internal/repository/booking/create_booking.go).
func runMigrations(database *sql.DB) error {
	_, err := database.Exec(`
		CREATE EXTENSION IF NOT EXISTS btree_gist;

		DO $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'booking_status') THEN
				CREATE TYPE booking_status AS ENUM (
				  'PENDING_PAYMENT',
				  'CONFIRMED',
				  'COMPLETED',
				  'CANCELLED',
				  'EXPIRED',
				  'PENDING_OWNER_APPROVAL'
				);
			END IF;
		END$$;

		CREATE TABLE IF NOT EXISTS bookings (
			id                       UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
			parking_spot_id          UUID          NOT NULL,
			user_id                  UUID          NOT NULL,
			time_range               TSTZRANGE     NOT NULL,
			status                   booking_status NOT NULL,
			hold_expires_at          TIMESTAMPTZ,
			owner_response_deadline  TIMESTAMPTZ,
			amount                   INT           NOT NULL,
			payment_id               TEXT,
			capacity                 INT           NOT NULL DEFAULT 1,
			kind                     TEXT          NOT NULL DEFAULT 'HOURLY' CHECK (kind IN ('HOURLY', 'STAY')),
			created_at               TIMESTAMPTZ   NOT NULL DEFAULT now(),
			updated_at               TIMESTAMPTZ   NOT NULL DEFAULT now()
		);

		CREATE INDEX IF NOT EXISTS idx_bookings_spot_range
			ON bookings USING GIST (parking_spot_id, time_range);

		CREATE TABLE IF NOT EXISTS booking_status_history (
			id          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
			booking_id  UUID          NOT NULL REFERENCES bookings(id),
			from_status booking_status,
			to_status   booking_status NOT NULL,
			reason      TEXT,
			created_at  TIMESTAMPTZ   NOT NULL DEFAULT now()
		);

		CREATE INDEX IF NOT EXISTS idx_bsh_booking_id ON booking_status_history (booking_id);

		-- A review is tied to one completed booking and targets one of three
		-- things: the spot, its owner, or the driver. The UNIQUE constraint
		-- stops the same reviewer from rating the same target twice for the
		-- same booking.
		CREATE TABLE IF NOT EXISTS reviews (
			id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			booking_id   UUID        NOT NULL REFERENCES bookings(id),
			reviewer_id  UUID        NOT NULL,
			target_type  TEXT        NOT NULL CHECK (target_type IN ('spot', 'owner', 'driver')),
			target_id    UUID        NOT NULL,
			rating       SMALLINT    NOT NULL CHECK (rating BETWEEN 1 AND 5),
			comment      TEXT,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

			UNIQUE (booking_id, reviewer_id, target_type)
		);

		CREATE INDEX IF NOT EXISTS idx_reviews_target ON reviews (target_type, target_id);
		CREATE INDEX IF NOT EXISTS idx_reviews_booking ON reviews (booking_id);

		-- migrations/006: for databases created before capacity existed / with the
		-- old EXCLUDE constraint (a capacity-N "playa" allows N overlapping bookings).
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS capacity INT NOT NULL DEFAULT 1;
		DO $$
		DECLARE cname text;
		BEGIN
			SELECT conname INTO cname FROM pg_constraint
				WHERE conrelid = 'bookings'::regclass AND contype = 'x';
			IF cname IS NOT NULL THEN
				EXECUTE format('ALTER TABLE bookings DROP CONSTRAINT %I', cname);
			END IF;
		END$$;

		-- migrations/007: for databases created before "estadía" (day/week/month)
		-- bookings existed. Every pre-existing row is an hourly slot booking.
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'HOURLY'
			CHECK (kind IN ('HOURLY', 'STAY'));

		-- migrations/008: adds IN_PROGRESS to booking_status for databases created
		-- before in-person verification existed. Safe to run alongside the rest of this
		-- script because nothing else here references the new value -- Postgres only
		-- forbids using a brand-new enum value in the same transaction that added it.
		ALTER TYPE booking_status ADD VALUE IF NOT EXISTS 'IN_PROGRESS';

		-- migrations/009: in-person verification -- the driver reads a 4-digit code in
		-- the app and tells it to the owner, who enters it to prove the driver actually
		-- showed up (see usecase.VerifyBookingCodeUseCase).
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS verification_code VARCHAR(4);
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS verification_attempts INT NOT NULL DEFAULT 0;
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ;

		-- migrations/010: in-app messaging between a booking's driver and its spot's
		-- owner (e.g. access instructions before the driver arrives).
		CREATE TABLE IF NOT EXISTS booking_messages (
			id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			booking_id  UUID        NOT NULL REFERENCES bookings(id),
			sender_id   UUID        NOT NULL,
			body        TEXT        NOT NULL,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS idx_booking_messages_booking ON booking_messages (booking_id, created_at);

		-- migrations/011: booking-scoped support tickets, opened by the driver or the
		-- owner, answered by ParkLink staff via admin-dashboard (see usecase.AdminReplyUseCase).
		CREATE TABLE IF NOT EXISTS support_tickets (
			id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			booking_id  UUID        NOT NULL REFERENCES bookings(id),
			created_by  UUID        NOT NULL,
			subject     TEXT        NOT NULL,
			status      TEXT        NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'IN_PROGRESS', 'RESOLVED', 'CLOSED')),
			created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS idx_support_tickets_booking ON support_tickets (booking_id, created_at);
		CREATE INDEX IF NOT EXISTS idx_support_tickets_status ON support_tickets (status, created_at);
		CREATE TABLE IF NOT EXISTS support_ticket_messages (
			id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			ticket_id   UUID        NOT NULL REFERENCES support_tickets(id),
			sender_id   UUID        NOT NULL,
			is_admin    BOOLEAN     NOT NULL DEFAULT false,
			body        TEXT        NOT NULL,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS idx_support_ticket_messages_ticket ON support_ticket_messages (ticket_id, created_at);

		-- migrations/012: records the moment a booking's payment was actually confirmed
		-- by Mercado Pago, independent of updated_at (which every later transition also
		-- bumps) -- see ConfirmBooking.
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS paid_at TIMESTAMPTZ;

		-- migrations/013: distinguishes why a booking ended up CANCELLED (driver
		-- cancelled it themselves vs. the owner rejected it vs. the owner never
		-- responded in time) -- see CancelBooking/RejectBooking/ExpireStalePendingApprovals.
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS cancellation_reason TEXT
			CHECK (cancellation_reason IN ('OWNER_REJECTED', 'OWNER_EXPIRED', 'DRIVER_CANCELLED'));

		-- migrations/014: what ParkLink owes a parking spot's owner for a single completed,
		-- paid booking, net of platform commission -- see usecase.GeneratePendingPayoutsUseCase
		-- and (since migrations/016/017) usecase.DraftSettlementUseCase/ConfirmSettlementUseCase.
		CREATE TABLE IF NOT EXISTS payouts (
			id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			booking_id        UUID        NOT NULL UNIQUE REFERENCES bookings(id),
			owner_id          UUID        NOT NULL,
			payment_id        TEXT        NOT NULL,
			gross_amount      INT         NOT NULL,
			commission_amount INT         NOT NULL,
			net_amount        INT         NOT NULL,
			status            TEXT        NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PAID')),
			paid_at           TIMESTAMPTZ,
			paid_by_admin_id  UUID,
			proof_url         TEXT,
			created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS idx_payouts_status ON payouts (status);
		CREATE INDEX IF NOT EXISTS idx_payouts_owner ON payouts (owner_id);

		-- migrations/015: singleton row for ParkLink-wide settings the admin panel can
		-- change without a deploy -- currently just the platform commission rate (basis
		-- points; 500 = 5.00%).
		CREATE TABLE IF NOT EXISTS platform_settings (
			id                  BOOLEAN     PRIMARY KEY DEFAULT true CHECK (id),
			commission_rate_bp  INT         NOT NULL DEFAULT 500,
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_by_admin_id UUID
		);
		INSERT INTO platform_settings (id, commission_rate_bp)
		VALUES (true, 500)
		ON CONFLICT (id) DO NOTHING;

		-- migrations/016: a single payout event to a parking-spot owner, covering every
		-- payout that was unattached for them at the moment an admin drafted it -- see
		-- usecase.DraftSettlementUseCase and usecase.ConfirmSettlementUseCase. Two-phase:
		-- PENDING (drafted, amount frozen, nothing has moved yet) -> PAID (an admin
		-- confirmed the bank transfer actually went out).
		CREATE TABLE IF NOT EXISTS settlements (
			id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			owner_id            UUID        NOT NULL,
			gross_amount        INT         NOT NULL,
			commission_amount   INT         NOT NULL,
			net_amount          INT         NOT NULL,
			status              TEXT        NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PAID')),
			proof_url           TEXT,
			paid_at             TIMESTAMPTZ,
			paid_by_admin_id    UUID,
			created_by_admin_id UUID        NOT NULL,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS idx_settlements_owner ON settlements (owner_id);
		CREATE INDEX IF NOT EXISTS idx_settlements_status ON settlements (status);

		-- migrations/017: links an individual payout to the settlement draft that swept it
		-- up -- non-null as soon as a payout is drafted into a settlement, regardless of
		-- whether that settlement has been confirmed yet.
		ALTER TABLE payouts ADD COLUMN IF NOT EXISTS settlement_id UUID REFERENCES settlements(id);
		CREATE INDEX IF NOT EXISTS idx_payouts_settlement ON payouts (settlement_id);

		-- migrations/018: the owner's own price at the moment this booking was priced,
		-- before the platform commission was added on top for the driver -- stored once so
		-- a later admin change to the commission rate can never retroactively change what
		-- an already-made booking's owner gets paid out. Nullable: a booking from before
		-- this column existed has no way to recover what rate was actually in effect for
		-- it (see GeneratePendingPayoutsUseCase's fallback for that case).
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS owner_amount INT;

		-- migrations/019: an entirely optional snapshot of whichever vehicle (if any) the
		-- driver picked from their own saved list at booking time -- copied in, not a live
		-- reference, so editing/deleting that saved vehicle later never changes an
		-- already-made booking's record.
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS vehicle_plate TEXT;
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS vehicle_brand TEXT;
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS vehicle_model TEXT;
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS vehicle_country TEXT;

		-- migrations/022: the type of that vehicle (car/suv/pickup/van/motorcycle/bicycle)
		-- -- decides the price of a spot that charges per vehicle type. A bicycle has a type
		-- but no plate.
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS vehicle_type TEXT;

		-- migrations/020: flags a paid booking that completed without its in-person arrival
		-- code ever being verified, for an admin to review before the owner is paid --
		-- PENDING blocks the payout until it's APPROVED (paid out) or REFUNDED (driver gets
		-- the money back). NULL for verified bookings and anything completed before this.
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS verification_review TEXT
			CHECK (verification_review IN ('PENDING', 'APPROVED', 'REFUNDED'));

		-- migrations/021: bookings cancelled because one side's account was closed or
		-- suspended (see usecase.AccountClosureUseCase) -- the driver always got a full refund.
		ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_cancellation_reason_check;
		ALTER TABLE bookings ADD CONSTRAINT bookings_cancellation_reason_check
			CHECK (cancellation_reason IN (
				'OWNER_REJECTED', 'OWNER_EXPIRED', 'DRIVER_CANCELLED',
				'OWNER_ACCOUNT_CLOSED', 'DRIVER_ACCOUNT_CLOSED'
			)) NOT VALID; -- this whole script re-runs on every boot: a later migration widens the
			-- list, so rows it allows must not fail this older step (see migrations/025).
		CREATE INDEX IF NOT EXISTS idx_bookings_verification_review_pending
			ON bookings (updated_at) WHERE verification_review = 'PENDING';

		-- migrations/023: when each reminder about a booking went out (see
		-- SendBookingRemindersUseCase) -- set in the same UPDATE that picks it up, so each
		-- reminder is sent once.
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS approval_reminder_sent_at TIMESTAMPTZ;
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS start_reminder_sent_at TIMESTAMPTZ;
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS end_reminder_sent_at TIMESTAMPTZ;

		-- migrations/024: admin tools (see usecase/booking/admin_*):
		-- * a booking ParkLink staff cancelled or refunded by hand: how much of it went back to the
		--   driver, and why (shown in the panel);
		-- * reviews staff hid (offensive, fake) -- kept, but left out of every public listing and
		--   average;
		-- * reports users file from the app about a spot, a person or a review.
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS refunded_amount INTEGER NOT NULL DEFAULT 0;
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS admin_note TEXT;

		ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_cancellation_reason_check;
		ALTER TABLE bookings ADD CONSTRAINT bookings_cancellation_reason_check
			CHECK (cancellation_reason IN (
				'OWNER_REJECTED', 'OWNER_EXPIRED', 'DRIVER_CANCELLED',
				'OWNER_ACCOUNT_CLOSED', 'DRIVER_ACCOUNT_CLOSED', 'ADMIN_CANCELLED'
			)) NOT VALID; -- see migrations/021 above

		ALTER TABLE reviews ADD COLUMN IF NOT EXISTS hidden_at TIMESTAMPTZ;
		ALTER TABLE reviews ADD COLUMN IF NOT EXISTS hidden_reason TEXT;
		ALTER TABLE reviews ADD COLUMN IF NOT EXISTS hidden_by TEXT;

		CREATE TABLE IF NOT EXISTS reports (
			id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			reporter_id  UUID NOT NULL,
			target_type  TEXT NOT NULL CHECK (target_type IN ('spot', 'user', 'review')),
			target_id    TEXT NOT NULL,
			reason       TEXT NOT NULL,
			details      TEXT NOT NULL DEFAULT '',
			status       TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'RESOLVED', 'DISMISSED')),
			admin_note   TEXT,
			resolved_by  TEXT,
			resolved_at  TIMESTAMPTZ,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS idx_reports_pending ON reports (created_at) WHERE status = 'PENDING';

		-- migrations/025: changes to a booking after it was made:
		-- * an extension is its own booking, chained to the one it continues (parent_booking_id);
		-- * the owner can cancel a booking they had accepted (OWNER_CANCELLED, with a reason in
		--   cancellation_note), and a driver who cancels an accepted booking in its last hour gets
		--   part of it back (DRIVER_LATE_CANCELLED).
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS parent_booking_id UUID REFERENCES bookings(id);
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS cancellation_note TEXT;
		CREATE INDEX IF NOT EXISTS idx_bookings_parent_booking_id ON bookings (parent_booking_id) WHERE parent_booking_id IS NOT NULL;

		ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_cancellation_reason_check;
		ALTER TABLE bookings ADD CONSTRAINT bookings_cancellation_reason_check
			CHECK (cancellation_reason IN (
				'OWNER_REJECTED', 'OWNER_EXPIRED', 'DRIVER_CANCELLED',
				'OWNER_ACCOUNT_CLOSED', 'DRIVER_ACCOUNT_CLOSED', 'ADMIN_CANCELLED',
				'OWNER_CANCELLED', 'DRIVER_LATE_CANCELLED'
			));

		-- migrations/026: reports about a booking (the driver never came or stayed past the
		-- end; the spot was taken), filed by its driver or its spot's owner.
		ALTER TABLE reports DROP CONSTRAINT IF EXISTS reports_target_type_check;
		ALTER TABLE reports ADD CONSTRAINT reports_target_type_check
			CHECK (target_type IN ('spot', 'user', 'review', 'booking'));

		-- migrations/027: the driver's exit code and what staying past the end cost (see
		-- usecase.BookingExitUseCase), plus the "your booking ended" notice mark.
		CREATE TABLE IF NOT EXISTS booking_exits (
			booking_id            UUID        PRIMARY KEY REFERENCES bookings(id) ON DELETE CASCADE,
			exit_code             VARCHAR(4)  NOT NULL,
			exit_attempts         INT         NOT NULL DEFAULT 0,
			exited_at             TIMESTAMPTZ,
			closed_by             TEXT        CHECK (closed_by IN ('OWNER_CODE', 'SUPPORT')),
			overstay_minutes      INT         NOT NULL DEFAULT 0,
			charged_minutes       INT         NOT NULL DEFAULT 0,
			overstay_amount       INT         NOT NULL DEFAULT 0,
			overstay_owner_amount INT         NOT NULL DEFAULT 0,
			created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS end_notice_sent_at TIMESTAMPTZ;

		-- migrations/028: what staying past the end costs becomes a debt that rides on the driver's
		-- next reservation (see usecase.BookingDebtUseCase), and ParkLink pays the owner for it.
		CREATE TABLE IF NOT EXISTS booking_debts (
			id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id            UUID        NOT NULL,
			source_booking_id  UUID        NOT NULL UNIQUE REFERENCES bookings(id),
			charged_booking_id UUID        REFERENCES bookings(id),
			minutes            INT         NOT NULL,
			amount             INT         NOT NULL CHECK (amount > 0),
			status             TEXT        NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'RESERVED', 'PAID')),
			created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
			paid_at            TIMESTAMPTZ
		);
		CREATE INDEX IF NOT EXISTS idx_booking_debts_user ON booking_debts (user_id, status);
		CREATE INDEX IF NOT EXISTS idx_booking_debts_charged ON booking_debts (charged_booking_id);
		ALTER TABLE bookings ADD COLUMN IF NOT EXISTS debt_amount INT NOT NULL DEFAULT 0;
		ALTER TABLE payouts ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'BOOKING' CHECK (kind IN ('BOOKING', 'OVERSTAY'));
		ALTER TABLE payouts DROP CONSTRAINT IF EXISTS payouts_booking_id_key;
		CREATE UNIQUE INDEX IF NOT EXISTS idx_payouts_booking_kind ON payouts (booking_id, kind);
	`)
	return err
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
