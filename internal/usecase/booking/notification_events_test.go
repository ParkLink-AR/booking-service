package booking_test

import (
	"context"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

// The notifications the use cases announce: reminders, the admin's decision on an
// unverified booking, a review received and a support ticket closed.

func TestNotificationEventsSuite(t *testing.T) {
	suite.Run(t, new(notificationEventsSuite))
}

type notificationEventsSuite struct {
	suite.Suite
	controller *gomock.Controller
	spots      *mocks.MockSpotDetailsLookup
	events     *mocks.MockEventPublisher
	context    context.Context
	published  []published
}

type published struct {
	routingKey string
	body       map[string]any
}

const (
	notifDriverID = "11111111-1111-1111-1111-111111111111"
	notifOwnerID  = "22222222-2222-2222-2222-222222222222"
	notifSpotID   = "33333333-3333-3333-3333-333333333333"
)

func (s *notificationEventsSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.spots = mocks.NewMockSpotDetailsLookup(s.controller)
	s.events = mocks.NewMockEventPublisher(s.controller)
	s.context = context.Background()
	s.published = nil
	s.spots.EXPECT().GetSpotDetails(gomock.Any(), notifSpotID).
		Return(entity.SpotDetails{OwnerID: notifOwnerID, Title: "Cochera Palermo"}, nil).AnyTimes()
	s.events.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, routingKey string, payload any) error {
			raw, err := json.Marshal(payload)
			s.Require().NoError(err)
			var body map[string]any
			s.Require().NoError(json.Unmarshal(raw, &body))
			s.published = append(s.published, published{routingKey: routingKey, body: body})
			return nil
		}).AnyTimes()
}

func (s *notificationEventsSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *notificationEventsSuite) booking(id string) entity.Booking {
	return entity.Booking{ID: id, ParkingSpotID: notifSpotID, UserID: notifDriverID, Start: time.Now().Add(time.Hour), End: time.Now().Add(2 * time.Hour)}
}

func (s *notificationEventsSuite) TestReminders_AnnounceEachDueReminderOnce() {
	claimer := mocks.NewMockReminderClaimer(s.controller)
	claimer.EXPECT().ClaimApprovalReminders(gomock.Any(), booking.ApprovalReminderLead).Return([]entity.Booking{s.booking("a")}, nil)
	claimer.EXPECT().ClaimStartReminders(gomock.Any(), booking.StartReminderLead, booking.StartReminderMinLead).
		Return([]entity.Booking{s.booking("b"), s.booking("c")}, nil)
	claimer.EXPECT().ClaimEndReminders(gomock.Any(), booking.EndReminderLead).Return(nil, nil)
	useCase := booking.NewSendBookingRemindersUseCase(claimer, s.spots, s.events)

	sent, err := useCase.Execute(s.context)

	s.Require().NoError(err)
	s.Equal(3, sent)
	s.Equal([]string{"booking.approval_reminder", "booking.starting_soon", "booking.starting_soon"},
		[]string{s.published[0].routingKey, s.published[1].routingKey, s.published[2].routingKey})
	s.Equal("Cochera Palermo", s.published[1].body["parking_spot_title"])
	s.Equal(notifOwnerID, s.published[1].body["owner_user_id"])
}

