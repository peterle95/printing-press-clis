import fs from "node:fs";
import readline from "node:readline/promises";
import { stdin as input, stdout as output } from "node:process";
import { Command } from "commander";
import { resolveBrowserProfile } from "../core/config.js";
import { loadCliConfig } from "./helpers.js";

const LOGIN_URL = "https://www.kleinanzeigen.de/m-einloggen.html";

export function registerAuthCommands(program: Command): void {
  const auth = program.command("auth").description("Manage the local browser session.");

  auth
    .command("login")
    .description("Open a visible browser for manual Kleinanzeigen login.")
    .option("--dry-run", "print the login URL without opening the browser")
    .action(async (options, command: Command) => {
      const config = loadCliConfig(command);
      if (options.dryRun) {
        console.log(LOGIN_URL);
        return;
      }
      const { navigateHumanVisible, openVisibleBrowser, waitForEnter } = await import("../core/browser.js");
      const session = await openVisibleBrowser(config);
      try {
        await navigateHumanVisible(session.page, LOGIN_URL);
        console.log("Log in manually in the browser. The CLI will not ask for or store your password.");
        await waitForEnter("After login/2FA/CAPTCHA is complete, press Enter to close the browser session.");
      } finally {
        await session.close();
      }
    });

  auth
    .command("status")
    .description("Show local browser profile status.")
    .action((command: Command) => {
      const config = loadCliConfig(command);
      const profilePath = resolveBrowserProfile(config);
      console.log(fs.existsSync(profilePath) ? `Browser profile exists: ${profilePath}` : `No browser profile at ${profilePath}`);
      console.log("This is a local profile check only; no password or session token is displayed.");
    });

  auth
    .command("logout")
    .description("Delete the local persistent browser profile.")
    .option("--dry-run", "show what would be removed")
    .action(async (options, command: Command) => {
      const config = loadCliConfig(command);
      const profilePath = resolveBrowserProfile(config);
      if (options.dryRun) {
        console.log(`Would remove ${profilePath}`);
        return;
      }

      const rl = readline.createInterface({ input, output });
      try {
        const answer = await rl.question(`Delete local browser profile ${profilePath}? Type LOGOUT to confirm: `);
        if (answer !== "LOGOUT") {
          console.log("Cancelled.");
          return;
        }
      } finally {
        rl.close();
      }

      const { removeBrowserProfile } = await import("../core/browser.js");
      await removeBrowserProfile(config);
      console.log("Local browser profile removed.");
    });
}
