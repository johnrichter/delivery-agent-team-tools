#!/usr/bin/env python3
"""Golden-contract-fixture capture/verification for SC-DAT-FROZEN (M12.P1.T1).

Two independent proofs, run against BOTH the canonical (ai-shared-lib) and the
plugin-embedded copy of build-helpers' main.go, so neither baseline can drift
unnoticed from the committed golden (surface.json) without this script catching it:

  1. Enumeration completeness — every dispatch-table command name (parsed straight out
     of each main.go's source, never trusted to documentation) must appear in the golden.
     A name present in a source but missing from the golden is a completeness failure —
     the exact condition that forces a pause under this task's acceptance.
  2. Help-text reproducibility — the canonical binary's `--help` output, run twice, must
     be byte-identical, and must match the committed help.txt. Wording is documentation
     and never diffed against the other baseline; only the reproducibility property is
     checked here.

Usage:
  python3 capture.py --canonical-main-go <path> --plugin-main-go <path> [--canonical-dir <path>]

Regenerating help.txt after a deliberate, reviewed canonical change:
  python3 capture.py --canonical-dir <path/to/build-helpers> --write-help
"""
from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
import tempfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
GOLDEN_PATH = HERE / "surface.json"
HELP_PATH = HERE / "help.txt"

CASE_LABEL_RE = re.compile(r'"([^"]*)"')


def func_body(src: str, signature: str) -> str:
    idx = src.index(signature)
    start = src.index("{", idx)
    depth = 0
    for i in range(start, len(src)):
        if src[i] == "{":
            depth += 1
        elif src[i] == "}":
            depth -= 1
            if depth == 0:
                return src[start : i + 1]
    raise ValueError(f"unbalanced braces reading {signature!r}")


def case_labels(body: str) -> list[str]:
    out = []
    for line in body.splitlines():
        line = line.strip()
        if not line.startswith('case "'):
            continue
        out.extend(CASE_LABEL_RE.findall(line))
    return out


def dispatch_names(main_go_path: Path) -> set[str]:
    src = main_go_path.read_text()
    main_body = func_body(src, "func main() {")
    feedback_body = func_body(src, "func runFeedback(rest []string) {")
    names = set()
    for label in case_labels(main_body):
        if label in ("-h", "--help", "help"):
            names.add("help")
        elif label == "feedback":
            continue  # fans out via the nested switch below
        else:
            names.add(label)
    for sub in case_labels(feedback_body):
        names.add(f"feedback {sub}")
    return names


def golden_names(golden: dict) -> set[str]:
    return {c["name"] for c in golden["commands"]}


def check_completeness(golden: dict, canonical_main_go: Path, plugin_main_go: Path) -> list[str]:
    """Returns a list of problems (empty = clean)."""
    problems = []
    wanted = golden_names(golden)
    for label, path in (("canonical", canonical_main_go), ("plugin-embedded", plugin_main_go)):
        found = dispatch_names(path)
        missing_from_golden = sorted(found - wanted)
        missing_from_source = sorted(wanted - found)
        if missing_from_golden:
            problems.append(f"{label} dispatch table names a command the golden never enumerates: {missing_from_golden}")
        if missing_from_source:
            problems.append(f"golden enumerates a command {label}'s dispatch table no longer has: {missing_from_source}")
    return problems


def run_help(binary: Path) -> str:
    return subprocess.run([str(binary), "--help"], capture_output=True, text=True, check=False).stdout + subprocess.run(
        [str(binary), "--help"], capture_output=True, text=True, check=False
    ).stderr


def build_and_capture_help(build_helpers_dir: Path) -> tuple[str, str]:
    """Builds the binary at build_helpers_dir and returns two independent --help captures."""
    with tempfile.TemporaryDirectory() as td:
        binary = Path(td) / "bh"
        subprocess.run(["go", "build", "-o", str(binary), "."], cwd=build_helpers_dir, check=True)
        first = run_help(binary)
        second = run_help(binary)
        return first, second


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--canonical-main-go", type=Path, help="path to the ai-shared-lib canonical build-helpers/main.go")
    ap.add_argument("--plugin-main-go", type=Path, help="path to the plugin-embedded build-helpers/main.go")
    ap.add_argument("--canonical-dir", type=Path, help="path to the canonical build-helpers/ directory (for --write-help / reproducibility)")
    ap.add_argument("--write-help", action="store_true", help="regenerate help.txt from --canonical-dir (only after a reviewed, deliberate canonical change)")
    args = ap.parse_args()

    golden = json.loads(GOLDEN_PATH.read_text())

    if args.write_help:
        if not args.canonical_dir:
            print("--write-help requires --canonical-dir", file=sys.stderr)
            return 2
        first, second = build_and_capture_help(args.canonical_dir)
        if first != second:
            print("FAIL: --help is not reproducible across two runs of the same build — refusing to write", file=sys.stderr)
            return 1
        HELP_PATH.write_text(first)
        print(f"wrote {HELP_PATH}")
        return 0

    missing = [
        name
        for name, val in (
            ("--canonical-main-go", args.canonical_main_go),
            ("--plugin-main-go", args.plugin_main_go),
            ("--canonical-dir", args.canonical_dir),
        )
        if not val
    ]
    if missing:
        # A partial (or bare) verify run must fail loudly, never skip-and-exit-0: a silent
        # no-op would false-green SC-DAT-FROZEN's forces_pause gate against incomplete capture.
        print(
            f"FAIL: verification requires {', '.join(missing)} — refusing to run a partial check "
            "(a silent skip would false-green the incomplete-enumeration forces_pause gate)",
            file=sys.stderr,
        )
        return 2

    exit_code = 0

    problems = check_completeness(golden, args.canonical_main_go, args.plugin_main_go)
    if problems:
        print("FAIL: enumeration completeness — forces_pause per SC-DAT-FROZEN:", file=sys.stderr)
        for p in problems:
            print(f"  - {p}", file=sys.stderr)
        exit_code = 1
    else:
        print(f"OK: {len(golden_names(golden))} command names agree across the golden and both baselines' dispatch tables")

    first, second = build_and_capture_help(args.canonical_dir)
    if first != second:
        print("FAIL: --help is not reproducible across two runs of the canonical binary", file=sys.stderr)
        exit_code = 1
    elif not HELP_PATH.exists():
        print(f"FAIL: {HELP_PATH} is missing — it must be committed, never regenerated by this check", file=sys.stderr)
        exit_code = 1
    elif first != HELP_PATH.read_text():
        print(f"FAIL: canonical --help no longer matches the committed {HELP_PATH.name} — regenerate with --write-help after review", file=sys.stderr)
        exit_code = 1
    else:
        print("OK: canonical --help is reproducible and matches the committed help.txt")

    return exit_code


if __name__ == "__main__":
    raise SystemExit(main())