func (s *notificationEventsSuite) TestReminders_OneKindFailingDoesNotStopTheOthers() {
	claimer := mocks.NewMockReminderClaimer(s.controller)
	claimer.EXPECT().ClaimApprovalReminders(gomock.Any(), gomock.Any()).Return(nil, context.DeadlineExceeded)
	claimer.EXPECT().ClaimStartReminders(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
	claimer.EXPECT().ClaimEndReminders(gomock.Any(), gomock.Any()).Return([]entity.Booking{s.booking("d")}, nil)
	useCase := booking.NewSendBookingRemindersUseCase(claimer, s.spots, s.events)

	sent, err := useCase.Execute(s.context)

	s.Error(err)
	s.Equal(1, sent)
	s.Equal("booking.ending_soon", s.published[0].routingKey)
}

func (s *notificationEventsSuite) TestVerificationReview_AnnouncesTheDecision() {
	resolver := mocks.NewMockVerificationReviewResolver(s.controller)
	refunder := mocks.NewMockPaymentRefunder(s.controller)
	review := enum.VerificationReviewRefunded
	resolved := s.booking("e")
	resolved.PaymentID = "mp-1"
	resolved.VerificationReview = &review
	resolver.EXPECT().ResolveVerificationReview(gomock.Any(), "e", enum.VerificationReviewRefunded).Return(resolved, nil)
	refunder.EXPECT().RefundPayment(gomock.Any(), "mp-1").Return(nil)
	useCase := booking.NewResolveVerificationReviewUseCase(resolver, refunder, s.spots, s.events)

	_, err := useCase.Execute(s.context, "e", enum.VerificationReviewRefunded)

	s.Require().NoError(err)
	s.Require().Len(s.published, 1)
	s.Equal("booking.verification_reviewed", s.published[0].routingKey)
	s.Equal("refunded", s.published[0].body["decision"])
	s.Equal(notifOwnerID, s.published[0].body["owner_user_id"])
}

func (s *notificationEventsSuite) TestReview_TellsTheReviewedPersonButNotForASpotReview() {
	getter := mocks.NewMockBookingGetter(s.controller)
	owners := mocks.NewMockSpotOwnerLookup(s.controller)
	reviews := mocks.NewMockReviewCreator(s.controller)
	completed := s.booking("f")
	completed.Status = enum.BookingStatusCompleted
	getter.EXPECT().GetByID(gomock.Any(), "f").Return(completed, nil).Times(2)
	owners.EXPECT().GetSpotOwnerID(gomock.Any(), notifSpotID).Return(notifOwnerID, nil)
	reviews.EXPECT().CreateReview(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, review entity.Review) (bool, entity.Review, error) { return true, review, nil }).Times(2)
	useCase := booking.NewSubmitReviewUseCase(getter, owners, reviews, s.spots, s.events)

	_, err := useCase.Execute(s.context, "f", notifDriverID, enum.ReviewTargetSpot, 5, "")
	s.Require().NoError(err)
	_, err = useCase.Execute(s.context, "f", notifDriverID, enum.ReviewTargetOwner, 4, "")
	s.Require().NoError(err)

	s.Require().Len(s.published, 1)
	s.Equal("review.received", s.published[0].routingKey)
	s.Equal(notifOwnerID, s.published[0].body["recipient_user_id"])
	s.Equal("owner", s.published[0].body["recipient_role"])
	s.InDelta(4, s.published[0].body["rating"], 0)
}

func (s *notificationEventsSuite) TestTicketStatus_TellsTheCreatorWhenClosedOnly() {
	updater := mocks.NewMockTicketStatusUpdater(s.controller)
	bookings := mocks.NewMockTicketBookingGetter(s.controller)
	ticket := entity.SupportTicket{ID: "t1", BookingID: "g", CreatedBy: notifOwnerID, Subject: "No me pagaron"}
	updater.EXPECT().UpdateTicketStatus(gomock.Any(), "t1", enum.TicketStatusInProgress).Return(ticket, nil)
	closed := ticket
	closed.Status = enum.TicketStatusResolved
	updater.EXPECT().UpdateTicketStatus(gomock.Any(), "t1", enum.TicketStatusResolved).Return(closed, nil)
	bookings.EXPECT().GetByID(gomock.Any(), "g").Return(s.booking("g"), nil)
	useCase := booking.NewUpdateTicketStatusUseCase(updater, bookings, s.events)

	_, err := useCase.Execute(s.context, "t1", "IN_PROGRESS")
	s.Require().NoError(err)
	_, err = useCase.Execute(s.context, "t1", "RESOLVED")
	s.Require().NoError(err)

	s.Require().Len(s.published, 1)
	s.Equal("support.ticket_closed", s.published[0].routingKey)
	s.Equal(notifOwnerID, s.published[0].body["recipient_user_id"])
	s.Equal("owner", s.published[0].body["recipient_role"])
	s.Equal("RESOLVED", s.published[0].body["status"])
}

func (s *notificationEventsSuite) TestDraftSettlement_TellsTheOwnerItsBeingPrepared() {
	drafter := mocks.NewMockSettlementDrafter(s.controller)
	settlement := entity.Settlement{ID: "set-1", OwnerID: notifOwnerID, NetAmount: 1500000}
	drafter.EXPECT().CreateSettlementDraft(gomock.Any(), notifOwnerID, "admin-1").
		Return(settlement, []entity.Payout{{ID: "p1"}, {ID: "p2"}}, nil)
	useCase := booking.NewDraftSettlementUseCase(drafter, s.events)

	_, _, err := useCase.Execute(s.context, notifOwnerID, "admin-1")

	s.Require().NoError(err)
	s.Require().Len(s.published, 1)
	s.Equal("settlement.drafted", s.published[0].routingKey)
	s.Equal(notifOwnerID, s.published[0].body["recipient_user_id"])
	s.InDelta(1500000, s.published[0].body["net_amount"], 0)
	s.InDelta(2, s.published[0].body["booking_count"], 0)
}
