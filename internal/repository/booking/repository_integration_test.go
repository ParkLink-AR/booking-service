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
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/repository/booking"
)

// TestBookingRepositoryIntegrationSuite exercises GetActiveBookingsInRange against a real
// PostgreSQL instance, since its correctness depends on TSTZRANGE overlap and the
// EXCLUDE USING GIST constraint — behavior a mocked *sql.DB cannot verify. It is skipped
// unless BOOKING_TEST_DSN is set (e.g. to a disposable postgres:16-alpine container with
// the migrations in migrations/001-004_*.sql applied).
func TestBookingRepositoryIntegrationSuite(t *testing.T) {
	dataSourceName := os.Getenv("BOOKING_TEST_DSN")
	if dataSourceName == "" {
		t.Skip("BOOKING_TEST_DSN not set, skipping repository integration test")
	}
	suite.Run(t, &bookingRepositoryIntegrationSuite{dataSourceName: dataSourceName})
}

type bookingRepositoryIntegrationSuite struct {
	suite.Suite
	dataSourceName string
	database       *sql.DB
	repository     *booking.Repository
	context        context.Context
}

func (s *bookingRepositoryIntegrationSuite) SetupTest() {
	database, err := sql.Open("postgres", s.dataSourceName)
	s.Require().NoError(err)
	s.Require().NoError(database.Ping())

	s.database = database
	s.repository = booking.NewRepository(database)
	s.context = context.Background()

	_, err = database.Exec("DELETE FROM bookings")
	s.Require().NoError(err)
}

func (s *bookingRepositoryIntegrationSuite) TearDownTest() {
	s.database.Close()
}

func (s *bookingRepositoryIntegrationSuite) insertBooking(parkingSpotID string, start, end time.Time, status string, holdExpiresAt *time.Time) {
	_, err := s.database.Exec(`
		INSERT INTO bookings (parking_spot_id, user_id, time_range, status, hold_expires_at, amount)
		VALUES ($1, gen_random_uuid(), tstzrange($2, $3, '[)'), $4, $5, 1000)
	`, parkingSpotID, start, end, status, holdExpiresAt)
	s.Require().NoError(err)
}

func (s *bookingRepositoryIntegrationSuite) TestConfirmedBookingIsActive() {
	parkingSpotID := "11111111-1111-1111-1111-111111111111"
	start := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 18, 11, 0, 0, 0, time.UTC)
	s.insertBooking(parkingSpotID, start, end, "CONFIRMED", nil)

	got, err := s.repository.GetActiveBookingsInRange(s.context, parkingSpotID,
		time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
	)

	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.True(got[0].Start.Equal(start))
	s.True(got[0].End.Equal(end))
}

// This is the acceptance criterion from spec.md section 10: an expired PENDING_PAYMENT
// hold must not occupy availability.
func (s *bookingRepositoryIntegrationSuite) TestExpiredPendingPaymentDoesNotOccupyAvailability() {
	parkingSpotID := "22222222-2222-2222-2222-222222222222"
	start := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 18, 11, 0, 0, 0, time.UTC)
	expiredHold := time.Now().Add(-time.Hour)
	s.insertBooking(parkingSpotID, start, end, "PENDING_PAYMENT", &expiredHold)

	got, err := s.repository.GetActiveBookingsInRange(s.context, parkingSpotID,
		time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
	)

	s.Require().NoError(err)
	s.Empty(got, "an expired hold must not be returned as an active booking")
}

func (s *bookingRepositoryIntegrationSuite) TestUnexpiredPendingPaymentOccupiesAvailability() {
	parkingSpotID := "33333333-3333-3333-3333-333333333333"
	start := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 18, 11, 0, 0, 0, time.UTC)
	futureHold := time.Now().Add(time.Hour)
	s.insertBooking(parkingSpotID, start, end, "PENDING_PAYMENT", &futureHold)

	got, err := s.repository.GetActiveBookingsInRange(s.context, parkingSpotID,
		time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
	)

	s.Require().NoError(err)
	s.Require().Len(got, 1)
}

var inactiveStatusCases = []string{"COMPLETED", "CANCELLED", "EXPIRED"}

