#!/usr/bin/env bash
# verify.sh -- proves SC-DAT-FROZEN end to end against disposable fixtures
# only: never touches a live plugin checkout or a live effort directory.
#
#   1. Builds this repo's build-helpers binary and serves it from a local
#      file:// release mirror shaped like release.yml's real output.
#   2. Copies fixtures/plugin-copy into a scratch dir, drops that binary in
#      as its embedded-copy prebuilt, and runs the (real, unmodified)
#      SessionStart hook to resolve $BUILD_HELPERS the pre-swap way.
#   3. Runs the fixed-project harness (validate -> init-exec -> batch) via
#      that path and captures stdout ("BEFORE").
#   4. Applies swap-plugin.sh (twice, to prove idempotency), then re-runs the
#      hook -- now provisioning via download-script.sh -- against a fresh
#      copy of the same fixed project ("AFTER").
#   5. Byte-compares BEFORE vs AFTER for every harness command: any
#      difference is a SC-DAT-FROZEN violation.
#   6. Separately proves migrate-effort-dir.sh's .pbwt -> .dat rename is
#      correct, idempotent, and leaves .anoikis untouched.
#
# Set PLUGIN_SWAP_KEEP_TMP=1 to skip cleanup and inspect the scratch dir
# after a run (its path is printed either way).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FIXTURES="$SCRIPT_DIR/fixtures"
VERSION="0.0.0-fixture"

WORKDIR="$(mktemp -d)"
cleanup() {
  if [ "${PLUGIN_SWAP_KEEP_TMP:-0}" = "1" ]; then
    echo "verify.sh: PLUGIN_SWAP_KEEP_TMP=1 -- leaving $WORKDIR in place"
  else
    rm -rf "$WORKDIR"
  fi
}
trap cleanup EXIT
echo "verify.sh: scratch dir $WORKDIR"

fail() {
  echo "verify.sh: FAIL - $*" >&2
  exit 1
}

# --- Build the binary + a local file:// release mirror ---------------------
mkdir -p "$WORKDIR/bin"
( cd "$REPO_ROOT" && go build -trimpath -o "$WORKDIR/bin/build-helpers" ./cmd/build-helpers )

os="$(go env GOOS)"
arch="$(go env GOARCH)"
relroot="$WORKDIR/release"
reldir="$relroot/v$VERSION"
mkdir -p "$reldir"
archive_name="build-helpers_${VERSION}_${os}_${arch}.tar.gz"
tar -C "$WORKDIR/bin" -czf "$reldir/$archive_name" build-helpers
( cd "$reldir" && sha256sum "$archive_name" > checksums.txt )

# --- BEFORE: fixture plugin copy, embedded-copy provisioning path ----------
HOST_KEY="$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m)"
plugin_copy="$WORKDIR/plugin-copy"
cp -r "$FIXTURES/plugin-copy" "$plugin_copy"
mkdir -p "$plugin_copy/skills/build-with-team/build-helpers/.bin"
cp "$WORKDIR/bin/build-helpers" "$plugin_copy/skills/build-with-team/build-helpers/.bin/build-helpers-$HOST_KEY"
chmod +x "$plugin_copy/skills/build-with-team/build-helpers/.bin/build-helpers-$HOST_KEY"

run_hook() {
  local plugin_root="$1" plugin_data="$2" env_file="$3"
  : > "$env_file"
  CLAUDE_PLUGIN_ROOT="$plugin_root" CLAUDE_PLUGIN_DATA="$plugin_data" CLAUDE_ENV_FILE="$env_file" \
    bash "$plugin_root/hooks/bootstrap.sh" >/dev/null
}

run_harness() {
  local bh="$1" plan="$2" exec_out="$3"
  "$bh" validate "$plan"
  "$bh" init-exec "$plan" --slug smoke --pause none --topic tooling --at 2026-01-01T00:00:00Z | tee "$exec_out"
  "$bh" batch "$exec_out" "$plan" --max 4
}

