# Trello OpenAPI provenance

- Source: <https://developer.atlassian.com/cloud/trello/rest/>
- Retrieved: 2026-07-12
- Source type: official Atlassian documentation
- Source representation: the OpenAPI 3.0 schema embedded in the REST reference page
- Archived file: `spec.json`
- Normalized SHA-256: `756e02dd85c635734e32a5d2630eec44eab015c7eb6956a07084c03f0708c145`
- Printed with CLI Printing Press 4.6.1
- Generated surface: 191 paths, 261 endpoints, 123 resource groups

Normalization was limited to changing the display title to `Trello Calendar`, adding a workflow description, and replacing the two query credential schemes with one composed `Authorization` header scheme whose values come from `TRELLO_API_KEY` and `TRELLO_TOKEN`. Operation security requirements were mechanically updated to reference that scheme. No endpoint paths or operations were removed.

The API-specific Google Calendar workflow is not represented as fake Trello endpoints. It lives in separately injected adapters and novel Cobra commands, with every generated-tree registration recorded in `.printing-press-patches.json`.

