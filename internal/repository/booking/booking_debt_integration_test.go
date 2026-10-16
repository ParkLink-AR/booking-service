package booking_test

import (
	"time"

	"github.com/example/bookings-service/internal/domain/entity"
	"github.com/example/bookings-service/internal/domain/enum"
)

const (
	debtOwnerID  = "66666666-6666-6666-6666-666666666666"
	debtDriverID = "77777777-7777-7777-7777-777777777777"
)

// insertDebt adds a debt of amount centavos owed by debtDriverID for overstaying sourceID.
func (s *bookingExitIntegrationSuite) insertDebt(sourceID string, amount int, status string, chargedID *string) {
	_, err := s.database.Exec(`
		INSERT INTO booking_debts (user_id, source_booking_id, charged_booking_id, minutes, amount, status)
		VALUES ($1, $2, $3, 20, $4, $5)
	`, debtDriverID, sourceID, chargedID, amount, status)
	s.Require().NoError(err)
}

func (s *bookingExitIntegrationSuite) debtStatus(sourceID string) (string, *string) {
	var status string
	var chargedID *string
	s.Require().NoError(s.database.QueryRow(
		`SELECT status, charged_booking_id::text FROM booking_debts WHERE source_booking_id = $1`, sourceID).Scan(&status, &chargedID))
	return status, chargedID
}

func (s *bookingExitIntegrationSuite) debtAmountOf(bookingID string) int {
	var amount int
	s.Require().NoError(s.database.QueryRow(`SELECT debt_amount FROM bookings WHERE id = $1`, bookingID).Scan(&amount))
	return amount
}

// newBookingAt creates a PENDING_PAYMENT booking of debtDriverID starting offset from now, through
// the repository.
func (s *bookingExitIntegrationSuite) newBookingAt(offset time.Duration) entity.Booking {
	hold := time.Now().Add(5 * time.Minute)
	ownerAmount := 200000
	created, err := s.repository.CreateBooking(s.context, entity.Booking{
		ParkingSpotID: exitSpotID, UserID: debtDriverID,
		Start: time.Now().Add(offset), End: time.Now().Add(offset + 2*time.Hour),
		Status: enum.BookingStatusPendingPayment, HoldExpiresAt: &hold,
		Amount: 240000, OwnerAmount: &ownerAmount, Kind: enum.BookingKindHourly, Capacity: 1,
	})
	s.Require().NoError(err)
	return created
}

func (s *bookingExitIntegrationSuite) TestCloseExit_PastTheGracePeriod_LeavesADebtAndAnOverstayPayout() {
	bookingID := s.insert(30*time.Minute, 2*time.Hour, exitBookingOptions{status: "IN_PROGRESS", verified: true, userID: debtDriverID})
	_, err := s.repository.EnsureExit(s.context, bookingID, "4821")
	s.Require().NoError(err)

	_, err = s.repository.CloseExit(s.context, bookingID, time.Now(), entity.ExitClosedByOwnerCode,
		entity.Overstay{Minutes: 30, ChargedMinutes: 30, Amount: 60000, OwnerAmount: 50000}, debtOwnerID)

	s.Require().NoError(err)
	status, charged := s.debtStatus(bookingID)
	s.Equal(entity.DebtPending, status)
	s.Nil(charged)
	var net, commission int
	var kind, owner string
	s.Require().NoError(s.database.QueryRow(
		`SELECT kind, owner_id::text, net_amount, commission_amount FROM payouts WHERE booking_id = $1`, bookingID).Scan(&kind, &owner, &net, &commission))
	s.Equal("OVERSTAY", kind)
	s.Equal(debtOwnerID, owner)
	s.Equal(50000, net)
	s.Zero(commission)
}

