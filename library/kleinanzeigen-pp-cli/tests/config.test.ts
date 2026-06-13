import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { initConfig, loadConfig, saveConfig } from "../src/core/config.js";

describe("config", () => {
  it("creates and reloads the default config", () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "ka-config-"));
    const configPath = path.join(dir, "config.yaml");

    const created = initConfig(configPath);
    expect(created.location.postal_code).toBe("12045");

    const loaded = loadConfig({ path: configPath });
    expect(loaded.location.city).toBe("Berlin");
    expect(loaded.safety.require_send_confirmation).toBe(true);
  });

  it("normalizes partial configs", () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "ka-config-"));
    const configPath = path.join(dir, "config.yaml");
    const config = loadConfig();
    config.location.radius_km = 10;
    saveConfig(config, configPath);

    const loaded = loadConfig({ path: configPath });
    expect(loaded.location.radius_km).toBe(10);
    expect(loaded.search.max_pages).toBeGreaterThan(0);
  });
});
