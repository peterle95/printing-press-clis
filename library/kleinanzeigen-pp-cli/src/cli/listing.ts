import { Command } from "commander";
import { listingOpenUrl } from "../core/kleinanzeigenUrls.js";
import { printListings, printObject } from "../core/output.js";
import { loadCliConfig, openCliDb } from "./helpers.js";

export function registerListingCommands(program: Command): void {
  const listing = program.command("listing").description("Work with cached listings.");

  listing
    .command("open")
    .description("Open a cached listing in the visible browser.")
    .argument("<listingIdOrUrl>", "cached listing id or full URL")
    .option("--dry-run", "print the URL without opening the browser")
    .action(async (listingIdOrUrl: string, options, command: Command) => {
      const config = loadCliConfig(command);
      const db = await openCliDb(config);
      try {
        const cached = db.getListing(listingIdOrUrl);
        const url = listingOpenUrl(listingIdOrUrl, cached?.url);
        if (options.dryRun) {
          console.log(url);
          return;
        }
        const { navigateHumanVisible, openVisibleBrowser, waitForEnter } = await import("../core/browser.js");
        const session = await openVisibleBrowser(config);
        try {
          await navigateHumanVisible(session.page, url);
          await waitForEnter("Listing opened. Press Enter to close the browser session.");
        } finally {
          await session.close();
        }
      } finally {
        db.close();
      }
    });

  listing
    .command("show")
    .description("Show cached listing information.")
    .argument("<listingIdOrUrl>", "cached listing id or full URL")
    .option("--json", "print JSON")
    .option("--markdown", "print Markdown")
    .action(async (listingIdOrUrl: string, options, command: Command) => {
      const db = await openCliDb(loadCliConfig(command));
      try {
        const cached = db.getListing(listingIdOrUrl);
        if (!cached) {
          throw new Error(`No cached listing found for ${listingIdOrUrl}. Run a search first.`);
        }
        if (options.markdown || !options.json) {
          printListings([cached], options);
        } else {
          printObject(cached, { json: true });
        }
      } finally {
        db.close();
      }
    });

  listing
    .command("notes")
    .description("Add a local note for a cached listing.")
    .argument("<listingIdOrUrl>", "cached listing id or full URL")
    .argument("<note>", "note text")
    .action(async (listingIdOrUrl: string, note: string, command: Command) => {
      const db = await openCliDb(loadCliConfig(command));
      try {
        const cached = db.getListing(listingIdOrUrl);
        if (!cached) {
          throw new Error(`No cached listing found for ${listingIdOrUrl}. Run a search first.`);
        }
        const noteId = db.addNote(cached.id, note);
        console.log(`Saved note ${noteId} for ${cached.id}.`);
      } finally {
        db.close();
      }
    });
}
