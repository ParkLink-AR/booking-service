package booking_test

import (
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/goccy/go-json"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/example/bookings-service/internal/controller/booking"
	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/pkg/mocks"
)

func TestHandlerSuite(t *testing.T) {
	suite.Run(t, new(handlerSuite))
}

type handlerSuite struct {
	suite.Suite
	controller                            *gomock.Controller
	listBookingsUseCase                   *mocks.MockListBookingsUseCase
	listBookingsByUserUseCase             *mocks.MockListBookingsByUserUseCase
	createBookingUseCase                  *mocks.MockCreateBookingUseCase
	confirmBookingUseCase                 *mocks.MockConfirmBookingUseCase
	getBookableSpotsUseCase               *mocks.MockGetBookableSpotsUseCase
	approveBookingUseCase                 *mocks.MockApproveBookingUseCase
	rejectBookingUseCase                  *mocks.MockRejectBookingUseCase
	cancelBookingUseCase                  *mocks.MockCancelBookingUseCase
	listBookingsDetailedUseCase           *mocks.MockListBookingsDetailedUseCase
	getBookingUseCase                     *mocks.MockGetBookingUseCase
	submitReviewUseCase                   *mocks.MockSubmitReviewUseCase
	listReviewsByTargetUseCase            *mocks.MockListReviewsByTargetUseCase
	listReviewsByBookingUseCase           *mocks.MockListReviewsByBookingUseCase
	getSpotRatingsUseCase                 *mocks.MockGetSpotRatingsUseCase
	verifyBookingCodeUseCase              *mocks.MockVerifyBookingCodeUseCase
	sendMessageUseCase                    *mocks.MockSendMessageUseCase
	listMessagesUseCase                   *mocks.MockListMessagesUseCase
	createTicketUseCase                   *mocks.MockCreateTicketUseCase
	listMyTicketsUseCase                  *mocks.MockListMyTicketsUseCase
	getTicketUseCase                      *mocks.MockGetTicketUseCase
	listTicketMessagesUseCase             *mocks.MockListTicketMessagesUseCase
	addTicketMessageUseCase               *mocks.MockAddTicketMessageUseCase
	listAdminTicketsUseCase               *mocks.MockListAdminTicketsUseCase
	getAdminTicketUseCase                 *mocks.MockGetAdminTicketUseCase
	listAdminTicketMessagesUseCase        *mocks.MockListAdminTicketMessagesUseCase
	adminReplyUseCase                     *mocks.MockAdminReplyUseCase
	updateTicketStatusUseCase             *mocks.MockUpdateTicketStatusUseCase
	listAdminPayoutsUseCase               *mocks.MockListAdminPayoutsUseCase
	getAdminPayoutUseCase                 *mocks.MockGetAdminPayoutUseCase
	listOwnerBookingsUseCase              *mocks.MockListOwnerBookingsUseCase
	getCommissionUseCase                  *mocks.MockGetCommissionUseCase
	setCommissionUseCase                  *mocks.MockSetCommissionUseCase
	getOwnerPendingBalancesUseCase        *mocks.MockGetOwnerPendingBalancesUseCase
	draftSettlementUseCase                *mocks.MockDraftSettlementUseCase
	confirmSettlementUseCase              *mocks.MockConfirmSettlementUseCase
	cancelSettlementUseCase               *mocks.MockCancelSettlementUseCase
	getSettlementUseCase                  *mocks.MockGetSettlementUseCase
	listAdminSettlementsUseCase           *mocks.MockListAdminSettlementsUseCase
	listSettlementsByOwnerUseCase         *mocks.MockListSettlementsByOwnerUseCase
	getOwnSettlementUseCase               *mocks.MockGetOwnSettlementUseCase
	listTicketsByUserUseCase              *mocks.MockListTicketsByUserUseCase
	listPendingVerificationReviewsUseCase *mocks.MockListPendingVerificationReviewsUseCase
	resolveVerificationReviewUseCase      *mocks.MockResolveVerificationReviewUseCase
	router                                chi.Router
	location                              *time.Location
}

const validParkingSpotID = "8f14e45f-ceea-4b0e-b6b4-5a1b6c9a2d0f"

