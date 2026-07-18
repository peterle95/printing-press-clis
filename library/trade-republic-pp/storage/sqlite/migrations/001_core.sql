CREATE TABLE instruments (
    isin TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT '',
    symbol TEXT NOT NULL DEFAULT '',
    country TEXT NOT NULL DEFAULT '',
    sector TEXT NOT NULL DEFAULT '',
    domicile TEXT NOT NULL DEFAULT '',
    base_currency TEXT NOT NULL DEFAULT '',
    trading_currency TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE instrument_aliases (
    id INTEGER PRIMARY KEY,
    alias_key TEXT NOT NULL,
    alias TEXT NOT NULL,
    isin TEXT NOT NULL REFERENCES instruments(isin) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE(alias_key, isin, kind)
);

CREATE INDEX instrument_aliases_key_idx ON instrument_aliases(alias_key);
CREATE INDEX instruments_name_idx ON instruments(name COLLATE NOCASE);
CREATE INDEX instruments_symbol_idx ON instruments(symbol COLLATE NOCASE);

CREATE TABLE sync_runs (
    id TEXT PRIMARY KEY,
    request_key TEXT NOT NULL UNIQUE,
    provider TEXT NOT NULL,
    adapter TEXT NOT NULL DEFAULT '',
    adapter_version TEXT NOT NULL DEFAULT '',
    snapshot_as_of TEXT NOT NULL,
    requested_since TEXT,
    include_documents INTEGER NOT NULL DEFAULT 0 CHECK(include_documents IN (0, 1)),
    dry_run INTEGER NOT NULL DEFAULT 0 CHECK(dry_run IN (0, 1)),
    request_metadata_json TEXT NOT NULL DEFAULT '{}',
    warnings_json TEXT NOT NULL DEFAULT '[]',
    status TEXT NOT NULL CHECK(status IN ('running', 'success', 'failed')),
    instruments_count INTEGER NOT NULL DEFAULT 0,
    positions_count INTEGER NOT NULL DEFAULT 0,
    cash_balances_count INTEGER NOT NULL DEFAULT 0,
    transactions_count INTEGER NOT NULL DEFAULT 0,
    documents_count INTEGER NOT NULL DEFAULT 0,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    error_text TEXT NOT NULL DEFAULT ''
);

CREATE INDEX sync_runs_started_idx ON sync_runs(started_at DESC);
CREATE INDEX sync_runs_status_idx ON sync_runs(status, started_at DESC);

CREATE TABLE transactions (
    id TEXT PRIMARY KEY,
    fingerprint TEXT NOT NULL UNIQUE,
    occurred_at TEXT NOT NULL,
    original_timestamp TEXT NOT NULL DEFAULT '',
    timezone_assumption TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL,
    isin TEXT REFERENCES instruments(isin),
    quantity_i INTEGER NOT NULL,
    amount_i INTEGER NOT NULL,
    fees_i INTEGER NOT NULL,
    taxes_i INTEGER NOT NULL,
    currency TEXT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL,
    source_ref TEXT NOT NULL DEFAULT '',
    raw_json TEXT NOT NULL DEFAULT '',
    sync_run_id TEXT NOT NULL REFERENCES sync_runs(id),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX transactions_occurred_idx ON transactions(occurred_at DESC);
CREATE INDEX transactions_isin_occurred_idx ON transactions(isin, occurred_at DESC);
CREATE INDEX transactions_kind_occurred_idx ON transactions(kind, occurred_at DESC);

CREATE TABLE positions (
    isin TEXT PRIMARY KEY REFERENCES instruments(isin),
    name TEXT NOT NULL,
    quantity_i INTEGER NOT NULL,
    average_cost_i INTEGER NOT NULL,
    price_i INTEGER NOT NULL,
    market_value_i INTEGER NOT NULL,
    currency TEXT NOT NULL,
    as_of TEXT NOT NULL,
    source TEXT NOT NULL,
    sync_run_id TEXT NOT NULL REFERENCES sync_runs(id),
    updated_at TEXT NOT NULL
);

CREATE INDEX positions_value_idx ON positions(market_value_i DESC);

CREATE TABLE cash_movements (
    id TEXT PRIMARY KEY,
    transaction_id TEXT NOT NULL UNIQUE REFERENCES transactions(id) ON DELETE CASCADE,
    occurred_at TEXT NOT NULL,
    kind TEXT NOT NULL,
    amount_i INTEGER NOT NULL,
    currency TEXT NOT NULL,
    sync_run_id TEXT NOT NULL REFERENCES sync_runs(id)
);

CREATE INDEX cash_movements_occurred_idx ON cash_movements(occurred_at DESC);

CREATE TABLE cash_balances (
    currency TEXT PRIMARY KEY,
    amount_i INTEGER NOT NULL,
    as_of TEXT NOT NULL,
    source TEXT NOT NULL,
    sync_run_id TEXT NOT NULL REFERENCES sync_runs(id),
    updated_at TEXT NOT NULL
);

CREATE TABLE dividends (
    id TEXT PRIMARY KEY,
    transaction_id TEXT NOT NULL UNIQUE REFERENCES transactions(id) ON DELETE CASCADE,
    isin TEXT REFERENCES instruments(isin),
    occurred_at TEXT NOT NULL,
    gross_i INTEGER NOT NULL,
    net_i INTEGER NOT NULL,
    taxes_i INTEGER NOT NULL,
    currency TEXT NOT NULL,
    sync_run_id TEXT NOT NULL REFERENCES sync_runs(id)
);

CREATE INDEX dividends_isin_occurred_idx ON dividends(isin, occurred_at DESC);

CREATE TABLE fees (
    id TEXT PRIMARY KEY,
    transaction_id TEXT NOT NULL UNIQUE REFERENCES transactions(id) ON DELETE CASCADE,
    isin TEXT REFERENCES instruments(isin),
    occurred_at TEXT NOT NULL,
    amount_i INTEGER NOT NULL,
    currency TEXT NOT NULL,
    sync_run_id TEXT NOT NULL REFERENCES sync_runs(id)
);

CREATE INDEX fees_occurred_idx ON fees(occurred_at DESC);

CREATE TABLE taxes (
    id TEXT PRIMARY KEY,
    transaction_id TEXT NOT NULL UNIQUE REFERENCES transactions(id) ON DELETE CASCADE,
    isin TEXT REFERENCES instruments(isin),
    occurred_at TEXT NOT NULL,
    amount_i INTEGER NOT NULL,
    currency TEXT NOT NULL,
    sync_run_id TEXT NOT NULL REFERENCES sync_runs(id)
);

CREATE INDEX taxes_occurred_idx ON taxes(occurred_at DESC);

CREATE TABLE documents (
    id TEXT PRIMARY KEY,
    sha256 TEXT NOT NULL UNIQUE,
    path TEXT NOT NULL,
    filename TEXT NOT NULL,
    document_type TEXT NOT NULL,
    occurred_at TEXT,
    isin TEXT REFERENCES instruments(isin),
    source TEXT NOT NULL,
    imported_at TEXT NOT NULL,
    parser_version TEXT NOT NULL DEFAULT '',
    sync_run_id TEXT NOT NULL REFERENCES sync_runs(id),
    created_at TEXT NOT NULL
);

CREATE INDEX documents_isin_occurred_idx ON documents(isin, occurred_at DESC);

CREATE TABLE price_history (
    id INTEGER PRIMARY KEY,
    isin TEXT NOT NULL REFERENCES instruments(isin),
    price_i INTEGER NOT NULL,
    currency TEXT NOT NULL,
    venue TEXT NOT NULL DEFAULT '',
    as_of TEXT NOT NULL,
    source TEXT NOT NULL,
    source_url TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    UNIQUE(isin, currency, venue, as_of, source)
);

CREATE INDEX price_history_latest_idx ON price_history(isin, as_of DESC);

CREATE TABLE research_reports (
    id TEXT PRIMARY KEY,
    identifier TEXT NOT NULL,
    isin TEXT REFERENCES instruments(isin),
    symbol TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    as_of TEXT NOT NULL,
    schema_version INTEGER NOT NULL,
    report_json TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX research_reports_identifier_idx ON research_reports(identifier, as_of DESC);
CREATE INDEX research_reports_isin_idx ON research_reports(isin, as_of DESC);

