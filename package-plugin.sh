#!/bin/bash
# Package the Fledge OData data source for distribution / catalog submission.
# The archive must contain exactly one root folder named after the plugin id.

set -e

PLUGIN_ID="fledge-odata-datasource"
VERSION=$(grep '"version":' package.json | head -1 | sed 's/.*"version": "\(.*\)".*/\1/')
PACKAGE_NAME="${PLUGIN_ID}-${VERSION}.zip"

echo "Packaging ${PLUGIN_ID} version ${VERSION}..."

echo "Building frontend..."
npm run build

echo "Building backend (all platforms, via mage)..."
go run github.com/magefile/mage@latest -d . buildAll

# Package with the plugin id as the single root directory
TEMP_DIR=$(mktemp -d)
PLUGIN_DIR="${TEMP_DIR}/${PLUGIN_ID}"

mkdir -p "${PLUGIN_DIR}"
cp -r dist/* "${PLUGIN_DIR}/"

echo "Creating zip archive..."
cd "${TEMP_DIR}"
zip -qr "${PACKAGE_NAME}" "${PLUGIN_ID}"
mv "${PACKAGE_NAME}" "${OLDPWD}/"

cd "${OLDPWD}"
rm -rf "${TEMP_DIR}"

SHA1=$(shasum "${PACKAGE_NAME}" | cut -d' ' -f1)
echo "${SHA1}" > "${PACKAGE_NAME}.sha1"

echo ""
echo "Package created: ${PACKAGE_NAME}"
echo "SHA1 (needed for the catalog submission form): ${SHA1}"
echo ""
echo "Contents:"
unzip -l "${PACKAGE_NAME}" | head -25
