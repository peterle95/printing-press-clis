import * as cheerio from "cheerio";
import type { AnyNode } from "domhandler";
import { isListingUrl, listingIdFromUrl, normalizeListingUrl } from "./kleinanzeigenUrls.js";

export interface ParsedListing {
  id: string;
  title: string;
  price?: string;
  location?: string;
  distance?: string;
  posted_at?: string;
  url: string;
  thumbnail_url?: string;
  seller_name?: string;
  category?: string;
  snippet?: string;
  raw_text?: string;
}

export function parseSearchResults(html: string, pageUrl = "https://www.kleinanzeigen.de/"): ParsedListing[] {
  const $ = cheerio.load(html);
  const byId = new Map<string, ParsedListing>();

  $("a[href*='/s-anzeige/'], a[href*=\"/s-anzeige/\"]").each((_, element) => {
    const href = $(element).attr("href");
    if (!href) {
      return;
    }

    const url = normalizeListingUrl(href, pageUrl);
    if (!url || !isListingUrl(url)) {
      return;
    }

    const id = listingIdFromUrl(url);
    if (!id || byId.has(id)) {
      return;
    }

    const card = bestListingCard($, element);
    if (isHidden(card)) {
      return;
    }

    const rawText = cleanText(card.text());
    const title = firstText($, card, [
      "[data-testid*='title']",
      ".aditem-main--middle--title",
      ".ellipsis",
      "h2",
      "h3",
      "a[href*='/s-anzeige/']",
    ]) || cleanText($(element).text()) || "Untitled listing";

    const price = firstText($, card, [
      "[data-testid*='price']",
      ".aditem-main--middle--price-shipping--price",
      "[class*='price']",
    ]) || extractPrice(rawText);

    const location = firstText($, card, [
      "[data-testid*='location']",
      ".aditem-main--top--left",
      ".aditem-main--bottom--left",
      "[class*='location']",
    ]) || extractLocation(rawText);

    const postedAt = firstText($, card, [
      "time",
      "[datetime]",
      ".aditem-main--top--right",
      "[class*='date']",
      "[class*='time']",
    ]) || extractPostedAt(rawText);

    const listing: ParsedListing = {
      id,
      title,
      url,
      price,
      location,
      distance: extractDistance(rawText),
      posted_at: postedAt,
      thumbnail_url: firstImage($, card, pageUrl),
      seller_name: firstText($, card, ["[data-testid*='seller']", "[class*='seller']", "[class*='user']"]),
      category: firstText($, card, ["[data-testid*='category']", "[class*='category']"]),
      snippet: firstText($, card, [".aditem-main--middle--description", "[data-testid*='description']", "p"]),
      raw_text: rawText,
    };

    byId.set(id, stripEmpty(listing));
  });

  return Array.from(byId.values());
}

export function detectAccessChallenge(html: string): string | null {
  const text = cleanText(cheerio.load(html).text()).toLowerCase();
  const challengePatterns = [
    "captcha",
    "sicherheitsprüfung",
    "ungewöhnliche aktivitäten",
    "unusual traffic",
    "zugriff verweigert",
    "access denied",
    "bot",
  ];
  return challengePatterns.find((pattern) => text.includes(pattern)) ?? null;
}

function bestListingCard($: cheerio.CheerioAPI, element: AnyNode): cheerio.Cheerio<AnyNode> {
  const anchor = $(element);
  const selectors = [
    "article",
    "li",
    "[data-adid]",
    "[data-testid*='ad']",
    ".aditem",
    ".ad-listitem",
    "div",
  ];
  for (const selector of selectors) {
    const candidate = anchor.closest(selector);
    if (candidate.length > 0) {
      return candidate;
    }
  }
  return anchor.parent();
}

function firstText(
  $: cheerio.CheerioAPI,
  root: cheerio.Cheerio<AnyNode>,
  selectors: string[],
): string | undefined {
  for (const selector of selectors) {
    const value = cleanText(root.find(selector).first().text());
    if (value) {
      return value;
    }
    const attrValue = cleanText(root.find(selector).first().attr("aria-label") ?? "");
    if (attrValue) {
      return attrValue;
    }
  }
  return undefined;
}

function firstImage($: cheerio.CheerioAPI, root: cheerio.Cheerio<AnyNode>, pageUrl: string): string | undefined {
  const image = root.find("img").first();
  const raw = image.attr("src") ?? image.attr("data-src") ?? image.attr("data-lazy-src");
  if (!raw) {
    return undefined;
  }
  try {
    return new URL(raw, pageUrl).toString();
  } catch {
    return undefined;
  }
}

function extractPrice(text: string): string | undefined {
  return text.match(/(?:VB\s*)?(?:\d{1,3}(?:[.,]\d{3})*|\d+)(?:[.,]\d{2})?\s*€/u)?.[0];
}

function extractDistance(text: string): string | undefined {
  return text.match(/\b\d+(?:[.,]\d+)?\s*km\b/iu)?.[0];
}

function extractLocation(text: string): string | undefined {
  return text.match(/\b\d{5}\s+[A-Za-zÄÖÜäöüß .-]+/u)?.[0];
}

function extractPostedAt(text: string): string | undefined {
  return text.match(/\b(?:Heute|Gestern|\d{1,2}\.\d{1,2}\.\d{2,4})(?:,\s*\d{1,2}:\d{2})?\b/u)?.[0];
}

function cleanText(text: string): string {
  return text.replace(/\s+/g, " ").trim();
}

function isHidden(root: cheerio.Cheerio<AnyNode>): boolean {
  const style = root.attr("style") ?? "";
  const ariaHidden = root.attr("aria-hidden") ?? "";
  return /display\s*:\s*none|visibility\s*:\s*hidden/i.test(style) || ariaHidden === "true";
}

function stripEmpty<T extends object>(value: T): T {
  return Object.fromEntries(
    Object.entries(value as Record<string, unknown>).filter(([, item]) => item !== undefined && item !== ""),
  ) as T;
}
