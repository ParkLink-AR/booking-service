package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/booking_creation_service_mock.go -package=mocks -source=./create_booking_usecase.go

// BookingCreator creates a new booking, including all its business validation (range,
// published hours, overlap with existing bookings). Implemented by internal/service/booking.
type BookingCreator interface {
	CreateBooking(ctx context.Context, input entity.CreateBookingInput) (entity.Booking, error)
}

// AccountStatusChecker reports the status of a user's account in auth-service.
// Implemented by internal/repository/authclient.
type AccountStatusChecker interface {
	GetAccountStatus(ctx context.Context, userID string) (string, error)
}

// activeAccountStatus is the only account status (as auth-service names it) that can book:
// not a suspended account, nor one that's being deleted.
const activeAccountStatus = "ACTIVE"

// CreateBookingUseCase orchestrates the booking-creation flow: it checks the driver's
// account can book, then hands the rest (validation, pricing, overlap) to the booking service.
type CreateBookingUseCase struct {
	bookingCreator BookingCreator
	accounts       AccountStatusChecker
}

func NewCreateBookingUseCase(bookingCreator BookingCreator, accounts AccountStatusChecker) *CreateBookingUseCase {
	return &CreateBookingUseCase{bookingCreator: bookingCreator, accounts: accounts}
}

// Execute creates a new booking from input.
func (useCase *CreateBookingUseCase) Execute(ctx context.Context, input entity.CreateBookingInput) (entity.Booking, error) {
	status, err := useCase.accounts.GetAccountStatus(ctx, input.UserID)
	if err != nil {
		return entity.Booking{}, err
	}
	if status != activeAccountStatus {
		return entity.Booking{}, errors.NewForbidden("tu cuenta no puede hacer reservas nuevas")
	}
	return useCase.bookingCreator.CreateBooking(ctx, input)
}
