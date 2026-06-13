export const SCHEMA_SQL = `
CREATE TABLE IF NOT EXISTS searches (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  query TEXT NOT NULL,
  options_json TEXT NOT NULL,
  search_url TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS listings (
  id TEXT PRIMARY KEY,
  url TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL DEFAULT '',
  price TEXT,
  location TEXT,
  distance TEXT,
  posted_at TEXT,
  thumbnail_url TEXT,
  seller_name TEXT,
  category TEXT,
  snippet TEXT,
  raw_json TEXT NOT NULL DEFAULT '{}',
  first_seen_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  source_search_id INTEGER,
  FOREIGN KEY (source_search_id) REFERENCES searches(id)
);

CREATE INDEX IF NOT EXISTS listings_last_seen_idx ON listings(last_seen_at);
CREATE INDEX IF NOT EXISTS listings_url_idx ON listings(url);

CREATE TABLE IF NOT EXISTS watch_rules (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  query TEXT NOT NULL,
  radius_km REAL,
  max_price REAL,
  sort TEXT,
  options_json TEXT NOT NULL,
  active INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  last_run_at TEXT
);

CREATE TABLE IF NOT EXISTS watch_results (
  watch_id INTEGER NOT NULL,
  listing_id TEXT NOT NULL,
  seen_at TEXT NOT NULL,
  is_new INTEGER NOT NULL DEFAULT 1,
  PRIMARY KEY (watch_id, listing_id),
  FOREIGN KEY (watch_id) REFERENCES watch_rules(id),
  FOREIGN KEY (listing_id) REFERENCES listings(id)
);

CREATE TABLE IF NOT EXISTS message_drafts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  listing_id TEXT NOT NULL,
  listing_url TEXT NOT NULL,
  template TEXT,
  message_text TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (listing_id) REFERENCES listings(id)
);

CREATE TABLE IF NOT EXISTS sent_messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  listing_id TEXT NOT NULL,
  listing_url TEXT NOT NULL,
  message_text TEXT NOT NULL,
  confirmation_method TEXT NOT NULL,
  sent_at TEXT NOT NULL,
  FOREIGN KEY (listing_id) REFERENCES listings(id)
);

CREATE TABLE IF NOT EXISTS user_notes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  listing_id TEXT NOT NULL,
  note TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (listing_id) REFERENCES listings(id)
);

PRAGMA user_version = 1;
`;
