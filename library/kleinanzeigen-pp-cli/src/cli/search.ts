import { Command } from "commander";
import { buildSearchUrl, type SearchUrlOptions } from "../core/kleinanzeigenUrls.js";
import { parseSearchResults, type ParsedListing } from "../core/parser.js";
import { printListings, printObject } from "../core/output.js";
import { randomDelayMs, sleep } from "../core/safety.js";
import {
  assertBrowserAcknowledged,
  clampMaxPages,
  isAgentMode,
  loadCliConfig,
  openCliDb,
  parseNumberOption,
} from "./helpers.js";
import type { KleinanzeigenConfig, SortOption } from "../core/config.js";
import type { KleinanzeigenDb } from "../core/db.js";

export interface SearchCommandOptions {
  radiusKm?: number;
  maxPrice?: number;
  minPrice?: number;
  sort?: SortOption;
  maxPages?: number;
  json?: boolean;
  markdown?: boolean;
  dryRun?: boolean;
  openBrowser?: boolean;
  browserOk?: string;
}

export function registerSearchCommand(program: Command): void {
  program
    .command("search")
    .description("Build a Kleinanzeigen search URL; optionally open a visible browser to cache listings.")
    .argument("<query>", "search query")
    .option("--radius-km <km>", "override configured radius", parseNumberOption)
    .option("--max-price <eur>", "maximum price", parseNumberOption)
    .option("--min-price <eur>", "minimum price", parseNumberOption)
    .option("--sort <sort>", "distance, date, price, price_asc, price_desc, or relevance")
    .option("--max-pages <pages>", "maximum pages to inspect, capped at 5", parseNumberOption)
    .option("--json", "print JSON")
    .option("--markdown", "print Markdown")
    .option("--open-browser", "open a visible browser and cache visible listings only when explicitly requested")
    .option("--browser-ok <ack>", "required with --open-browser; pass USER_REQUESTED_BROWSER")
    .option("--dry-run", "print the search URL without opening Kleinanzeigen")
    .action(async (query: string, options: SearchCommandOptions, command: Command) => {
      const config = loadCliConfig(command);
      const searchOptions = buildOptions(config, query, options);
      const firstUrl = buildSearchUrl({ ...searchOptions, page: 1 });
      const agent = isAgentMode(command);

      if (options.dryRun || !options.openBrowser) {
        printSearchPlan(query, firstUrl, Boolean(options.openBrowser), {
          json: Boolean(options.json || agent),
          markdown: Boolean(options.markdown),
          compact: agent,
        });
        return;
      }

      assertBrowserAcknowledged(options);
      const db = await openCliDb(config);
      try {
        const listings = await runSearch(config, db, query, searchOptions, options.maxPages);
        printListings(listings, { ...options, json: Boolean(options.json || agent), compact: agent });
      } finally {
        db.close();
      }
    });
}

function printSearchPlan(
  query: string,
  searchUrl: string,
  openBrowserRequested: boolean,
  options: { json?: boolean; markdown?: boolean; compact?: boolean },
): void {
  if (options.json) {
    printObject(
      {
        query,
        search_url: searchUrl,
        browser_opened: false,
        cached_results: false,
        next_step: openBrowserRequested
          ? "Remove --dry-run and include --browser-ok USER_REQUESTED_BROWSER only if the user explicitly asked for browser use."
          : "Use the URL manually. Add --open-browser --browser-ok USER_REQUESTED_BROWSER only after an explicit browser request.",
      },
      { json: true, compact: options.compact },
    );
    return;
  }
  if (options.markdown) {
    console.log(`[${query}](${searchUrl})`);
    return;
  }
  console.log(searchUrl);
}

export function buildOptions(
  config: KleinanzeigenConfig,
  query: string,
  options: SearchCommandOptions,
): SearchUrlOptions {
  return {
    query,
    postalCode: config.location.postal_code,
    city: config.location.city,
    radiusKm: options.radiusKm ?? config.location.radius_km,
    sort: options.sort ?? config.search.default_sort,
    maxPrice: options.maxPrice,
    minPrice: options.minPrice,
  };
}

export async function runSearch(
  config: KleinanzeigenConfig,
  db: KleinanzeigenDb,
  query: string,
  searchOptions: SearchUrlOptions,
  requestedMaxPages?: number,
): Promise<ParsedListing[]> {
  const maxPages = clampMaxPages(requestedMaxPages ?? config.search.max_pages);
  const firstUrl = buildSearchUrl({ ...searchOptions, page: 1 });
  const searchId = db.recordSearch(query, { ...searchOptions, maxPages }, firstUrl);
  const byId = new Map<string, ParsedListing>();

  const { navigateHumanVisible, openVisibleBrowser } = await import("../core/browser.js");
  const session = await openVisibleBrowser(config);
  try {
    for (let pageNumber = 1; pageNumber <= maxPages; pageNumber += 1) {
      const url = buildSearchUrl({ ...searchOptions, page: pageNumber });
      await navigateHumanVisible(session.page, url);
      const html = await session.page.content();
      const listings = parseSearchResults(html, session.page.url());
      for (const listing of listings) {
        byId.set(listing.id, listing);
        db.upsertListing(listing, searchId);
      }

      if (pageNumber < maxPages) {
        await sleep(randomDelayMs(config.search.min_delay_ms, config.search.max_delay_ms));
      }
    }
  } finally {
    await session.close();
  }

  return Array.from(byId.values());
}
