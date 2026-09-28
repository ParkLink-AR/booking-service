package booking_test

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/suite"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
	"github.com/example/bookings-service/internal/domain/errors"
	"github.com/example/bookings-service/internal/repository/booking"
)

// TestPayoutRepositoryIntegrationSuite exercises the payout/settlement/platform_settings
// repository methods against a real PostgreSQL instance -- ListEligibleBookingsWithoutPayout's
// LEFT JOIN filter and CreateSettlementDraft/ConfirmSettlement/CancelSettlement's
// SELECT ... FOR UPDATE locking both depend on real SQL semantics a mocked *sql.DB cannot
// verify. Skipped unless BOOKING_TEST_DSN is set (see repository_integration_test.go), with
// migrations/001-017_*.sql applied.
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
	_, err = database.Exec("DELETE FROM settlements")
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
		BookingID: alreadyPaidOutBookingID, OwnerID: "33333333-3333-3333-3333-333333333333", GrossAmount: 10000, CommissionAmount: 500, NetAmount: 9500,
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

// insertPendingPayout inserts a bare payout row (no owning booking needed for these tests --
// the FK is on bookings, not payouts, for the booking_id column referenced by real bookings
// created above where relevant) directly, since CreateSettlementDraft only cares about
// owner_id/status/settlement_id.
func (s *payoutRepositoryIntegrationSuite) insertPendingPayout(ownerID string, netAmount int) string {
	bookingID := s.insertCompletedPaidBooking("88888888-8888-8888-8888-888888888888", time.Now().Add(-25*time.Hour))
	payout, err := s.repository.CreatePayout(s.context, entity.Payout{
		BookingID: bookingID, OwnerID: ownerID, GrossAmount: netAmount + 500, CommissionAmount: 500, NetAmount: netAmount,
	})
	s.Require().NoError(err)
	return payout.ID
}

func (s *payoutRepositoryIntegrationSuite) TestCreateSettlementDraft_SweepsUpEveryUnattachedPendingPayout() {
	ownerID := "99999999-9999-9999-9999-999999999999"
	adminID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	payoutID1 := s.insertPendingPayout(ownerID, 9500)
	payoutID2 := s.insertPendingPayout(ownerID, 4500)

	settlement, payouts, err := s.repository.CreateSettlementDraft(s.context, ownerID, adminID)

	s.Require().NoError(err)
	s.Equal(enum.SettlementStatusPending, settlement.Status)
	s.Equal(14000, settlement.NetAmount)
	s.Equal(adminID, settlement.CreatedByAdminID)
	s.Nil(settlement.PaidAt)
	s.Len(payouts, 2)
	ids := []string{payouts[0].ID, payouts[1].ID}
	s.Contains(ids, payoutID1)
	s.Contains(ids, payoutID2)

	// Drafting attaches settlement_id but leaves each payout's own status PENDING -- it only
	// becomes PAID once the settlement is confirmed.
	fetched, err := s.repository.GetPayoutByID(s.context, payoutID1)
	s.Require().NoError(err)
	s.Equal(enum.PayoutStatusPending, fetched.Status)
	s.Require().NotNil(fetched.SettlementID)
	s.Equal(settlement.ID, *fetched.SettlementID)
}

func (s *payoutRepositoryIntegrationSuite) TestCreateSettlementDraft_NoPendingPayouts_ReturnsNothingToSettle() {
	ownerID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	adminID := "cccccccc-cccc-cccc-cccc-cccccccccccc"

	_, _, err := s.repository.CreateSettlementDraft(s.context, ownerID, adminID)

	s.Require().Error(err)
	s.True(errors.IsInvalidInput(err))
	s.Equal("NOTHING_TO_SETTLE", errors.InvalidInputCode(err))
}

func (s *payoutRepositoryIntegrationSuite) TestCreateSettlementDraft_AlreadyDraftedPayout_IsNotSweptTwice() {
	ownerID := "dddddddd-dddd-dddd-dddd-dddddddddddd"
	adminID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	s.insertPendingPayout(ownerID, 9500)

	firstSettlement, firstPayouts, err := s.repository.CreateSettlementDraft(s.context, ownerID, adminID)
	s.Require().NoError(err)
	s.Len(firstPayouts, 1)

	balances, err := s.repository.GetOwnerPendingBalances(s.context)
	s.Require().NoError(err)
	for _, b := range balances {
		s.NotEqual(ownerID, b.OwnerID)
	}

	_, _, err = s.repository.CreateSettlementDraft(s.context, ownerID, adminID)
	s.True(errors.IsInvalidInput(err))
	s.NotEmpty(firstSettlement.ID)
}

