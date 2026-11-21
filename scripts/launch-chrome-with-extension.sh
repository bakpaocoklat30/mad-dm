#!/usr/bin/env bash

EXTENSION_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../extension" && pwd)"

echo "⚡ Launching Google Chrome with MAD-DM Extension..."
open -a "Google Chrome" --args --load-extension="$EXTENSION_DIR"
