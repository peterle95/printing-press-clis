import type { SortOption } from "./config.js";

export const KLEINANZEIGEN_BASE_URL = "https://www.kleinanzeigen.de";

export interface SearchUrlOptions {
  query: string;
  postalCode: string;
  city: string;
  radiusKm: number;
  sort?: SortOption;
  maxPrice?: number;
  minPrice?: number;
  page?: number;
}

const SORTING_FIELD: Record<SortOption, string> = {
  distance: "DISTANCE",
  date: "POSTING_DATE",
  price: "PRICE_ASCENDING",
  price_asc: "PRICE_ASCENDING",
  price_desc: "PRICE_DESCENDING",
  relevance: "RELEVANCE",
};

export function buildSearchUrl(options: SearchUrlOptions): string {
  const url = new URL("/s-suchanfrage.html", KLEINANZEIGEN_BASE_URL);
  url.searchParams.set("keywords", options.query.trim());
  url.searchParams.set("locationStr", `${options.postalCode} ${options.city}`.trim());
  url.searchParams.set("radius", String(options.radiusKm));
  url.searchParams.set("sortingField", SORTING_FIELD[options.sort ?? "distance"]);

  if (options.maxPrice !== undefined) {
    url.searchParams.set("maxPrice", String(options.maxPrice));
  }
  if (options.minPrice !== undefined) {
    url.searchParams.set("minPrice", String(options.minPrice));
  }
  if (options.page !== undefined && options.page > 1) {
    url.searchParams.set("pageNum", String(options.page));
  }
  return url.toString();
}

export function normalizeListingUrl(rawUrl: string, baseUrl = KLEINANZEIGEN_BASE_URL): string | null {
  if (!rawUrl.trim()) {
    return null;
  }
  try {
    const url = new URL(rawUrl, baseUrl);
    if (url.hostname !== "www.kleinanzeigen.de" && url.hostname !== "kleinanzeigen.de") {
      return null;
    }
    url.hash = "";
    return url.toString();
  } catch {
    return null;
  }
}

export function isListingUrl(url: string): boolean {
  return /\/s-anzeige\//.test(url);
}

export function listingIdFromUrl(url: string): string | null {
  const normalized = normalizeListingUrl(url);
  if (!normalized || !isListingUrl(normalized)) {
    return null;
  }

  const path = new URL(normalized).pathname;
  const idMatch = path.match(/\/(\d{6,})(?:-[^/]*)?$/) ?? path.match(/(\d{6,})/g);
  if (Array.isArray(idMatch)) {
    return idMatch[idMatch.length - 1] ?? null;
  }
  return null;
}

export function listingOpenUrl(idOrUrl: string, knownUrl?: string): string {
  if (knownUrl) {
    const normalized = normalizeListingUrl(knownUrl);
    if (normalized) {
      return normalized;
    }
  }
  const normalized = normalizeListingUrl(idOrUrl);
  if (normalized) {
    return normalized;
  }
  throw new Error(`No cached listing URL found for ${idOrUrl}. Run a search first or pass a full listing URL.`);
}
