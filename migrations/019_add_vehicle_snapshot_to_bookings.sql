-- An entirely optional snapshot of whichever vehicle (if any) the driver picked from their
-- own saved list (auth-service's /users/{id}/vehicles) at the moment this booking was
-- created -- copied in, not a live reference, so later editing or deleting that saved
-- vehicle never changes an already-made booking's record. All four are NULL together (no
-- vehicle picked) or set together (never partially set) -- enforced at the service layer,
-- not a DB constraint, since Postgres has no convenient "all-or-none" column check short of
-- a CHECK expression repeating all four columns.
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS vehicle_plate TEXT;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS vehicle_brand TEXT;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS vehicle_model TEXT;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS vehicle_country TEXT;
