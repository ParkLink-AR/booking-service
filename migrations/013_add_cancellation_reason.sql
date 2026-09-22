-- Distinguishes *why* a booking ended up CANCELLED -- the driver cancelling their own
-- booking (CancelBooking) looks identical in `status` to the owner rejecting it
-- (RejectBooking) or never responding in time (ExpireStalePendingApprovals). Without this,
-- there is no way to tell a driver-initiated cancellation apart from an owner rejection,
-- which is exactly what made the driver's notification feed call every cancelled booking
-- "rejected by the owner", even the ones the driver cancelled themselves.
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS cancellation_reason TEXT
	CHECK (cancellation_reason IN ('OWNER_REJECTED', 'OWNER_EXPIRED', 'DRIVER_CANCELLED'));
