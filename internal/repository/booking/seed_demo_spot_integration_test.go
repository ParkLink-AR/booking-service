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
	"github.com/example/bookings-service/internal/repository/booking"
	ucbooking "github.com/example/bookings-service/internal/usecase/booking"
)

// Runs against a real PostgreSQL (BOOKING_TEST_DSN), like the other repository suites.
func TestSeedDemoSpotIntegrationSuite(t *testing.T) {
	dataSourceName := os.Getenv("BOOKING_TEST_DSN")
	if dataSourceName == "" {
		t.Skip("BOOKING_TEST_DSN not set, skipping repository integration test")
	}
	suite.Run(t, &seedDemoSpotIntegrationSuite{dataSourceName: dataSourceName})
}

type seedDemoSpotIntegrationSuite struct {
	suite.Suite
	dataSourceName string
	database       *sql.DB
	repository     *booking.Repository
	context        context.Context
}

const demoSeedSpotID = "ff602e93-534d-5d48-b908-0000000000aa"

func (s *seedDemoSpotIntegrationSuite) SetupTest() {
	database, err := sql.Open("postgres", s.dataSourceName)
	s.Require().NoError(err)
	s.Require().NoError(database.Ping())
	s.database = database
	s.repository = booking.NewRepository(database)
	s.context = context.Background()
	s.cleanup()
}

func (s *seedDemoSpotIntegrationSuite) TearDownTest() {
	s.cleanup()
	s.database.Close()
}

func (s *seedDemoSpotIntegrationSuite) cleanup() {
	_, err := s.database.Exec(`DELETE FROM reviews WHERE target_id = $1`, demoSeedSpotID)
	s.Require().NoError(err)
	_, err = s.database.Exec(`DELETE FROM bookings WHERE parking_spot_id = $1`, demoSeedSpotID)
	s.Require().NoError(err)
}

func (s *seedDemoSpotIntegrationSuite) seed(price float64) entity.DemoSpotSeed {
	return ucbooking.BuildDemoSpotSeed(entity.DemoSpot{ID: demoSeedSpotID, OwnerID: "0e000000-0000-4000-8000-000000000001", PricePerHour: &price}, time.Now())
}

func (s *seedDemoSpotIntegrationSuite) count(query string) int {
	var n int
	s.Require().NoError(s.database.QueryRow(query, demoSeedSpotID).Scan(&n))
	return n
}

func (s *seedDemoSpotIntegrationSuite) TestStoresOneFinishedUnpaidBookingAndAllItsReviews() {
	seed := s.seed(7500)

	inserted, err := s.repository.SeedDemoSpot(s.context, seed)

	s.Require().NoError(err)
	s.Equal(len(seed.Reviews), inserted)
	s.Equal(1, s.count(`SELECT count(*) FROM bookings WHERE parking_spot_id = $1`))
	s.Equal(len(seed.Reviews), s.count(`SELECT count(*) FROM reviews WHERE target_type = 'spot' AND target_id = $1`))

	var status string
	var paidAt, paymentID sql.NullString
	var amount, ownerAmount int
	s.Require().NoError(s.database.QueryRow(
		`SELECT status::text, paid_at::text, payment_id, amount, owner_amount FROM bookings WHERE parking_spot_id = $1`, demoSeedSpotID,
	).Scan(&status, &paidAt, &paymentID, &amount, &ownerAmount))
	s.Equal("COMPLETED", status)
	s.False(paidAt.Valid, "a demo booking must never be paid, or the payout job would pick it up")
	s.False(paymentID.Valid)
	s.Equal(1500000, amount)
	s.Equal(1500000, ownerAmount)
}

func (s *seedDemoSpotIntegrationSuite) TestLoadingTheSameSpotAgainAddsNothing() {
	seed := s.seed(6000)
	first, err := s.repository.SeedDemoSpot(s.context, seed)
	s.Require().NoError(err)

	second, err := s.repository.SeedDemoSpot(s.context, s.seed(6000))

	s.Require().NoError(err)
	s.Greater(first, 0)
	s.Equal(0, second)
	s.Equal(1, s.count(`SELECT count(*) FROM bookings WHERE parking_spot_id = $1`))
	s.Equal(first, s.count(`SELECT count(*) FROM reviews WHERE target_id = $1`))
}

func (s *seedDemoSpotIntegrationSuite) TestTheRatingAverageAndCountAreWhatTheReviewsSay() {
	seed := s.seed(6000)
	_, err := s.repository.SeedDemoSpot(s.context, seed)
	s.Require().NoError(err)

	ratings, err := s.repository.GetSpotRatings(s.context, []string{demoSeedSpotID})

	s.Require().NoError(err)
	total := 0
	for _, review := range seed.Reviews {
		total += review.Rating
	}
	s.Equal(len(seed.Reviews), ratings[demoSeedSpotID].Count)
	s.InDelta(float64(total)/float64(len(seed.Reviews)), ratings[demoSeedSpotID].Average, 0.001)
}

func (s *seedDemoSpotIntegrationSuite) TestReviewsAreListedWithTheirCommentsAndSomeHaveNone() {
	seed := s.seed(6000)
	_, err := s.repository.SeedDemoSpot(s.context, seed)
	s.Require().NoError(err)

	reviews, err := s.repository.ListReviewsByTarget(s.context, 0, demoSeedSpotID)

	s.Require().NoError(err)
	s.Len(reviews, len(seed.Reviews))
	withComment := 0
	for _, review := range reviews {
		if review.Comment != "" {
			withComment++
		}
	}
	s.Greater(withComment, 0)
	s.Less(withComment, len(reviews)+1)
}

func (s *seedDemoSpotIntegrationSuite) TestAPartialRunIsCompletedByTheNextOne() {
	seed := s.seed(6000)
	full := len(seed.Reviews)
	partial := seed
	partial.Reviews = seed.Reviews[:5]
	_, err := s.repository.SeedDemoSpot(s.context, partial)
	s.Require().NoError(err)

	inserted, err := s.repository.SeedDemoSpot(s.context, seed)

	s.Require().NoError(err)
	s.Equal(full-5, inserted)
	s.Equal(full, s.count(`SELECT count(*) FROM reviews WHERE target_id = $1`))
}
