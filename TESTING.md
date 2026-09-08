# Testing guidance

This document describes how to functionally test the Fledge OData data source, for example as part of a Grafana plugin catalog review. No Fledge account or credentials are required.

## Option 1: Docker compose with the bundled test server (recommended)

The repository ships an OData V4 test server with sample time series data, plus a Grafana instance with pre-provisioned data sources.

Prerequisites: Docker.

```bash
npm install
npm run build            # build the frontend into dist/
go run github.com/magefile/mage@latest -d . buildAll   # build the backend
npm run server           # docker compose up: Grafana on :3000, test server on :4004
```

Then:

1. Open http://localhost:3000 (anonymous admin access is enabled in the dev image).
2. Go to **Connections > Data sources > OData-Test** and click **Save & test** — the health check should pass.
3. Create a new dashboard panel:
   - Select the **OData-Test** data source.
   - Pick an entity set, a time property, and one or more metrics.
   - Add a filter condition to verify `$filter` handling.

## Option 2: Any public OData V4 service

The plugin works against standards-compliant OData V4 services. For example, configure a data source with:

- **URL**: `https://services.odata.org/V4/OData/OData.svc`
- No authentication.

Entity sets from the service metadata (e.g. `Products`) should appear in the query editor. These reference services have no datetime-typed properties in most entity sets, so use table visualizations, or leave the time property empty.

## Fledge OAuth2 authentication

The OAuth2 API-key flow authenticates against Fledge's own OAuth2 server (`https://oauth2.fledge.nl`) and requires a Fledge customer API key, so it cannot be exercised without a Fledge account. What can be verified without one:

- The OAuth2 configuration UI (toggle, base URL, API key field) in the data source settings.
- The API key is stored via Grafana's `secureJsonData` (encrypted at rest, never returned to the frontend).
- With OAuth2 disabled, the plugin performs plain requests — the flow is strictly additive.

The token-handling code lives in the backend under `pkg/plugin/` and is covered by unit tests (`go test ./...`).

## Automated tests

```bash
npm run test:ci     # frontend unit tests (Jest)
npm run typecheck   # TypeScript
npm run lint        # ESLint
go test ./...       # backend unit tests
npm run e2e         # Playwright end-to-end tests (requires the docker compose stack)
```
