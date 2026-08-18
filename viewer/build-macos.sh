#!/bin/bash
set -euo pipefail

viewer_dir="$(cd "$(dirname "$0")" && pwd)"
build_dir="$viewer_dir/build"
app_dir="$build_dir/VWP Viewer.app"
archive_path="$build_dir/VWP-Viewer-macOS.zip"
macos_dir="$app_dir/Contents/MacOS"

rm -rf "$app_dir"
rm -f "$archive_path"
mkdir -p "$macos_dir"
cp "$viewer_dir/macos/Info.plist" "$app_dir/Contents/Info.plist"

(cd "$viewer_dir/helper" && go build -trimpath -o "$macos_dir/vwp-viewer-helper" .)

architecture="$(uname -m)"
MACOSX_DEPLOYMENT_TARGET=13.0 swiftc \
  -swift-version 5 \
  -parse-as-library \
  -O \
  -target "${architecture}-apple-macos13.0" \
  -framework SwiftUI \
  -framework AppKit \
  -framework QuickLookUI \
  -framework WebKit \
  "$viewer_dir/macos/VWPViewer.swift" \
  -o "$macos_dir/VWP Viewer"

codesign --force --deep --sign - "$app_dir"
ditto -c -k --sequesterRsrc --keepParent "$app_dir" "$archive_path"
echo "$app_dir"
echo "$archive_path"
