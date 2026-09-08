# Fledge OData data source for Grafana

Visualize your [Fledge](https://www.fledge.nl) data in Grafana. This data source plugin connects Grafana to Fledge's OData V4 API, with built-in support for Fledge's OAuth2 authentication. It also works with other standards-compliant OData V4 services.

This plugin is a fork of the [d-velop Grafana OData data source](https://github.com/d-velop/grafana-odata-datasource), extended with Fledge-specific authentication and improvements. See [Acknowledgments](#acknowledgments).

## Features

- **Query OData V4 services**: select an entity set, pick properties and metrics, and add OData filters — no manual URL building required.
- **Fledge OAuth2 authentication**: authenticate against Fledge's OAuth2 server using an API key. Tokens are fetched and refreshed automatically, and the API key is stored securely in Grafana's encrypted secret store.
- **Forward OAuth identity**: optionally forward the Grafana user's OAuth token to the OData service.
- **Metadata-driven**: entity sets, properties, and types are discovered from the service's `$metadata` document.
- **Time series and table data**: use any datetime property as the time column, or query plain tables without one.
- **Alerting support**: use OData queries in Grafana alert rules.

## Installation

Install from the Grafana plugin catalog, or with the Grafana CLI:

```
grafana cli plugins install fledge-odata-datasource
```

For manual installation from a release archive, see the [installation instructions](https://github.com/fledge-solutions/fledge-odata-datasource/blob/master/INSTALLATION_INSTRUCTIONS.md).

## Configuration

1. In Grafana, go to **Connections > Data sources** and click **Add new data source**.
2. Search for **Fledge OData** and select it.
3. Configure the data source:
   - **URL**: the root URL of your OData service.
   - **URL space encoding**: select **Percent (%20)** — required for Fledge's OData service.
   - **OAuth2 settings** (when connecting to Fledge):
     - Enable OAuth2.
     - Enter your OAuth2 base URL (for example `https://oauth2.fledge.nl`).
     - Enter your API key. It is stored encrypted and never displayed again.
4. Click **Save & test**.

## Creating queries

1. Create a new panel and select your Fledge OData data source.
2. Choose an **entity set** from the dropdown.
3. Optionally select a **time property** to get time series data.
4. Select the properties or metrics you want to visualize.
5. Add filter conditions as needed, for example `administration_id eq 1`.

Filters use standard [OData filter syntax](https://www.odata.org/getting-started/basic-tutorial/#filter) and are passed to the service as `$filter` query parameters.

## Documentation

- [Installation instructions](https://github.com/fledge-solutions/fledge-odata-datasource/blob/master/INSTALLATION_INSTRUCTIONS.md)
- [Contributing and development guide](https://github.com/fledge-solutions/fledge-odata-datasource/blob/master/CONTRIBUTING.md)
- [Changelog](https://github.com/fledge-solutions/fledge-odata-datasource/blob/master/CHANGELOG.md)
- [Learn more about OData](https://www.odata.org/)

## Acknowledgments

This plugin is based on the [d-velop Grafana OData data source](https://github.com/d-velop/grafana-odata-datasource) by d.velop AG and extends it with Fledge-specific features.

## License

Apache License 2.0 — see [LICENSE](https://github.com/fledge-solutions/fledge-odata-datasource/blob/master/LICENSE).

Original work Copyright (c) d.velop AG.
Modified work Copyright (c) Fledge.
