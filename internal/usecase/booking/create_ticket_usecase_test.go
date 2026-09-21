package booking_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/usecase/booking"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestCreateTicketUseCaseSuite(t *testing.T) {
	suite.Run(t, new(createTicketUseCaseSuite))
}

type createTicketUseCaseSuite struct {
	suite.Suite
	controller *gomock.Controller
	getter     *mocks.MockBookingGetter
	owners     *mocks.MockSpotOwnerLookup
	tickets    *mocks.MockTicketCreator
	messages   *mocks.MockTicketMessageCreator
	useCase    *booking.CreateTicketUseCase
	context    context.Context

	bookingID string
	driverID  string
	ownerID   string
}

func (s *createTicketUseCaseSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.getter = mocks.NewMockBookingGetter(s.controller)
	s.owners = mocks.NewMockSpotOwnerLookup(s.controller)
	s.tickets = mocks.NewMockTicketCreator(s.controller)
	s.messages = mocks.NewMockTicketMessageCreator(s.controller)
	s.useCase = booking.NewCreateTicketUseCase(s.getter, s.owners, s.tickets, s.messages)
	s.context = context.Background()

	s.bookingID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"
	s.driverID = "22222222-2222-2222-2222-222222222222"
	s.ownerID = "33333333-3333-3333-3333-333333333333"
}

func (s *createTicketUseCaseSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *createTicketUseCaseSuite) booking() entity.Booking {
	return entity.Booking{ID: s.bookingID, UserID: s.driverID, ParkingSpotID: "spot-1"}
}

func (s *createTicketUseCaseSuite) TestExecute_Driver_CreatesTicketAndOpeningMessage() {
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.booking(), nil)
	s.tickets.EXPECT().CreateTicket(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, t entity.SupportTicket) (entity.SupportTicket, error) {
			s.Equal(s.driverID, t.CreatedBy)
			s.Equal("No me devolvieron el pago", t.Subject)
			t.ID = "ticket-1"
			return t, nil
		},
	)
	s.messages.EXPECT().CreateTicketMessage(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, m entity.SupportTicketMessage) (entity.SupportTicketMessage, error) {
			s.Equal("ticket-1", m.TicketID)
			s.Equal(s.driverID, m.SenderID)
			s.False(m.IsAdmin)
			s.Equal("Cancelé hace 3 días y no vi el reembolso", m.Body)
			return m, nil
		},
	)

	got, err := s.useCase.Execute(s.context, s.bookingID, s.driverID, "No me devolvieron el pago", "Cancelé hace 3 días y no vi el reembolso")

	s.Require().NoError(err)
	s.Equal("ticket-1", got.ID)
}

func (s *createTicketUseCaseSuite) TestExecute_Owner_CreatesTicket() {
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.booking(), nil)
	s.owners.EXPECT().GetSpotOwnerID(gomock.Any(), "spot-1").Return(s.ownerID, nil)
	s.tickets.EXPECT().CreateTicket(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, t entity.SupportTicket) (entity.SupportTicket, error) {
			s.Equal(s.ownerID, t.CreatedBy)
			t.ID = "ticket-1"
			return t, nil
		},
	)
	s.messages.EXPECT().CreateTicketMessage(gomock.Any(), gomock.Any()).Return(entity.SupportTicketMessage{}, nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, s.ownerID, "El conductor rompió la barrera", "Necesito coordinar la reparación")

	s.Require().NoError(err)
}

func (s *createTicketUseCaseSuite) TestExecute_Stranger_Forbidden() {
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).Return(s.booking(), nil)
	s.owners.EXPECT().GetSpotOwnerID(gomock.Any(), "spot-1").Return(s.ownerID, nil)

	_, err := s.useCase.Execute(s.context, s.bookingID, "someone-else", "asunto", "cuerpo")

	s.True(errors.IsForbidden(err))
}

func (s *createTicketUseCaseSuite) TestExecute_EmptySubject_InvalidInput() {
	_, err := s.useCase.Execute(s.context, s.bookingID, s.driverID, "   ", "cuerpo")

	s.True(errors.IsInvalidInput(err))
	s.Equal("EMPTY_SUBJECT", errors.InvalidInputCode(err))
}

func (s *createTicketUseCaseSuite) TestExecute_SubjectTooLong_InvalidInput() {
	_, err := s.useCase.Execute(s.context, s.bookingID, s.driverID, strings.Repeat("a", 201), "cuerpo")

	s.True(errors.IsInvalidInput(err))
	s.Equal("SUBJECT_TOO_LONG", errors.InvalidInputCode(err))
}

func (s *createTicketUseCaseSuite) TestExecute_EmptyBody_InvalidInput() {
	_, err := s.useCase.Execute(s.context, s.bookingID, s.driverID, "asunto", "   ")

	s.True(errors.IsInvalidInput(err))
	s.Equal("EMPTY_MESSAGE", errors.InvalidInputCode(err))
}

func (s *createTicketUseCaseSuite) TestExecute_BodyTooLong_InvalidInput() {
	_, err := s.useCase.Execute(s.context, s.bookingID, s.driverID, "asunto", strings.Repeat("a", 2001))

	s.True(errors.IsInvalidInput(err))
	s.Equal("MESSAGE_TOO_LONG", errors.InvalidInputCode(err))
}

func (s *createTicketUseCaseSuite) TestExecute_BookingNotFound_ReturnsNotFoundError() {
	s.getter.EXPECT().GetByID(gomock.Any(), s.bookingID).
		Return(entity.Booking{}, errors.NewNotFound("booking", s.bookingID))

	_, err := s.useCase.Execute(s.context, s.bookingID, s.driverID, "asunto", "cuerpo")

	s.True(errors.IsNotFound(err))
}