func (s *bookingExitIntegrationSuite) TestCloseExit_WithinTheGracePeriod_ChargesNothing() {
	bookingID := s.insert(10*time.Minute, 2*time.Hour, exitBookingOptions{status: "IN_PROGRESS", verified: true, userID: debtDriverID})
	_, err := s.repository.EnsureExit(s.context, bookingID, "4821")
	s.Require().NoError(err)

	_, err = s.repository.CloseExit(s.context, bookingID, time.Now(), entity.ExitClosedByOwnerCode,
		entity.Overstay{Minutes: 10}, debtOwnerID)

	s.Require().NoError(err)
	var debts, payouts int
	s.Require().NoError(s.database.QueryRow(`SELECT count(*) FROM booking_debts`).Scan(&debts))
	s.Require().NoError(s.database.QueryRow(`SELECT count(*) FROM payouts`).Scan(&payouts))
	s.Zero(debts)
	s.Zero(payouts)
}

func (s *bookingExitIntegrationSuite) TestPayouts_TheOverstayPayoutDoesNotHideTheBookingsOwn() {
	bookingID := s.insert(30*time.Hour, 2*time.Hour, exitBookingOptions{status: "IN_PROGRESS", verified: true, userID: debtDriverID})
	_, err := s.repository.EnsureExit(s.context, bookingID, "4821")
	s.Require().NoError(err)
	_, err = s.repository.CloseExit(s.context, bookingID, time.Now().Add(-29*time.Hour), entity.ExitClosedByOwnerCode,
		entity.Overstay{Minutes: 30, ChargedMinutes: 30, Amount: 60000, OwnerAmount: 50000}, debtOwnerID)
	s.Require().NoError(err)

	eligible, err := s.repository.ListEligibleBookingsWithoutPayout(s.context)

	s.Require().NoError(err)
	s.Equal([]string{bookingID}, ids(eligible), "the booking's own payout is still to be generated")
	payout, err := s.repository.GetPayoutByBooking(s.context, bookingID)
	s.Require().NoError(err)
	s.Nil(payout, "GetPayoutByBooking is the booking's own payout, not the overstay one")
}

func (s *bookingExitIntegrationSuite) TestCreateBooking_TakesOnThePendingDebts() {
	first := s.insert(48*time.Hour, time.Hour, exitBookingOptions{status: "COMPLETED", userID: debtDriverID})
	second := s.insert(72*time.Hour, time.Hour, exitBookingOptions{status: "COMPLETED", userID: debtDriverID})
	paid := s.insert(96*time.Hour, time.Hour, exitBookingOptions{status: "COMPLETED", userID: debtDriverID})
	s.insertDebt(first, 6000, entity.DebtPending, nil)
	s.insertDebt(second, 4000, entity.DebtPending, nil)
	s.insertDebt(paid, 9000, entity.DebtPaid, nil)

	created := s.newBookingAt(24 * time.Hour)

	s.Equal(10000, created.DebtAmount)
	s.Equal(10000, s.debtAmountOf(created.ID))
	status, charged := s.debtStatus(first)
	s.Equal(entity.DebtReserved, status)
	s.Require().NotNil(charged)
	s.Equal(created.ID, *charged)
	status, _ = s.debtStatus(paid)
	s.Equal(entity.DebtPaid, status, "what was already settled is not charged again")

	again := s.newBookingAt(48 * time.Hour)
	s.Zero(again.DebtAmount, "a second booking made meanwhile has nothing left to take on")
}

func (s *bookingExitIntegrationSuite) TestConfirmBooking_SettlesTheDebtsItTookOn() {
	source := s.insert(48*time.Hour, time.Hour, exitBookingOptions{status: "COMPLETED", userID: debtDriverID})
	s.insertDebt(source, 6000, entity.DebtPending, nil)
	created := s.newBookingAt(24 * time.Hour)

	_, err := s.repository.ConfirmBooking(s.context, created.ID, "pay-9", enum.BookingStatusPendingOwnerApproval, nil, nil)

	s.Require().NoError(err)
	status, charged := s.debtStatus(source)
	s.Equal(entity.DebtPaid, status)
	s.Require().NotNil(charged)
	s.Equal(created.ID, *charged)
}

