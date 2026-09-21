package paymentclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/example/bookings-service/internal/domain/constants"
	"github.com/example/bookings-service/internal/repository/paymentclient"
)

func TestPaymentClientSuite(t *testing.T) {
	suite.Run(t, new(paymentClientSuite))
}

type paymentClientSuite struct {
	suite.Suite
	context      context.Context
	requestCount int
}

func (s *paymentClientSuite) SetupTest() {
	s.context = context.Background()
	s.requestCount = 0
}

func (s *paymentClientSuite) newServer(statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requestCount++
		w.WriteHeader(statusCode)
	}))
}

func (s *paymentClientSuite) TestRefundPayment_CashSentinel_SkipsCallEntirely() {
	server := s.newServer(http.StatusOK)
	defer server.Close()
	client := paymentclient.NewClient(server.URL, "internal-key")

	err := client.RefundPayment(s.context, constants.CashPaymentID)

	s.NoError(err)
	s.Equal(0, s.requestCount, "a cash booking has nothing to refund -- payment-service should never be called")
}

func (s *paymentClientSuite) TestRefundPayment_RealPaymentID_CallsPaymentService() {
	server := s.newServer(http.StatusOK)
	defer server.Close()
	client := paymentclient.NewClient(server.URL, "internal-key")

	err := client.RefundPayment(s.context, "mp-payment-123")

	s.NoError(err)
	s.Equal(1, s.requestCount)
}

func (s *paymentClientSuite) TestRefundPayment_PaymentServiceRejects_ReturnsUpstreamError() {
	server := s.newServer(http.StatusBadGateway)
	defer server.Close()
	client := paymentclient.NewClient(server.URL, "internal-key")

	err := client.RefundPayment(s.context, "mp-payment-123")

	s.Error(err)
}
