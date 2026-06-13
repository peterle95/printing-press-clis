import { describe, expect, it } from "vitest";
import { KleinanzeigenDb } from "../src/core/db.js";

describe("database", () => {
  it("deduplicates listings by id and watch results by watch/listing", async () => {
    const db = await KleinanzeigenDb.memory();
    try {
      const listing = {
        id: "1234567890",
        title: "Monitor",
        url: "https://www.kleinanzeigen.de/s-anzeige/monitor/1234567890-225-9633",
        price: "50 €",
      };
      db.upsertListing(listing);
      db.upsertListing({ ...listing, price: "45 €" });
      expect(db.getListing("1234567890")?.price).toBe("45 €");

      const watchId = db.addWatchRule("monitor", { radiusKm: 5, maxPrice: 80 });
      expect(db.addWatchResultIfNew(watchId, listing.id)).toBe(true);
      expect(db.addWatchResultIfNew(watchId, listing.id)).toBe(false);
    } finally {
      db.close();
    }
  });
});
