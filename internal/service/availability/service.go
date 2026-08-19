package availability

import (
	"context"
	"fmt"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/active_bookings_repository_mock.go -package=mocks -source=./service.go

const dateOnlyLayout = "2006-01-02"

// ActiveBookingsRepository loads active bookings that occupy availability for a parking spot.
type ActiveBookingsRepository interface {
	GetActiveBookingsInRange(ctx context.Context, parkingSpotID string, from, to time.Time) ([]entity.TimeWindow, error)
}

// Service computes effective per-day parking spot availability. It owns all the business
// logic for this domain: validating the requested date range, loading active bookings, and
// subtracting them from the published availability base.
type Service struct {
	activeBookingsRepository ActiveBookingsRepository
	maxRangeDays             int
}

func NewService(activeBookingsRepository ActiveBookingsRepository, maxRangeDays int) *Service {
	return &Service{activeBookingsRepository: activeBookingsRepository, maxRangeDays: maxRangeDays}
}

// GetAvailability validates the half-open range [from, to), loads active bookings for
// parkingSpotID, and returns effective per-day availability by subtracting those bookings
// from the published availabilityBase.
//
// from and to must be local midnight instants (i.e. carry the target timezone's Location)
// for the returned entity.DayAvailability.Date values to land on the correct calendar day.
func (service *Service) GetAvailability(
	ctx context.Context,
	parkingSpotID string,
	from, to time.Time,
	availabilityBase *entity.AvailabilityBase,
) ([]entity.DayAvailability, error) {
	if err := service.validateRange(from, to); err != nil {
		return nil, err
	}

	activeBookings, err := service.activeBookingsRepository.GetActiveBookingsInRange(ctx, parkingSpotID, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}

	baseByDate := indexBaseByDate(availabilityBase)

	rangeDays := int(to.Sub(from).Hours() / 24)
	days := make([]entity.DayAvailability, 0, rangeDays)
	for day := from; day.Before(to); day = day.AddDate(0, 0, 1) {
		days = append(days, service.buildDayAvailability(day, baseByDate, activeBookings))
	}

	return days, nil
}

func (service *Service) validateRange(from, to time.Time) error {
	if !from.Before(to) {
		return errors.NewInvalidInput("INVALID_DATE_RANGE", "from must be before to")
	}
	rangeDays := int(to.Sub(from).Hours() / 24)
	if rangeDays > service.maxRangeDays {
		return errors.NewInvalidInput("RANGE_TOO_LARGE",
			fmt.Sprintf("date range exceeds maximum of %d days", service.maxRangeDays))
	}
	return nil
}

func (service *Service) buildDayAvailability(
	day time.Time,
	baseByDate map[string]entity.DayBase,
	activeBookings []entity.TimeWindow,
) entity.DayAvailability {
	dayBase, found := baseByDate[day.Format(dateOnlyLayout)]
	baseIsEmpty := !found || len(dayBase.Windows) == 0

	var freeWindows []entity.TimeWindow
	if !baseIsEmpty {
		baseWindowsUTC := convertWindowsToUTC(dayBase.Windows)
		dayStart := day.UTC()
		dayEnd := day.AddDate(0, 0, 1).UTC()
		bookingsForDay := filterOverlapping(activeBookings, dayStart, dayEnd)
		freeWindows = subtractIntervals(baseWindowsUTC, bookingsForDay)
	}

	return entity.DayAvailability{
		Date:        day,
		Status:      determineStatus(baseIsEmpty, freeWindows),
		FreeWindows: freeWindows,
	}
}

func indexBaseByDate(availabilityBase *entity.AvailabilityBase) map[string]entity.DayBase {
	baseByDate := make(map[string]entity.DayBase, len(availabilityBase.Days))
	for _, dayBase := range availabilityBase.Days {
		baseByDate[dayBase.Date.Format(dateOnlyLayout)] = dayBase
	}
	return baseByDate
}
