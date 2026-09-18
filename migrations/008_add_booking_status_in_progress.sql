-- Adds IN_PROGRESS to the booking_status enum: reached from CONFIRMED once the owner
-- verifies the driver's in-person verification code (see 009_add_verification_code.sql).
-- Postgres does not allow a new enum value to be used in the same transaction that adds
-- it, so this migration only adds the value -- nothing in this file references it yet.
ALTER TYPE booking_status ADD VALUE IF NOT EXISTS 'IN_PROGRESS';