const testInternalServiceKey = "test-internal-key"

func (s *handlerSuite) SetupTest() {
	s.controller = gomock.NewController(s.T())
	s.listBookingsUseCase = mocks.NewMockListBookingsUseCase(s.controller)
	s.listBookingsByUserUseCase = mocks.NewMockListBookingsByUserUseCase(s.controller)
	s.createBookingUseCase = mocks.NewMockCreateBookingUseCase(s.controller)
	s.confirmBookingUseCase = mocks.NewMockConfirmBookingUseCase(s.controller)
	s.getBookableSpotsUseCase = mocks.NewMockGetBookableSpotsUseCase(s.controller)
	s.approveBookingUseCase = mocks.NewMockApproveBookingUseCase(s.controller)
	s.rejectBookingUseCase = mocks.NewMockRejectBookingUseCase(s.controller)
	s.cancelBookingUseCase = mocks.NewMockCancelBookingUseCase(s.controller)
	s.listBookingsDetailedUseCase = mocks.NewMockListBookingsDetailedUseCase(s.controller)
	s.getBookingUseCase = mocks.NewMockGetBookingUseCase(s.controller)
	s.submitReviewUseCase = mocks.NewMockSubmitReviewUseCase(s.controller)
	s.listReviewsByTargetUseCase = mocks.NewMockListReviewsByTargetUseCase(s.controller)
	s.listReviewsByBookingUseCase = mocks.NewMockListReviewsByBookingUseCase(s.controller)
	s.getSpotRatingsUseCase = mocks.NewMockGetSpotRatingsUseCase(s.controller)
	s.verifyBookingCodeUseCase = mocks.NewMockVerifyBookingCodeUseCase(s.controller)
	s.sendMessageUseCase = mocks.NewMockSendMessageUseCase(s.controller)
	s.listMessagesUseCase = mocks.NewMockListMessagesUseCase(s.controller)
	s.createTicketUseCase = mocks.NewMockCreateTicketUseCase(s.controller)
	s.listMyTicketsUseCase = mocks.NewMockListMyTicketsUseCase(s.controller)
	s.getTicketUseCase = mocks.NewMockGetTicketUseCase(s.controller)
	s.listTicketMessagesUseCase = mocks.NewMockListTicketMessagesUseCase(s.controller)
	s.addTicketMessageUseCase = mocks.NewMockAddTicketMessageUseCase(s.controller)
	s.listAdminTicketsUseCase = mocks.NewMockListAdminTicketsUseCase(s.controller)
	s.getAdminTicketUseCase = mocks.NewMockGetAdminTicketUseCase(s.controller)
	s.listAdminTicketMessagesUseCase = mocks.NewMockListAdminTicketMessagesUseCase(s.controller)
	s.adminReplyUseCase = mocks.NewMockAdminReplyUseCase(s.controller)
	s.updateTicketStatusUseCase = mocks.NewMockUpdateTicketStatusUseCase(s.controller)
	s.listAdminPayoutsUseCase = mocks.NewMockListAdminPayoutsUseCase(s.controller)
	s.getAdminPayoutUseCase = mocks.NewMockGetAdminPayoutUseCase(s.controller)
	s.listOwnerBookingsUseCase = mocks.NewMockListOwnerBookingsUseCase(s.controller)
	s.getCommissionUseCase = mocks.NewMockGetCommissionUseCase(s.controller)
	s.setCommissionUseCase = mocks.NewMockSetCommissionUseCase(s.controller)
	s.getOwnerPendingBalancesUseCase = mocks.NewMockGetOwnerPendingBalancesUseCase(s.controller)
	s.draftSettlementUseCase = mocks.NewMockDraftSettlementUseCase(s.controller)
	s.confirmSettlementUseCase = mocks.NewMockConfirmSettlementUseCase(s.controller)
	s.cancelSettlementUseCase = mocks.NewMockCancelSettlementUseCase(s.controller)
	s.getSettlementUseCase = mocks.NewMockGetSettlementUseCase(s.controller)
	s.listAdminSettlementsUseCase = mocks.NewMockListAdminSettlementsUseCase(s.controller)
	s.listSettlementsByOwnerUseCase = mocks.NewMockListSettlementsByOwnerUseCase(s.controller)
	s.getOwnSettlementUseCase = mocks.NewMockGetOwnSettlementUseCase(s.controller)
	s.listTicketsByUserUseCase = mocks.NewMockListTicketsByUserUseCase(s.controller)
	s.listPendingVerificationReviewsUseCase = mocks.NewMockListPendingVerificationReviewsUseCase(s.controller)
	s.resolveVerificationReviewUseCase = mocks.NewMockResolveVerificationReviewUseCase(s.controller)

	location, err := time.LoadLocation("America/Argentina/Buenos_Aires")
	s.Require().NoError(err)
	s.location = location

	s.router = chi.NewRouter()
	booking.NewHandler(
		s.listBookingsUseCase, s.listBookingsByUserUseCase, s.createBookingUseCase,
		s.confirmBookingUseCase, s.getBookableSpotsUseCase,
		s.approveBookingUseCase, s.rejectBookingUseCase, s.cancelBookingUseCase,
		s.listBookingsDetailedUseCase,
		s.getBookingUseCase,
		s.submitReviewUseCase, s.listReviewsByTargetUseCase, s.listReviewsByBookingUseCase,
		s.getSpotRatingsUseCase, s.verifyBookingCodeUseCase,
		s.sendMessageUseCase, s.listMessagesUseCase,
		s.createTicketUseCase, s.listMyTicketsUseCase, s.getTicketUseCase,
		s.listTicketMessagesUseCase, s.addTicketMessageUseCase,
		s.listAdminTicketsUseCase, s.getAdminTicketUseCase, s.listAdminTicketMessagesUseCase,
		s.adminReplyUseCase, s.updateTicketStatusUseCase,
		s.listAdminPayoutsUseCase, s.getAdminPayoutUseCase,
		s.listOwnerBookingsUseCase, s.getCommissionUseCase, s.setCommissionUseCase,
		s.getOwnerPendingBalancesUseCase, s.draftSettlementUseCase, s.confirmSettlementUseCase,
		s.cancelSettlementUseCase, s.getSettlementUseCase, s.listAdminSettlementsUseCase,
		s.listSettlementsByOwnerUseCase, s.getOwnSettlementUseCase,
		s.listTicketsByUserUseCase,
		s.location,
		testInternalServiceKey,
		s.listPendingVerificationReviewsUseCase, s.resolveVerificationReviewUseCase,
	).RegisterRoutes(s.router)
}

