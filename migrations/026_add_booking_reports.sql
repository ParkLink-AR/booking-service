-- Reports about a booking (the driver never came or stayed past the end; the spot was
-- taken), filed by its driver or its spot's owner.
ALTER TABLE reports DROP CONSTRAINT IF EXISTS reports_target_type_check;
ALTER TABLE reports ADD CONSTRAINT reports_target_type_check
	CHECK (target_type IN ('spot', 'user', 'review', 'booking'));
