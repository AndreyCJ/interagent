#!/usr/bin/env bash
# Downloads the STT models (whisper ggml-base + Silero VAD) into the app's
# models directory and verifies their SHA-256 against
# internal/adapter/models/checksums.txt — the same source the Go downloader
# uses (app.go DownloadSTTModel / usecase.Models).
#
# The app loads models from os.UserConfigDir()/interagent/models, which on macOS
# is ~/Library/Application Support/interagent/models. Without these files the
# whisper stream fails at startup ("cannot load model") and nothing reaches the
# chat panel. See ADR-005 (local STT).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODELS_DIR="${IA_MODELS_DIR:-$HOME/Library/Application Support/interagent/models}"
CHECKSUMS="$ROOT/internal/adapter/models/checksums.txt"

fetch() {
    local url="$1" file="$2" want="$3"
    local dest="$MODELS_DIR/$file"
    if [ -f "$dest" ] && [ "$(shasum -a 256 "$dest" | awk '{print $1}')" = "$want" ]; then
        echo "OK   $file (already present)"
        return
    fi
    echo "GET  $file"
    curl -fL --retry 3 -C - -o "$dest" "$url"
    local got
    got="$(shasum -a 256 "$dest" | awk '{print $1}')"
    if [ "$got" != "$want" ]; then
        echo "checksum mismatch for $file: got $got want $want" >&2
        rm -f "$dest"
        exit 1
    fi
    echo "OK   $file ($got)"
}

mkdir -p "$MODELS_DIR"

while read -r name sha; do
    case "$name" in
        ggml-base.bin)
            fetch "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin" "$name" "$sha"
            ;;
        ggml-silero-v6.2.0.bin)
            fetch "https://huggingface.co/ggml-org/whisper-vad/resolve/main/ggml-silero-v6.2.0.bin" "$name" "$sha"
            ;;
        ggml-tiny.bin)
            fetch "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin" "$name" "$sha"
            ;;
        ggml-small.bin)
            fetch "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin" "$name" "$sha"
            ;;
    esac
done < "$CHECKSUMS"

echo "Done: $MODELS_DIR"
