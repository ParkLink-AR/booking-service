-- Links an individual payout to the settlement draft that swept it up (see
-- usecase.DraftSettlementUseCase). Non-null as soon as a payout is drafted into a settlement,
-- regardless of whether that settlement has been confirmed yet -- a payout with a non-null
-- settlement_id no longer counts toward GetOwnerPendingBalances / a future draft, whether or
-- not it has itself transitioned to PAID.
ALTER TABLE payouts ADD COLUMN IF NOT EXISTS settlement_id UUID REFERENCES settlements(id);
CREATE INDEX IF NOT EXISTS idx_payouts_settlement ON payouts (settlement_id);
