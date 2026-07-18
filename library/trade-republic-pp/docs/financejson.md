# FinanceJSON v1

FinanceJSON is a portable, versioned import and fixture format. It is not a
second database; after validation, SQLite is authoritative.

The root schema identifier is `trpp.finance/v1`. Times use RFC 3339, ISINs are
uppercase and check-digit validated, currencies are ISO 4217 codes, and every
financial decimal is a JSON string with up to eight fractional places.

Synthetic minimal example:

```json
{
  "schema": "trpp.finance/v1",
  "generated_at": "2026-07-16T10:00:00Z",
  "as_of": "2026-07-16T10:00:00Z",
  "provider": "fixture",
  "instruments": [
    {
      "isin": "IE00B4L5Y983",
      "name": "Synthetic World ETF",
      "kind": "etf",
      "sector": "Diversified",
      "country": "Global",
      "trading_currency": "EUR"
    }
  ],
  "positions": [
    {
      "isin": "IE00B4L5Y983",
      "name": "Synthetic World ETF",
      "quantity": "2",
      "average_cost": "90",
      "price": "100",
      "market_value": "200",
      "currency": "EUR",
      "as_of": "2026-07-16T10:00:00Z"
    }
  ],
  "cash_balances": [
    {
      "currency": "EUR",
      "amount": "500",
      "as_of": "2026-07-16T10:00:00Z"
    }
  ],
  "transactions": [],
  "documents": [],
  "research_reports": []
}
```

Research reports use `schema_version: 1`, `kind: etf` or `kind: company`, an
`as_of` timestamp, the corresponding structured section, and at least one
HTTP(S) citation. Free text is treated as untrusted display data and never
becomes SQL, a shell argument, or an order field.

