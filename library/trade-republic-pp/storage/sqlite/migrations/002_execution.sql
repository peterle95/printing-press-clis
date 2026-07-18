CREATE TABLE execution_previews (
    id TEXT PRIMARY KEY,
    idempotency_key TEXT NOT NULL UNIQUE,
    isin TEXT NOT NULL REFERENCES instruments(isin),
    side TEXT NOT NULL CHECK(side IN ('buy', 'sell')),
    quantity_i INTEGER NOT NULL,
    amount_i INTEGER NOT NULL,
    limit_price_i INTEGER NOT NULL,
    currency TEXT NOT NULL,
    mode TEXT NOT NULL CHECK(mode IN ('paper', 'export')),
    price_as_of TEXT NOT NULL,
    balance_as_of TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending', 'approved', 'expired', 'cancelled', 'exported')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX execution_previews_created_idx ON execution_previews(created_at DESC);

CREATE TABLE execution_approvals (
    id TEXT PRIMARY KEY,
    preview_id TEXT NOT NULL REFERENCES execution_previews(id) ON DELETE CASCADE,
    challenge_hash TEXT NOT NULL,
    approved_by TEXT NOT NULL,
    approved_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    UNIQUE(preview_id, challenge_hash)
);

CREATE INDEX execution_approvals_preview_idx ON execution_approvals(preview_id, approved_at DESC);

CREATE TABLE execution_audit_log (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL UNIQUE,
    preview_id TEXT REFERENCES execution_previews(id),
    event_type TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    previous_hash TEXT NOT NULL DEFAULT '',
    entry_hash TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL
);

CREATE INDEX execution_audit_preview_idx ON execution_audit_log(preview_id, sequence);

CREATE TABLE execution_idempotency (
    idempotency_key TEXT PRIMARY KEY,
    operation TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('reserved', 'completed', 'failed')),
    result_json TEXT NOT NULL DEFAULT '',
    error_text TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE INDEX execution_idempotency_expiry_idx ON execution_idempotency(expires_at);
