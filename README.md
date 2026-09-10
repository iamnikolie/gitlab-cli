# gitlab-cli (`gl`)

[![CI](https://github.com/iamnikolie/gitlab-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/iamnikolie/gitlab-cli/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/iamnikolie/gitlab-cli.svg)](https://pkg.go.dev/github.com/iamnikolie/gitlab-cli)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

An agent-facing GitLab CLI. Replaces `glab` / GitLab MCP in Claude Code and
agent workflows: the agent runs `gl <command>` from Bash and reads token-lean
output (rendered tables/KV by default, `--json`/`--format` for machines).

Works against gitlab.com and self-hosted instances alike — the host is per
profile, so `work` on a company GitLab and `personal` on gitlab.com coexist
without env juggling.

> Unofficial, community-built tool. Not affiliated with, endorsed by, or supported by GitLab Inc.

Binary name is `gl` — no PATH conflict with `glab`. Removing an existing `glab`
is a separate manual step (e.g. `brew uninstall glab`); this project does not
touch it.

## Install

**Prebuilt binary** — download the archive for your platform from
[Releases](https://github.com/iamnikolie/gitlab-cli/releases), then:

```bash
tar xzf gitlab-cli_*_darwin_arm64.tar.gz
sudo mv gl /usr/local/bin/
```

**With Go** (1.24+):

```bash
go install github.com/iamnikolie/gitlab-cli@latest
```

**From source** — `make install` symlinks the binary, so a later `make build`
updates the installed CLI without reinstalling:

```bash
git clone https://github.com/iamnikolie/gitlab-cli.git
cd gitlab-cli
make install        # symlink → ~/.local/bin/gl
```

Check what you got with `gl version`.

## Setup

There is no default profile — every command requires `--config <name>` (or the
`GL_CONFIG` env var). Without it, commands exit 1 with a hint. (`gl skill` and
`gl --help` are the only exceptions.)

```bash
gl --config work config init
```

Prompts for the GitLab host (default `gitlab.com`) and a personal access token
(`glpat-...`). Config is saved to `~/.gl/work/config.yaml` (mode 0600).

Each profile gets its own subdirectory under `~/.gl/` and is fully isolated
(its own host + token):

```bash
gl --config work mr list --project group/repo
gl --config personal me
```

Env alternative: `GL_CONFIG=work gl mr list --project group/repo`.

Token/host can also come from the environment (still requires a profile name
to be selected): `GL_TOKEN` (fallback `GITLAB_TOKEN`), `GL_HOST` (fallback
`GITLAB_HOST`; default `gitlab.com`). `--host gitlab.company.com` overrides the
profile host ad-hoc (highest precedence).

The profile file is mode 0600 and holds the personal access token in plain text
— the same posture as `~/.aws/credentials` or a `.netrc`. Scope the token to
what you actually need (`read_api` alone is enough for every read command) and
keep it out of repositories and dotfile backups. Revoke a leaked one under
GitLab → Preferences → Access tokens.

## Project context

Every project-scoped command needs `--project`:

- `--project group/sub/repo` — path form, URL-encoded automatically.
- `--project 12345` — numeric project ID.

Without `--project`, project-scoped commands exit 1 with a hint.

## Command reference

Run `gl skill` for the full agent-facing reference, or `gl <command> --help` for
any command.

### Plumbing
- `gl config init` — write host + token to the profile.
- `gl config show` — show the active profile, host, base URL, and token state (masked).
- `gl api <METHOD> <path>` — raw REST v4 (`-f key=val` form fields, `--data`/`--data-file` raw JSON body, `--paginate` to fetch all pages on GET). Bracketed `-f` keys are refused: GitLab drops nested form fields while still answering 2xx — send such bodies as JSON.
- `gl api graphql -f query=...` — GraphQL escape hatch.
- `gl me` — current user.
- `gl version` — version (also `gl --version`).
- `gl skill` — print the embedded skill document.

### Merge requests
Commands taking `<id|branch>` accept an MR iid or a source branch name.
- `gl mr list` — `--state opened|merged|closed|all`, `--author`, `--label`, `--limit`.
- `gl mr view <id|branch>` — `--comments`.
- `gl mr create` — `--source`, `--target`, `--title`, `--description`/`--description-file`, `--draft` (glab-style `--source-branch`/`--target-branch` accepted as aliases).
- `gl mr update <id|branch>` — `--title`, `--description`/`--description-file`, `--state`, `--label`, `--target`.
- `gl mr close|reopen <id|branch>`.
- `gl mr rebase <id|branch>` — `--skip-ci`.
- `gl mr merge <id|branch>` — `--yes` (required), `--squash`, `--remove-source-branch`.
- `gl mr approve <id|branch>`.
- `gl mr note <id|branch> <text>` — `--thread` creates a resolvable thread and returns its `discussion_id`.
- `gl mr note-delete <id|branch> <note-id>...` — delete one or more comments (requires `--yes`).
- `gl mr comment <id|branch> [text]` — inline comment anchored to a diff line: `--path` (required) plus `--line` (new file) or `--old-line` (deleted line); `--body-file` for long text. Resolves the sha triple and the old/new line pair from the API, then reads the note back and fails if the anchor was dropped.
- `gl mr discussions <id|branch>` — list threads (`discussion_id`, `type`, `path`, `line`, resolvable/resolved, body); `--system` includes system threads.
- `gl mr reply <id|branch> <discussion-id> <text>` — reply into a thread.
- `gl mr resolve | unresolve <id|branch> <discussion-id>` — resolve/unresolve a thread.
- `gl mr diff <id|branch>` — unified patch.

### Pipelines & CI
- `gl pipeline list` — `--ref`, `--status`.
- `gl pipeline status [id]` — latest for `--ref` when id omitted.
- `gl ci run` — `--ref` (required), `--var KEY=VAL` (repeatable); create and run a pipeline.
- `gl job list <pipeline-id>`.
- `gl job trace <job-id>` — `--follow` streams the log until the job finishes (exit 1 if it fails).
- `gl job retry <job-id>`.
- `gl job cancel <job-id>` — `--yes` (required).
- `gl ci lint [file]` — lints `.gitlab-ci.yml` (default: file in cwd); exits non-zero if the config is invalid.
- `gl ci run` and `gl mr create`/`gl mr note` accept `--id-only` to print just the new id (for piping).

### Repo & files
- `gl file get <path>` — `--ref`.
- `gl branch list|create|delete` — `create` takes `--ref`; `delete` requires `--yes`.
- `gl commit list|view` — `list` takes `--ref`; `view <sha>`.
- `gl tag list|create|delete` — `create` takes `--ref`; `delete` requires `--yes`.
- `gl release list|view|create` — `create` takes `--name`, `--description`, `--ref`.
- `gl project search <q>|view` — `view` uses `--project`.

## Persistent flags

| Flag | Description |
|---|---|
| `--config <name>` | **Required.** Use `~/.gl/<name>/` profile (env `GL_CONFIG`); no default profile |
| `--host <host>` | Override profile host |
| `--project <path-or-id>` | Project for project-scoped commands |
| `--format table\|json\|csv\|tsv` | Output format (default rendered) |
| `--json` | Alias for `--format json` (full raw object) |
| `--fields a,b,...` | Columns for table/csv/tsv (default: a curated set; dotted paths like `author.username` flatten one level) |
| `--verbose` | Dump API request/response to stderr |
| `--yes` | Confirm destructive operations |

## Notes

- Retries on HTTP 429 with exponential backoff.
- `--limit` caps total results; the client paginates (`per_page`/`page`) until
  the limit or exhaustion. A stderr note prints when the limit is hit.
- Not-found responses exit 1 (not silent success).
- API errors go to stderr only — stdout stays empty, so piping stdout into a
  JSON parser never sees an error object masquerading as data.
- Destructive commands (`mr merge`, `branch delete`, `tag delete`, `job cancel`)
  require `--yes`.
- List/view output is token-lean: a curated column set by default. Use
  `--fields` to change columns, or `--json` for the full raw object.
- Long text (MR/release descriptions, comments) can come from a file or stdin:
  `--description-file <path>` / `--body-file <path>` (use `-` for stdin),
  avoiding shell-quoting. Mutually exclusive with the inline form. `gl api`
  takes `--data-file <path>` for the same reason.

## Development

```bash
make test          # go test ./...
make vet           # go vet ./...
make fmt           # gofmt -w .
make build         # build ./gl, version stamped from git describe
make install       # symlink to ~/.local/bin
```

CI runs gofmt, `go vet` and `go test -race` on Linux and macOS for every push
and pull request. Tests never touch the network — they exercise pure helpers
(URL encoding, field flattening, diff-position math, config resolution).

## Contributing

Issues and pull requests are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE) © Mykola Klitovchenko