func (s *handlerSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *handlerSuite) doGet(path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	return recorder
}

func (s *handlerSuite) doPost(path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	return recorder
}

// doPostAsDriver issues a POST as the gateway would forward it for validUserID.
func (s *handlerSuite) doPostAsDriver(path, body string) *httptest.ResponseRecorder {
	return s.doWithHeaders(http.MethodPost, path, body, map[string]string{"X-User-Id": validUserID})
}

func (s *handlerSuite) doWithHeaders(method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	return recorder
}

// doPostMultipartNoProof issues a POST with a valid but empty multipart/form-data body (no
// "proof" field) -- the shape a confirm-settlement request without a proof image takes on the
// wire; FormFile("proof") sees http.ErrMissingFile, which both settlement endpoints that parse
// multipart forms treat as "no proof supplied", not an error.
func (s *handlerSuite) doPostMultipartNoProof(path string, headers map[string]string) *httptest.ResponseRecorder {
	var body strings.Builder
	writer := multipart.NewWriter(&body)
	s.Require().NoError(writer.Close())

	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body.String()))
	request.Header.Set("Content-Type", writer.FormDataContentType())
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)
	return recorder
}

func (s *handlerSuite) TestListBookingsEndpoint_SuccessNoFilters() {
	windowStart := time.Date(2026, 8, 18, 10, 0, 0, 0, s.location).UTC()
	windowEnd := time.Date(2026, 8, 18, 11, 0, 0, 0, s.location).UTC()

	days := []entity.DayBookings{
		{
			Date: time.Date(2026, 8, 18, 0, 0, 0, 0, s.location),
			Bookings: []entity.BookingWindow{
				{Start: windowStart, End: windowEnd, Status: enum.BookingStatusConfirmed},
			},
		},
	}
	s.listBookingsUseCase.EXPECT().
		Execute(gomock.Any(), validParkingSpotID, (*time.Time)(nil), (*time.Time)(nil), (*enum.BookingStatus)(nil), false).
		Return(days, nil)

	recorder := s.doGet("/parking-spots/" + validParkingSpotID + "/bookings")

	s.Equal(http.StatusOK, recorder.Code)

	var body struct {
		Bookings []struct {
			Date     string `json:"date"`
			Bookings []struct {
				Start  string `json:"start"`
				End    string `json:"end"`
				Status string `json:"status"`
			} `json:"bookings"`
		} `json:"bookings"`
	}
	s.Require().NoError(json.NewDecoder(recorder.Body).Decode(&body))
	s.Require().Len(body.Bookings, 1)
	s.Equal("2026-08-18", body.Bookings[0].Date)
	s.Require().Len(body.Bookings[0].Bookings, 1)
	s.Equal("2026-08-18T10:00:00-03:00", body.Bookings[0].Bookings[0].Start)
	s.Equal("2026-08-18T11:00:00-03:00", body.Bookings[0].Bookings[0].End)
	s.Equal("CONFIRMED", body.Bookings[0].Bookings[0].Status)
}

