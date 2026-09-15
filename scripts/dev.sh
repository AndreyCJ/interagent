#!/usr/bin/env bash
# Starts the Wails dev server with the whisper.cpp CGO env exported.
#
# The upstream binding (third_party/whisper.cpp/bindings/go, ADR-005) carries
# only -l flags in its #cgo directives — the -I/-L paths come from env
# (scripts/whisper-env.sh). Run this instead of a bare `wails dev`:
#   ./scripts/dev.sh -tags "webkit2_41"
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [ ! -e "$ROOT/third_party/whisper.cpp/dist/lib/libwhisper.a" ]; then
    echo "whisper.cpp libs not built yet — running ./scripts/build-whisper.sh ..."
    ./scripts/build-whisper.sh
fi

# shellcheck disable=SC1091
source scripts/whisper-env.sh

exec wails dev "$@"