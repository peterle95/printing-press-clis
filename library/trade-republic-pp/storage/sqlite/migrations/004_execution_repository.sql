ALTER TABLE execution_previews ADD COLUMN account_id TEXT NOT NULL DEFAULT '';
ALTER TABLE execution_previews ADD COLUMN binding_hash TEXT NOT NULL DEFAULT '';

CREATE INDEX execution_previews_risk_state_idx
    ON execution_previews(account_id, currency, created_at, expires_at);

ALTER TABLE execution_approvals ADD COLUMN approval_json TEXT NOT NULL DEFAULT '{}';

CREATE UNIQUE INDEX execution_approvals_one_per_preview_idx
    ON execution_approvals(preview_id);

ALTER TABLE execution_audit_log ADD COLUMN account_id TEXT NOT NULL DEFAULT '';
ALTER TABLE execution_audit_log ADD COLUMN subject_id TEXT NOT NULL DEFAULT '';
ALTER TABLE execution_audit_log ADD COLUMN data_hash TEXT NOT NULL DEFAULT '';