before_project="$WORKDIR/fixed-project-before"
cp -r "$FIXTURES/fixed-project" "$before_project"

run_hook "$plugin_copy" "$WORKDIR/plugin-data-before" "$WORKDIR/env-before.sh"
# shellcheck disable=SC1091
source "$WORKDIR/env-before.sh"
[ -x "${BUILD_HELPERS:-}" ] || fail "pre-swap hook did not export a runnable BUILD_HELPERS"
before_bh="$BUILD_HELPERS"

before_out="$(run_harness "$before_bh" "$before_project/plan.json" "$before_project/execution.json")"

# --- Swap the fixture plugin copy onto the provisioned binary ---------------
bash "$SCRIPT_DIR/swap-plugin.sh" --target "$plugin_copy" --release-base-url "file://$relroot" --version "$VERSION"
[ -d "$plugin_copy/skills/build-with-team/build-helpers" ] && fail "embedded copy was not retired"

# Idempotency: a second swap must succeed cleanly and change nothing further.
bash "$SCRIPT_DIR/swap-plugin.sh" --target "$plugin_copy" --release-base-url "file://$relroot" --version "$VERSION"

# --- AFTER: same fixture plugin, provisioned-binary path, fresh project copy
after_project="$WORKDIR/fixed-project-after"
cp -r "$FIXTURES/fixed-project" "$after_project"

run_hook "$plugin_copy" "$WORKDIR/plugin-data-after" "$WORKDIR/env-after.sh"
# shellcheck disable=SC1091
source "$WORKDIR/env-after.sh"
[ -x "${BUILD_HELPERS:-}" ] || fail "post-swap hook did not export a runnable BUILD_HELPERS"
after_bh="$BUILD_HELPERS"

after_out="$(run_harness "$after_bh" "$after_project/plan.json" "$after_project/execution.json")"

# --- No observable behavior change ------------------------------------------
if [ "$before_out" != "$after_out" ]; then
  diff <(echo "$before_out") <(echo "$after_out") || true
  fail "harness output differs before/after the swap (SC-DAT-FROZEN violation)"
fi
echo "verify.sh: PASS - validate/init-exec/batch stdout is byte-identical before and after the swap"

# --- .pbwt -> .dat migration: correctness, idempotency, .anoikis untouched -
effort_root="$WORKDIR/effort-project"
mkdir -p "$effort_root/.pbwt/smoke" "$effort_root/.anoikis"
echo '{"tasks": []}' > "$effort_root/.pbwt/smoke/execution.json"
echo "marker" > "$effort_root/.anoikis/marker.txt"

bash "$SCRIPT_DIR/migrate-effort-dir.sh" "$effort_root"
[ -f "$effort_root/.dat/smoke/execution.json" ] || fail "migrate-effort-dir.sh did not produce .dat/smoke/execution.json"
[ -e "$effort_root/.pbwt" ] && fail ".pbwt still present after migration"
[ "$(cat "$effort_root/.anoikis/marker.txt")" = "marker" ] || fail ".anoikis contents were altered by the migration"

# Idempotent re-run: no error, no change.
before_dat_content="$(cat "$effort_root/.dat/smoke/execution.json")"
bash "$SCRIPT_DIR/migrate-effort-dir.sh" "$effort_root"
[ "$(cat "$effort_root/.dat/smoke/execution.json")" = "$before_dat_content" ] || fail "re-running the migration changed .dat contents"
[ -e "$effort_root/.pbwt" ] && fail "re-running the migration resurrected .pbwt"
[ "$(cat "$effort_root/.anoikis/marker.txt")" = "marker" ] || fail ".anoikis contents changed on the idempotent re-run"

echo "verify.sh: PASS - .pbwt -> .dat migration is correct, idempotent, and leaves .anoikis untouched"
echo "verify.sh: ALL CHECKS PASSED"
