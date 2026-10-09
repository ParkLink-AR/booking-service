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
			));
		CREATE INDEX IF NOT EXISTS idx_bookings_verification_review_pending
			ON bookings (updated_at) WHERE verification_review = 'PENDING';
	`)
	return err
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
