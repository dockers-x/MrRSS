#!/bin/bash
# Script to create a macOS DMG installer for MrRSS

set -e

APP_NAME="MrRSS"
VERSION="1.1.0"
BUILD_DIR="build/bin"
DMG_DIR="build/dmg"
APP_PATH="${BUILD_DIR}/${APP_NAME}.app"
DMG_NAME="${APP_NAME}-${VERSION}-darwin-universal.dmg"

echo "Creating DMG for ${APP_NAME} ${VERSION}..."

# Check if app exists
if [ ! -d "${APP_PATH}" ]; then
    echo "Error: Application not found at ${APP_PATH}"
    echo "Please build the application first with: wails build -platform darwin/universal"
    exit 1
fi

# Create DMG directory
rm -rf "${DMG_DIR}"
mkdir -p "${DMG_DIR}"

# Copy app to DMG directory
echo "Copying application..."
cp -R "${APP_PATH}" "${DMG_DIR}/"

# Create Applications symlink
echo "Creating Applications symlink..."
ln -s /Applications "${DMG_DIR}/Applications"

# Create DMG
echo "Creating DMG image..."
rm -f "${BUILD_DIR}/${DMG_NAME}"

# Use hdiutil to create the DMG
hdiutil create -volname "${APP_NAME}" \
    -srcfolder "${DMG_DIR}" \
    -ov -format UDZO \
    "${BUILD_DIR}/${DMG_NAME}"

# Clean up
rm -rf "${DMG_DIR}"

echo "DMG created successfully: ${BUILD_DIR}/${DMG_NAME}"
echo ""
echo "Installation instructions:"
echo "1. Open the DMG file"
echo "2. Drag ${APP_NAME}.app to the Applications folder"
echo "3. Launch ${APP_NAME} from Applications"
echo ""
echo "User data will be stored in: ~/Library/Application Support/MrRSS/"
