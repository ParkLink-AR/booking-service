package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/verify_booking_code_repository_mock.go -package=mocks -source=./verify_booking_code_usecase.go

// BookingCodeVerifier compares a caller-entered code against a CONFIRMED booking's stored
// verification code and, on match, transitions it to IN_PROGRESS. Implemented by
// internal/repository/booking.
type BookingCodeVerifier interface {
	GetByID(ctx context.Context, bookingID string) (entity.Booking, error)
	VerifyBookingCode(ctx context.Context, bookingID, code string) (entity.Booking, error)
}

// VerificationOpensBeforeStart is how long before a booking's start the owner may already
// enter the driver's arrival code -- a driver showing up a little early is normal, but a
// code entered days ahead would only mean it was passed along remotely, not checked in
// person. Verification stays open until the booking's end.
const VerificationOpensBeforeStart = 30 * time.Minute

// VerifyBookingCodeUseCase lets the owner of a booking's parking spot confirm, in person,
// that the driver who showed up is the one who actually booked it: the driver reads a
// 4-digit code in their own app and tells it to the owner, who enters it here.
type VerifyBookingCodeUseCase struct {
	verifier BookingCodeVerifier
	spots    SpotDetailsLookup
	events   EventPublisher
}

func NewVerifyBookingCodeUseCase(verifier BookingCodeVerifier, spots SpotDetailsLookup, events EventPublisher) *VerifyBookingCodeUseCase {
	return &VerifyBookingCodeUseCase{verifier: verifier, spots: spots, events: events}
}

// Execute verifies code against bookingID on behalf of callerID, who must own the
// booking's parking spot, only within the booking's verification window (from
// VerificationOpensBeforeStart before its start until its end), and publishes booking.verified on success so the driver is
// notified their arrival was confirmed.
func (uc *VerifyBookingCodeUseCase) Execute(ctx context.Context, bookingID, callerID, code string) (entity.Booking, error) {
	booking, err := uc.verifier.GetByID(ctx, bookingID)
	if err != nil {
		return entity.Booking{}, err
	}

	spot, err := uc.spots.GetSpotDetails(ctx, booking.ParkingSpotID)
	if err != nil {
		return entity.Booking{}, err
	}
	if spot.OwnerID != callerID {
		return entity.Booking{}, errors.NewForbidden("no sos el dueño de esta cochera")
	}

	now := time.Now()
	if now.Before(booking.Start.Add(-VerificationOpensBeforeStart)) {
		return entity.Booking{}, errors.NewInvalidInput("VERIFICATION_NOT_OPEN_YET",
			"el código se puede verificar desde 30 minutos antes del inicio de la reserva")
	}
	if !now.Before(booking.End) {
		return entity.Booking{}, errors.NewInvalidInput("VERIFICATION_WINDOW_CLOSED",
			"la reserva ya terminó, no se puede verificar el código")
	}

	verified, err := uc.verifier.VerifyBookingCode(ctx, bookingID, code)
	if err != nil {
		return entity.Booking{}, err
	}

	emitBookingEvent(ctx, uc.events, RoutingKeyBookingVerified,
		newBookingEvent(RoutingKeyBookingVerified, verified, spot))
	return verified, nil
}
