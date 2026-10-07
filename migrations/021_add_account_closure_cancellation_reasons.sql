-- Bookings cancelled because one side's account was closed or suspended (see
-- usecase.AccountClosureUseCase): OWNER_ACCOUNT_CLOSED when it was the spot's owner,
-- DRIVER_ACCOUNT_CLOSED when it was the driver. Either way the driver got a full refund.
ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_cancellation_reason_check;
ALTER TABLE bookings ADD CONSTRAINT bookings_cancellation_reason_check
    CHECK (cancellation_reason IN (
        'OWNER_REJECTED', 'OWNER_EXPIRED', 'DRIVER_CANCELLED',
        'OWNER_ACCOUNT_CLOSED', 'DRIVER_ACCOUNT_CLOSED'
    ));
