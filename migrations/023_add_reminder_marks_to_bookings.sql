-- When each reminder about a booking went out (see SendBookingRemindersUseCase): to the
-- owner before a booking request expires unanswered, before a booking starts, and to the
-- driver before a stay ends. Set in the same UPDATE that picks the booking up, so each
-- reminder is sent once even with several booking-service instances running the job.
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS approval_reminder_sent_at TIMESTAMPTZ;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS start_reminder_sent_at TIMESTAMPTZ;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS end_reminder_sent_at TIMESTAMPTZ;
