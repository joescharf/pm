#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# Health check: ensure pm serve is running
echo "Checking pm serve is running..."
if ! curl -sf http://localhost:8080/api/v1/projects > /dev/null 2>&1; then
  echo "ERROR: pm serve is not running on http://localhost:8080"
  echo "Start it with: pm serve"
  exit 1
fi
echo "pm serve is running."

# Capture screenshots to temporary directory
echo "Capturing screenshots..."
mkdir -p img-raw
shot-scraper multi shots.yaml

# Add browser frames (output to img/ with original names)
echo "Adding browser frames..."
mkdir -p img
uv run ./add_browser_frame.py img-raw/ -o img/ --keep-name --title "PM - Project Manager"

# Compress images
echo "Compressing images..."
bunx imageoptim-cli img/*.png

# Clean up raw screenshots
rm -rf img-raw

# Copy to docs
echo "Copying screenshots to docs..."
mkdir -p ../docs/docs/img
cp img/*.png ../docs/docs/img/

echo "Done! Screenshots in scripts/img/ and docs/docs/img/"
