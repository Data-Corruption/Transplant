# Contributing

Thanks for looking <3 Read [AGENTS.md](AGENTS.md) and the
[architecture notes](docs/ARCHITECTURE.md) before changing code.

Transplant is a small front end for a checkout's own Sprout cutter. Keep feature
expansion and source rewriting in Sprout, and keep wizard answers out of the
application database. Open an issue before adding a new workflow or dependency.

Develop on Linux or WSL with the Go version in `go.mod`, Git, Bash, curl, and GCC.
Run `./scripts/test.sh`, `go vet ./...`, and `GOOS=windows go vet ./...`. For
wizard or contract changes, also run the real-checkout compatibility test in
the README. Tests use the standard `testing` package and real subprocesses.

Keep errors explicit and refuse unfamiliar contracts, dirty checkouts, and
symlinks at paths we edit. Never silently repair or reset someone's checkout.
Use casual, clear prompts and provide flags for every answer.

Changes to installation or release behavior need the inherited release and
lifecycle harnesses too; see `docs/MAINTENANCE.md`. Do not move or rename
`.github/workflows/release.yml`: its path is the signing identity.

Bug reports are most useful with the exact command, error, OS, Go version,
Transplant version, and template commit. Keep security reports private through
the repository's Security tab rather than a public issue.

Contributions use the repository's [MIT license](LICENSE.md).