func (s *handlerSuite) TestListBookingsEndpoint_WithRangeAndStatusFilters() {
	from := time.Date(2026, 8, 18, 0, 0, 0, 0, s.location)
	to := time.Date(2026, 8, 20, 0, 0, 0, 0, s.location)
	status := enum.BookingStatusConfirmed

	s.listBookingsUseCase.EXPECT().
		Execute(gomock.Any(), validParkingSpotID, &from, &to, &status, false).
		Return(nil, nil)

	recorder := s.doGet("/parking-spots/" + validParkingSpotID + "/bookings?from=2026-08-18&to=2026-08-20&status=CONFIRMED")

	s.Equal(http.StatusOK, recorder.Code)
}

func (s *handlerSuite) TestListBookingsEndpoint_InvalidSpotID() {
	recorder := s.doGet("/parking-spots/not-a-uuid/bookings")

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_SPOT_ID")
}

func (s *handlerSuite) TestListBookingsEndpoint_InvalidDate() {
	recorder := s.doGet("/parking-spots/" + validParkingSpotID + "/bookings?from=not-a-date")

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_DATE")
}

func (s *handlerSuite) TestListBookingsEndpoint_InvalidStatus() {
	recorder := s.doGet("/parking-spots/" + validParkingSpotID + "/bookings?status=NOT_A_STATUS")

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_STATUS")
}

func (s *handlerSuite) TestListBookingsEndpoint_UsecaseInvalidDateRange() {
	s.listBookingsUseCase.EXPECT().
		Execute(gomock.Any(), validParkingSpotID, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.NewInvalidInput("INVALID_DATE_RANGE", "from must be before to"))

	recorder := s.doGet("/parking-spots/" + validParkingSpotID + "/bookings?from=2026-08-20&to=2026-08-18")

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_DATE_RANGE")
}

func (s *handlerSuite) TestSetCommissionEndpoint_RejectsWithoutSuperadminRole() {
	recorder := s.doWithHeaders(http.MethodPut, "/admin/settings/commission", `{"commission_rate_bp":700}`, map[string]string{
		"X-User-Id":    "admin-1",
		"X-Admin-Role": "ADMIN",
	})

	s.Equal(http.StatusForbidden, recorder.Code)
	s.Contains(recorder.Body.String(), "FORBIDDEN")
}

func (s *handlerSuite) TestSetCommissionEndpoint_RejectsMissingAdminRoleHeader() {
	recorder := s.doWithHeaders(http.MethodPut, "/admin/settings/commission", `{"commission_rate_bp":700}`, map[string]string{
		"X-User-Id": "admin-1",
	})

	s.Equal(http.StatusForbidden, recorder.Code)
}

