package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/booking_exit_repository_mock.go -package=mocks -source=./booking_exit_usecase.go

// BookingExitRepository reads and records how a booking's vehicle left. Implemented by
// internal/repository/booking.
type BookingExitRepository interface {
	GetByID(ctx context.Context, bookingID string) (entity.Booking, error)
	FindExit(ctx context.Context, bookingID string) (*entity.BookingExit, error)
	EnsureExit(ctx context.Context, bookingID, code string) (entity.BookingExit, error)
	VerifyExitCode(ctx context.Context, bookingID, code string) error
	CloseExit(ctx context.Context, bookingID string, exitedAt time.Time, closedBy string, overstay entity.Overstay, ownerID string) (entity.Booking, error)
}

// BookingExitSpots is what BookingExitUseCase needs from parking-service. Implemented by
// internal/repository/parkingclient.
type BookingExitSpots interface {
	GetSpotDetails(ctx context.Context, spotID string) (entity.SpotDetails, error)
}

// exitClockSkew is how far ahead of the server's clock a departure time may be.
const exitClockSkew = time.Minute

// BookingExitUseCase is how a verified hourly booking ends: the driver reads a second 4-digit
// code in their app and tells it to the owner as the vehicle leaves; the owner enters it, and
// that is when the vehicle left. Past the booking's end it is also how long they stayed beyond
// what they paid for (see entity.ComputeOverstay). If the code never gets entered the booking
// stays in progress until ParkLink support closes it by hand.
type BookingExitUseCase struct {
	bookings BookingExitRepository
	spots    BookingExitSpots
	events   EventPublisher
	now      func() time.Time // injectable for testing
}

func NewBookingExitUseCase(bookings BookingExitRepository, spots BookingExitSpots, events EventPublisher) *BookingExitUseCase {
	return &BookingExitUseCase{bookings: bookings, spots: spots, events: events, now: time.Now}
}

// hasPaidExtension reports whether the booking goes on in an extension that was paid: the
// departure is recorded on that one, which is where the stay ends.
func hasPaidExtension(booking entity.Booking) bool {
	return booking.Extension != nil && booking.Extension.Status != enum.BookingStatusPendingPayment
}

// View is bookingID's exit for viewerID: the driver (who, the first time they ask while the
// booking awaits its exit, gets the code), the owner of the spot (who only sees where it
// stands), or an admin.
func (uc *BookingExitUseCase) View(ctx context.Context, bookingID string, viewer entity.BookingViewer) (entity.ExitView, error) {
	booking, err := uc.bookings.GetByID(ctx, bookingID)
	if err != nil {
		return entity.ExitView{}, err
	}
	role, err := uc.roleOf(ctx, booking, viewer)
	if err != nil {
		return entity.ExitView{}, err
	}

	exit, err := uc.bookings.FindExit(ctx, bookingID)
	if err != nil {
		return entity.ExitView{}, err
	}
	awaiting := booking.AwaitingExit() && !hasPaidExtension(booking) && (exit == nil || !exit.Closed())
	if awaiting && exit == nil && role == entity.ExitRoleDriver {
		code, err := entity.GenerateVerificationCode()
		if err != nil {
			return entity.ExitView{}, errors.NewInternalError("booking.BookingExitUseCase.View", err)
		}
		created, err := uc.bookings.EnsureExit(ctx, bookingID, code)
		if err != nil {
			return entity.ExitView{}, err
		}
		exit = &created
	}
	return entity.ExitView{Role: role, Awaiting: awaiting, Exit: exit}, nil
}

func (uc *BookingExitUseCase) roleOf(ctx context.Context, booking entity.Booking, viewer entity.BookingViewer) (entity.ExitRole, error) {
	if viewer.Privileged {
		return entity.ExitRoleAdmin, nil
	}
	if booking.UserID == viewer.UserID {
		return entity.ExitRoleDriver, nil
	}
	spot, err := uc.spots.GetSpotDetails(ctx, booking.ParkingSpotID)
	if err != nil {
		return 0, err
	}
	if spot.OwnerID == viewer.UserID {
		return entity.ExitRoleOwner, nil
	}
	return 0, errors.NewForbidden("esta reserva no es tuya ni de una cochera tuya")
}

// Verify records the vehicle's departure now, when callerID -- who must own the booking's
// parking spot -- enters the driver's exit code. It returns the booking, completed, and its
// exit with what staying past the end cost, and announces the booking as completed (the
// completion sweep leaves a verified booking alone until this) so both sides are asked to
// review each other.
func (uc *BookingExitUseCase) Verify(ctx context.Context, bookingID, callerID, code string) (entity.Booking, entity.BookingExit, error) {
	booking, spot, err := uc.exitable(ctx, bookingID)
	if err != nil {
		return entity.Booking{}, entity.BookingExit{}, err
	}
	if spot.OwnerID != callerID {
		return entity.Booking{}, entity.BookingExit{}, errors.NewForbidden("no sos el dueño de esta cochera")
	}
	if err := uc.bookings.VerifyExitCode(ctx, bookingID, code); err != nil {
		return entity.Booking{}, entity.BookingExit{}, err
	}
	return uc.close(ctx, booking, spot, uc.now(), entity.ExitClosedByOwnerCode)
}

