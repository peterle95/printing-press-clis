import { describe, expect, it } from "vitest";
import { DEFAULT_CONFIG } from "../src/core/config.js";
import { isExactSendConfirmation, resolveDryRun } from "../src/core/safety.js";

describe("message confirmation safety", () => {
  it("requires exact SEND", () => {
    expect(isExactSendConfirmation("SEND")).toBe(true);
    expect(isExactSendConfirmation("send")).toBe(false);
    expect(isExactSendConfirmation(" SEND ")).toBe(false);
  });

  it("defaults messaging to dry run unless live is requested", () => {
    expect(resolveDryRun(DEFAULT_CONFIG, {})).toBe(true);
    expect(resolveDryRun(DEFAULT_CONFIG, { live: true })).toBe(false);
    expect(resolveDryRun(DEFAULT_CONFIG, { live: true, dryRun: true })).toBe(true);
  });
});
