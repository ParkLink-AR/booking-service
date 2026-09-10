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
	`)
	return err
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
