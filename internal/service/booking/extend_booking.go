package booking

import (
	"context"
	"math"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

// maxExtensionHours is how far past its end a booking can be extended in one go.
const maxExtensionHours = 12

// ExtensionOptions lists how far parent (an hourly booking) can be extended, an hour at a
// time from its end: each option stays within the spot's published hours and open days and
// fits next to the other bookings (the parent itself doesn't count against it). It stops at
// the first hour that doesn't fit -- an extension is one unbroken stretch. Each option is
// priced like a new hourly booking (the owner's rate for the booking's vehicle plus the
// platform's commission).
func (service *Service) ExtensionOptions(ctx context.Context, parent entity.Booking) ([]entity.ExtensionOption, error) {
	options, _, err := service.extensionOptions(ctx, parent)
	return options, err
}

func (service *Service) extensionOptions(ctx context.Context, parent entity.Booking) ([]entity.ExtensionOption, entity.SpotBookingRules, error) {
	rules, err := service.parkingClient.GetSpotBookingRules(ctx, parent.ParkingSpotID)
	if err != nil {
		return nil, rules, err
	}
	if !rules.Active || !rules.HourlyEnabled {
		return nil, rules, errors.NewInvalidInput("SPOT_NOT_ACTIVE", "esta cochera no está tomando reservas por hora ahora")
	}
	vehicleType := ""
	if parent.VehicleType != nil {
		vehicleType = *parent.VehicleType
	}
	pricePerHour, err := service.parkingClient.GetHourlyPrice(ctx, parent.ParkingSpotID, vehicleType)
	if err != nil {
		return nil, rules, err
	}
	settings, err := service.commissionRates.GetCommissionRate(ctx)
	if err != nil {
		return nil, rules, err
	}

	start := parent.End
	active, err := service.bookingRepository.GetActiveBookingsInRangeExcluding(
		ctx, parent.ParkingSpotID, start, start.Add(maxExtensionHours*time.Hour), parent.ID)
	if err != nil {
		return nil, rules, err
	}

	var options []entity.ExtensionOption
	for hours := 1; hours <= maxExtensionHours; hours++ {
		end := start.Add(time.Duration(hours) * time.Hour)
		if !isWithinPublishedHours(start, end, rules.Schedule, service.location) ||
			entity.OverlapsDateRanges(rules.BlockedDates, start, end, service.location) ||
			entity.MaxConcurrency(active, start, end) >= rules.Capacity {
			break
		}
		ownerAmount := int(math.Round(pricePerHour * float64(hours) * 100))
		amount := ownerAmount + ownerAmount*settings.CommissionRateBP/10000
		options = append(options, entity.ExtensionOption{End: end, Amount: amount, OwnerAmount: ownerAmount})
	}
	return options, rules, nil
}

// CreateExtension books parent's spot from parent's end until end -- which has to be one of
// ExtensionOptions -- for the same driver and vehicle, as a new booking awaiting payment
// chained to parent. Once paid it's confirmed straight away (see ConfirmBookingUseCase): the
// owner already accepted this driver for the stay it continues.
func (service *Service) CreateExtension(ctx context.Context, parent entity.Booking, end time.Time) (entity.Booking, error) {
	options, rules, err := service.extensionOptions(ctx, parent)
	if err != nil {
		return entity.Booking{}, err
	}
	var chosen *entity.ExtensionOption
	for index := range options {
		if options[index].End.Equal(end) {
			chosen = &options[index]
			break
		}
	}
	if chosen == nil {
		return entity.Booking{}, errors.NewInvalidInput("EXTENSION_NOT_AVAILABLE", "la cochera no está libre hasta ese horario")
	}

	holdExpiresAt := time.Now().Add(service.holdDuration)
	ownerAmount := chosen.OwnerAmount
	parentID := parent.ID
	return service.bookingRepository.CreateBooking(ctx, entity.Booking{
		ParkingSpotID:   parent.ParkingSpotID,
		UserID:          parent.UserID,
		Start:           parent.End.UTC(),
		End:             end.UTC(),
		Status:          enum.BookingStatusPendingPayment,
		HoldExpiresAt:   &holdExpiresAt,
		Amount:          chosen.Amount,
		OwnerAmount:     &ownerAmount,
		Capacity:        rules.Capacity,
		Kind:            enum.BookingKindHourly,
		VehicleType:     parent.VehicleType,
		VehiclePlate:    parent.VehiclePlate,
		VehicleBrand:    parent.VehicleBrand,
		VehicleModel:    parent.VehicleModel,
		VehicleCountry:  parent.VehicleCountry,
		ParentBookingID: &parentID,
	})
}
