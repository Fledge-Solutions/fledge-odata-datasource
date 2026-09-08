# Change Log

## [1.4.0] 2026-09-08

### Breaking Changes
- **Minimum supported Grafana version raised to 13.2.1** (previously 10.4.0). Upgraded `@grafana/data`/`@grafana/ui`/`@grafana/runtime` to 13.2.1, which requires React 19 - Grafana hosts older than 13.2.1 will not have a compatible `@grafana/ui` available at runtime.

### Features
- `$apply`-based aggregation (grouped and ungrouped) for entity sets that support it
- Support for `Edm.DateTime` properties (distinct from `Edm.DateTimeOffset`) as filter, time, and query properties

### Bug Fixes
- Fixed date-format handling in query construction
- Fixed bucketing in the frontend

### Dependencies
- Upgraded `@grafana/data`, `@grafana/runtime`, `@grafana/schema`, `@grafana/ui` to 13.2.1; `react`/`react-dom` to 19.2.8 to match (Grafana supplies these at runtime for all plugins - see [Grafana's React 19 migration notes](https://grafana.com/blog/react-19-is-coming-to-grafana-what-plugin-developers-need-to-know/))
- Fixed high-severity vulnerabilities in `google.golang.org/grpc`, `browserslist`, `fast-uri`, `js-yaml`, `nanoid`, `postcss`

## [1.3.0] 2026-07-07

### Changes
- Renamed plugin to "Fledge OData" in preparation for publication in the Grafana plugin catalog
- Moved repository to https://github.com/fledge-solutions/fledge-odata-datasource
- Backend is now built with the Grafana plugin SDK build tooling (mage)
- Updated Grafana plugin SDK for Go and frontend dependencies

## [1.2.1] 2026-03-04

### Bug Fixes
- Replace `panic` in `mapNumber` with proper error handling
- Close response body in `CheckHealth` to prevent resource leak
- Handle errors from `ds.im.Get()` in `getClientInstance`
- Handle errors from `url.ParseQuery()` in `buildQueryUrl`

## [1.2.0] 2026-02-28

### Features
- [#1](https://github.com/d-velop/grafana-odata-datasource/issues/1): Add 'Forward OAuth Identity' support


## [1.1.1] 2025-04-17

### Features
- Updated to go version `1.24`
- Updated to Grafana `11.6`

## [1.1.0] 2024-03-14

### Features
- [#2](https://github.com/d-velop/grafana-odata-datasource/issues/2): Made time property optional
- Updated to go version `1.21`
- Updated to Grafana `10.2`
- Configurable space encoding (`%20`or `+`)

## [1.0.0] 2023-05-12

### Features
- Initial revision