func (s *handlerSuite) TestSetCommissionEndpoint_SucceedsForSuperadmin() {
	s.setCommissionUseCase.EXPECT().
		Execute(gomock.Any(), 700, "admin-1").
		Return(entity.PlatformSettings{CommissionRateBP: 700}, nil)

	recorder := s.doWithHeaders(http.MethodPut, "/admin/settings/commission", `{"commission_rate_bp":700}`, map[string]string{
		"X-User-Id":    "admin-1",
		"X-Admin-Role": "SUPERADMIN",
	})

	s.Equal(http.StatusOK, recorder.Code)
	s.Contains(recorder.Body.String(), "700")
}

func (s *handlerSuite) TestListAdminPayoutsEndpoint_WithOwnerIDFilter_PassesItThrough() {
	ownerID := "owner-1"
	s.listAdminPayoutsUseCase.EXPECT().Execute(gomock.Any(), (*enum.PayoutStatus)(nil), &ownerID).
		Return([]entity.Payout{{ID: "p1", OwnerID: ownerID}}, nil)

	recorder := s.doGet("/admin/payouts?owner_id=" + ownerID)

	s.Equal(http.StatusOK, recorder.Code)
	s.Contains(recorder.Body.String(), "p1")
}

func (s *handlerSuite) TestListAdminPayoutsEndpoint_NoFilters() {
	s.listAdminPayoutsUseCase.EXPECT().Execute(gomock.Any(), (*enum.PayoutStatus)(nil), (*string)(nil)).
		Return([]entity.Payout{{ID: "p1"}}, nil)

	recorder := s.doGet("/admin/payouts")

	s.Equal(http.StatusOK, recorder.Code)
}

func (s *handlerSuite) TestGetOwnerPendingBalancesEndpoint_Success() {
	s.getOwnerPendingBalancesUseCase.EXPECT().Execute(gomock.Any()).
		Return([]entity.OwnerPendingBalance{{OwnerID: "owner-1", PendingNetAmount: 9500, PendingCount: 1}}, nil)

	recorder := s.doGet("/admin/owners/pending-balances")

	s.Equal(http.StatusOK, recorder.Code)
	s.Contains(recorder.Body.String(), "owner-1")
}

func (s *handlerSuite) TestDraftSettlementEndpoint_Success() {
	s.draftSettlementUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID, "admin-1").
		Return(entity.Settlement{ID: "settlement-1", OwnerID: validParkingSpotID}, []entity.Payout{{ID: "p1"}}, nil)

	recorder := s.doWithHeaders(http.MethodPost, "/admin/owners/"+validParkingSpotID+"/settlements", "", map[string]string{
		"X-User-Id": "admin-1",
	})

	s.Equal(http.StatusCreated, recorder.Code)
	s.Contains(recorder.Body.String(), "settlement-1")
}

func (s *handlerSuite) TestDraftSettlementEndpoint_MissingUserID() {
	recorder := s.doPost("/admin/owners/"+validParkingSpotID+"/settlements", "")

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "MISSING_USER_ID")
}

func (s *handlerSuite) TestDraftSettlementEndpoint_InvalidOwnerID() {
	recorder := s.doWithHeaders(http.MethodPost, "/admin/owners/not-a-uuid/settlements", "", map[string]string{
		"X-User-Id": "admin-1",
	})

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_OWNER_ID")
}

func (s *handlerSuite) TestDraftSettlementEndpoint_NothingToSettle() {
	s.draftSettlementUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID, "admin-1").
		Return(entity.Settlement{}, nil, errors.NewInvalidInput("NOTHING_TO_SETTLE", "el propietario no tiene pagos pendientes"))

	recorder := s.doWithHeaders(http.MethodPost, "/admin/owners/"+validParkingSpotID+"/settlements", "", map[string]string{
		"X-User-Id": "admin-1",
	})

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "NOTHING_TO_SETTLE")
}

func (s *handlerSuite) TestConfirmBookingEndpoint_SucceedsWithInternalKey() {
	s.confirmBookingUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID, "mp-123", 150000).
		Return(entity.Booking{ID: validParkingSpotID, Status: enum.BookingStatusPendingOwnerApproval}, nil)

	recorder := s.doWithHeaders(http.MethodPost, "/bookings/"+validParkingSpotID+"/confirm", `{"payment_id":"mp-123","amount":150000}`,
		map[string]string{"X-Internal-Key": testInternalServiceKey})

	s.Equal(http.StatusOK, recorder.Code)
}

