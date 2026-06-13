import readline from "node:readline/promises";
import { stdin as input, stdout as output } from "node:process";
import type { KleinanzeigenConfig } from "./config.js";

export interface DryRunOptions {
  dryRun?: boolean;
  live?: boolean;
}

export interface SendPreview {
  title: string;
  url: string;
  message: string;
}

export function resolveDryRun(config: KleinanzeigenConfig, options: DryRunOptions): boolean {
  if (options.dryRun) {
    return true;
  }
  if (options.live) {
    return false;
  }
  return config.safety.dry_run_default;
}

export function isExactSendConfirmation(value: string): boolean {
  return value === "SEND";
}

export function renderSendPreview(preview: SendPreview): string {
  return [
    "About to send this message to listing:",
    `Title: ${preview.title}`,
    `URL: ${preview.url}`,
    "Message:",
    preview.message,
    "",
    "Type SEND to confirm:",
  ].join("\n");
}

export async function confirmSend(preview: SendPreview): Promise<boolean> {
  const rl = readline.createInterface({ input, output });
  try {
    const answer = await rl.question(`${renderSendPreview(preview)} `);
    return isExactSendConfirmation(answer);
  } finally {
    rl.close();
  }
}

export function assertMessagingAllowed(config: KleinanzeigenConfig): void {
  if (config.safety.allow_bulk_messaging) {
    throw new Error("Refusing to run with allow_bulk_messaging=true. This CLI never supports bulk messaging.");
  }
  if (!config.safety.require_send_confirmation) {
    throw new Error("Refusing to send with require_send_confirmation=false. Exact SEND confirmation is mandatory.");
  }
}

export function randomDelayMs(min: number, max: number): number {
  const lower = Math.max(1000, Math.min(min, max));
  const upper = Math.max(lower, max);
  return Math.floor(lower + Math.random() * (upper - lower + 1));
}

export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