// SupportClose records the vehicle's departure at exitedAt on behalf of ParkLink support, for
// a booking whose code never got entered (e.g. the driver left without handing it over).
// exitedAt can't be before the booking started nor in the future.
func (uc *BookingExitUseCase) SupportClose(ctx context.Context, bookingID string, exitedAt time.Time) (entity.Booking, entity.BookingExit, error) {
	booking, spot, err := uc.exitable(ctx, bookingID)
	if err != nil {
		return entity.Booking{}, entity.BookingExit{}, err
	}
	if exitedAt.Before(booking.Start) {
		return entity.Booking{}, entity.BookingExit{}, errors.NewInvalidInput("INVALID_EXIT_TIME", "la salida no puede ser anterior al inicio de la reserva")
	}
	if exitedAt.After(uc.now().Add(exitClockSkew)) {
		return entity.Booking{}, entity.BookingExit{}, errors.NewInvalidInput("INVALID_EXIT_TIME", "la salida no puede ser en el futuro")
	}
	// Nobody may have asked for the code yet; the row is what the departure is recorded on.
	if exit, err := uc.bookings.FindExit(ctx, bookingID); err != nil {
		return entity.Booking{}, entity.BookingExit{}, err
	} else if exit == nil {
		code, err := entity.GenerateVerificationCode()
		if err != nil {
			return entity.Booking{}, entity.BookingExit{}, errors.NewInternalError("booking.BookingExitUseCase.SupportClose", err)
		}
		if _, err := uc.bookings.EnsureExit(ctx, bookingID, code); err != nil {
			return entity.Booking{}, entity.BookingExit{}, err
		}
	}
	return uc.close(ctx, booking, spot, exitedAt, entity.ExitClosedBySupport)
}

// exitable returns bookingID and its spot if its departure can be recorded: a verified hourly
// booking still waiting for it, which isn't carried on by a paid extension.
func (uc *BookingExitUseCase) exitable(ctx context.Context, bookingID string) (entity.Booking, entity.SpotDetails, error) {
	booking, err := uc.bookings.GetByID(ctx, bookingID)
	if err != nil {
		return entity.Booking{}, entity.SpotDetails{}, err
	}
	if !booking.AwaitingExit() {
		return entity.Booking{}, entity.SpotDetails{}, errors.NewInvalidInput("NOT_AWAITING_EXIT",
			"esta reserva no está esperando su salida")
	}
	if hasPaidExtension(booking) {
		return entity.Booking{}, entity.SpotDetails{}, errors.NewInvalidInput("ALREADY_EXTENDED",
			"esta reserva fue extendida: la salida se registra en la extensión")
	}
	spot, err := uc.spots.GetSpotDetails(ctx, booking.ParkingSpotID)
	if err != nil {
		return entity.Booking{}, entity.SpotDetails{}, err
	}
	return booking, spot, nil
}

func (uc *BookingExitUseCase) close(
	ctx context.Context, booking entity.Booking, spot entity.SpotDetails, exitedAt time.Time, closedBy string,
) (entity.Booking, entity.BookingExit, error) {
	overstay := entity.ComputeOverstay(booking, exitedAt)
	completed, err := uc.bookings.CloseExit(ctx, booking.ID, exitedAt, closedBy, overstay, spot.OwnerID)
	if err != nil {
		return entity.Booking{}, entity.BookingExit{}, err
	}
	emitBookingEvent(ctx, uc.events, RoutingKeyBookingCompleted,
		newBookingEvent(RoutingKeyBookingCompleted, completed, spot))
	// Staying past the grace period was charged: the driver is told what they owe, the owner what
	// they were paid for it.
	if overstay.ChargedMinutes > 0 && overstay.Amount > 0 {
		emitBookingEvent(ctx, uc.events, RoutingKeyBookingOverstayCharged,
			newBookingOverstayEvent(completed, spot, overstay))
	}

	exit, err := uc.bookings.FindExit(ctx, booking.ID)
	if err != nil {
		return entity.Booking{}, entity.BookingExit{}, err
	}
	if exit == nil {
		return completed, entity.BookingExit{BookingID: booking.ID, ExitedAt: &exitedAt, ClosedBy: closedBy, Overstay: overstay}, nil
	}
	return completed, *exit, nil
}
