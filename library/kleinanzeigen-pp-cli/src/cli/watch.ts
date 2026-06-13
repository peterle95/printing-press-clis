import { Command } from "commander";
import { buildSearchUrl } from "../core/kleinanzeigenUrls.js";
import { printListings, printObject, printTable } from "../core/output.js";
import { sleep, randomDelayMs } from "../core/safety.js";
import { buildOptions, runSearch } from "./search.js";
import { assertBrowserAcknowledged, isAgentMode, loadCliConfig, openCliDb, parseNumberOption } from "./helpers.js";
import type { ParsedListing } from "../core/parser.js";
import type { SortOption } from "../core/config.js";

export function registerWatchCommands(program: Command): void {
  const watch = program.command("watch").description("Manage manual saved search watches.");

  watch
    .command("add")
    .description("Add a manual watch rule.")
    .argument("<query>", "search query")
    .option("--radius-km <km>", "override configured radius", parseNumberOption)
    .option("--max-price <eur>", "maximum price", parseNumberOption)
    .option("--sort <sort>", "distance, date, price, price_asc, price_desc, or relevance")
    .action(async (query: string, options: { radiusKm?: number; maxPrice?: number; sort?: SortOption }, command: Command) => {
      const db = await openCliDb(loadCliConfig(command));
      try {
        const id = db.addWatchRule(query, options);
        console.log(`Added watch ${id}.`);
      } finally {
        db.close();
      }
    });

  watch
    .command("list")
    .description("List configured watch rules.")
    .option("--json", "print JSON")
    .action(async (options, command: Command) => {
      const db = await openCliDb(loadCliConfig(command));
      try {
        const rows = db.listWatchRules(false);
        if (options.json) {
          printObject(rows, { json: true });
          return;
        }
        printTable(rows.map((row) => ({
          id: row.id,
          active: row.active,
          query: row.query,
          radius_km: row.radius_km ?? "",
          max_price: row.max_price ?? "",
          sort: row.sort ?? "",
          last_run_at: row.last_run_at ?? "",
        })));
      } finally {
        db.close();
      }
    });

  watch
    .command("run")
    .description("Print active watch URLs; optionally open a visible browser and print new listings.")
    .option("--json", "print JSON")
    .option("--markdown", "print Markdown")
    .option("--open-browser", "open a visible browser and scan each watch once only when explicitly requested")
    .option("--browser-ok <ack>", "required with --open-browser; pass USER_REQUESTED_BROWSER")
    .option("--dry-run", "show active rules without opening Kleinanzeigen")
    .action(async (options, command: Command) => {
      const config = loadCliConfig(command);
      const agent = isAgentMode(command);
      const db = await openCliDb(config);
      try {
        const rules = db.listWatchRules(true);
        const plannedRuns = rules.map((rule) => {
          const ruleOptions = JSON.parse(rule.options_json) as { radiusKm?: number; maxPrice?: number; sort?: SortOption };
          const searchOptions = buildOptions(config, rule.query, {
            radiusKm: ruleOptions.radiusKm,
            maxPrice: ruleOptions.maxPrice,
            sort: ruleOptions.sort,
          });
          return {
            id: rule.id,
            query: rule.query,
            search_url: buildSearchUrl({ ...searchOptions, page: 1 }),
            last_run_at: rule.last_run_at ?? null,
          };
        });

        if (options.dryRun || !options.openBrowser) {
          if (options.json || agent) {
            printObject(plannedRuns, { json: true, compact: agent });
            return;
          }
          printTable(plannedRuns.map((run) => ({
            id: run.id,
            query: run.query,
            url: run.search_url,
            last_run_at: run.last_run_at ?? "",
          })));
          return;
        }

        assertBrowserAcknowledged(options);
        const newListings: ParsedListing[] = [];
        for (const rule of rules) {
          const ruleOptions = JSON.parse(rule.options_json) as { radiusKm?: number; maxPrice?: number; sort?: SortOption };
          const searchOptions = buildOptions(config, rule.query, {
            radiusKm: ruleOptions.radiusKm,
            maxPrice: ruleOptions.maxPrice,
            sort: ruleOptions.sort,
          });
          const listings = await runSearch(config, db, rule.query, searchOptions, config.search.max_pages);
          for (const listing of listings) {
            if (db.addWatchResultIfNew(rule.id, listing.id)) {
              newListings.push(listing);
            }
          }
          db.markWatchRun(rule.id);
          await sleep(randomDelayMs(config.search.min_delay_ms, config.search.max_delay_ms));
        }

        printListings(newListings, { ...options, json: Boolean(options.json || agent), compact: agent });
      } finally {
        db.close();
      }
    });

  watch
    .command("remove")
    .description("Deactivate a watch rule.")
    .argument("<watchId>", "watch id", parseNumberOption)
    .action(async (watchId: number, command: Command) => {
      const db = await openCliDb(loadCliConfig(command));
      try {
        const removed = db.removeWatchRule(watchId);
        console.log(removed ? `Removed watch ${watchId}.` : `No active watch found for ${watchId}.`);
      } finally {
        db.close();
      }
    });
}
