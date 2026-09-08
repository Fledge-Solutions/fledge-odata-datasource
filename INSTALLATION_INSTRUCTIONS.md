# Fledge OData Datasource Plugin - Installation Instructions

## Prerequisites
- Grafana version 10.4.0 or higher
- Access to the Grafana server (SSH or local access)
- Administrator privileges on Grafana

## Installation Steps

### 1. Download the Plugin
Download the `fledge-odata-datasource-<version>.zip` file provided by Fledge.

### 2. Locate Grafana Plugins Directory

The plugins directory location depends on your Grafana installation:

**Linux (standard installation):**
```
/var/lib/grafana/plugins/
```

**Linux (Docker):**
```
/var/lib/grafana/plugins/ (inside container)
```
Or mount a volume: `-v /path/on/host:/var/lib/grafana/plugins`

**macOS (Homebrew):**
```
/opt/homebrew/var/lib/grafana/plugins/
```

**Windows:**
```
C:\Program Files\GrafanaLabs\grafana\data\plugins\
```

### 3. Extract the Plugin

**Important:** the zip already contains a top-level `fledge-odata-datasource/` folder. Extract it *directly into* the plugins directory - do not create a `fledge-odata-datasource` folder yourself and unzip inside that, or the plugin ends up nested two levels deep (`plugins/fledge-odata-datasource/fledge-odata-datasource/`) and Grafana will not find it.

If you are replacing an older install, remove the old plugin folder completely first - don't extract on top of it:
```bash
sudo rm -rf /var/lib/grafana/plugins/fledge-odata-datasource
```

**On Linux/macOS:**
```bash
cd /var/lib/grafana/plugins/
sudo unzip /path/to/fledge-odata-datasource-<version>.zip
```

**On Windows:**
1. Extract the ZIP file directly into: `C:\Program Files\GrafanaLabs\grafana\data\plugins\`
   (this creates `...\plugins\fledge-odata-datasource\` - do not extract into a pre-made subfolder)

After extracting, verify the depth is correct - this file must exist:
```
<plugins directory>/fledge-odata-datasource/plugin.json
```

### 4. Configure Grafana to Allow Unsigned Plugins

Since this is a custom plugin, Grafana needs to be configured to allow it.

**Option A: Edit grafana.ini**

Find your `grafana.ini` configuration file and add/modify:

```ini
[plugins]
allow_loading_unsigned_plugins = fledge-odata-datasource
```

**Option B: Using Environment Variable**

Set the environment variable before starting Grafana:

```bash
export GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=fledge-odata-datasource
```

**For Docker:**
```bash
docker run -d \
  -p 3000:3000 \
  -e "GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=fledge-odata-datasource" \
  -v /path/to/plugins:/var/lib/grafana/plugins \
  grafana/grafana
```

### 5. Restart Grafana

**Linux (systemd):**
```bash
sudo systemctl restart grafana-server
```

**Linux (init.d):**
```bash
sudo service grafana-server restart
```

**macOS (Homebrew):**
```bash
brew services restart grafana
```

**Windows:**
- Restart the Grafana service from Services management console
- Or restart Grafana from the Start menu

**Docker:**
```bash
docker restart <container-name>
```

### 6. Verify Installation

1. Open Grafana in your browser: `http://localhost:3000` (or your Grafana URL)
2. Log in as an administrator
3. Go to **Configuration** → **Data Sources**
4. Click **Add data source**
5. Search for **OData** - you should see the "OData" datasource in the list

## Configuring the OData Datasource

1. Click on **OData** from the data sources list
2. Configure the following settings:

   **Basic Settings:**
   - **Name**: Give your datasource a name (e.g., "Fledge OData")
   - **URL**: Your OData service endpoint (e.g., `https://api.example.com/odata`)

   **Additional Settings:**
   - **URL space encoding**: Select **Percent (%20)** (recommended for most OData services)

   **OAuth2 Authentication** (if needed):
   - Enable OAuth2 if your OData service requires it
   - Enter the OAuth2 Base URL
   - Enter your API Key

3. Click **Save & Test** to verify the connection

## Using the Plugin

### Creating a Query

1. Create a new dashboard or open an existing one
2. Add a new panel
3. Select your OData datasource
4. Configure your query:
   - **Entity set**: Select the OData entity you want to query
   - **Time property**: Select a date/time field for time-series data (optional)
   - **Select**: Choose the properties/columns you want to retrieve
   - **Filter**: Add filter conditions to narrow down results

## Troubleshooting

### Plugin Not Showing Up
- Verify `plugin.json` is exactly one folder level below the plugins directory (`<plugins directory>/fledge-odata-datasource/plugin.json`) - a common mistake is extracting the zip into an extra wrapper folder, which nests it too deep for Grafana to discover
- Check that the unsigned plugin is allowed in configuration
- Check Grafana logs: `/var/log/grafana/grafana.log` (Linux) or check the Grafana UI logs
- In Grafana, go to **Administration → Plugins**, search "OData", and confirm the version shown matches what you just installed - if it still shows an old version after a reinstall, the old plugin folder likely wasn't fully removed first

### Connection Errors (400 Bad Request)
- Make sure **URL space encoding** is set to **Percent (%20)** in datasource settings
- Verify your OData service URL is correct
- Check that filters are properly configured

### Authentication Errors
- Verify OAuth2 credentials if using OAuth2
- Check that your API key is correct and has proper permissions

## Support

For questions or issues with the plugin:
- **Fledge Support**: visit [fledge.nl](https://www.fledge.nl) and contact Fledge support
- **Plugin Repository**: https://github.com/fledge-solutions/fledge-odata-datasource

## Version Information

- **Plugin Version**: 1.4.0
- **Grafana Compatibility**: ≥10.4.0
- **OData Version**: V4
