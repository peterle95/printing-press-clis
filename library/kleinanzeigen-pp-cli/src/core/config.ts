import fs from "node:fs";
import path from "node:path";
import YAML from "yaml";
import {
  configPath,
  defaultBrowserProfilePath,
  defaultDatabasePath,
  ensureParentDir,
  expandHome,
} from "./paths.js";

export type SortOption = "distance" | "date" | "price" | "price_asc" | "price_desc" | "relevance";

export interface KleinanzeigenConfig {
  location: {
    postal_code: string;
    city: string;
    radius_km: number;
  };
  search: {
    default_sort: SortOption;
    max_pages: number;
    min_delay_ms: number;
    max_delay_ms: number;
  };
  safety: {
    require_send_confirmation: boolean;
    allow_bulk_messaging: boolean;
    max_messages_per_session: number;
    dry_run_default: boolean;
  };
  browser_profile: string;
  storage: {
    db_path: string;
  };
  message_templates: Record<string, string>;
}

export interface LoadConfigOptions {
  path?: string;
  createIfMissing?: boolean;
}

export const DEFAULT_CONFIG: KleinanzeigenConfig = {
  location: {
    postal_code: "12045",
    city: "Berlin",
    radius_km: 5,
  },
  search: {
    default_sort: "distance",
    max_pages: 2,
    min_delay_ms: 3000,
    max_delay_ms: 9000,
  },
  safety: {
    require_send_confirmation: true,
    allow_bulk_messaging: false,
    max_messages_per_session: 5,
    dry_run_default: true,
  },
  browser_profile: defaultBrowserProfilePath(),
  storage: {
    db_path: defaultDatabasePath(),
  },
  message_templates: {},
};

export function renderConfig(config: KleinanzeigenConfig): string {
  return YAML.stringify(config);
}

export function initConfig(targetPath?: string, force = false): KleinanzeigenConfig {
  const resolved = configPath(targetPath);
  if (fs.existsSync(resolved) && !force) {
    throw new Error(`Config already exists at ${resolved}. Use --force to overwrite.`);
  }
  ensureParentDir(resolved);
  fs.writeFileSync(resolved, renderConfig(DEFAULT_CONFIG), { mode: 0o600 });
  return DEFAULT_CONFIG;
}

export function loadConfig(options: LoadConfigOptions = {}): KleinanzeigenConfig {
  const resolved = configPath(options.path);
  if (!fs.existsSync(resolved)) {
    if (options.createIfMissing) {
      return initConfig(resolved, false);
    }
    return DEFAULT_CONFIG;
  }

  const parsed = YAML.parse(fs.readFileSync(resolved, "utf8")) ?? {};
  return normalizeConfig(deepMerge(DEFAULT_CONFIG, parsed));
}

export function saveConfig(config: KleinanzeigenConfig, targetPath?: string): void {
  const resolved = configPath(targetPath);
  ensureParentDir(resolved);
  fs.writeFileSync(resolved, renderConfig(config), { mode: 0o600 });
}

export function resolvedConfigPath(targetPath?: string): string {
  return configPath(targetPath);
}

export function resolveBrowserProfile(config: KleinanzeigenConfig): string {
  return path.resolve(expandHome(config.browser_profile));
}

export function resolveDatabasePath(config: KleinanzeigenConfig): string {
  return path.resolve(expandHome(config.storage.db_path));
}

function normalizeConfig(config: KleinanzeigenConfig): KleinanzeigenConfig {
  return {
    ...config,
    location: {
      postal_code: String(config.location.postal_code),
      city: String(config.location.city),
      radius_km: toPositiveNumber(config.location.radius_km, DEFAULT_CONFIG.location.radius_km),
    },
    search: {
      default_sort: normalizeSort(config.search.default_sort),
      max_pages: clampInt(config.search.max_pages, 1, 5, DEFAULT_CONFIG.search.max_pages),
      min_delay_ms: clampInt(config.search.min_delay_ms, 1000, 60000, DEFAULT_CONFIG.search.min_delay_ms),
      max_delay_ms: clampInt(config.search.max_delay_ms, 1000, 120000, DEFAULT_CONFIG.search.max_delay_ms),
    },
    safety: {
      require_send_confirmation: Boolean(config.safety.require_send_confirmation),
      allow_bulk_messaging: Boolean(config.safety.allow_bulk_messaging),
      max_messages_per_session: clampInt(
        config.safety.max_messages_per_session,
        1,
        20,
        DEFAULT_CONFIG.safety.max_messages_per_session,
      ),
      dry_run_default: Boolean(config.safety.dry_run_default),
    },
    browser_profile: String(config.browser_profile),
    storage: {
      db_path: String(config.storage.db_path),
    },
    message_templates: config.message_templates ?? {},
  };
}

function normalizeSort(sort: unknown): SortOption {
  const value = String(sort);
  if (["distance", "date", "price", "price_asc", "price_desc", "relevance"].includes(value)) {
    return value as SortOption;
  }
  return DEFAULT_CONFIG.search.default_sort;
}

function toPositiveNumber(value: unknown, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

function clampInt(value: unknown, min: number, max: number, fallback: number): number {
  const parsed = Math.trunc(Number(value));
  if (!Number.isFinite(parsed)) {
    return fallback;
  }
  return Math.max(min, Math.min(max, parsed));
}

function deepMerge<T>(base: T, override: unknown): T {
  if (!isRecord(base) || !isRecord(override)) {
    return override === undefined ? base : (override as T);
  }

  const merged: Record<string, unknown> = { ...base };
  for (const [key, value] of Object.entries(override)) {
    const baseValue = (base as Record<string, unknown>)[key];
    merged[key] = isRecord(baseValue) && isRecord(value) ? deepMerge(baseValue, value) : value;
  }
  return merged as T;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
