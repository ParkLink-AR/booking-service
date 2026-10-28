package booking_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/suite"

	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/repository/booking"
)

// TestListBookingsByUserIntegrationSuite checks both orders of a driver's booking history against a
// real PostgreSQL instance (BOOKING_TEST_DSN, with every migration applied), like
// TestBookingRepositoryIntegrationSuite.
func TestListBookingsByUserIntegrationSuite(t *testing.T) {
	dataSourceName := os.Getenv("BOOKING_TEST_DSN")
	if dataSourceName == "" {
		t.Skip("BOOKING_TEST_DSN not set, skipping repository integration test")
	}
	suite.Run(t, &listBookingsByUserIntegrationSuite{dataSourceName: dataSourceName})
}

type listBookingsByUserIntegrationSuite struct {
	suite.Suite
	dataSourceName string
	database       *sql.DB
	repository     *booking.Repository
	context        context.Context
}

const historyUserID = "66666666-6666-6666-6666-666666666666"

func (s *listBookingsByUserIntegrationSuite) SetupTest() {
	database, err := sql.Open("postgres", s.dataSourceName)
	s.Require().NoError(err)
	s.Require().NoError(database.Ping())
	s.database = database
	s.repository = booking.NewRepository(database)
	s.context = context.Background()

	_, err = database.Exec("DELETE FROM payouts")
	s.Require().NoError(err)
	_, err = database.Exec("DELETE FROM booking_debts")
	s.Require().NoError(err)
	_, err = database.Exec("DELETE FROM bookings")
	s.Require().NoError(err)
}

func (s *listBookingsByUserIntegrationSuite) TearDownTest() {
	s.database.Close()
}

// insert adds one of historyUserID's bookings, each on its own spot, running from
// startsIn (negative: in the past) for length, created createdAgo ago; returns its id.
func (s *listBookingsByUserIntegrationSuite) insert(
	status string, startsIn time.Duration, length time.Duration, createdAgo time.Duration,
) string {
	start := time.Now().Add(startsIn)
	var id string
	err := s.database.QueryRow(`
		INSERT INTO bookings (parking_spot_id, user_id, time_range, status, amount, owner_amount, kind,
		                      payment_id, paid_at, created_at)
		VALUES (gen_random_uuid(), $1, tstzrange($2, $3, '[)'), $4::booking_status, 240000, 200000, 'HOURLY',
		        'pay-1', $2, $5)
		RETURNING id
	`, historyUserID, start, start.Add(length), status, time.Now().Add(-createdAgo)).Scan(&id)
	s.Require().NoError(err)
	return id
}

func (s *listBookingsByUserIntegrationSuite) ids(order enum.UserBookingsOrder, status *enum.BookingStatus) []string {
	bookings, err := s.repository.ListBookingsByUser(s.context, historyUserID, order, status)
	s.Require().NoError(err)
	ids := make([]string, len(bookings))
	for i, b := range bookings {
		ids[i] = b.ID
	}
	return ids
}

func (s *listBookingsByUserIntegrationSuite) TestOrders() {
	tomorrow := s.insert("CONFIRMED", 24*time.Hour, 2*time.Hour, 5*time.Hour)
	inAnHour := s.insert("CONFIRMED", time.Hour, 2*time.Hour, 4*time.Hour)
	underWay := s.insert("IN_PROGRESS", -time.Hour, 2*time.Hour, 3*time.Hour)
	// Ahead in time, but cancelled: it won't happen, so it goes with the past ones.
	cancelledAhead := s.insert("CANCELLED", 3*time.Hour, 2*time.Hour, 2*time.Hour)
	yesterday := s.insert("COMPLETED", -24*time.Hour, 2*time.Hour, 6*time.Hour)
	lastWeek := s.insert("COMPLETED", -7*24*time.Hour, 2*time.Hour, time.Hour)

	s.Equal([]string{lastWeek, cancelledAhead, underWay, inAnHour, tomorrow, yesterday},
		s.ids(enum.UserBookingsOrderRecent, nil))
	s.Equal([]string{underWay, inAnHour, tomorrow, cancelledAhead, yesterday, lastWeek},
		s.ids(enum.UserBookingsOrderUpcoming, nil))

	confirmed := enum.BookingStatusConfirmed
	s.Equal([]string{inAnHour, tomorrow}, s.ids(enum.UserBookingsOrderUpcoming, &confirmed))
	completed := enum.BookingStatusCompleted
	s.Equal([]string{lastWeek, yesterday}, s.ids(enum.UserBookingsOrderRecent, &completed))
}
