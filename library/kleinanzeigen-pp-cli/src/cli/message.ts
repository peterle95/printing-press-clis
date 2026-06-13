import { Command } from "commander";
import { loadConfig, saveConfig } from "../core/config.js";
import { printObject, printTable } from "../core/output.js";
import {
  assertMessagingAllowed,
  confirmSend,
  renderSendPreview,
  resolveDryRun,
} from "../core/safety.js";
import { DEFAULT_MESSAGE_TEMPLATES, renderMessageTemplate } from "../templates/messages.js";
import { loadCliConfig, openCliDb, parseNumberOption } from "./helpers.js";
import type { ParsedListing } from "../core/parser.js";

export function registerMessageCommands(program: Command): void {
  const message = program.command("message").description("Draft and send Kleinanzeigen messages with explicit confirmation.");

  message
    .command("draft")
    .description("Create a local message draft for a cached listing.")
    .argument("<listingIdOrUrl>", "cached listing id or full URL")
    .option("--template <name>", "template name")
    .option("--text <text>", "message text")
    .option("--offer-price <eur>", "offer price for templates that use {offer_price}", parseNumberOption)
    .option("--json", "print JSON")
    .action(async (listingIdOrUrl: string, options, command: Command) => {
      const config = loadCliConfig(command);
      const db = await openCliDb(config);
      try {
        const listing = requireListing(db.getListing(listingIdOrUrl), listingIdOrUrl);
        const rendered = resolveMessageText(config.message_templates, options);
        const draftId = db.createMessageDraft(listing, rendered, options.template);
        const draft = {
          id: draftId,
          listing_id: listing.id,
          listing_url: listing.url,
          template: options.template ?? "",
          message_text: rendered,
        };
        if (options.json) {
          printObject(draft, { json: true });
          return;
        }
        console.log(`Draft ${draftId}`);
        console.log(renderSendPreview({ title: listing.title, url: listing.url, message: rendered }));
        console.log("Draft only. Nothing was sent.");
      } finally {
        db.close();
      }
    });

  const templates = message.command("templates").description("Manage local message templates.");

  templates
    .command("list")
    .description("List default and configured message templates.")
    .option("--json", "print JSON")
    .action((options, command: Command) => {
      const config = loadCliConfig(command);
      const values = { ...DEFAULT_MESSAGE_TEMPLATES, ...config.message_templates };
      if (options.json) {
        printObject(values, { json: true });
        return;
      }
      printTable(Object.entries(values).map(([name, text]) => ({ name, text })));
    });

  templates
    .command("add")
    .description("Add or override a local message template.")
    .argument("<name>", "template name")
    .option("--text <text>", "template text")
    .action((name: string, options, command: Command) => {
      const root = command.optsWithGlobals<{ config?: string }>();
      const config = loadConfig({ path: root.config, createIfMissing: true });
      const text = options.text ?? DEFAULT_MESSAGE_TEMPLATES[name];
      if (!text) {
        throw new Error("Use --text for custom template names.");
      }
      config.message_templates[name] = text;
      saveConfig(config, root.config);
      console.log(`Saved template ${name}.`);
    });

  message
    .command("send")
    .description("Fill and send a message only after exact SEND confirmation.")
    .argument("<listingIdOrUrl>", "cached listing id or full URL")
    .option("--template <name>", "template name")
    .option("--text <text>", "message text")
    .option("--offer-price <eur>", "offer price for templates that use {offer_price}", parseNumberOption)
    .option("--dry-run", "preview only; never click send")
    .option("--live", "allow sending after exact SEND confirmation")
    .action(async (listingIdOrUrl: string, options, command: Command) => {
      const config = loadCliConfig(command);
      assertMessagingAllowed(config);
      const db = await openCliDb(config);
      try {
        const listing = requireListing(db.getListing(listingIdOrUrl), listingIdOrUrl);
        const rendered = resolveMessageText(config.message_templates, options);
        db.createMessageDraft(listing, rendered, options.template);

        const preview = { title: listing.title, url: listing.url, message: rendered };
        const dryRun = resolveDryRun(config, options);
        if (dryRun) {
          console.log(renderSendPreview(preview));
          console.log("Dry run: not opening the sender and not clicking send. Use --live to permit a confirmed send.");
          return;
        }

        const { navigateHumanVisible, openVisibleBrowser, fillMessageBox, clickSendButton } = await import("../core/browser.js");
        const session = await openVisibleBrowser(config);
        try {
          await navigateHumanVisible(session.page, listing.url);
          await fillMessageBox(session.page, rendered);
          const confirmed = await confirmSend(preview);
          if (!confirmed) {
            console.log("Cancelled. No message was sent.");
            return;
          }
          await clickSendButton(session.page);
          const sentId = db.logSentMessage(listing, rendered, "terminal SEND");
          console.log(`Clicked send and logged sent message ${sentId}.`);
        } finally {
          await session.close();
        }
      } finally {
        db.close();
      }
    });
}

function resolveMessageText(
  configuredTemplates: Record<string, string>,
  options: { text?: string; template?: string; offerPrice?: number },
): string {
  if (options.text) {
    return options.text;
  }
  const templateName = options.template ?? "availability";
  const template = configuredTemplates[templateName] ?? DEFAULT_MESSAGE_TEMPLATES[templateName];
  if (!template) {
    throw new Error(`Unknown template ${templateName}. Use "message templates list" or pass --text.`);
  }
  return renderMessageTemplate(template, { offer_price: options.offerPrice });
}

function requireListing(listing: ParsedListing | null, idOrUrl: string): ParsedListing {
  if (!listing) {
    throw new Error(`No cached listing found for ${idOrUrl}. Run a search first.`);
  }
  return listing;
}
