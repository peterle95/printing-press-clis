export const DEFAULT_MESSAGE_TEMPLATES: Record<string, string> = {
  availability: "Hallo, ist der Artikel noch verfügbar? Ich könnte ihn in Berlin abholen. Viele Grüße",
  polite_offer:
    "Hallo, ist der Artikel noch verfügbar? Wären Sie mit {offer_price} € einverstanden? Ich könnte ihn zeitnah abholen. Viele Grüße",
  pickup_question:
    "Hallo, ist der Artikel noch verfügbar? Wann wäre eine Abholung ungefähr möglich? Viele Grüße",
};

export function renderMessageTemplate(template: string, values: Record<string, string | number | undefined>): string {
  return template.replace(/\{([a-zA-Z0-9_]+)\}/g, (_, key: string) => {
    const value = values[key];
    return value === undefined ? `{${key}}` : String(value);
  });
}
