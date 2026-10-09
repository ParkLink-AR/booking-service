-- The type of the vehicle the driver picked (car/suv/pickup/van/motorcycle/bicycle), next
-- to the rest of its snapshot (see 019) -- it decides the price of a spot that charges per
-- vehicle type, and tells the owner what's coming. NULL when no vehicle was picked, and on
-- bookings made before vehicles had a type. A bicycle has a type but no plate.
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS vehicle_type TEXT;
