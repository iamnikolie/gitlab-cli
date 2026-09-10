# Contributing

Thanks for taking a look. This is a small, focused CLI — bug reports and pull
requests are welcome, and so is a plain question in an issue.

## Reporting a bug

Include the output of `gl version`, the exact command you ran, and what you
expected instead. `--verbose` dumps the API request and response to stderr,
which is usually enough to see what GitLab answered — **redact your host,
project paths and anything sensitive before pasting it.**

## Pull requests

Before opening one:

```bash
make fmt           # gofmt -w .
make vet           # go vet ./...
make test          # go test ./...
```

CI runs the same three (plus `-race`) on Linux and macOS, so a green local run
usually means a green PR.

House rules:

- **One concern per PR.** A bug fix and a refactor in the same diff take three
  times as long to review.
- **Tests for pure logic.** The suite never touches the network. When a feature
  needs the API, factor the parsing, URL building or field flattening into a
  function that can be tested without a client — that is how every existing
  command is structured.
- **Keep the output token-lean.** The default rendering exists so an agent can
  read it without burning context. New columns belong behind `--fields`, not in
  the default set.
- **Update the docs in the same commit.** Any change to the CLI surface (new
  flag, renamed subcommand, changed output) must also update `cmd/skill.md`
  (embedded in the binary, printed by `gl skill`) and `README.md`.
- **Conventional commit subjects** — `feat:`, `fix:`, `docs:`, `refactor:`,
  `test:`, `chore:`. Release notes are generated from them.

## Self-hosted GitLab

The host is per profile, so self-hosted instances are supported by design. If
something only reproduces on a self-hosted instance, say which version — the
REST API differs across releases and that is usually the answer.

## Releases

Maintainer-only. Tag and push:

```bash
git tag -a v1.2.3 -m "v1.2.3"
git push origin v1.2.3
```

GoReleaser builds archives for linux/darwin/windows on amd64 and arm64 and
publishes the GitHub release with a generated changelog.
