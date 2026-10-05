package enum

// VerificationReview is the admin review state of a paid booking that reached COMPLETED
// without the owner ever entering the driver's in-person arrival code (see
// repository.ExpireCompletedBookings). A booking that was verified never has one.
type VerificationReview int

const (
	// VerificationReviewPending blocks the booking's payout until an admin resolves it.
	VerificationReviewPending VerificationReview = iota
	// VerificationReviewApproved accepts the stay as having happened: the payout goes ahead.
	VerificationReviewApproved
	// VerificationReviewRefunded gives the driver their money back: no payout.
	VerificationReviewRefunded
)

var verificationReviewStrings = map[VerificationReview]string{
	VerificationReviewPending:  "PENDING",
	VerificationReviewApproved: "APPROVED",
	VerificationReviewRefunded: "REFUNDED",
}

var verificationReviewFromString = map[string]VerificationReview{
	"PENDING":  VerificationReviewPending,
	"APPROVED": VerificationReviewApproved,
	"REFUNDED": VerificationReviewRefunded,
}

func (r VerificationReview) String() string {
	if str, ok := verificationReviewStrings[r]; ok {
		return str
	}
	return "UNKNOWN"
}

// ParseVerificationReview maps a bookings.verification_review column value to its Go
// representation.
func ParseVerificationReview(s string) (VerificationReview, bool) {
	review, ok := verificationReviewFromString[s]
	return review, ok
}
