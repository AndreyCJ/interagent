#!/usr/bin/env bash
# Builds the macOS app bundle, signs it with a stable dev identity, and opens it.
#
# Why signing is required for dev:
#  - ScreenCaptureKit on Sequoia/Tahoe rejects unsigned / ad-hoc binaries at
#    startCapture (SCError 1003, CoreGraphicsErrorDomain kCGErrorInvalidConnection)
#    even when the TCC grant is present, and
#  - TCC screen-recording grants are keyed to the app's code signature, so an
#    ad-hoc identity (which changes on every rebuild) silently drops them.
# A self-signed identity is also insufficient on Tahoe: only an Apple-issued
# cert (free "Apple Development" from a personal team) carries a Team ID that
# TCC and the SCK daemon accept.
# See docs/adr/012-sc-content-filter-and-dev-signing.md.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP="$ROOT/build/bin/interagent.app"

# Identity selection: $IA_DEV_SIGN_IDENTITY wins (explicit override), then an
# Apple Development identity (has a Team ID, required by SCK on Tahoe), then the
# self-signed fallback. Parse "Matching identities" (not -v "valid only"): the
# cert chain may be incomplete but codesign still works with the cert+key pair.
pick_identity() {
    local apple selfsigned
    apple="$(security find-identity -p codesigning 2>/dev/null | awk -F'"' '/"Apple Development:/{print $2}' | head -1)"
    selfsigned="$(security find-identity -p codesigning 2>/dev/null | awk -F'"' '/"Interagent Dev Code Signing"/{print $2}' | head -1)"
    if [ -n "${IA_DEV_SIGN_IDENTITY:-}" ]; then
        printf '%s' "$IA_DEV_SIGN_IDENTITY"
    elif [ -n "$apple" ]; then
        printf '%s' "$apple"
    elif [ -n "$selfsigned" ]; then
        printf '%s' "$selfsigned"
    fi
}

IDENT="$(pick_identity)"
if [ -z "$IDENT" ]; then
    echo "No code-signing identity found. Create an Apple Development cert: Xcode > Settings > Accounts > Manage Certificates > + > Apple Development." >&2
    exit 1
fi

cd "$ROOT"
wails build

codesign --force --deep --sign "$IDENT" \
  --options runtime \
  --entitlements build/darwin/entitlements.plist \
  "$APP"

codesign --verify --strict --deep "$APP"
echo "Signed and verified: $APP ($IDENT)"
open "$APP"
