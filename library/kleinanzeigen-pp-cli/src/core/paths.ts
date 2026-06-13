import fs from "node:fs";
import os from "node:os";
import path from "node:path";

export const APP_NAME = "kleinanzeigen-pp-cli";

export function expandHome(value: string): string {
  if (value === "~") {
    return os.homedir();
  }
  if (value.startsWith("~/")) {
    return path.join(os.homedir(), value.slice(2));
  }
  return value;
}

export function configPath(override?: string): string {
  if (override) {
    return path.resolve(expandHome(override));
  }
  if (process.env.KLEINANZEIGEN_PP_CONFIG) {
    return path.resolve(expandHome(process.env.KLEINANZEIGEN_PP_CONFIG));
  }
  return path.join(os.homedir(), ".config", APP_NAME, "config.yaml");
}

export function dataDir(): string {
  if (process.env.KLEINANZEIGEN_PP_DATA_DIR) {
    return path.resolve(expandHome(process.env.KLEINANZEIGEN_PP_DATA_DIR));
  }
  return path.join(os.homedir(), ".local", "share", APP_NAME);
}

export function defaultBrowserProfilePath(): string {
  return path.join(dataDir(), "browser-profile");
}

export function defaultDatabasePath(): string {
  return path.join(dataDir(), "kleinanzeigen.sqlite");
}

export function ensureDir(dir: string): void {
  fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
}

export function ensureParentDir(filePath: string): void {
  ensureDir(path.dirname(filePath));
}
