import { describe, expect, it } from "vitest";
import { buildSearchUrl, listingIdFromUrl, normalizeListingUrl } from "../src/core/kleinanzeigenUrls.js";

describe("kleinanzeigen URL helpers", () => {
  it("builds a location and radius search URL", () => {
    const url = buildSearchUrl({
      query: "standing desk",
      postalCode: "12045",
      city: "Berlin",
      radiusKm: 5,
      sort: "distance",
      maxPrice: 80,
      page: 2,
    });
    const parsed = new URL(url);
    expect(parsed.hostname).toBe("www.kleinanzeigen.de");
    expect(parsed.searchParams.get("keywords")).toBe("standing desk");
    expect(parsed.searchParams.get("locationStr")).toBe("12045 Berlin");
    expect(parsed.searchParams.get("radius")).toBe("5");
    expect(parsed.searchParams.get("maxPrice")).toBe("80");
    expect(parsed.searchParams.get("pageNum")).toBe("2");
  });

  it("normalizes listing URLs and extracts listing ids", () => {
    const normalized = normalizeListingUrl("/s-anzeige/ikea-kallax/1234567890-88-1234");
    expect(normalized).toBe("https://www.kleinanzeigen.de/s-anzeige/ikea-kallax/1234567890-88-1234");
    expect(listingIdFromUrl(normalized ?? "")).toBe("1234567890");
  });
});
