package booking

import (
	"context"

	"github.com/example/bookings-service/internal/domain/entity"
)

//go:generate go tool mockgen -destination=../../../pkg/mocks/proof_url_resolver_mock.go -package=mocks -source=./proof_url.go

// ProofURLResolver turns a settlement/payout's stored proof reference (an object key in the
// private payout-proofs R2 bucket, or -- for one confirmed before that bucket existed -- a
// full legacy public URL) into a URL the caller can actually load. Implemented by
// internal/repository/storage.
type ProofURLResolver interface {
	ResolveProofURL(ctx context.Context, keyOrURL string) (string, error)
}

// resolveSettlementProofURL overwrites settlement.ProofURL in place with a freshly resolved
// URL. A nil ProofURL (no proof attached yet, or still PENDING) is left untouched -- there's
// nothing to resolve.
func resolveSettlementProofURL(ctx context.Context, resolver ProofURLResolver, settlement *entity.Settlement) error {
	if settlement.ProofURL == nil {
		return nil
	}
	resolved, err := resolver.ResolveProofURL(ctx, *settlement.ProofURL)
	if err != nil {
		return err
	}
	settlement.ProofURL = &resolved
	return nil
}

// resolvePayoutProofURL is resolveSettlementProofURL's Payout counterpart, used by the admin
// payout endpoints (GET /admin/payouts, GET /admin/payouts/{id}) -- unlike a settlement's own
// nested payouts list (settlementPayoutResponse deliberately omits proof_url as redundant with
// the settlement's own), a standalone Payout response does carry it.
func resolvePayoutProofURL(ctx context.Context, resolver ProofURLResolver, payout *entity.Payout) error {
	if payout.ProofURL == nil {
		return nil
	}
	resolved, err := resolver.ResolveProofURL(ctx, *payout.ProofURL)
	if err != nil {
		return err
	}
	payout.ProofURL = &resolved
	return nil
}
