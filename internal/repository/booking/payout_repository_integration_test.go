package booking_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/suite"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/repository/booking"
)

// TestPayoutRepositoryIntegrationSuite exercises the payout/platform_settings repository
// methods against a real PostgreSQL instance -- MarkPayoutPaid's already-PAID rejection and
// ListEligibleBookingsWithoutPayout's LEFT JOIN filter both depend on real SQL semantics a
// mocked *sql.DB cannot verify. Skipped unless BOOKING_TEST_DSN is set (see
// repository_integration_test.go), with migrations/001-015_*.sql applied.
func TestPayoutRepositoryIntegrationSuite(t *testing.T) {
	dataSourceName := os.Getenv("BOOKING_TEST_DSN")
	if dataSourceName == "" {
		t.Skip("BOOKING_TEST_DSN not set, skipping repository integration test")
	}
	suite.Run(t, &payoutRepositoryIntegrationSuite{dataSourceName: dataSourceName})
}

type payoutRepositoryIntegrationSuite struct {
	suite.Suite
	dataSourceName string
	database       *sql.DB
	repository     *booking.Repository
	context        context.Context
}

func (s *payoutRepositoryIntegrationSuite) SetupTest() {
	database, err := sql.Open("postgres", s.dataSourceName)
	s.Require().NoError(err)
	s.Require().NoError(database.Ping())

	s.database = database
	s.repository = booking.NewRepository(database)
	s.context = context.Background()

	_, err = database.Exec("DELETE FROM payouts")
	s.Require().NoError(err)
	_, err = database.Exec("DELETE FROM bookings")
	s.Require().NoError(err)
	_, err = database.Exec("UPDATE platform_settings SET commission_rate_bp = 500, updated_by_admin_id = NULL WHERE id = true")
	s.Require().NoError(err)
}

func (s *payoutRepositoryIntegrationSuite) TearDownTest() {
	s.database.Close()
}

func (s *payoutRepositoryIntegrationSuite) insertCompletedPaidBooking(parkingSpotID string, endedAt time.Time) string {
	var bookingID string
	err := s.database.QueryRow(`
		INSERT INTO bookings (parking_spot_id, user_id, time_range, status, amount, paid_at)
		VALUES ($1, gen_random_uuid(), tstzrange($2, $3, '[)'), 'COMPLETED', 10000, now())
		RETURNING id
	`, parkingSpotID, endedAt.Add(-time.Hour), endedAt).Scan(&bookingID)
	s.Require().NoError(err)
	return bookingID
}

func (s *payoutRepositoryIntegrationSuite) TestListEligibleBookingsWithoutPayout_ExcludesBookingsThatAlreadyHaveAPayout() {
	spotID := "11111111-1111-1111-1111-111111111111"
	eligibleBookingID := s.insertCompletedPaidBooking(spotID, time.Now().Add(-25*time.Hour))
	alreadyPaidOutBookingID := s.insertCompletedPaidBooking(spotID, time.Now().Add(-25*time.Hour))
	tooRecentBookingID := s.insertCompletedPaidBooking(spotID, time.Now().Add(-1*time.Hour))

	_, err := s.repository.CreatePayout(s.context, entity.Payout{
		BookingID: alreadyPaidOutBookingID, OwnerID: "owner", GrossAmount: 10000, CommissionAmount: 500, NetAmount: 9500,
	})
	s.Require().NoError(err)

	got, err := s.repository.ListEligibleBookingsWithoutPayout(s.context)

	s.Require().NoError(err)
	ids := make([]string, len(got))
	for i, b := range got {
		ids[i] = b.ID
	}
	s.Contains(ids, eligibleBookingID)
	s.NotContains(ids, alreadyPaidOutBookingID)
	s.NotContains(ids, tooRecentBookingID)
}

func (s *payoutRepositoryIntegrationSuite) TestMarkPayoutPaid_AlreadyPaid_ReturnsConflict() {
	spotID := "22222222-2222-2222-2222-222222222222"
	bookingID := s.insertCompletedPaidBooking(spotID, time.Now().Add(-25*time.Hour))
	payout, err := s.repository.CreatePayout(s.context, entity.Payout{
		BookingID: bookingID, OwnerID: "owner", GrossAmount: 10000, CommissionAmount: 500, NetAmount: 9500,
	})
	s.Require().NoError(err)

	adminID := "33333333-3333-3333-3333-333333333333"
	_, err = s.repository.MarkPayoutPaid(s.context, payout.ID, adminID, nil)
	s.Require().NoError(err)

	_, err = s.repository.MarkPayoutPaid(s.context, payout.ID, adminID, nil)

	s.True(errors.IsConflict(err))
}

func (s *payoutRepositoryIntegrationSuite) TestMarkPayoutPaid_NotFound_ReturnsNotFound() {
	_, err := s.repository.MarkPayoutPaid(s.context, "99999999-9999-9999-9999-999999999999", "admin", nil)

	s.True(errors.IsNotFound(err))
}

func (s *payoutRepositoryIntegrationSuite) TestSetAndGetCommissionRate_RoundTrips() {
	adminID := "44444444-4444-4444-4444-444444444444"

	updated, err := s.repository.SetCommissionRate(s.context, 750, adminID)
	s.Require().NoError(err)
	s.Equal(750, updated.CommissionRateBP)

	got, err := s.repository.GetCommissionRate(s.context)
	s.Require().NoError(err)
	s.Equal(750, got.CommissionRateBP)
	s.Require().NotNil(got.UpdatedByAdminID)
	s.Equal(adminID, *got.UpdatedByAdminID)
}

func (s *payoutRepositoryIntegrationSuite) TestListBookingsByOwnerSpots_ReturnsBookingsAcrossAllGivenSpots() {
	spot1 := "55555555-5555-5555-5555-555555555555"
	spot2 := "66666666-6666-6666-6666-666666666666"
	otherSpot := "77777777-7777-7777-7777-777777777777"
	b1 := s.insertCompletedPaidBooking(spot1, time.Now().Add(-25*time.Hour))
	b2 := s.insertCompletedPaidBooking(spot2, time.Now().Add(-25*time.Hour))
	other := s.insertCompletedPaidBooking(otherSpot, time.Now().Add(-25*time.Hour))

	got, err := s.repository.ListBookingsByOwnerSpots(s.context, []string{spot1, spot2})

	s.Require().NoError(err)
	ids := make([]string, len(got))
	for i, b := range got {
		ids[i] = b.ID
	}
	s.Contains(ids, b1)
	s.Contains(ids, b2)
	s.NotContains(ids, other)
}
