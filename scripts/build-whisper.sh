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

CMAKE_FLAGS="-DCMAKE_BUILD_TYPE=Release -DBUILD_SHARED_LIBS=OFF \
  -DWHISPER_BUILD_EXAMPLES=OFF -DWHISPER_BUILD_TESTS=OFF -DWHISPER_BUILD_SERVER=OFF"

VULKAN=0
if [ "$(uname -s)" = "Linux" ] && [ -f /usr/include/vulkan/vulkan.h ] && { command -v glslc >/dev/null || command -v glslangValidator >/dev/null; }; then
  CMAKE_FLAGS="$CMAKE_FLAGS -DGGML_VULKAN=ON"
  VULKAN=1
fi

echo "cmake flags: $CMAKE_FLAGS"
cmake -S "$W" -B "$W/build" $CMAKE_FLAGS
cmake --build "$W/build" --config Release --target whisper --parallel

mkdir -p "$W/dist/lib"
find "$W/build" \( -name 'libwhisper.a' -o -name 'libggml*.a' \) -exec cp {} "$W/dist/lib/" \;
echo "whisper static libs -> $W/dist/lib"

rm -f "$W/dist/.vulkan"
if [ "$VULKAN" = "1" ]; then
  CORE="$W/dist/lib/libggml-core.a"
  mv "$W/dist/lib/libggml.a" "$CORE"
  ar -M >/dev/null <<EOF
CREATE $W/dist/lib/libggml.a
ADDLIB $CORE
ADDLIB $W/dist/lib/libggml-vulkan.a
SAVE
END
EOF
  touch "$W/dist/.vulkan"
  echo "Vulkan backend enabled (marker $W/dist/.vulkan; ggml-vulkan merged into libggml.a)"
fi

# Self-verify: the Go binding must compile and link against the freshly built
# libs using the CGO env from whisper-env.sh. Scoped to the binding package so
# this works on a fresh clone before the (gitignored) frontend/dist exists.
source "$ROOT/scripts/whisper-env.sh"
CGO_ENABLED=1 go build github.com/ggerganov/whisper.cpp/bindings/go/...
echo "CGO link OK: github.com/ggerganov/whisper.cpp/bindings/go"
