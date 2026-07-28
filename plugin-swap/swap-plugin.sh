#!/usr/bin/env bash
# swap-plugin.sh -- moves a delivery-agent-team plugin checkout off its
# embedded build-helpers copy (Go source + vendor + prebuilt .bin/) and onto
# the provisioned binary this repo releases (SC-DAT-FROZEN). Two effects,
# applied together so a partial run never leaves the plugin without a
# runnable build-helpers:
#
#   1. Installs download-script.sh + a version pin into the target's hooks/,
#      and replaces its SessionStart hook's build-helpers provisioning block
#      with a call to that script -- same exported env var (BUILD_HELPERS),
#      same fail-safe/no-stamp-on-error contract, so the skills that read
#      $BUILD_HELPERS see no behavioral difference.
#   2. Deletes the embedded skills/build-with-team/build-helpers tree (Go
#      source, vendor, prebuilt .bin/) -- nothing left to fall back to or
#      drift from once the provisioned binary is live.
#
# Idempotent: re-running against an already-swapped target re-slices the same
# marked block and refreshes the version pin without erroring; the embedded
# tree is skipped if already absent.
#
# SAFETY: refuses a target whose path contains "marketplace/plugins/" unless
# --force-live is given. This tool is meant to run against a disposable copy
# or an isolated fixture -- swapping a live, in-use plugin checkout is a
# separate, explicitly-confirmed operation, never a side effect of testing
# this script.
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
usage: swap-plugin.sh --target DIR --release-base-url URL --version X.Y.Z [--force-live]

  --target DIR           plugin root (contains hooks/bootstrap.sh and
                          skills/build-with-team/build-helpers/)
  --release-base-url URL PF_RELEASE_BASE_URL the provisioned hook will fetch
                          from (file:// for local/test mirrors)
  --version X.Y.Z         build-helpers version to pin
  --force-live            allow a target path under "marketplace/plugins/"
                          (default: refused)
EOF
  exit 2
}

target=""
release_base_url=""
version=""
force_live=0

while [ $# -gt 0 ]; do
  case "$1" in
  --target) target="$2"; shift 2 ;;
  --release-base-url) release_base_url="$2"; shift 2 ;;
  --version) version="$2"; shift 2 ;;
  --force-live) force_live=1; shift ;;
  *) usage ;;
  esac
done

if [ -z "$target" ] || [ -z "$release_base_url" ] || [ -z "$version" ]; then
  usage
fi

target="$(cd "$target" && pwd)"

case "$target" in
*marketplace/plugins/*)
  if [ "$force_live" -ne 1 ]; then
    echo "swap-plugin: refusing '$target' -- looks like a live marketplace plugin checkout. Pass --force-live to override (not for use against the actively-running plugin)." >&2
    exit 1
  fi
  ;;
esac

hooks_dir="$target/hooks"
bootstrap="$hooks_dir/bootstrap.sh"
embedded_dir="$target/skills/build-with-team/build-helpers"

[ -f "$bootstrap" ] || { echo "swap-plugin: $bootstrap not found" >&2; exit 1; }

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# --- Install the provisioner + version pin --------------------------------
cp "$script_dir/download-script.sh" "$hooks_dir/download-script.sh"
chmod +x "$hooks_dir/download-script.sh"
printf '{"version": "%s"}\n' "$version" > "$hooks_dir/build-helpers.version.json"

# --- Splice the provisioning block into bootstrap.sh -----------------------
# Everything from the last "# --- Block 2:" marker to EOF is replaced,
# whether that marker is the original embedded-copy block or a prior run of
# this same swap (idempotent either way).
marker_line="$(grep -n '^# --- Block 2:' "$bootstrap" | tail -n1 | cut -d: -f1)"
[ -n "$marker_line" ] || { echo "swap-plugin: no '# --- Block 2:' marker found in $bootstrap" >&2; exit 1; }

head_lines=$((marker_line - 1))
tmp="$(mktemp)"
head -n "$head_lines" "$bootstrap" > "$tmp"

cat >> "$tmp" <<EOF
# --- Block 2: build-helpers binary provisioning (swapped, SC-DAT-FROZEN) ---
# Provisioned via download-script.sh -- the embedded Go source/vendor/.bin
# copy is retired. Same exported env var (BUILD_HELPERS) as before, so the
# skills that read \$BUILD_HELPERS see no behavioral difference.
DOWNLOAD_SCRIPT="\$CLAUDE_PLUGIN_ROOT/hooks/download-script.sh"
VERSION_FILE="\$CLAUDE_PLUGIN_ROOT/hooks/build-helpers.version.json"

export PF_CLI_NAME=build-helpers
export PF_PLUGIN_DATA="\$CLAUDE_PLUGIN_DATA"
export PF_RELEASE_BASE_URL="$release_base_url"
export PF_VERSION_FILE="\$VERSION_FILE"
export PF_BIN_ENV=BUILD_HELPERS
export PF_ENV_FILE="\${CLAUDE_ENV_FILE:-}"

if ! "\$DOWNLOAD_SCRIPT" >/dev/null; then
  echo "\$TOOL bootstrap: no verified build-helpers binary provisioned" >&2
  exit 1
fi

printf '{"watchPaths": ["%s", "%s", "%s"]}\n' "\$PIN" "\$VERSION_FILE" "\$DOWNLOAD_SCRIPT"
EOF

mv "$tmp" "$bootstrap"
chmod +x "$bootstrap"

# --- Retire the embedded copy ----------------------------------------------
if [ -d "$embedded_dir" ]; then
  rm -rf "$embedded_dir"
  echo "swap-plugin: retired embedded copy at $embedded_dir"
else
  echo "swap-plugin: embedded copy already retired (nothing at $embedded_dir)"
fi

echo "swap-plugin: $target now provisions build-helpers $version from $release_base_url"
