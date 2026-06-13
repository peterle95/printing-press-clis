import { Command } from "commander";
import {
  initConfig,
  loadConfig,
  renderConfig,
  resolvedConfigPath,
  saveConfig,
} from "../core/config.js";
import { printObject } from "../core/output.js";
import { loadCliConfig, parseNumberOption } from "./helpers.js";

export function registerConfigCommands(program: Command): void {
  const config = program.command("config").description("Manage local CLI configuration.");

  config
    .command("init")
    .description("Create the default config file.")
    .option("--force", "overwrite an existing config file")
    .action((options, command: Command) => {
      const root = command.optsWithGlobals<{ config?: string }>();
      initConfig(root.config, Boolean(options.force));
      console.log(`Wrote ${resolvedConfigPath(root.config)}`);
    });

  config
    .command("show")
    .description("Show the effective config.")
    .option("--json", "print JSON")
    .action((options, command: Command) => {
      const root = command.optsWithGlobals<{ config?: string }>();
      const value = loadConfig({ path: root.config });
      if (options.json) {
        printObject(value, { json: true });
        return;
      }
      console.log(renderConfig(value));
    });

  config
    .command("set-location")
    .description("Set the default Kleinanzeigen location and radius.")
    .requiredOption("--postal-code <postalCode>", "postal code")
    .requiredOption("--city <city>", "city name")
    .requiredOption("--radius-km <km>", "search radius in kilometers", parseNumberOption)
    .action((options, command: Command) => {
      const root = command.optsWithGlobals<{ config?: string }>();
      const value = loadCliConfig(command);
      value.location = {
        postal_code: String(options.postalCode),
        city: String(options.city),
        radius_km: Number(options.radiusKm),
      };
      saveConfig(value, root.config);
      console.log(`Updated location to ${value.location.postal_code} ${value.location.city}, ${value.location.radius_km} km.`);
    });

  config
    .command("set-browser-profile")
    .description("Set the Playwright persistent browser profile directory.")
    .argument("<profilePath>", "browser profile path")
    .action((profilePath: string, command: Command) => {
      const root = command.optsWithGlobals<{ config?: string }>();
      const value = loadCliConfig(command);
      value.browser_profile = profilePath;
      saveConfig(value, root.config);
      console.log(`Updated browser profile to ${profilePath}.`);
    });
}
