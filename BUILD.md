# Build Instructions for Grafana OData Plugin

## Quick Start

The easiest way to build and install the plugin:

```bash
# Using the build script (recommended)
./build-and-install.sh

# Or using Make
make dev
```

## Build Methods

### Method 1: Build Script (Recommended)

```bash
./build-and-install.sh
```

This script will:
1. Build the frontend (React components)
2. Build the backend (Go binary) with the correct name
3. Install to Grafana's plugin directory
4. Optionally restart Grafana

### Method 2: Makefile

```bash
# Build and install
make install

# Build, install, and restart Grafana
make dev

# Just build (no install)
make build

# Build only frontend
make build-frontend

# Build only backend
make build-backend

# Build for all platforms
make build-backend-all

# Clean build artifacts
make clean

# Show help
make help
```

### Method 3: Manual Build

If you need to build manually:

```bash
# 1. Build frontend
npm run build

# 2. Build backend for your platform
# The binary MUST be named according to plugin.json (gpx_odata-datasource)

# For macOS ARM64 (M1/M2/M3):
go build -o ./dist/gpx_odata-datasource_darwin_arm64 ./pkg

# For macOS Intel:
go build -o ./dist/gpx_odata-datasource_darwin_amd64 ./pkg

# For Linux AMD64:
GOOS=linux GOARCH=amd64 go build -o ./dist/gpx_odata-datasource_linux_amd64 ./pkg

# For Linux ARM64:
GOOS=linux GOARCH=arm64 go build -o ./dist/gpx_odata-datasource_linux_arm64 ./pkg

# 3. Install to Grafana plugins directory
cp -r dist/* /opt/homebrew/var/lib/grafana/plugins/fledge-odata-datasource/
cp plugin.json /opt/homebrew/var/lib/grafana/plugins/fledge-odata-datasource/
# ... copy other files

# 4. Restart Grafana
brew services restart grafana
```

## Important Notes

### Binary Naming

⚠️ **CRITICAL**: The binary name MUST match what's specified in `plugin.json`:

```json
"executable": "gpx_odata-datasource"
```

This means the platform-specific binaries should be:
- `gpx_odata-datasource_darwin_arm64` (macOS ARM)
- `gpx_odata-datasource_darwin_amd64` (macOS Intel)
- `gpx_odata-datasource_linux_amd64` (Linux AMD64)
- `gpx_odata-datasource_linux_arm64` (Linux ARM64)
- `gpx_odata-datasource_windows_amd64.exe` (Windows)

### Plugin Directory

The default Grafana plugin directory for Homebrew installations is:
```
/opt/homebrew/var/lib/grafana/plugins/fledge-odata-datasource/
```

For other installations, check your Grafana config for `plugins` path.

## Troubleshooting

### Plugin not loading?

1. Check the binary name matches the pattern above
2. Check the binary is in the correct directory
3. Check Grafana logs: `tail -f /opt/homebrew/var/log/grafana/grafana.log`
4. Restart Grafana: `brew services restart grafana`

### Binary not found?

Make sure you built for the correct platform. Check with:
```bash
uname -s  # Should show: Darwin, Linux, etc.
uname -m  # Should show: arm64, x86_64, etc.
```

### Permission denied?

Make sure the binary is executable:
```bash
chmod +x /opt/homebrew/var/lib/grafana/plugins/fledge-odata-datasource/gpx_odata-datasource_*
```

## Development Workflow

For active development:

```bash
# Terminal 1: Watch frontend changes
npm run dev

# Terminal 2: When you make backend changes
make dev  # Rebuilds backend, installs, and restarts Grafana
```

## Testing Your Changes

After building and restarting Grafana:

1. Open Grafana at http://localhost:3000
2. Go to Data Sources
3. Find your OData data source
4. Test the connection
5. Create a query to test filters (eq, ge, gt, lt, le, ne)
6. Check logs for any errors:
   ```bash
   tail -f /opt/homebrew/var/log/grafana/grafana.log | grep odata
   ```
