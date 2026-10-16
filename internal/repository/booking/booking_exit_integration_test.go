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

// TestBookingExitIntegrationSuite exercises the exit code, the completion sweep, the payout pool and
// the end reminders against a real PostgreSQL instance (BOOKING_TEST_DSN, with every migration
// applied), like TestBookingRepositoryIntegrationSuite.
func TestBookingExitIntegrationSuite(t *testing.T) {
	dataSourceName := os.Getenv("BOOKING_TEST_DSN")
	if dataSourceName == "" {
		t.Skip("BOOKING_TEST_DSN not set, skipping repository integration test")
	}
	suite.Run(t, &bookingExitIntegrationSuite{dataSourceName: dataSourceName})
}

type bookingExitIntegrationSuite struct {
	suite.Suite
	dataSourceName string
	database       *sql.DB
	repository     *booking.Repository
	context        context.Context
}

const exitSpotID = "55555555-5555-5555-5555-555555555555"

func (s *bookingExitIntegrationSuite) SetupTest() {
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

func (s *bookingExitIntegrationSuite) TearDownTest() {
	s.database.Close()
}

type exitBookingOptions struct {
	kind     string
	status   string
	verified bool
	parentID *string
	// userID is the driver; a random one when empty.
	userID string
}

// insert adds a paid, two-hour-by-default booking ending endedAgo ago (negative: in the future)
// and returns its id.
func (s *bookingExitIntegrationSuite) insert(endedAgo time.Duration, length time.Duration, options exitBookingOptions) string {
	if options.kind == "" {
		options.kind = "HOURLY"
	}
	end := time.Now().Add(-endedAgo)
	start := end.Add(-length)
	var verifiedAt *time.Time
	if options.verified {
		verifiedAt = &start
	}
	var id string
	err := s.database.QueryRow(`
		INSERT INTO bookings (parking_spot_id, user_id, time_range, status, amount, owner_amount, kind,
		                      payment_id, paid_at, verified_at, parent_booking_id)
		VALUES ($1, COALESCE(NULLIF($8, '')::uuid, gen_random_uuid()), tstzrange($2, $3, '[)'), $4::booking_status, 240000, 200000, $5,
		        'pay-1', $2, $6, $7)
		RETURNING id
	`, exitSpotID, start, end, options.status, options.kind, verifiedAt, options.parentID, options.userID).Scan(&id)
	s.Require().NoError(err)
	return id
}

func (s *bookingExitIntegrationSuite) statusOf(bookingID string) string {
	var status string
	s.Require().NoError(s.database.QueryRow(`SELECT status::text FROM bookings WHERE id = $1`, bookingID).Scan(&status))
	return status
}

func ids(bookings []entity.Booking) []string {
	result := make([]string, len(bookings))
	for index, b := range bookings {
		result[index] = b.ID
	}
	return result
}

func (s *bookingExitIntegrationSuite) TestSweep_LeavesAVerifiedHourlyBookingInProgress() {
	waiting := s.insert(time.Hour, 2*time.Hour, exitBookingOptions{status: "IN_PROGRESS", verified: true})
	unverified := s.insert(time.Hour, 2*time.Hour, exitBookingOptions{status: "CONFIRMED"})
	verifiedStay := s.insert(time.Hour, 48*time.Hour, exitBookingOptions{kind: "STAY", status: "IN_PROGRESS", verified: true})

	completed, err := s.repository.ExpireCompletedBookings(s.context)

	s.Require().NoError(err)
	s.ElementsMatch([]string{unverified, verifiedStay}, ids(completed))
	s.Equal("IN_PROGRESS", s.statusOf(waiting))
	s.Equal("COMPLETED", s.statusOf(unverified))
}

func (s *bookingExitIntegrationSuite) TestSweep_CompletesAVerifiedBookingWhoseStayContinuesInAPaidExtension() {
	parent := s.insert(time.Minute, 2*time.Hour, exitBookingOptions{status: "IN_PROGRESS", verified: true})
	s.insert(-time.Hour, time.Hour+time.Minute, exitBookingOptions{status: "CONFIRMED", verified: true, parentID: &parent})

	completed, err := s.repository.ExpireCompletedBookings(s.context)

	s.Require().NoError(err)
	s.Equal([]string{parent}, ids(completed))
}

func (s *bookingExitIntegrationSuite) TestEnsureExit_KeepsTheFirstCode() {
	bookingID := s.insert(time.Hour, 2*time.Hour, exitBookingOptions{status: "IN_PROGRESS", verified: true})

	first, err := s.repository.EnsureExit(s.context, bookingID, "1111")
	s.Require().NoError(err)
	second, err := s.repository.EnsureExit(s.context, bookingID, "2222")
	s.Require().NoError(err)

	s.Equal("1111", first.Code)
	s.Equal("1111", second.Code)
}

func (s *bookingExitIntegrationSuite) TestFindExit_NilWhenThereIsNone() {
	bookingID := s.insert(time.Hour, 2*time.Hour, exitBookingOptions{status: "IN_PROGRESS", verified: true})

	exit, err := s.repository.FindExit(s.context, bookingID)

	s.Require().NoError(err)
	s.Nil(exit)
}

func (s *bookingExitIntegrationSuite) TestVerifyExitCode_WrongCodesCountAndEventuallyLock() {
	bookingID := s.insert(time.Hour, 2*time.Hour, exitBookingOptions{status: "IN_PROGRESS", verified: true})
	_, err := s.repository.EnsureExit(s.context, bookingID, "4821")
	s.Require().NoError(err)

	for attempt := 0; attempt < 5; attempt++ {
		err = s.repository.VerifyExitCode(s.context, bookingID, "0000")
		s.Equal("INVALID_CODE", errors.InvalidInputCode(err))
	}
	err = s.repository.VerifyExitCode(s.context, bookingID, "4821")

	s.True(errors.IsConflict(err), "past the attempt limit even the right code is refused")
}

func (s *bookingExitIntegrationSuite) TestVerifyExitCode_RightCodeMatches() {
	bookingID := s.insert(time.Hour, 2*time.Hour, exitBookingOptions{status: "IN_PROGRESS", verified: true})
	_, err := s.repository.EnsureExit(s.context, bookingID, "4821")
	s.Require().NoError(err)

	s.NoError(s.repository.VerifyExitCode(s.context, bookingID, "4821"))
}

func (s *bookingExitIntegrationSuite) TestVerifyExitCode_NobodyHasACodeThatWasNeverShown() {
	bookingID := s.insert(time.Hour, 2*time.Hour, exitBookingOptions{status: "IN_PROGRESS", verified: true})

	err := s.repository.VerifyExitCode(s.context, bookingID, "4821")

	s.Equal("INVALID_CODE", errors.InvalidInputCode(err))
}

func (s *bookingExitIntegrationSuite) TestCloseExit_CompletesTheBookingAndRecordsWhatTheStayCost() {
	bookingID := s.insert(30*time.Minute, 2*time.Hour, exitBookingOptions{status: "IN_PROGRESS", verified: true})
	_, err := s.repository.EnsureExit(s.context, bookingID, "4821")
	s.Require().NoError(err)
	exitedAt := time.Now()
	overstay := entity.Overstay{Minutes: 30, ChargedMinutes: 30, Amount: 60000, OwnerAmount: 50000}

	completed, err := s.repository.CloseExit(s.context, bookingID, exitedAt, entity.ExitClosedByOwnerCode, overstay, debtOwnerID)

	s.Require().NoError(err)
	s.Equal(enum.BookingStatusCompleted, completed.Status)
	exit, err := s.repository.FindExit(s.context, bookingID)
	s.Require().NoError(err)
	s.Require().NotNil(exit)
	s.True(exit.Closed())
	s.Equal(entity.ExitClosedByOwnerCode, exit.ClosedBy)
	s.Equal(overstay, exit.Overstay)

	_, err = s.repository.CloseExit(s.context, bookingID, exitedAt, entity.ExitClosedBySupport, entity.Overstay{}, debtOwnerID)
	s.True(errors.IsConflict(err), "the first to close decides when the vehicle left")
}

func (s *bookingExitIntegrationSuite) TestPayouts_AVerifiedBookingStillWaitingForItsExitStillPaysTheOwner() {
	waiting := s.insert(25*time.Hour, 2*time.Hour, exitBookingOptions{status: "IN_PROGRESS", verified: true})
	s.insert(23*time.Hour, 2*time.Hour, exitBookingOptions{status: "IN_PROGRESS", verified: true})
	s.insert(25*time.Hour, 2*time.Hour, exitBookingOptions{status: "CONFIRMED"})
	completed := s.insert(25*time.Hour, 2*time.Hour, exitBookingOptions{status: "COMPLETED", verified: true})

	eligible, err := s.repository.ListEligibleBookingsWithoutPayout(s.context)

	s.Require().NoError(err)
	s.ElementsMatch([]string{waiting, completed}, ids(eligible))
}

func (s *bookingExitIntegrationSuite) TestHourlyEndReminders_GoOutOnceForABookingEndingSoon() {
	soon := s.insert(-10*time.Minute, 2*time.Hour, exitBookingOptions{status: "CONFIRMED"})
	s.insert(-2*time.Hour, 3*time.Hour, exitBookingOptions{status: "CONFIRMED"})
	s.insert(-10*time.Minute, 2*time.Hour, exitBookingOptions{kind: "STAY", status: "CONFIRMED"})
	extended := s.insert(-10*time.Minute, 2*time.Hour, exitBookingOptions{status: "CONFIRMED"})
	s.insert(-time.Hour, time.Hour, exitBookingOptions{status: "CONFIRMED", parentID: &extended})

	first, err := s.repository.ClaimHourlyEndReminders(s.context, 15*time.Minute)
	s.Require().NoError(err)
	second, err := s.repository.ClaimHourlyEndReminders(s.context, 15*time.Minute)
	s.Require().NoError(err)

	s.Equal([]string{soon}, ids(first))
	s.Empty(second)
}

func (s *bookingExitIntegrationSuite) TestEndNotices_OnlyForAVerifiedBookingStillWaitingForItsExit() {
	waiting := s.insert(5*time.Minute, 2*time.Hour, exitBookingOptions{status: "IN_PROGRESS", verified: true})
	s.insert(5*time.Minute, 2*time.Hour, exitBookingOptions{status: "CONFIRMED"})
	s.insert(5*time.Minute, 2*time.Hour, exitBookingOptions{status: "COMPLETED", verified: true})
	s.insert(-30*time.Minute, 2*time.Hour, exitBookingOptions{status: "IN_PROGRESS", verified: true})

	first, err := s.repository.ClaimEndNotices(s.context)
	s.Require().NoError(err)
	second, err := s.repository.ClaimEndNotices(s.context)
	s.Require().NoError(err)

	s.Equal([]string{waiting}, ids(first))
	s.Empty(second)
}
