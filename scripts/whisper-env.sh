#!/usr/bin/env bash
# Exports the environment needed to build and test the Go code that links
# whisper.cpp (the bindings/go package used directly from the submodule,
# ADR-005; cgo flags come from env, not a forked binding).
#
# The submodule's #cgo directives carry -lwhisper -lggml-* and the platform
# libraries, but no -I/-L. Those are supplied here as absolute paths built
# relative to the repo root. LIBRARY_PATH is set in addition to -L so the -l
# flags resolve regardless of ordering with the directive lines.
#
# This file is MEANT TO BE SOURCED (do not invoke as a script), from anywhere
# inside the repo:
#   source scripts/whisper-env.sh
#   go test ./...
#
# Keep it free of `set -e`/other shell option mutations — sourcing would leak
# them into the caller's shell. The repo root is located by walking up from
# $PWD (not from BASH_SOURCE, which is 1-indexed/absent in zsh) so it works
# in bash and zsh alike.

ROOT=""
DIR="$(pwd -P)"
while [ "$DIR" != "/" ]; do
  if [ -f "$DIR/go.mod" ] && grep -q '^module interagent$' "$DIR/go.mod"; then
    ROOT="$DIR"
    break
  fi
  DIR="$(dirname "$DIR")"
done
if [ -z "$ROOT" ]; then
  echo "whisper-env: error: source from inside the interagent repo" >&2
  return 1
fi

W="$ROOT/third_party/whisper.cpp"
DIST_LIB="$W/dist/lib"

missing=""
[ -d "$W/bindings/go" ] || missing="$missing $W (git submodule update --init --recursive)"
[ -e "$DIST_LIB/libwhisper.a" ] || missing="$missing $DIST_LIB (./scripts/build-whisper.sh)"
if [ -n "$missing" ]; then
  echo "whisper-env: missing:$missing" >&2
  return 1
fi

export CGO_CPPFLAGS="-I$W/include -I$W/ggml/include"
export CGO_LDFLAGS="-L$DIST_LIB"
if [ -f "$W/dist/.vulkan" ]; then
  export CGO_LDFLAGS="$CGO_LDFLAGS -lggml-vulkan -lvulkan"
fi
export LIBRARY_PATH="$DIST_LIB"