func (s *handlerSuite) TestConfirmBookingEndpoint_HoldExpiredReturnsItsCode() {
	s.confirmBookingUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID, "mp-123", 150000).
		Return(entity.Booking{}, errors.NewInvalidInput("HOLD_EXPIRED", "la reserva venció"))

	recorder := s.doWithHeaders(http.MethodPost, "/bookings/"+validParkingSpotID+"/confirm", `{"payment_id":"mp-123","amount":150000}`,
		map[string]string{"X-Internal-Key": testInternalServiceKey})

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "HOLD_EXPIRED")
}

func (s *handlerSuite) TestConfirmBookingEndpoint_RejectsMissingAmount() {
	recorder := s.doWithHeaders(http.MethodPost, "/bookings/"+validParkingSpotID+"/confirm", `{"payment_id":"mp-123"}`,
		map[string]string{"X-Internal-Key": testInternalServiceKey})

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "MISSING_AMOUNT")
}

func (s *handlerSuite) TestGetInternalBookingEndpoint_RequiresInternalKey() {
	recorder := s.doWithHeaders(http.MethodGet, "/internal/bookings/"+validParkingSpotID, "", nil)

	s.Equal(http.StatusUnauthorized, recorder.Code)
}

func (s *handlerSuite) TestGetInternalBookingEndpoint_ReturnsServerSidePrice() {
	s.getBookingUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID, entity.BookingViewer{Privileged: true}).
		Return(entity.Booking{ID: validParkingSpotID, UserID: validUserID, Status: enum.BookingStatusPendingPayment, Amount: 150000}, nil)

	recorder := s.doWithHeaders(http.MethodGet, "/internal/bookings/"+validParkingSpotID, "",
		map[string]string{"X-Internal-Key": testInternalServiceKey})

	s.Equal(http.StatusOK, recorder.Code)
	s.Contains(recorder.Body.String(), `"amount":150000`)
	s.Contains(recorder.Body.String(), `"status":"PENDING_PAYMENT"`)
}

func (s *handlerSuite) TestListBookingsByUserEndpoint_ForbidsOtherUsers() {
	recorder := s.doWithHeaders(http.MethodGet, "/users/"+validParkingSpotID+"/bookings/history", "",
		map[string]string{"X-User-Id": validUserID})

	s.Equal(http.StatusForbidden, recorder.Code)
}

func (s *handlerSuite) TestListSettlementsByUserEndpoint_ForbidsOtherUsers() {
	recorder := s.doWithHeaders(http.MethodGet, "/users/"+validParkingSpotID+"/settlements", "",
		map[string]string{"X-User-Id": validUserID})

	s.Equal(http.StatusForbidden, recorder.Code)
}

func (s *handlerSuite) TestListTicketsByUserEndpoint_AllowsAdmin() {
	s.listTicketsByUserUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID).Return(nil, nil)

	recorder := s.doWithHeaders(http.MethodGet, "/users/"+validParkingSpotID+"/tickets", "",
		map[string]string{"X-User-Id": "admin-1", "X-Admin-Role": "ADMIN"})

	s.Equal(http.StatusOK, recorder.Code)
}

func (s *handlerSuite) TestConfirmBookingEndpoint_RejectsMissingInternalKey() {
	recorder := s.doPost("/bookings/"+validParkingSpotID+"/confirm", `{"payment_id":"mp-123"}`)

	s.Equal(http.StatusUnauthorized, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_INTERNAL_KEY")
}

func (s *handlerSuite) TestConfirmBookingEndpoint_RejectsWrongInternalKey() {
	recorder := s.doWithHeaders(http.MethodPost, "/bookings/"+validParkingSpotID+"/confirm", `{"payment_id":"mp-123"}`,
		map[string]string{"X-Internal-Key": "not-the-key"})

	s.Equal(http.StatusUnauthorized, recorder.Code)
}

func (s *handlerSuite) TestResolveVerificationReviewEndpoint_Approved() {
	review := enum.VerificationReviewApproved
	s.resolveVerificationReviewUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID, enum.VerificationReviewApproved).
		Return(entity.Booking{ID: validParkingSpotID, Status: enum.BookingStatusCompleted, VerificationReview: &review}, nil)

	recorder := s.doPost("/admin/bookings/"+validParkingSpotID+"/verification-review", `{"decision":"APPROVED"}`)

	s.Equal(http.StatusOK, recorder.Code)
	s.Contains(recorder.Body.String(), `"verification_review":"APPROVED"`)
}

