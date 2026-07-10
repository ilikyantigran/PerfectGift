#!/usr/bin/env bash
#
# Runs the PerfectGiftKit unit tests with `swift test`.
#
# Under a full Xcode toolchain, `swift test` finds Swift Testing automatically and you do
# not need this script. Under an Xcode **Command Line Tools**-only toolchain (no Xcode.app),
# SwiftPM does not add the Testing.framework search paths / macro plugin, so this script
# discovers them and passes them through. Package.swift is kept clean/portable on purpose.
#
set -euo pipefail
cd "$(dirname "$0")"

# Prefer an Xcode toolchain if one is selected; otherwise use Command Line Tools.
DEV_DIR="$(xcode-select -p 2>/dev/null || echo /Library/Developer/CommandLineTools)"

FW="$DEV_DIR/Library/Developer/Frameworks"
PLUGIN="$DEV_DIR/usr/lib/swift/host/plugins/testing/libTestingMacros.dylib"
INTEROP="$DEV_DIR/Library/Developer/usr/lib"

if [[ -d "$FW/Testing.framework" && -f "$PLUGIN" ]]; then
  echo "Running tests with Command Line Tools Swift Testing support…"
  exec swift test \
    -Xswiftc -F -Xswiftc "$FW" \
    -Xswiftc -load-plugin-library -Xswiftc "$PLUGIN" \
    -Xlinker -F -Xlinker "$FW" \
    -Xlinker -rpath -Xlinker "$FW" \
    -Xlinker -rpath -Xlinker "$INTEROP"
else
  echo "Testing.framework not found under $DEV_DIR — assuming a full toolchain that wires it automatically."
  exec swift test
fi
