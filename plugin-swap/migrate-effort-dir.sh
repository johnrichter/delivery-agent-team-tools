#!/usr/bin/env bash
# migrate-effort-dir.sh -- renames a repo's effort-dir convention directory
# from .pbwt/ to .dat/. Idempotent: a repeat run against an already-migrated
# or never-migrated root is a no-op success, not an error. .anoikis/ (the
# separate, next-gen planning successor) is never touched -- this script
# only ever reads/writes .pbwt and .dat.
set -euo pipefail

usage() {
  echo "usage: migrate-effort-dir.sh <repo-root>" >&2
  exit 2
}

[ $# -eq 1 ] || usage
root="$1"
[ -d "$root" ] || { echo "migrate-effort-dir: $root is not a directory" >&2; exit 1; }

pbwt="$root/.pbwt"
dat="$root/.dat"

if [ -e "$pbwt" ] && [ -e "$dat" ]; then
  echo "migrate-effort-dir: both $pbwt and $dat exist -- refusing to clobber either. Resolve manually." >&2
  exit 1
fi

if [ -e "$pbwt" ]; then
  mv "$pbwt" "$dat"
  echo "migrate-effort-dir: renamed $pbwt -> $dat"
elif [ -e "$dat" ]; then
  echo "migrate-effort-dir: $dat already present, nothing to migrate (idempotent no-op)"
else
  echo "migrate-effort-dir: neither .pbwt nor .dat present under $root, nothing to migrate"
fi

if [ -e "$root/.anoikis" ]; then
  echo "migrate-effort-dir: $root/.anoikis present -- left untouched (next-gen successor, out of scope)"
fi