// TestCreateSettlementDraft_ConcurrentDraftsForSameOwner_OnlyOneSucceeds proves the
// SELECT ... FOR UPDATE lock actually serializes concurrent draft attempts for the same owner:
// two goroutines race to draft the same owner's single pending payout, and exactly one must
// come away with it -- the other must see zero unattached rows left (NOTHING_TO_SETTLE),
// never a duplicate/overlapping draft of the same payout.
func (s *payoutRepositoryIntegrationSuite) TestCreateSettlementDraft_ConcurrentDraftsForSameOwner_OnlyOneSucceeds() {
	ownerID := "ffffffff-ffff-ffff-ffff-ffffffffffff"
	adminID := "12121212-1212-1212-1212-121212121212"
	s.insertPendingPayout(ownerID, 9500)

	var wg sync.WaitGroup
	successes := make([]bool, 2)
	errs := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, payouts, err := s.repository.CreateSettlementDraft(s.context, ownerID, adminID)
			errs[index] = err
			successes[index] = err == nil && len(payouts) == 1
		}(i)
	}
	wg.Wait()

	successCount := 0
	for i, ok := range successes {
		if ok {
			successCount++
		} else {
			s.True(errors.IsInvalidInput(errs[i]))
		}
	}
	s.Equal(1, successCount)
}

func (s *payoutRepositoryIntegrationSuite) TestConfirmSettlement_FlipsSettlementAndPayoutsToPaid() {
	ownerID := "13131313-1313-1313-1313-131313131313"
	adminID := "14141414-1414-1414-1414-141414141414"
	confirmingAdminID := "15151515-1515-1515-1515-151515151515"
	payoutID := s.insertPendingPayout(ownerID, 9500)
	draft, _, err := s.repository.CreateSettlementDraft(s.context, ownerID, adminID)
	s.Require().NoError(err)

	proofURL := "https://cdn.example.com/payout-proofs/" + ownerID + "/" + draft.ID + ".jpg"
	confirmed, payouts, err := s.repository.ConfirmSettlement(s.context, draft.ID, confirmingAdminID, &proofURL)

	s.Require().NoError(err)
	s.Equal(enum.SettlementStatusPaid, confirmed.Status)
	s.Require().NotNil(confirmed.PaidByAdminID)
	s.Equal(confirmingAdminID, *confirmed.PaidByAdminID)
	s.Require().NotNil(confirmed.PaidAt)
	s.Require().NotNil(confirmed.ProofURL)
	s.Equal(proofURL, *confirmed.ProofURL)

	s.Require().Len(payouts, 1)
	s.Equal(payoutID, payouts[0].ID)
	s.Equal(enum.PayoutStatusPaid, payouts[0].Status)
	s.Require().NotNil(payouts[0].PaidByAdminID)
	s.Equal(confirmingAdminID, *payouts[0].PaidByAdminID)
}

func (s *payoutRepositoryIntegrationSuite) TestConfirmSettlement_AlreadyConfirmed_ReturnsConflict() {
	ownerID := "16161616-1616-1616-1616-161616161616"
	adminID := "17171717-1717-1717-1717-171717171717"
	s.insertPendingPayout(ownerID, 9500)
	draft, _, err := s.repository.CreateSettlementDraft(s.context, ownerID, adminID)
	s.Require().NoError(err)
	_, _, err = s.repository.ConfirmSettlement(s.context, draft.ID, adminID, nil)
	s.Require().NoError(err)

	_, _, err = s.repository.ConfirmSettlement(s.context, draft.ID, adminID, nil)

	s.True(errors.IsConflict(err))
}

func (s *payoutRepositoryIntegrationSuite) TestConfirmSettlement_NotFound_ReturnsNotFound() {
	_, _, err := s.repository.ConfirmSettlement(s.context, "18181818-1818-1818-1818-181818181818", "admin", nil)

	s.True(errors.IsNotFound(err))
}

func (s *payoutRepositoryIntegrationSuite) TestCancelSettlement_ReleasesPayoutsBackToUnattached() {
	ownerID := "19191919-1919-1919-1919-191919191919"
	adminID := "20202020-2020-2020-2020-202020202020"
	s.insertPendingPayout(ownerID, 9500)
	draft, _, err := s.repository.CreateSettlementDraft(s.context, ownerID, adminID)
	s.Require().NoError(err)

	err = s.repository.CancelSettlement(s.context, draft.ID)
	s.Require().NoError(err)

	balances, err := s.repository.GetOwnerPendingBalances(s.context)
	s.Require().NoError(err)
	var found bool
	for _, b := range balances {
		if b.OwnerID == ownerID {
			found = true
			s.Equal(9500, b.PendingNetAmount)
		}
	}
	s.True(found, "owner's payout should be unattached and eligible again after cancel")

	_, err = s.repository.GetSettlementByID(s.context, draft.ID)
	s.True(errors.IsNotFound(err))
}

func (s *payoutRepositoryIntegrationSuite) TestCancelSettlement_AlreadyConfirmed_ReturnsConflict() {
	ownerID := "21212121-2121-2121-2121-212121212121"
	adminID := "22222222-2222-2222-2222-222222222223"
	s.insertPendingPayout(ownerID, 9500)
	draft, _, err := s.repository.CreateSettlementDraft(s.context, ownerID, adminID)
	s.Require().NoError(err)
	_, _, err = s.repository.ConfirmSettlement(s.context, draft.ID, adminID, nil)
	s.Require().NoError(err)

	err = s.repository.CancelSettlement(s.context, draft.ID)

	s.True(errors.IsConflict(err))
}
