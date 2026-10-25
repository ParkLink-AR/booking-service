package booking

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/booking_changes_repository_mock.go -package=mocks -source=./booking_changes_usecase.go

const maxOwnerCancellationNoteLength = 500

// BookingChangesRepository reads and changes bookings for BookingChangesUseCase.
// Implemented by internal/repository/booking.
type BookingChangesRepository interface {
	GetByID(ctx context.Context, bookingID string) (entity.Booking, error)
	OwnerCancelBooking(ctx context.Context, bookingID, note string) (entity.Booking, error)
	CancelExtensions(ctx context.Context, parentID, reason string) ([]entity.Booking, error)
	CountOwnerCancellations(ctx context.Context, spotIDs []string) (entity.OwnerCancellationStats, error)
}

// BookingExtender prices and creates extensions. Implemented by internal/service/booking.
type BookingExtender interface {
	ExtensionOptions(ctx context.Context, parent entity.Booking) ([]entity.ExtensionOption, error)
	CreateExtension(ctx context.Context, parent entity.Booking, end time.Time) (entity.Booking, error)
}

// BookingChangesSpots is what BookingChangesUseCase needs from parking-service.
// Implemented by internal/repository/parkingclient.
type BookingChangesSpots interface {
	GetSpotDetails(ctx context.Context, spotID string) (entity.SpotDetails, error)
	GetAccessInstructions(ctx context.Context, spotID string) (string, error)
	GetSpotIDsForOwner(ctx context.Context, ownerID string) ([]string, error)
}

// BookingChangesUseCase covers what can happen to a booking after it was made, besides the
// driver cancelling it (CancelBookingUseCase): the owner calling off one they accepted, the
// driver extending it, and the driver reading how to get into the spot.
type BookingChangesUseCase struct {
	bookings BookingChangesRepository
	extender BookingExtender
	spots    BookingChangesSpots
	refunder PaymentRefunder
	events   EventPublisher
	now      func() time.Time // injectable for testing
}

func NewBookingChangesUseCase(
	bookings BookingChangesRepository,
	extender BookingExtender,
	spots BookingChangesSpots,
	refunder PaymentRefunder,
	events EventPublisher,
) *BookingChangesUseCase {
	return &BookingChangesUseCase{bookings: bookings, extender: extender, spots: spots, refunder: refunder, events: events, now: time.Now}
}

// OwnerCancel calls off a booking the owner (ownerID) had accepted, before it starts, with
// the reason they give. The driver gets everything back, and whatever extends the booking
// goes with it. Each one is counted (see OwnerCancellationStats) so staff can spot owners
// who do it often.
func (uc *BookingChangesUseCase) OwnerCancel(ctx context.Context, bookingID, ownerID, reason string) (entity.Booking, error) {
	note := strings.TrimSpace(reason)
	if note == "" {
		return entity.Booking{}, errors.NewInvalidInput("REASON_REQUIRED", "contale al conductor por qué cancelás")
	}
	if utf8.RuneCountInString(note) > maxOwnerCancellationNoteLength {
		return entity.Booking{}, errors.NewInvalidInput("REASON_TOO_LONG", "el motivo es demasiado largo")
	}
	booking, err := uc.bookings.GetByID(ctx, bookingID)
	if err != nil {
		return entity.Booking{}, err
	}
	spot, err := uc.spots.GetSpotDetails(ctx, booking.ParkingSpotID)
	if err != nil {
		return entity.Booking{}, err
	}
	if spot.OwnerID != ownerID {
		return entity.Booking{}, errors.NewForbidden("esta reserva no es de una cochera tuya")
	}
	if booking.Status != enum.BookingStatusConfirmed {
		return entity.Booking{}, errors.NewInvalidInput("NOT_CANCELLABLE", "solo se puede cancelar una reserva confirmada que todavía no empezó")
	}
	if !uc.now().Before(booking.Start) {
		return entity.Booking{}, errors.NewInvalidInput("ALREADY_STARTED", "la reserva ya empezó: escribinos a soporte si hay un problema")
	}

	cancelled, err := uc.bookings.OwnerCancelBooking(ctx, bookingID, note)
	if err != nil {
		return entity.Booking{}, err
	}
	emitBookingEvent(ctx, uc.events, RoutingKeyBookingCancelled, newBookingCancelledEvent(cancelled, spot, CancelledByOwner))
	refundErr := uc.refunder.RefundPayment(ctx, cancelled.PaymentID)
	if extensions, err := uc.bookings.CancelExtensions(ctx, cancelled.ID, constants.CancellationReasonOwnerCancelled); err == nil {
		for _, extension := range extensions {
			_ = uc.refunder.RefundPayment(ctx, extension.PaymentID)
		}
	}
	// Already CANCELLED either way -- a failed refund is retried from the admin panel.
	if refundErr != nil {
		return entity.Booking{}, refundErr
	}
	return cancelled, nil
}