func (s *bookingExitIntegrationSuite) TestExpiredHold_GivesTheDebtsBack() {
	source := s.insert(48*time.Hour, time.Hour, exitBookingOptions{status: "COMPLETED", userID: debtDriverID})
	s.insertDebt(source, 6000, entity.DebtPending, nil)
	created := s.newBookingAt(24 * time.Hour)
	_, err := s.database.Exec(`UPDATE bookings SET hold_expires_at = now() - interval '1 minute' WHERE id = $1`, created.ID)
	s.Require().NoError(err)

	expired, err := s.repository.ExpireStaleHolds(s.context)

	s.Require().NoError(err)
	s.Equal(int64(1), expired)
	status, charged := s.debtStatus(source)
	s.Equal(entity.DebtPending, status)
	s.Nil(charged)
}

// paidBookingWithDebt returns a debt's source booking and the booking whose payment settled it.
func (s *bookingExitIntegrationSuite) paidBookingWithDebt() (string, string) {
	source := s.insert(48*time.Hour, time.Hour, exitBookingOptions{status: "COMPLETED", userID: debtDriverID})
	s.insertDebt(source, 6000, entity.DebtPending, nil)
	created := s.newBookingAt(24 * time.Hour)
	_, err := s.repository.ConfirmBooking(s.context, created.ID, "pay-9", enum.BookingStatusConfirmed, nil, nil)
	s.Require().NoError(err)
	return source, created.ID
}

func (s *bookingExitIntegrationSuite) TestCancelBooking_RefundedInFull_GivesTheDebtsBack() {
	source, paidBooking := s.paidBookingWithDebt()

	_, err := s.repository.CancelBooking(s.context, paidBooking, "DRIVER_CANCELLED", 0)

	s.Require().NoError(err)
	status, charged := s.debtStatus(source)
	s.Equal(entity.DebtPending, status, "the whole payment went back, the debt part included")
	s.Nil(charged)
}

func (s *bookingExitIntegrationSuite) TestCancelBooking_LateRefund_KeepsTheDebtPaid() {
	source, paidBooking := s.paidBookingWithDebt()

	_, err := s.repository.CancelBooking(s.context, paidBooking, "DRIVER_LATE_CANCELLED", 120000)

	s.Require().NoError(err)
	status, charged := s.debtStatus(source)
	s.Equal(entity.DebtPaid, status, "only the booking's own share was given back")
	s.Require().NotNil(charged)
	s.Equal(paidBooking, *charged)
}

func (s *bookingExitIntegrationSuite) TestRejectBooking_GivesTheDebtsBack() {
	source, paidBooking := s.paidBookingWithDebt()
	_, err := s.database.Exec(`UPDATE bookings SET status = 'PENDING_OWNER_APPROVAL' WHERE id = $1`, paidBooking)
	s.Require().NoError(err)

	_, err = s.repository.RejectBooking(s.context, paidBooking)

	s.Require().NoError(err)
	status, _ := s.debtStatus(source)
	s.Equal(entity.DebtPending, status)
}

func (s *bookingExitIntegrationSuite) TestListDebtsByUser_ShowsWhichBookingEachCameFrom() {
	older := s.insert(72*time.Hour, 2*time.Hour, exitBookingOptions{status: "COMPLETED", userID: debtDriverID})
	newer := s.insert(48*time.Hour, 2*time.Hour, exitBookingOptions{status: "COMPLETED", userID: debtDriverID})
	s.insertDebt(older, 4000, entity.DebtPending, nil)
	s.insertDebt(newer, 6000, entity.DebtPending, nil)
	other := s.insert(24*time.Hour, 2*time.Hour, exitBookingOptions{status: "COMPLETED"})
	_, err := s.database.Exec(`INSERT INTO booking_debts (user_id, source_booking_id, minutes, amount) VALUES (gen_random_uuid(), $1, 20, 1000)`, other)
	s.Require().NoError(err)

	debts, err := s.repository.ListDebtsByUser(s.context, debtDriverID)

	s.Require().NoError(err)
	s.Require().Len(debts, 2, "someone else's debt is not listed")
	s.ElementsMatch([]string{older, newer}, []string{debts[0].SourceBookingID, debts[1].SourceBookingID})
	s.Equal(exitSpotID, debts[0].ParkingSpotID)
}
