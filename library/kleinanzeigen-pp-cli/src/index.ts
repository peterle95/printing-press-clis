#!/usr/bin/env node
import { Command } from "commander";
import { registerAuthCommands } from "./cli/auth.js";
import { registerConfigCommands } from "./cli/config.js";
import { handleError } from "./cli/helpers.js";
import { registerListingCommands } from "./cli/listing.js";
import { registerMessageCommands } from "./cli/message.js";
import { registerSearchCommand } from "./cli/search.js";
import { registerWatchCommands } from "./cli/watch.js";

const program = new Command();

program
  .name("kleinanzeigen-pp-cli")
  .description("Personal, lightweight Kleinanzeigen CLI with conservative safety defaults.")
  .version("0.1.0")
  .option("--config <path>", "config file path")
  .option("--agent", "compact JSON-friendly output and no browser unless explicitly requested")
  .showHelpAfterError()
  .addHelpText(
    "after",
    `
Safety: search and watch are CLI-only by default. Browser use requires an explicit browser command or acknowledgement.
This tool never stores passwords, does not bypass CAPTCHA or bot checks, does not support bulk messaging,
and only sends after exact terminal confirmation.
`,
  );

registerConfigCommands(program);
registerSearchCommand(program);
registerListingCommands(program);
registerWatchCommands(program);
registerAuthCommands(program);
registerMessageCommands(program);

program.parseAsync(process.argv).catch(handleError);
