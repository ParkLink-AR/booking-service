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
