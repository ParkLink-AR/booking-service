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

// runMigrations creates the booking-service schema (bookings + booking_status_history).
// Mirrors migrations/001-004_*.sql for environments that run migrations inline at startup.
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
				  'EXPIRED'
				);
			END IF;
		END$$;

		CREATE TABLE IF NOT EXISTS bookings (
			id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
			parking_spot_id  UUID          NOT NULL,
			user_id          UUID          NOT NULL,
			time_range       TSTZRANGE     NOT NULL,
			status           booking_status NOT NULL,
			hold_expires_at  TIMESTAMPTZ,
			amount           INT           NOT NULL,
			payment_id       TEXT,
			created_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
			updated_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),

			EXCLUDE USING GIST (
				parking_spot_id WITH =,
				time_range WITH &&
			) WHERE (status IN ('PENDING_PAYMENT', 'CONFIRMED'))
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
	`)
	return err
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
