import fs from "node:fs";
import readline from "node:readline/promises";
import { stdin as input, stdout as output } from "node:process";
import { chromium, type BrowserContext, type Page } from "playwright";
import type { KleinanzeigenConfig } from "./config.js";
import { resolveBrowserProfile } from "./config.js";
import { ensureDir } from "./paths.js";
import { detectAccessChallenge } from "./parser.js";

export interface BrowserSession {
  context: BrowserContext;
  page: Page;
  close: () => Promise<void>;
}

export async function openVisibleBrowser(config: KleinanzeigenConfig, url?: string): Promise<BrowserSession> {
  const profilePath = resolveBrowserProfile(config);
  ensureDir(profilePath);

  const context = await chromium.launchPersistentContext(profilePath, {
    headless: false,
    viewport: { width: 1280, height: 900 },
    locale: "de-DE",
  });
  const page = context.pages()[0] ?? (await context.newPage());
  if (url) {
    await page.goto(url, { waitUntil: "domcontentloaded", timeout: 60000 });
  }
  return {
    context,
    page,
    close: async () => {
      await context.close();
    },
  };
}

export async function navigateHumanVisible(page: Page, url: string): Promise<void> {
  await page.goto(url, { waitUntil: "domcontentloaded", timeout: 60000 });
  await page.waitForLoadState("networkidle", { timeout: 15000 }).catch(() => undefined);
  const html = await page.content();
  const challenge = detectAccessChallenge(html);
  if (challenge) {
    throw new Error(
      `Kleinanzeigen appears to be showing a block or challenge (${challenge}). Handle it manually in the browser; this CLI will not bypass it.`,
    );
  }
}

export async function waitForEnter(prompt: string): Promise<void> {
  const rl = readline.createInterface({ input, output });
  try {
    await rl.question(prompt);
  } finally {
    rl.close();
  }
}

export async function removeBrowserProfile(config: KleinanzeigenConfig): Promise<void> {
  const profilePath = resolveBrowserProfile(config);
  if (fs.existsSync(profilePath)) {
    fs.rmSync(profilePath, { recursive: true, force: true });
  }
}

export function browserProfileExists(config: KleinanzeigenConfig): boolean {
  return fs.existsSync(resolveBrowserProfile(config));
}

export async function fillMessageBox(page: Page, message: string): Promise<void> {
  await maybeOpenMessageComposer(page);

  const selectors = [
    "textarea[name='message']",
    "textarea[name*='message']",
    "textarea",
    "[contenteditable='true']",
    "[role='textbox']",
  ];

  for (const selector of selectors) {
    const candidate = page.locator(selector).first();
    if ((await candidate.count()) > 0 && (await candidate.isVisible().catch(() => false))) {
      await candidate.fill(message);
      return;
    }
  }

  throw new Error(
    "Could not find a visible message box. If Kleinanzeigen changed the page or shows a challenge, handle it manually and retry.",
  );
}

export async function clickSendButton(page: Page): Promise<void> {
  const roleButton = page.getByRole("button", { name: /^(senden|nachricht senden|absenden)$/i }).last();
  if ((await roleButton.count()) > 0 && (await roleButton.isVisible().catch(() => false))) {
    await roleButton.click();
    return;
  }

  const fallback = page.locator("button[type='submit'], input[type='submit']").last();
  if ((await fallback.count()) > 0 && (await fallback.isVisible().catch(() => false))) {
    await fallback.click();
    return;
  }

  throw new Error("Could not find a visible send button. No message was sent.");
}

async function maybeOpenMessageComposer(page: Page): Promise<void> {
  const textbox = page.locator("textarea, [contenteditable='true'], [role='textbox']").first();
  if ((await textbox.count()) > 0 && (await textbox.isVisible().catch(() => false))) {
    return;
  }

  const button = page
    .getByRole("button", { name: /nachricht|anschreiben|kontakt|anfragen|schreiben/i })
    .or(page.getByRole("link", { name: /nachricht|anschreiben|kontakt|anfragen|schreiben/i }))
    .first();

  if ((await button.count()) > 0 && (await button.isVisible().catch(() => false))) {
    await button.click();
    await page.waitForTimeout(1500);
  }
}
