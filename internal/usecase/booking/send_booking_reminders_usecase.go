package booking

import (
	"context"
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/send_booking_reminders_repository_mock.go -package=mocks -source=./send_booking_reminders_usecase.go

// How long before each moment its reminder goes out.
const (
	// Before a booking request expires unanswered (the owner's answer deadline).
	ApprovalReminderLead = 30 * time.Minute
	// Before a booking starts: at most this far ahead, at least StartReminderMinLead -- in
	// between, half of how far ahead it was booked (see ReminderClaimer.ClaimStartReminders).
	StartReminderLead    = time.Hour
	StartReminderMinLead = 5 * time.Minute
	// Before a stay ends.
	EndReminderLead = time.Hour
)

// ReminderClaimer picks up (and marks as sent, in the same step) the bookings each
// reminder is due for. Implemented by internal/repository/booking.
type ReminderClaimer interface {
	ClaimApprovalReminders(ctx context.Context, within time.Duration) ([]entity.Booking, error)
	ClaimStartReminders(ctx context.Context, maxLead, minLead time.Duration) ([]entity.Booking, error)
	ClaimEndReminders(ctx context.Context, within time.Duration) ([]entity.Booking, error)
}

// SendBookingRemindersUseCase sends the reminders that keep a booking from slipping: to the
// owner before a request expires unanswered (booking.approval_reminder), to both sides
// before a booking starts (booking.starting_soon), and to the driver before a stay ends
// (booking.ending_soon). Each goes out once per booking. Meant to run on a ticker (see
// cmd.runRemindersJob).
type SendBookingRemindersUseCase struct {
	claimer ReminderClaimer
	spots   SpotDetailsLookup
	events  EventPublisher
}

func NewSendBookingRemindersUseCase(claimer ReminderClaimer, spots SpotDetailsLookup, events EventPublisher) *SendBookingRemindersUseCase {
	return &SendBookingRemindersUseCase{claimer: claimer, spots: spots, events: events}
}

// Execute returns how many reminders it sent. A failure claiming one kind doesn't stop the
// others; the first such error is returned after all were tried.
func (uc *SendBookingRemindersUseCase) Execute(ctx context.Context) (int, error) {
	kinds := []struct {
		routingKey string
		lead       time.Duration
		claim      func(context.Context, time.Duration) ([]entity.Booking, error)
	}{
		{RoutingKeyBookingApprovalReminder, ApprovalReminderLead, uc.claimer.ClaimApprovalReminders},
		{RoutingKeyBookingStartingSoon, StartReminderLead, func(ctx context.Context, lead time.Duration) ([]entity.Booking, error) {
			return uc.claimer.ClaimStartReminders(ctx, lead, StartReminderMinLead)
		}},
		{RoutingKeyBookingEndingSoon, EndReminderLead, uc.claimer.ClaimEndReminders},
	}
	sent := 0
	var firstErr error
	for _, kind := range kinds {
		bookings, err := kind.claim(ctx, kind.lead)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for _, b := range bookings {
			spot, _ := uc.spots.GetSpotDetails(ctx, b.ParkingSpotID)
			emitBookingEvent(ctx, uc.events, kind.routingKey, newBookingEvent(kind.routingKey, b, spot))
			sent++
		}
	}
	return sent, firstErr
}
