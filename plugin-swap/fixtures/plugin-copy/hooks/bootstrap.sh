#!/usr/bin/env bash
# plugin-swap fixture: a disposable copy of delivery-agent-team's
# SessionStart hook, trimmed to the build-helpers provisioning block. Block
# 1 (the Python-venv provisioning for resign-commits) is out of scope for
# the build-helpers swap this fixture exercises and is omitted; PIN is kept
# as a plain variable so the shared watchPaths line at the end of Block 2
# still has a value to report, matching the real hook's shape.
set -euo pipefail

TOOL=delivery-agent-team
PIN="$CLAUDE_PLUGIN_ROOT/hooks/requirements.txt"

# --- Block 2: build-helpers Go binary provisioning --------------------------
# Ship a committed per-host-key prebuilt (.bin/build-helpers-<os>-<arch>, LFS)
# named by lowercased `uname -s`-`uname -m` and selected by exact match — so
# selection is a filename lookup, never OS/format inference. Acquire order:
# (i) that prebuilt if it is a materialized binary that actually runs,
# (ii) local `go build` from source, (iii) fail safe with guidance.
GO_SRC_DIR="$CLAUDE_PLUGIN_ROOT/skills/build-with-team/build-helpers"
GO_MOD="$GO_SRC_DIR/go.mod"
BIN_DIR="$CLAUDE_PLUGIN_DATA/bin"
BIN_OUT="$BIN_DIR/build-helpers"

# Host key = lowercased `uname -s`-`uname -m`; the committed prebuilt for this
# host is named to match exactly, so a wrong-OS binary can never be selected.
HOST_KEY="$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m)"
COMMITTED_BIN="$GO_SRC_DIR/.bin/build-helpers-$HOST_KEY"

# sha256 of a file, preferring sha256sum (Linux) then shasum (macOS); empty
# when neither tool nor the file is present.
sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" 2>/dev/null | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" 2>/dev/null | awk '{print $1}'; fi
}

# Stamp key: committed-binary content hash + go.mod hash + host key — any
# change (new committed build, edited source, different host) invalidates the
# stamp and re-provisions. watchPaths (below) covers the file-change side.
STAMP="$BIN_DIR/.stamp"
STAMP_KEY="$(sha256_of "$COMMITTED_BIN")|$(sha256_of "$GO_MOD")|$HOST_KEY"

if [ ! -f "$STAMP" ] || [ "$(cat "$STAMP" 2>/dev/null)" != "$STAMP_KEY" ]; then
  # Clear any partial/stale provisioning before recreate.
  rm -rf "$BIN_DIR"
  mkdir -p "$BIN_DIR"

  PROVISIONED=0

  # (i) Committed prebuilt for this exact host key — usable only if it is a
  # real, materialized binary (NOT an unsmudged LFS pointer — a pointer is
  # ~130 bytes of ASCII starting "version https://git-lfs...") that actually
  # runs. A failed run falls through to the source build rather than
  # hard-failing, so a bad or missing prebuilt self-heals wherever Go is present.
  if [ -f "$COMMITTED_BIN" ] \
      && ! head -c 200 "$COMMITTED_BIN" 2>/dev/null | grep -q "^version https://git-lfs" \
      && cp "$COMMITTED_BIN" "$BIN_OUT" \
      && chmod +x "$BIN_OUT" \
      && "$BIN_OUT" --help >/dev/null 2>&1; then
    PROVISIONED=1
  fi

  # (ii) Else build from source with a local Go toolchain.
  if [ "$PROVISIONED" -eq 0 ] && command -v go >/dev/null 2>&1; then
    if (cd "$GO_SRC_DIR" && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$BIN_OUT" .) \
        && chmod +x "$BIN_OUT" \
        && "$BIN_OUT" --help >/dev/null 2>&1; then
      PROVISIONED=1
    fi
  fi

  # (iii) Else fail safe — no stamp, no export, clear guidance.
  if [ "$PROVISIONED" -eq 0 ]; then
    echo "$TOOL bootstrap: no runnable build-helpers for $HOST_KEY — no committed prebuilt and no working Go toolchain to build from source" >&2
    exit 1
  fi

  # Stamp ONLY on full success — every failure path above `exit 1`s (or leaves
  # PROVISIONED=0) before reaching this line, so a partial/broken binary is
  # never stamped and the next session re-attempts from a clean rm -rf.
  printf '%s' "$STAMP_KEY" > "$STAMP"
fi

# Reached only on a fresh full success or a prior stamped success — never on
# a failure (those exit 1 above) — so a broken/missing binary is never
# advertised via $CLAUDE_ENV_FILE.
if [ -n "${CLAUDE_ENV_FILE:-}" ]; then
  echo "export BUILD_HELPERS=$CLAUDE_PLUGIN_DATA/bin/build-helpers" >> "$CLAUDE_ENV_FILE"
fi

printf '{"watchPaths": ["%s", "%s", "%s"]}\n' "$PIN" "$COMMITTED_BIN" "$GO_MOD"
