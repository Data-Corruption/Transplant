# Transplant

Transplant is a CLI wizard / generator for setting up
[Sprout](https://sproutcli.dev/) apps. After using the Sprout template and
cloning to your machine, run this in your clone's root and It will automate the
setup process.

It's also a working example of an app built using Sprout.

## Usage

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
=> the version in `go.mod`. Windows binaries still ship for the sake of this
being a good Sprout example.

Run `transplant update` to check for and apply updates.

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

`--update=none` leaves updates to re-running the installer. `check` retains
discovery and notices, `manual` also supports applying on request, and `auto`
adds unattended application. Auto needs a service and is initially disabled in
the generated app; its operator can enable it with
`<app> update --automatic=true`.

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

## Working on Transplant

See [AGENTS.md](AGENTS.md), [the architecture notes](docs/ARCHITECTURE.md), and
[the maintenance protocol](docs/MAINTENANCE.md). For everything else see the
[docs](https://sproutcli.dev/docs/).

```sh
./scripts/test.sh
# Test pending changes using a clean Sprout clone:
TRANSPLANT_SPROUT=path/to/sprout/clone go test ./internal/transplant \
  -run '^TestSproutCompatibility$' -v -count=1 -timeout=25m
```

The compatibility test finalizes, tests, and builds HTTPS/auto, CLI/no-update,
and headless/manual trees. CI runs it against Sprout main and requires it before
release. Sprout must support `scripts/cut --list-features-json` contract version
1; older or unfamiliar templates are refused before any checkout edits.

[MIT](LICENSE.md)
