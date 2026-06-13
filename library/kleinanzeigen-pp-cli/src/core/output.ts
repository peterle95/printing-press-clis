import type { ParsedListing } from "./parser.js";

export type OutputMode = "table" | "json" | "markdown";

export interface OutputOptions {
  json?: boolean;
  markdown?: boolean;
  compact?: boolean;
}

export function outputMode(options: OutputOptions): OutputMode {
  if (options.json) {
    return "json";
  }
  if (options.markdown) {
    return "markdown";
  }
  return "table";
}

export function printListings(listings: ParsedListing[], options: OutputOptions = {}): void {
  const mode = outputMode(options);
  if (mode === "json") {
    printJson(listings, options.compact);
    return;
  }
  if (mode === "markdown") {
    printMarkdownListings(listings);
    return;
  }
  printTable(
    listings.map((listing) => ({
      id: listing.id,
      title: listing.title,
      price: listing.price ?? "",
      location: listing.location ?? "",
      distance: listing.distance ?? "",
      url: listing.url,
    })),
  );
}

export function printObject(value: unknown, options: OutputOptions = {}): void {
  if (options.json) {
    printJson(value, options.compact);
    return;
  }
  if (typeof value === "string") {
    console.log(value);
    return;
  }
  printTable(Array.isArray(value) ? value : [value as Record<string, unknown>]);
}

export function printTable(rows: Record<string, unknown>[]): void {
  if (rows.length === 0) {
    console.log("No results.");
    return;
  }
  const columns = Object.keys(rows[0] ?? {});
  const widths = columns.map((column) =>
    Math.min(
      60,
      Math.max(
        column.length,
        ...rows.map((row) => printable(row[column]).length),
      ),
    ),
  );
  const formatRow = (row: Record<string, unknown>) =>
    columns.map((column, index) => truncate(printable(row[column]), widths[index]).padEnd(widths[index])).join("  ");

  console.log(formatRow(Object.fromEntries(columns.map((column) => [column, column]))));
  console.log(widths.map((width) => "-".repeat(width)).join("  "));
  for (const row of rows) {
    console.log(formatRow(row));
  }
}

function printMarkdownListings(listings: ParsedListing[]): void {
  if (listings.length === 0) {
    console.log("No results.");
    return;
  }
  for (const listing of listings) {
    console.log(`- [${escapeMarkdown(listing.title)}](${listing.url})`);
    const details = [listing.price, listing.location, listing.distance, listing.posted_at].filter(Boolean).join(" | ");
    if (details) {
      console.log(`  ${details}`);
    }
  }
}

function printable(value: unknown): string {
  if (value === null || value === undefined) {
    return "";
  }
  return String(value).replace(/\s+/g, " ").trim();
}

function truncate(value: string, maxLength: number): string {
  if (value.length <= maxLength) {
    return value;
  }
  return `${value.slice(0, Math.max(0, maxLength - 3))}...`;
}

function escapeMarkdown(value: string): string {
  return value.replace(/([\\[\]()])/g, "\\$1");
}

function printJson(value: unknown, compact = false): void {
  console.log(compact ? JSON.stringify(value) : JSON.stringify(value, null, 2));
}
