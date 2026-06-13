import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import initSqlJs, { type Database as SqlDatabase, type SqlJsStatic } from "sql.js";
import type { ParsedListing } from "./parser.js";
import { ensureParentDir } from "./paths.js";
import { SCHEMA_SQL } from "../db/schema.js";

let sqlPromise: Promise<SqlJsStatic> | undefined;

export interface SearchRecord {
  id: number;
  query: string;
  options_json: string;
  search_url: string;
  created_at: string;
}

export interface WatchRule {
  id: number;
  query: string;
  radius_km?: number;
  max_price?: number;
  sort?: string;
  options_json: string;
  active: number;
  created_at: string;
  last_run_at?: string;
}

export interface MessageDraft {
  id: number;
  listing_id: string;
  listing_url: string;
  template?: string;
  message_text: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export class KleinanzeigenDb {
  private constructor(
    private readonly db: SqlDatabase,
    private readonly filePath: string | null,
  ) {}

  static async open(filePath: string): Promise<KleinanzeigenDb> {
    const SQL = await getSql();
    ensureParentDir(filePath);
    const db = fs.existsSync(filePath)
      ? new SQL.Database(new Uint8Array(fs.readFileSync(filePath)))
      : new SQL.Database();
    const wrapper = new KleinanzeigenDb(db, filePath);
    wrapper.migrate();
    wrapper.persist();
    return wrapper;
  }

  static async memory(): Promise<KleinanzeigenDb> {
    const SQL = await getSql();
    const wrapper = new KleinanzeigenDb(new SQL.Database(), null);
    wrapper.migrate();
    return wrapper;
  }

  close(): void {
    this.persist();
    this.db.close();
  }

  recordSearch(query: string, options: unknown, searchUrl: string): number {
    this.run(
      "INSERT INTO searches (query, options_json, search_url, created_at) VALUES (?, ?, ?, ?)",
      [query, JSON.stringify(options), searchUrl, now()],
    );
    const id = Number(this.get<{ id: number }>("SELECT last_insert_rowid() AS id")?.id);
    this.persist();
    return id;
  }

  upsertListing(listing: ParsedListing, sourceSearchId?: number): void {
    const existing = this.get<{ first_seen_at: string }>("SELECT first_seen_at FROM listings WHERE id = ?", [listing.id]);
    const timestamp = now();
    this.run(
      `INSERT INTO listings (
        id, url, title, price, location, distance, posted_at, thumbnail_url,
        seller_name, category, snippet, raw_json, first_seen_at, last_seen_at, source_search_id
      ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
      ON CONFLICT(id) DO UPDATE SET
        url = excluded.url,
        title = excluded.title,
        price = excluded.price,
        location = excluded.location,
        distance = excluded.distance,
        posted_at = excluded.posted_at,
        thumbnail_url = excluded.thumbnail_url,
        seller_name = excluded.seller_name,
        category = excluded.category,
        snippet = excluded.snippet,
        raw_json = excluded.raw_json,
        last_seen_at = excluded.last_seen_at,
        source_search_id = excluded.source_search_id`,
      [
        listing.id,
        listing.url,
        listing.title,
        listing.price ?? null,
        listing.location ?? null,
        listing.distance ?? null,
        listing.posted_at ?? null,
        listing.thumbnail_url ?? null,
        listing.seller_name ?? null,
        listing.category ?? null,
        listing.snippet ?? null,
        JSON.stringify(listing),
        existing?.first_seen_at ?? timestamp,
        timestamp,
        sourceSearchId ?? null,
      ],
    );
    this.persist();
  }

  getListing(id: string): ParsedListing | null {
    const row = this.get<Record<string, unknown>>("SELECT * FROM listings WHERE id = ? OR url = ?", [id, id]);
    return row ? rowToListing(row) : null;
  }

  listRecentListings(limit = 20): ParsedListing[] {
    return this.all<Record<string, unknown>>("SELECT * FROM listings ORDER BY last_seen_at DESC LIMIT ?", [limit]).map(rowToListing);
  }

  addNote(listingId: string, note: string): number {
    this.run("INSERT INTO user_notes (listing_id, note, created_at) VALUES (?, ?, ?)", [listingId, note, now()]);
    const id = Number(this.get<{ id: number }>("SELECT last_insert_rowid() AS id")?.id);
    this.persist();
    return id;
  }

  addWatchRule(query: string, options: { radiusKm?: number; maxPrice?: number; sort?: string }): number {
    this.run(
      `INSERT INTO watch_rules (query, radius_km, max_price, sort, options_json, active, created_at)
       VALUES (?, ?, ?, ?, ?, 1, ?)`,
      [
        query,
        options.radiusKm ?? null,
        options.maxPrice ?? null,
        options.sort ?? null,
        JSON.stringify(options),
        now(),
      ],
    );
    const id = Number(this.get<{ id: number }>("SELECT last_insert_rowid() AS id")?.id);
    this.persist();
    return id;
  }

  listWatchRules(activeOnly = false): WatchRule[] {
    const sql = activeOnly
      ? "SELECT * FROM watch_rules WHERE active = 1 ORDER BY id"
      : "SELECT * FROM watch_rules ORDER BY id";
    return this.all<WatchRule>(sql);
  }

  removeWatchRule(id: number): boolean {
    this.run("UPDATE watch_rules SET active = 0 WHERE id = ?", [id]);
    const changed = Number(this.get<{ changed: number }>("SELECT changes() AS changed")?.changed ?? 0);
    this.persist();
    return changed > 0;
  }

  markWatchRun(id: number): void {
    this.run("UPDATE watch_rules SET last_run_at = ? WHERE id = ?", [now(), id]);
    this.persist();
  }

  addWatchResultIfNew(watchId: number, listingId: string): boolean {
    this.run(
      "INSERT OR IGNORE INTO watch_results (watch_id, listing_id, seen_at, is_new) VALUES (?, ?, ?, 1)",
      [watchId, listingId, now()],
    );
    const changed = Number(this.get<{ changed: number }>("SELECT changes() AS changed")?.changed ?? 0);
    this.persist();
    return changed > 0;
  }

  createMessageDraft(listing: ParsedListing, messageText: string, template?: string): number {
    const timestamp = now();
    this.run(
      `INSERT INTO message_drafts
       (listing_id, listing_url, template, message_text, status, created_at, updated_at)
       VALUES (?, ?, ?, ?, 'draft', ?, ?)`,
      [listing.id, listing.url, template ?? null, messageText, timestamp, timestamp],
    );
    const id = Number(this.get<{ id: number }>("SELECT last_insert_rowid() AS id")?.id);
    this.persist();
    return id;
  }

  logSentMessage(listing: ParsedListing, messageText: string, confirmationMethod: string): number {
    this.run(
      `INSERT INTO sent_messages
       (listing_id, listing_url, message_text, confirmation_method, sent_at)
       VALUES (?, ?, ?, ?, ?)`,
      [listing.id, listing.url, messageText, confirmationMethod, now()],
    );
    const id = Number(this.get<{ id: number }>("SELECT last_insert_rowid() AS id")?.id);
    this.persist();
    return id;
  }

  countSentMessagesSince(sinceIso: string): number {
    return Number(
      this.get<{ count: number }>("SELECT COUNT(*) AS count FROM sent_messages WHERE sent_at >= ?", [sinceIso])?.count ?? 0,
    );
  }

  private migrate(): void {
    this.db.run(SCHEMA_SQL);
  }

  private run(sql: string, params: SqlParam[] = []): void {
    this.db.run(sql, params);
  }

  private get<T extends object>(sql: string, params: SqlParam[] = []): T | null {
    return this.all<T>(sql, params)[0] ?? null;
  }

  private all<T extends object>(sql: string, params: SqlParam[] = []): T[] {
    const statement = this.db.prepare(sql);
    try {
      statement.bind(params);
      const rows: T[] = [];
      while (statement.step()) {
        rows.push(statement.getAsObject() as T);
      }
      return rows;
    } finally {
      statement.free();
    }
  }

  private persist(): void {
    if (!this.filePath) {
      return;
    }
    ensureParentDir(this.filePath);
    const tmpPath = `${this.filePath}.${process.pid}.tmp`;
    fs.writeFileSync(tmpPath, Buffer.from(this.db.export()), { mode: 0o600 });
    fs.renameSync(tmpPath, this.filePath);
  }
}

type SqlParam = string | number | null | Uint8Array;

async function getSql(): Promise<SqlJsStatic> {
  const packageRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");
  sqlPromise ??= initSqlJs({
    locateFile: (file) => path.join(packageRoot, "node_modules", "sql.js", "dist", file),
  });
  return sqlPromise;
}

function now(): string {
  return new Date().toISOString();
}

function rowToListing(row: Record<string, unknown>): ParsedListing {
  const raw = safeParseObject(row.raw_json);
  return {
    ...raw,
    id: String(row.id),
    url: String(row.url),
    title: String(row.title ?? raw.title ?? "Untitled listing"),
    price: optionalString(row.price),
    location: optionalString(row.location),
    distance: optionalString(row.distance),
    posted_at: optionalString(row.posted_at),
    thumbnail_url: optionalString(row.thumbnail_url),
    seller_name: optionalString(row.seller_name),
    category: optionalString(row.category),
    snippet: optionalString(row.snippet),
  };
}

function optionalString(value: unknown): string | undefined {
  if (value === null || value === undefined || value === "") {
    return undefined;
  }
  return String(value);
}

function safeParseObject(value: unknown): Record<string, unknown> {
  if (typeof value !== "string") {
    return {};
  }
  try {
    const parsed = JSON.parse(value);
    return typeof parsed === "object" && parsed !== null && !Array.isArray(parsed) ? parsed : {};
  } catch {
    return {};
  }
}
