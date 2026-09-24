#!/usr/bin/env bash
# =============================================================================
# Installs the native library the cgo bindings link against. After running
# either mode once, `go build ./...` just works: the cgo flags search
# ffi/lib (prebuilt) first, then ../tulip_rs_ffi/target/{release,debug}
# (source build).
#
# Usage:
#   ./bootstrap.sh --source   [REF]   # the fast path (benchmark default):
#                                     # clone ../tulip_rs_ffi at REF if
#                                     # missing, then cargo build --release
#                                     # with full native CPU tuning
#   ./bootstrap.sh --prebuilt [REF]   # no Rust toolchain needed: download
#                                     # the GitHub-release cdylib for this
#                                     # GOOS/GOARCH (x86-64-v3 / aarch64
#                                     # baseline: portable, not CPU-tuned)
#   ./bootstrap.sh --help
#
# ffi/ is entirely GENERATED (headers + lib) and gitignored: --source copies
# headers from ../tulip_rs_ffi/include, --prebuilt unpacks them from the
# release tarball. Headers therefore always match the lib that is linked.
#
# REF defaults to $FFI_REF or the pinned FFI_DEFAULT_REF below.
# =============================================================================
set -euo pipefail

REPO="me60732/tulip_rs_ffi"
FFI_DEFAULT_REF="v0.2.10"

REPO_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
SIBLING="$REPO_DIR/../tulip_rs_ffi"

MODE="${1:---help}"
REF="${2:-${FFI_REF:-$FFI_DEFAULT_REF}}"

case "$MODE" in
  --source)
    if [ ! -d "$SIBLING/.git" ]; then
      echo "==> cloning $REPO@$REF into $SIBLING"
      git clone --depth 1 --branch "$REF" "https://github.com/$REPO" "$SIBLING" \
        || git clone --depth 1 --branch "$REF" "git@github.com:$REPO" "$SIBLING"
    else
      echo "==> using existing sibling checkout: $SIBLING"
    fi
    echo "==> cargo build --release (native CPU tuning from the ffi repo)"
    # MUST cd into the ffi repo: cargo reads .cargo/config.toml by walking up
    # from the CWD, not from --manifest-path. Invoked from elsewhere, the
    # `-C target-cpu=native` rustflags are silently dropped and the whole
    # build degrades to baseline x86-64 codegen.
    (cd "$SIBLING" && cargo build --release)
    echo "==> syncing generated headers from $SIBLING/include"
    mkdir -p "$REPO_DIR/ffi/include" "$REPO_DIR/ffi/lib"
    cp "$SIBLING"/include/*.h "$REPO_DIR/ffi/include/"
    echo "==> done. Verify: go build ./... && go run ./examples/adx"
    ;;

  --prebuilt)
    if ! command -v go >/dev/null 2>&1; then
      echo "error: 'go' is required to resolve GOOS/GOARCH" >&2
      exit 1
    fi
    GOOS="$(go env GOOS)"
    GOARCH="$(go env GOARCH)"
    NAME="tulip_rs_ffi-${GOOS}-${GOARCH}.tar.gz"
    URL="https://github.com/$REPO/releases/download/$REF/$NAME"
    TMP="$(mktemp)"
    trap 'rm -f "$TMP"' EXIT
    echo "==> downloading $URL"
    if ! curl -fsSL "$URL" -o "$TMP"; then
      echo "error: no prebuilt '$NAME' for $REF available yet." >&2
      echo "       prebuilds exist from the first tagged ffi release onward;" >&2
      echo "       use --source (or a newer REF) until then." >&2
      exit 1
    fi
    mkdir -p "$REPO_DIR/ffi"
    # tarball layout: ./lib/libtulip_rs_ffi.{so,dylib} + ./include/*.h —
    # extracting both keeps ffi/include in sync with the downloaded lib.
    tar xzf "$TMP" -C "$REPO_DIR/ffi"
    echo "==> installed $REPO_DIR/ffi/lib (prebuilt $REF, $GOOS/$GOARCH)"
    echo "==> verify: go build ./... && go run ./examples/adx"
    ;;

  --help | -h | *)
    sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
    exit 1
    ;;
esac
