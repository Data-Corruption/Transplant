# transplant

Give a fresh [Sprout](https://sproutcli.dev/) checkout its own home. Transplant
asks what you're building, runs that checkout's own cutter, fills in the project
settings, tidies the inherited docs, tests and builds the result, and offers a
setup commit. No GitHub token, cloning, or second templating engine involved.

It's also a working Sprout app: per-user installation, signed updates, and the
same release pipeline that your generated app can use.

## Run it

Install on Linux or in WSL:

```sh
curl -fsSL https://releases.sproutcli.dev/transplant/install.sh | sh
```

Then use [Sprout as a GitHub template](https://github.com/Data-Corruption/Sprout/generate)
and clone your new repository. From its clean checkout root:

```sh
transplant
```

Development needs Linux or WSL, Git, Bash, curl, GCC for the race tests, and Go
at least as new as the version in that checkout's `go.mod`. Windows binaries
still ship as part of the deployed Sprout example; the setup wizard points you
to WSL. Installer, config, update, and uninstall commands work on Windows.

Run `transplant update` to check for and apply a signed update. Background
checks start disabled for Transplant; `transplant update --background=true`
opts in.

## Script it

Every question has a flag. `--yes` accepts the defaults, creates the `setup`
branch, confirms finalization, and commits after verification succeeds. Pass
`--branch=false` or `--commit=false` to opt out of those steps.

```sh
transplant --yes \
  --module=github.com/YOU/YOUR_APP --app-name=your-app \
  --service=headless --update=manual \
  --release-url=https://releases.example.com/ \
  --contact-url=https://github.com/YOU/YOUR_APP \
  --author="Your Name" --year=2026 --docs=markdown
```

The module defaults to the GitHub origin URL, the binary name to a cleaned-up
repo name, and the copyright name to `git config user.name`. If a required
default isn't available, pass its flag. No answers are stored in Transplant's
database, and these update choices never change Transplant's own preferences.

| Flag | Choices / default |
| --- | --- |
| `--service` | `none`, `headless`, `https` (default) |
| `--update` | `none`, `check`, `manual` (default), `auto` |
| `--docs` | `keep`, `markdown` (default), `none` |
| `--default-log-level` | `debug`, `info`, `warn` (default), `error`, `none` |
| `--service-desc` | Default: `<app-name> service`; only with a service |
| `--service-default-port` | Default: `8484`; only with HTTPS |
| `--year` | Default: current year |
| `--preview` | Print the plan without edits, a branch, tests, or a commit |
| `--skip-verify` | Skip the generated project's tests and dev build |

`--update=none` leaves updates to the installer. `check` retains discovery and
notices, `manual` also supports applying on request, and `auto` adds unattended
application. Auto needs a service and is initially disabled in the generated
app; its operator can enable it with `<app> update --automatic=true`.

Markdown-only docs keeps `docs/content/docs/**` and `docs/MAINTENANCE.md` and
removes everything else under `docs/`. README gets a short stub; CONTRIBUTING
stays for you to edit. Your copyright notice is added while the original MIT
notice stays intact.

## If something stops halfway

Transplant stops at the failed step, leaves the changes for review, and prints
a recovery command. It never resets or cleans your checkout itself. If tests
or the build fail after the cut, fix the generated project and rerun
`./scripts/test.sh` and `./scripts/build.sh`; the cutter has already removed
itself, so rerunning Transplant isn't a repair path.

Keep the suggested setup commit message: it records the template-copy HEAD as
`Data-Corruption/Sprout@<sha>`. With GitHub's template button this is your copy's
initial commit, not necessarily a commit in upstream Sprout's history.

## Working on Transplant

See [AGENTS.md](AGENTS.md), [the architecture notes](docs/ARCHITECTURE.md), and
[the maintenance protocol](docs/MAINTENANCE.md). The [Sprout guide](https://sproutcli.dev/docs/getting-started/)
covers the template and its release system.

```sh
./scripts/test.sh
# Test pending changes in both sibling repos, or point at a clean Sprout clone:
TRANSPLANT_SPROUT=../Sprout go test ./internal/transplant \
  -run '^TestSproutCompatibility$' -v -count=1 -timeout=25m
```

The compatibility test finalizes, tests, and builds HTTPS/auto, CLI/no-update,
and headless/manual trees. CI runs it against Sprout main and requires it before
release. Sprout must support `scripts/cut --list-features-json` contract version
1; older or unfamiliar templates are refused before any checkout edits.

[MIT](LICENSE.md). Built with love and a little dirt under the fingernails <3