func (s *bookingRepositoryIntegrationSuite) TestInactiveStatusesDoNotOccupyAvailability() {
	for index, status := range inactiveStatusCases {
		parkingSpotID := "44444444-4444-4444-4444-44444444444" + string(rune('0'+index))
		start := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
		end := time.Date(2026, 8, 18, 11, 0, 0, 0, time.UTC)
		s.insertBooking(parkingSpotID, start, end, status, nil)

		got, err := s.repository.GetActiveBookingsInRange(s.context, parkingSpotID,
			time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
		)

		s.Require().NoError(err)
		s.Empty(got, "status %s must not occupy availability", status)
	}
}

// The EXCLUDE USING GIST constraint must reject overlapping active bookings for the same spot.
func (s *bookingRepositoryIntegrationSuite) TestOverlappingActiveBookingsAreRejectedByConstraint() {
	parkingSpotID := "55555555-5555-5555-5555-555555555555"
	s.insertBooking(parkingSpotID,
		time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC),
		"CONFIRMED", nil,
	)

	_, err := s.database.Exec(`
		INSERT INTO bookings (parking_spot_id, user_id, time_range, status, amount)
		VALUES ($1, gen_random_uuid(), tstzrange($2, $3, '[)'), 'CONFIRMED', 1000)
	`, parkingSpotID, time.Date(2026, 8, 18, 11, 0, 0, 0, time.UTC), time.Date(2026, 8, 18, 13, 0, 0, 0, time.UTC))

	s.Require().Error(err, "overlapping active bookings for the same spot must be rejected")
}

func (s *bookingRepositoryIntegrationSuite) TestListBookings_NoFiltersReturnsAllOrderedByStart() {
	parkingSpotID := "66666666-6666-6666-6666-666666666666"
	later := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	earlier := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	s.insertBooking(parkingSpotID, later, later.Add(time.Hour), "CONFIRMED", nil)
	s.insertBooking(parkingSpotID, earlier, earlier.Add(time.Hour), "CANCELLED", nil)

	got, err := s.repository.ListBookings(s.context, parkingSpotID, nil, nil, nil)

	s.Require().NoError(err)
	s.Require().Len(got, 2)
	s.True(got[0].Start.Equal(earlier), "expected earlier booking first")
	s.True(got[1].Start.Equal(later), "expected later booking second")
}

func (s *bookingRepositoryIntegrationSuite) TestListBookings_StatusFilter() {
	parkingSpotID := "77777777-7777-7777-7777-777777777777"
	start := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	s.insertBooking(parkingSpotID, start, start.Add(time.Hour), "CONFIRMED", nil)
	s.insertBooking(parkingSpotID, start.Add(2*time.Hour), start.Add(3*time.Hour), "CANCELLED", nil)

	confirmed := enum.BookingStatusConfirmed
	got, err := s.repository.ListBookings(s.context, parkingSpotID, nil, nil, &confirmed)

	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.Equal(enum.BookingStatusConfirmed, got[0].Status)
}

func (s *bookingRepositoryIntegrationSuite) TestListBookings_DateRangeFilter() {
	parkingSpotID := "88888888-8888-8888-8888-888888888888"
	inRange := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	outOfRange := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	s.insertBooking(parkingSpotID, inRange, inRange.Add(time.Hour), "CONFIRMED", nil)
	s.insertBooking(parkingSpotID, outOfRange, outOfRange.Add(time.Hour), "CONFIRMED", nil)

	from := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	got, err := s.repository.ListBookings(s.context, parkingSpotID, &from, &to, nil)

	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.True(got[0].Start.Equal(inRange))
}

func (s *bookingRepositoryIntegrationSuite) TestListBookings_OnlyFromGivenIsUnboundedAbove() {
	parkingSpotID := "99999999-9999-9999-9999-999999999999"
	before := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	after := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	s.insertBooking(parkingSpotID, before, before.Add(time.Hour), "CONFIRMED", nil)
	s.insertBooking(parkingSpotID, after, after.Add(time.Hour), "CONFIRMED", nil)

	from := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	got, err := s.repository.ListBookings(s.context, parkingSpotID, &from, nil, nil)

	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.True(got[0].Start.Equal(after))
}

