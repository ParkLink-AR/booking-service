-- A single payout event to a parking-spot owner, covering every payout that was unattached
-- for them at the moment an admin drafted it -- see usecase.DraftSettlementUseCase and
-- usecase.ConfirmSettlementUseCase. Two-phase: PENDING (drafted, amount frozen, nothing has
-- moved yet) -> PAID (an admin confirmed the bank transfer actually went out). paid_at/
-- paid_by_admin_id/proof_url are only ever set at the PAID transition -- created_by_admin_id
-- is set at draft time and may name a different admin than the one who later confirms it.
CREATE TABLE IF NOT EXISTS settlements (
	id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
	owner_id            UUID        NOT NULL,
	gross_amount        INT         NOT NULL,
	commission_amount   INT         NOT NULL,
	net_amount          INT         NOT NULL,
	status              TEXT        NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PAID')),
	proof_url           TEXT,
	paid_at             TIMESTAMPTZ,
	paid_by_admin_id    UUID,
	created_by_admin_id UUID        NOT NULL,
	created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_settlements_owner ON settlements (owner_id);
CREATE INDEX IF NOT EXISTS idx_settlements_status ON settlements (status);
