# contract-fixtures

Golden contract fixture for `build-helpers`'s CLI surface, as consumed by the
`delivery-agent-team` plugin (`build-with-team` + `plan-with-team` skills, and the
`magistrate` agent). This is the hard pre-build task for **SC-DAT-FROZEN**: the
frozen-API decomposition of `build-helpers` into `delivery-agent-team-tools`
(library → CLI → plugin) is verified against this fixture — unchanged argv, stdin
schema, stdout schema, and exit-code contract per subcommand.

## Contents

| File | Role |
| --- | --- |
| `surface.json` | the golden fixture — every subcommand: argv (positionals, flags, mutual exclusions), stdin schema, stdout schema, exit-code contract, and an `invoked_by` citation into the plugin's own docs |
| `help.txt` | the canonical (post-M0) binary's `--help` capture, byte-reproducible across two runs |
| `capture.py` | stdlib-only verification: dispatch-table enumeration completeness against both baselines, and `--help` reproducibility |

## Scope: every subcommand, cited

`surface.json` enumerates all 29 dispatch-table entries (26 top-level commands plus
`feedback`'s 3 sub-verbs) — the complete CLI surface, not a filtered subset. Each entry's
`invoked_by` field cites where the plugin calls it: `build-with-team/SKILL.md`,
`plan-with-team/SKILL.md`, or `agents/magistrate.md`, by phase/step. A command with no
callsite in either skill's documented procedure is labeled "shipped CLI surface" with a
one-line reason (e.g. superseded by a broader call, or operator/maintainer-invoked only).
Capturing the full surface — not only the subset a grep of the skill docs turns up — is
deliberate: SC-DAT-FROZEN's own risk framing is that a missed subcommand is a latent break
of the live harness, and the two skills' documented procedure is not guaranteed to be the
only path that reaches the binary.

## Baseline sources

- **Canonical**: `ai-shared-lib/go/build-helpers` (source of record; SC-MODELROSTER's argv
  note applies here — this fixture pins the **post-M0** shape).
- **Plugin-embedded**: `marketplace/plugins/delivery-agent-team/skills/build-with-team/build-helpers`
  (the copy the live harness actually executes).

Both commits are pinned in `surface.json`'s `captured_from`.

## A found divergence (not silently reconciled)

The two baselines' dispatch-table **command names** are identical (verified by
`capture.py` and by direct source diff). Two commands diverge in **flag-level and
exit-code substance**: `self-check` (canonical adds `--band`, a roster-resolved band
lookup, plus a `roster_stale` field and exit code 3 — M0.P3.T2) and `record` (canonical
refuses a write that would leave a task `done` with no commit — M0.P8's writer fail-fast).
The plugin-embedded copy still has neither change — it was extracted once at
plugin-creation time and has not been re-synced since. Both facts are verified
empirically (build + run both binaries; see `surface.json`'s `baseline_divergences`) and
recorded as a flagged finding, not fixed here — reconciling the plugin's embedded copy is
downstream work (the frozen-API decomposition itself, or an interim sync), out of this
task's scope. **This fixture's canonical entries for `self-check`/`record` pin the
post-M0 shape**, per SC-MODELROSTER's argv note.

## Regenerating / verifying

```
python3 capture.py \
  --canonical-main-go <path-to-ai-shared-lib>/go/build-helpers/main.go \
  --plugin-main-go <path-to-marketplace>/plugins/delivery-agent-team/skills/build-with-team/build-helpers/main.go \
  --canonical-dir <path-to-ai-shared-lib>/go/build-helpers
```

Checks, independently:

1. **Enumeration completeness** — every dispatch-table name in *both* main.go sources
   is present in `surface.json`, and vice versa. A mismatch exits 1 (forces_pause
   condition: a missing/mismatched subcommand fails the capture).
2. **Help reproducibility** — the canonical binary's `--help`, built and run twice, is
   byte-identical, and matches the committed `help.txt`.

`surface.json`'s own content (flags, stdin/stdout schema, exit-code semantics,
`invoked_by`) is authored documentation cross-checked against both `main.go` sources and
empirical binary runs at capture time — it goes well beyond what a source-level name
extraction alone can recover (flag requiredness, mutual exclusions, per-condition exit
codes), so `capture.py` checks enumeration and help reproducibility, not a byte-for-byte
regeneration of the whole fixture. After a deliberate, reviewed canonical change, refresh
`help.txt` with `--write-help --canonical-dir <path>` and hand-edit `surface.json`'s
affected entries; never let this script silently rewrite either on a mismatch.

Neither the golden nor `help.txt` is ever created by this script when absent — a missing
committed artifact is a hard failure, not a fallback to regenerate one.
