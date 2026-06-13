import type { Command } from "commander";
import { loadConfig, type KleinanzeigenConfig } from "../core/config.js";
import { KleinanzeigenDb } from "../core/db.js";
import { resolveDatabasePath } from "../core/config.js";

export interface GlobalOptions {
  config?: string;
  agent?: boolean;
}

export function loadCliConfig(command: Command): KleinanzeigenConfig {
  const opts = command.optsWithGlobals<GlobalOptions>();
  return loadConfig({ path: opts.config, createIfMissing: true });
}

export async function openCliDb(config: KleinanzeigenConfig): Promise<KleinanzeigenDb> {
  return KleinanzeigenDb.open(resolveDatabasePath(config));
}

export function parseNumberOption(value: string): number {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    throw new Error(`Expected a number, got ${value}`);
  }
  return parsed;
}

export function clampMaxPages(value: number): number {
  return Math.max(1, Math.min(5, Math.trunc(value)));
}

export function isAgentMode(command: Command): boolean {
  return Boolean(command.optsWithGlobals<GlobalOptions>().agent);
}

export function assertBrowserAcknowledged(options: { browserOk?: string }): void {
  if (options.browserOk !== "USER_REQUESTED_BROWSER") {
    throw new Error(
      "Refusing to open a browser. Browser-backed search/watch requires an explicit user request and --browser-ok USER_REQUESTED_BROWSER.",
    );
  }
}

export function handleError(error: unknown): never {
  const message = error instanceof Error ? error.message : String(error);
  console.error(`Error: ${message}`);
  process.exit(1);
}