func (s *handlerSuite) TestResolveVerificationReviewEndpoint_RejectsPendingDecision() {
	recorder := s.doPost("/admin/bookings/"+validParkingSpotID+"/verification-review", `{"decision":"PENDING"}`)

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_DECISION")
}

func (s *handlerSuite) TestResolveVerificationReviewEndpoint_AlreadyResolved() {
	s.resolveVerificationReviewUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID, enum.VerificationReviewRefunded).
		Return(entity.Booking{}, errors.NewConflict("esta reserva no tiene una revisión pendiente"))

	recorder := s.doPost("/admin/bookings/"+validParkingSpotID+"/verification-review", `{"decision":"REFUNDED"}`)

	s.Equal(http.StatusConflict, recorder.Code)
	s.Contains(recorder.Body.String(), "REVIEW_NOT_PENDING")
}

func (s *handlerSuite) TestListPendingVerificationReviewsEndpoint_Success() {
	review := enum.VerificationReviewPending
	s.listPendingVerificationReviewsUseCase.EXPECT().Execute(gomock.Any()).
		Return([]entity.Booking{{ID: validParkingSpotID, Status: enum.BookingStatusCompleted, VerificationReview: &review}}, nil)

	recorder := s.doWithHeaders(http.MethodGet, "/admin/bookings/unverified", "", nil)

	s.Equal(http.StatusOK, recorder.Code)
	s.Contains(recorder.Body.String(), `"verification_review":"PENDING"`)
}

func (s *handlerSuite) TestConfirmSettlementEndpoint_NoProof_Success() {
	s.confirmSettlementUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID, "admin-1", []byte(nil), "").
		Return(entity.Settlement{ID: validParkingSpotID}, []entity.Payout{{ID: "p1"}}, nil)

	recorder := s.doPostMultipartNoProof("/admin/settlements/"+validParkingSpotID+"/confirm", map[string]string{
		"X-User-Id": "admin-1",
	})

	s.Equal(http.StatusOK, recorder.Code)
}

func (s *handlerSuite) TestConfirmSettlementEndpoint_AlreadyConfirmed() {
	s.confirmSettlementUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID, "admin-1", []byte(nil), "").
		Return(entity.Settlement{}, nil, errors.NewConflict("esta liquidación ya fue confirmada"))

	recorder := s.doPostMultipartNoProof("/admin/settlements/"+validParkingSpotID+"/confirm", map[string]string{
		"X-User-Id": "admin-1",
	})

	s.Equal(http.StatusConflict, recorder.Code)
	s.Contains(recorder.Body.String(), "ALREADY_CONFIRMED")
}

func (s *handlerSuite) TestConfirmSettlementEndpoint_NotFound() {
	s.confirmSettlementUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID, "admin-1", []byte(nil), "").
		Return(entity.Settlement{}, nil, errors.NewNotFound("settlement", validParkingSpotID))

	recorder := s.doPostMultipartNoProof("/admin/settlements/"+validParkingSpotID+"/confirm", map[string]string{
		"X-User-Id": "admin-1",
	})

	s.Equal(http.StatusNotFound, recorder.Code)
	s.Contains(recorder.Body.String(), "SETTLEMENT_NOT_FOUND")
}

func (s *handlerSuite) TestCancelSettlementEndpoint_Success() {
	s.cancelSettlementUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID).Return(nil)

	recorder := s.doPost("/admin/settlements/"+validParkingSpotID+"/cancel", "")

	s.Equal(http.StatusNoContent, recorder.Code)
}

func (s *handlerSuite) TestCancelSettlementEndpoint_AlreadyConfirmed() {
	s.cancelSettlementUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID).
		Return(errors.NewConflict("esta liquidación ya fue confirmada y no se puede cancelar"))

	recorder := s.doPost("/admin/settlements/"+validParkingSpotID+"/cancel", "")

	s.Equal(http.StatusConflict, recorder.Code)
	s.Contains(recorder.Body.String(), "ALREADY_CONFIRMED")
}

