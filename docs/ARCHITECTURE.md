# How Transplant Fits Together

Transplant is a CLI-only Sprout application with update discovery and manual
signed update application. There is no worker, HTTPS dashboard, or unattended
update application. Its default action is a setup wizard for a separate, fresh
Sprout checkout.

## The wizard

`cmd/main.go` adds `internal/transplant.Flags()` to the root command and invokes
`Wizard.Run` for the default action. Update, config, uninstall, build-variable,
and installer migration paths retain their inherited behavior. Windows rejects
the wizard before application initialization and points the user to WSL.

`internal/transplant` owns choices, validation, subprocess orchestration, and
small post-cut file edits. It first checks a clean Git root, `module sprout`,
the executable cutter, development tools and Go version, then asks the cutter
for its versioned JSON contract. Names and prerequisites must match exactly.
There is no fallback that parses human help or guesses what a new feature does.

The wizard gathers answers through a persistent `pkg/xterm/prompt.Reader`,
which supports piped lines, EOF, and context cancellation. Every answer has a
flag; `--yes` uses defaults and accepts the branch, finalization, and commit.
`--preview` creates no branch and changes no checkout files. The setup branch
is offered early but created only after the final confirmation.

The checkout's own `scripts/cut` prints the preview and performs finalization.
Transplant passes the direct cut choices and leaves prerequisite expansion to
the cutter. It then replaces exact project-block assignments, replaces README
with a stub, preserves the original license and adds the new notice, and handles
the selected docs pruning. Headless services have a description but no port
assignment; service fallbacks outside the project block are never edited.

Tests and a dev build run before the offered commit. A failure stops the flow
and leaves the checkout for review. No automatic reset, clean, or rollback is
attempted. Linux child commands get their own process group so cancellation
stops their descendants too. Wizard answers are never persisted in SQLite.

## The inherited app

Every ordinary installed invocation still creates an `App`, resolves its
per-user layout, holds a shared lifecycle lock, records a PID marker, opens
SQLite in WAL mode, and loads configuration. Cleanup runs in reverse order.
The database stores framework configuration and cached update observations.
Background update checks default to disabled for this infrequently used tool;
manual `transplant update` still works, and users may opt in to background checks.

Install/update/uninstall remain installer-owned transactions. The binary only
admits a verified detached installer; it does not replace its own executable.
Migration remains restricted to the installer's authorized `--migrate` path.
The precise protocol is in [the maintenance notes](MAINTENANCE.md).

## Verification

`./scripts/test.sh` runs the race-enabled Go suite. Wizard tests cover feature
mappings, contract rejection, exact edits, piped input, Git behavior, failure
recovery, and cancellation using real subprocesses and temporary repositories.

With `TRANSPLANT_SPROUT` pointing at a Sprout checkout, the compatibility canary
copies its current source into three fresh repositories and runs the wizard,
tests, and build for HTTPS/auto, CLI/no-update, and headless/manual. It also
previews CLI/manual to check transitive automatic-feature removal. CI obtains
Sprout main and gates publication on this canary alongside the inherited
Windows and lifecycle jobs. Sprout's own cut matrix owns exhaustive source-shape
coverage; its lifecycle harness owns mirror and update transaction behavior.

The Linux E2E and release jobs each provision Incus through the shared
`setup-incus` action on Ubuntu 24.04. The release job tests changed installers
against the staged release (and the current release, when present) before
publishing the installers or promoting the version. Jobs have separate runners,
so the earlier E2E job's daemon is not available during publication.