// extendable returns the booking driverID wants to extend, if it can be: an hourly booking or half-day
// of theirs that's accepted (or under way) and hasn't ended, not already extended.
func (uc *BookingChangesUseCase) extendable(ctx context.Context, bookingID, driverID string) (entity.Booking, error) {
	booking, err := uc.bookings.GetByID(ctx, bookingID)
	if err != nil {
		return entity.Booking{}, err
	}
	if booking.UserID != driverID {
		return entity.Booking{}, errors.NewForbidden("no sos el titular de esta reserva")
	}
	// A stay can't be extended; an hourly booking or a half-day can, by the hour, on spots that
	// take hourly bookings (Service.ExtensionOptions checks that, and the hours they're open).
	if booking.Kind == enum.BookingKindStay {
		return entity.Booking{}, errors.NewInvalidInput("NOT_EXTENDABLE", "solo se pueden extender las reservas por hora o de media estadía")
	}
	if booking.Status != enum.BookingStatusConfirmed && booking.Status != enum.BookingStatusInProgress {
		return entity.Booking{}, errors.NewInvalidInput("NOT_EXTENDABLE", "solo se puede extender una reserva confirmada")
	}
	if !uc.now().Before(booking.End) {
		return entity.Booking{}, errors.NewInvalidInput("NOT_EXTENDABLE", "la reserva ya terminó")
	}
	if booking.Extension != nil {
		if booking.Extension.Status == enum.BookingStatusPendingPayment {
			return entity.Booking{}, errors.NewInvalidInput("EXTENSION_AWAITING_PAYMENT", "ya tenés una extensión esperando el pago")
		}
		return entity.Booking{}, errors.NewInvalidInput("ALREADY_EXTENDED", "esta reserva ya está extendida: extendé desde la extensión")
	}
	return booking, nil
}

// ExtensionOptions lists until when driverID can extend bookingID, and for how much.
func (uc *BookingChangesUseCase) ExtensionOptions(ctx context.Context, bookingID, driverID string) ([]entity.ExtensionOption, error) {
	booking, err := uc.extendable(ctx, bookingID, driverID)
	if err != nil {
		return nil, err
	}
	return uc.extender.ExtensionOptions(ctx, booking)
}

// Extend books the spot from bookingID's end until end, as a new booking awaiting payment;
// paying it confirms it straight away.
func (uc *BookingChangesUseCase) Extend(ctx context.Context, bookingID, driverID string, end time.Time) (entity.Booking, error) {
	booking, err := uc.extendable(ctx, bookingID, driverID)
	if err != nil {
		return entity.Booking{}, err
	}
	return uc.extender.CreateExtension(ctx, booking, end)
}

// AccessInstructions returns how to get into a booking's spot. The owner can always read
// them; the driver only once the owner accepted the booking (and until it's over) -- a gate
// code isn't something to hand to whoever asks for a booking.
func (uc *BookingChangesUseCase) AccessInstructions(ctx context.Context, bookingID, viewerID string) (string, error) {
	booking, err := uc.bookings.GetByID(ctx, bookingID)
	if err != nil {
		return "", err
	}
	if booking.UserID != viewerID {
		spot, err := uc.spots.GetSpotDetails(ctx, booking.ParkingSpotID)
		if err != nil {
			return "", err
		}
		if spot.OwnerID != viewerID {
			return "", errors.NewForbidden("no sos parte de esta reserva")
		}
	} else if booking.Status != enum.BookingStatusConfirmed && booking.Status != enum.BookingStatusInProgress {
		return "", errors.NewForbidden("vas a ver cómo entrar cuando el propietario acepte la reserva")
	}
	return uc.spots.GetAccessInstructions(ctx, booking.ParkingSpotID)
}

// OwnerCancellationStats is how many accepted bookings ownerID called off -- for the admin
// panel.
func (uc *BookingChangesUseCase) OwnerCancellationStats(ctx context.Context, ownerID string) (entity.OwnerCancellationStats, error) {
	spotIDs, err := uc.spots.GetSpotIDsForOwner(ctx, ownerID)
	if err != nil {
		return entity.OwnerCancellationStats{}, err
	}
	return uc.bookings.CountOwnerCancellations(ctx, spotIDs)
}
