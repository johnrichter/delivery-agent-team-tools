# plugin-swap

Tooling that moves the delivery-agent-team plugin off its embedded
`build-helpers` copy (Go source + vendor + prebuilt `.bin/`) and onto the
binary this repo releases via `.github/workflows/release.yml`, plus the
`.pbwt/` -> `.dat/` effort-dir rename. Proven here against disposable
fixtures only -- applying `swap-plugin.sh` to the live plugin checkout, and
`migrate-effort-dir.sh` to a live effort directory, are separate,
explicitly-confirmed operations, not a side effect of running this tooling
or its verification.

## Contents

| File | Role |
| --- | --- |
| `download-script.sh` | generic per-OS/arch binary provisioner. A plugin's SessionStart hook execs a copy of this file with `PF_*` env set to its own CLI name, data dir, and release host. See its header for the full env/exit-code contract. |
| `swap-plugin.sh` | installs `download-script.sh` + a version pin into a target plugin's `hooks/`, replaces that plugin's build-helpers provisioning block with a call to it (same exported `BUILD_HELPERS` env var), and deletes the embedded `skills/build-with-team/build-helpers/` tree. Idempotent. Refuses a target under `marketplace/plugins/` unless `--force-live` is given. |
| `migrate-effort-dir.sh` | renames `<root>/.pbwt` to `<root>/.dat`. Idempotent (a no-op success on an already-migrated or never-migrated root); never touches `<root>/.anoikis` (the separate, next-gen planning successor). |
| `verify.sh` | fixture-only proof of both: byte-identical `validate`/`init-exec`/`batch` stdout on a fixed project before vs. after `swap-plugin.sh`, and a correctness + idempotency + `.anoikis`-untouched check of `migrate-effort-dir.sh`. Run: `bash plugin-swap/verify.sh`. |
| `fixtures/plugin-copy/` | disposable plugin-shaped fixture: the real (unmodified) SessionStart hook logic plus a stub embedded-copy tree, `.bin/` populated by `verify.sh` at run time (never committed) so no binary lands in this repo. |
| `fixtures/fixed-project/` | a small, deterministic 2-task plan (`validate`/`init-exec`/`batch` all pass) used as the "fixed project" `verify.sh` re-runs the harness against. |

## Applying the swap to a real plugin (deferred, out of scope here)

```sh
bash plugin-swap/swap-plugin.sh \
  --target /path/to/a/disposable/copy/of/the/plugin \
  --release-base-url https://github.com/johnrichter/delivery-agent-team-tools/releases/download \
  --version X.Y.Z
```

`--release-base-url` works unmodified against a real GitHub release: its
asset URLs already have the `<base>/v<version>/<asset>` shape
`download-script.sh` expects (see `release.yml`'s header). Run against the
live `marketplace/plugins/delivery-agent-team` checkout only as a deliberate,
separately-confirmed step -- never invoke `--force-live` as part of testing
or verifying this tooling.

## Applying the .pbwt -> .dat rename to a real repo (deferred, out of scope here)

```sh
bash plugin-swap/migrate-effort-dir.sh /path/to/repo-root
```

Safe to run more than once. Leaves `.anoikis/` alone unconditionally.
