#!/usr/bin/env bash
# Builds the whisper.cpp static libraries the Go binding links against.
# Prerequisite for any `go build` / `go test` after Task 7.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
W="$ROOT/third_party/whisper.cpp"

if [ ! -d "$W" ]; then
  echo "error: $W missing — run: git submodule update --init --recursive" >&2
  exit 1
fi

cmake -S "$W" -B "$W/build" -DCMAKE_BUILD_TYPE=Release -DBUILD_SHARED_LIBS=OFF \
  -DWHISPER_BUILD_EXAMPLES=OFF -DWHISPER_BUILD_TESTS=OFF -DWHISPER_BUILD_SERVER=OFF
cmake --build "$W/build" --config Release --target whisper --parallel

mkdir -p "$W/dist/lib"
find "$W/build" \( -name 'libwhisper.a' -o -name 'libggml*.a' \) -exec cp {} "$W/dist/lib/" \;
echo "whisper static libs -> $W/dist/lib"
