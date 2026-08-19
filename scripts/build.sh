#!/usr/bin/env bash
# Builds the cpa-quota-panel CPA plugin for the current platform.
#
#   ./scripts/build.sh                         # native build -> plugins/<GOOS>/<GOARCH>/
#   GOOS=linux GOARCH=amd64 ./scripts/build.sh # prints the docker cross-build command
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TARGET_OS="${GOOS:-$(go env GOOS)}"
TARGET_ARCH="${GOARCH:-$(go env GOARCH)}"

case "$TARGET_OS" in
  darwin)  EXT="dylib" ;;
  linux|freebsd) EXT="so" ;;
  windows) EXT="dll" ;;
  *) echo "unsupported GOOS: $TARGET_OS" >&2; exit 1 ;;
esac

if [ "$TARGET_OS" != "$(go env GOOS)" ] || [ "$TARGET_ARCH" != "$(go env GOARCH)" ]; then
  cat >&2 <<EOM
Cross-compiling a CGO c-shared plugin is not supported reliably.
Build on the target platform, or use docker:

  docker run --rm --platform $TARGET_OS/$TARGET_ARCH -v "$ROOT":/src -w /src \
    golang:1.26 sh -c "CGO_ENABLED=1 GOOS=$TARGET_OS GOARCH=$TARGET_ARCH \\
      go build -buildmode=c-shared -o plugins/$TARGET_OS/$TARGET_ARCH/cpa-quota-panel.$EXT . \\
      && rm -f plugins/$TARGET_OS/$TARGET_ARCH/cpa-quota-panel.h"
EOM
  exit 1
fi

OUT_DIR="$ROOT/plugins/$TARGET_OS/$TARGET_ARCH"
mkdir -p "$OUT_DIR"
echo "building $OUT_DIR/cpa-quota-panel.$EXT"
( cd "$ROOT" && CGO_ENABLED=1 go build -buildmode=c-shared -o "$OUT_DIR/cpa-quota-panel.$EXT" . )
rm -f "$OUT_DIR/cpa-quota-panel.h"
echo "done"