func (s *handlerSuite) TestCancelSettlementEndpoint_NotFound() {
	s.cancelSettlementUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID).
		Return(errors.NewNotFound("settlement", validParkingSpotID))

	recorder := s.doPost("/admin/settlements/"+validParkingSpotID+"/cancel", "")

	s.Equal(http.StatusNotFound, recorder.Code)
	s.Contains(recorder.Body.String(), "SETTLEMENT_NOT_FOUND")
}

func (s *handlerSuite) TestListAdminSettlementsEndpoint_Success() {
	s.listAdminSettlementsUseCase.EXPECT().Execute(gomock.Any(), (*enum.SettlementStatus)(nil)).
		Return([]entity.Settlement{{ID: "s1"}}, nil)

	recorder := s.doGet("/admin/settlements")

	s.Equal(http.StatusOK, recorder.Code)
	s.Contains(recorder.Body.String(), "s1")
}

func (s *handlerSuite) TestListAdminSettlementsEndpoint_InvalidStatus() {
	recorder := s.doGet("/admin/settlements?status=bogus")

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "INVALID_STATUS")
}

func (s *handlerSuite) TestGetAdminSettlementEndpoint_Success() {
	s.getSettlementUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID).
		Return(entity.Settlement{ID: validParkingSpotID}, []entity.Payout{{ID: "p1"}}, nil)

	recorder := s.doGet("/admin/settlements/" + validParkingSpotID)

	s.Equal(http.StatusOK, recorder.Code)
	s.Contains(recorder.Body.String(), "p1")
}

func (s *handlerSuite) TestGetAdminSettlementEndpoint_NotFound() {
	s.getSettlementUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID).
		Return(entity.Settlement{}, nil, errors.NewNotFound("settlement", validParkingSpotID))

	recorder := s.doGet("/admin/settlements/" + validParkingSpotID)

	s.Equal(http.StatusNotFound, recorder.Code)
	s.Contains(recorder.Body.String(), "SETTLEMENT_NOT_FOUND")
}

func (s *handlerSuite) TestListSettlementsByUserEndpoint_Success() {
	s.listSettlementsByOwnerUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID).
		Return([]entity.Settlement{{ID: "s1"}}, nil)

	recorder := s.doWithHeaders(http.MethodGet, "/users/"+validParkingSpotID+"/settlements", "",
		map[string]string{"X-User-Id": validParkingSpotID})

	s.Equal(http.StatusOK, recorder.Code)
	s.Contains(recorder.Body.String(), "s1")
}

func (s *handlerSuite) TestGetSettlementEndpoint_MissingUserID() {
	recorder := s.doGet("/settlements/" + validParkingSpotID)

	s.Equal(http.StatusBadRequest, recorder.Code)
	s.Contains(recorder.Body.String(), "MISSING_USER_ID")
}

func (s *handlerSuite) TestGetSettlementEndpoint_NotTheOwner_Forbidden() {
	s.getOwnSettlementUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID, "someone-else").
		Return(entity.Settlement{}, nil, errors.NewForbidden("no sos el dueño de esta liquidación"))

	recorder := s.doWithHeaders(http.MethodGet, "/settlements/"+validParkingSpotID, "", map[string]string{
		"X-User-Id": "someone-else",
	})

	s.Equal(http.StatusForbidden, recorder.Code)
}

func (s *handlerSuite) TestGetSettlementEndpoint_Success() {
	s.getOwnSettlementUseCase.EXPECT().Execute(gomock.Any(), validParkingSpotID, "owner-1").
		Return(entity.Settlement{ID: validParkingSpotID, OwnerID: "owner-1"}, []entity.Payout{{ID: "p1"}}, nil)

	recorder := s.doWithHeaders(http.MethodGet, "/settlements/"+validParkingSpotID, "", map[string]string{
		"X-User-Id": "owner-1",
	})

	s.Equal(http.StatusOK, recorder.Code)
	s.Contains(recorder.Body.String(), "p1")
}
