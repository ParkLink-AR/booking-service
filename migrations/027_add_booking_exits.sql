-- Leaving: in person, like arriving. Once the owner verified the driver's arrival, the driver
-- has a second 4-digit code to give the owner as they take their vehicle out; entering it
-- records when they actually left (exited_at) and closes the booking. Past the end of the
-- booking that is how long they stayed beyond what they paid for (see
-- entity.ComputeOverstay): nothing within the grace period, everything past it. Until the code
-- is entered a verified hourly booking stays in progress -- the completion sweep leaves it
-- alone -- and ParkLink support can close it by hand (closed_by = 'SUPPORT').
-- The row is created the first time the driver asks for the code, so there is no code for a
-- booking whose driver never looked at it.
CREATE TABLE IF NOT EXISTS booking_exits (
    booking_id            UUID        PRIMARY KEY REFERENCES bookings(id) ON DELETE CASCADE,
    exit_code             VARCHAR(4)  NOT NULL,
    exit_attempts         INT         NOT NULL DEFAULT 0,
    exited_at             TIMESTAMPTZ,
    closed_by             TEXT        CHECK (closed_by IN ('OWNER_CODE', 'SUPPORT')),
    overstay_minutes      INT         NOT NULL DEFAULT 0,
    charged_minutes       INT         NOT NULL DEFAULT 0,
    overstay_amount       INT         NOT NULL DEFAULT 0,
    overstay_owner_amount INT         NOT NULL DEFAULT 0,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- When the "your booking ended" notice went out (see SendBookingRemindersUseCase), once per booking.
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS end_notice_sent_at TIMESTAMPTZ;
