-- In-person verification: the driver reads a 4-digit code in the app and tells it to the
-- spot's owner, who enters it to prove the driver actually showed up at the right spot
-- (see usecase.VerifyBookingCodeUseCase). verification_code is set once, when the booking
-- reaches CONFIRMED (see ConfirmBookingUseCase/ApproveBookingUseCase); verification_attempts
-- counts wrong guesses (verification is refused past a limit, see
-- repository.maxVerificationAttempts); verified_at is set once, on success, and is never
-- cleared -- it's the only record of whether a since-COMPLETED booking was ever verified.
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS verification_code VARCHAR(4);
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS verification_attempts INT NOT NULL DEFAULT 0;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ;
