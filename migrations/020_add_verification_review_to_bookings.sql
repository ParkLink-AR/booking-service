-- Flags a paid booking that reached COMPLETED without the owner ever entering the driver's
-- in-person arrival code (verified_at IS NULL) for a ParkLink admin to review before the
-- owner gets paid: PENDING (set by the completion sweep, blocks the payout), then APPROVED
-- (the stay is accepted as having happened; the payout goes ahead) or REFUNDED (the driver
-- got their money back; no payout). NULL for every verified booking, and for any booking
-- completed before this column existed -- those are never retroactively held back.
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS verification_review TEXT
    CHECK (verification_review IN ('PENDING', 'APPROVED', 'REFUNDED'));
CREATE INDEX IF NOT EXISTS idx_bookings_verification_review_pending
    ON bookings (updated_at) WHERE verification_review = 'PENDING';