func (s *bookingRepositoryIntegrationSuite) TestCreateBooking_Success() {
	holdExpiresAt := time.Now().Add(15 * time.Minute)
	newBooking := entity.Booking{
		ParkingSpotID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		UserID:        "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		Start:         time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC),
		End:           time.Date(2026, 8, 18, 11, 0, 0, 0, time.UTC),
		Status:        enum.BookingStatusPendingPayment,
		HoldExpiresAt: &holdExpiresAt,
		Amount:        5000,
	}

	got, err := s.repository.CreateBooking(s.context, newBooking)

	s.Require().NoError(err)
	s.NotEmpty(got.ID)
	s.False(got.CreatedAt.IsZero())
	s.False(got.UpdatedAt.IsZero())
	s.Equal(newBooking.ParkingSpotID, got.ParkingSpotID)
	s.Equal(newBooking.UserID, got.UserID)
	s.True(got.Start.Equal(newBooking.Start))
	s.True(got.End.Equal(newBooking.End))
	s.Equal(enum.BookingStatusPendingPayment, got.Status)
	s.Equal(5000, got.Amount)
}

// This is the safety net for the race window between the service's pre-check and the
// insert: the EXCLUDE constraint rejects the overlap, and the repository maps that to a
// domain conflict error rather than a raw Postgres error.
func (s *bookingRepositoryIntegrationSuite) TestCreateBooking_OverlapIsMappedToConflictError() {
	parkingSpotID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	s.insertBooking(parkingSpotID,
		time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC),
		"CONFIRMED", nil,
	)

	newBooking := entity.Booking{
		ParkingSpotID: parkingSpotID,
		UserID:        "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		Start:         time.Date(2026, 8, 18, 11, 0, 0, 0, time.UTC),
		End:           time.Date(2026, 8, 18, 13, 0, 0, 0, time.UTC),
		Status:        enum.BookingStatusConfirmed,
		Amount:        5000,
	}

	_, err := s.repository.CreateBooking(s.context, newBooking)

	s.Require().Error(err)
	s.True(errors.IsConflict(err))
}

func (s *bookingRepositoryIntegrationSuite) TestGetBookingsForSpotsInRange_ReturnsBookingsAcrossMultipleSpots() {
	spotA := "dddddddd-dddd-dddd-dddd-dddddddddddd"
	spotB := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	spotC := "ffffffff-ffff-ffff-ffff-ffffffffffff" // no bookings at all

	s.insertBooking(spotA,
		time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC),
		"CONFIRMED", nil,
	)
	s.insertBooking(spotB,
		time.Date(2026, 8, 24, 10, 30, 0, 0, time.UTC),
		time.Date(2026, 8, 24, 11, 30, 0, 0, time.UTC),
		"CANCELLED", nil,
	)

	got, err := s.repository.GetBookingsForSpotsInRange(s.context, []string{spotA, spotB, spotC},
		time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC),
	)

	s.Require().NoError(err)
	s.Require().Len(got, 2, "spotC has no bookings and must not appear")

	bySpot := map[string]entity.SpotBooking{}
	for _, booking := range got {
		bySpot[booking.ParkingSpotID] = booking
	}
	s.Equal(enum.BookingStatusConfirmed, bySpot[spotA].Status)
	s.Equal(enum.BookingStatusCancelled, bySpot[spotB].Status)
}

func (s *bookingRepositoryIntegrationSuite) TestGetBookingsForSpotsInRange_DoesNotFilterByStatus() {
	spotID := "99999999-1111-1111-1111-111111111111"
	s.insertBooking(spotID,
		time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC),
		"EXPIRED", nil,
	)

	got, err := s.repository.GetBookingsForSpotsInRange(s.context, []string{spotID},
		time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC),
	)

	s.Require().NoError(err)
	s.Require().Len(got, 1, "the repository must not filter by status — that's the service's job")
	s.Equal(enum.BookingStatusExpired, got[0].Status)
}

func (s *bookingRepositoryIntegrationSuite) TestGetBookingsForSpotsInRange_OutOfRangeIsExcluded() {
	spotID := "99999999-2222-2222-2222-222222222222"
	s.insertBooking(spotID,
		time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 1, 11, 0, 0, 0, time.UTC),
		"CONFIRMED", nil,
	)

	got, err := s.repository.GetBookingsForSpotsInRange(s.context, []string{spotID},
		time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC),
	)

	s.Require().NoError(err)
	s.Empty(got)
}
