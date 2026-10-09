package entity

// VehicleTypeBicycle is the one vehicle type with no plate.
const VehicleTypeBicycle = "bicycle"

// SpotVehicleRules is what a parking spot asks of the vehicle a booking is for (see
// parking-service's ParkingSpot.requireVehicle/vehicleTypes and its per-vehicle-type
// prices): whether the driver has to pick one of their saved vehicles, and which types fit.
type SpotVehicleRules struct {
	// RequireVehicle is the owner's "Pedir el vehículo al reservar".
	RequireVehicle bool
	// HourlyPricedByVehicle/StayPricedByVehicle: the hourly/stay price depends on the
	// vehicle's type -- so a booking of that kind needs one, to know what to charge.
	HourlyPricedByVehicle bool
	StayPricedByVehicle   bool
	// AcceptedTypes are the vehicle types the spot fits.
	AcceptedTypes []string
}

// NeedsVehicle reports whether a booking of the given kind must name the driver's vehicle.
func (rules SpotVehicleRules) NeedsVehicle(stay bool) bool {
	if stay {
		return rules.RequireVehicle || rules.StayPricedByVehicle
	}
	return rules.RequireVehicle || rules.HourlyPricedByVehicle
}

// Accepts reports whether the spot fits a vehicle of the given type.
func (rules SpotVehicleRules) Accepts(vehicleType string) bool {
	for _, accepted := range rules.AcceptedTypes {
		if accepted == vehicleType {
			return true
		}
	}
	return false
}
