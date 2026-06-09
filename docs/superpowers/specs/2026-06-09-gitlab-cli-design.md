# gitlab-cli (`gl`) — design

Date: 2026-06-09
Status: approved (design), pending implementation plan

## Purpose

An agent-facing GitLab CLI, modeled 1:1 on `fibery-cli`. It replaces the official
`glab` in Claude Code / agent workflows: instead of calling an MCP server or
`glab`, the agent runs `gl <command>` from Bash and reads token-lean output.

Primary differentiators over `glab`:

- **Agent-facing output** — rendered table/KV by default, `--json`/`--format`
  for machines, stderr signals on truncation.
- **`gl skill`** — prints an embedded skill document for use as Claude context.
- **Clean multi-instance / multi-token** — named profiles (fibery model), one
  host + token per profile.

Binary name is `gl` (not `glab`) — no PATH conflict. Removing the user's existing
`glab` is a separate manual step; this project does not touch it.

## Non-goals (v1)

- Issues domain (deferred to a later iteration).
- Git-remote auto-detection of project or instance (explicit `--project` always).
- OS keyring token storage (plain file, mode 0600).
- Schema cache (GitLab REST v4 is statically typed; no dynamic schema to cache).

## Architecture

Go + cobra. Layout mirrors `fibery-cli`:

```
main.go                 -> cmd.Execute()
cmd/
  root.go               root command, persistent flags, PersistentPreRunE (load config, build client)
  config.go             gl config init
  api.go                gl api <METHOD> <path>  (+ gl api graphql)
  me.go                 gl me
  mr.go                 gl mr ... (subcommands)
  ci.go                 gl pipeline ... / gl job ... / gl ci lint
  repo.go               gl file / branch / commit / tag / release / project
  skill.go + skill.md   gl skill (embed)
internal/
  config/               profile load/save, env override, --config subdir model
  client/               HTTP client: base URL, PRIVATE-TOKEN, 429 retry, pagination, verbose dump
  render/               table/KV/json/csv/tsv (ported from fibery)
Makefile                build / install (symlink ~/.local/bin/gl) / test / vet
README.md
```

Dropped from fibery: `internal/cache` and all schema-sync logic.

Module path: `github.com/langgerone/gitlab-cli`.

### Code organization decision

Flat `cmd/` package, one file per domain (approach A). Domain grouping
(`gl mr list`) via cobra parent/child commands inside each domain file.
Rejected alternative: per-domain `internal/gitlab/{mr,ci,repo}` packages —
more boilerplate, diverges from fibery, unnecessary for v1 surface.

## Auth / config (key feature)

Named profiles. **No default profile** (revised 2026-06-09, post-implementation):
every command requires `--config <name>` (or `GL_CONFIG`); without it the command
exits 1 with a hint. `gl skill` and `gl --help` are the only profile-free commands.

- Named profile → `~/.gl/<name>/config.yaml`, selected via `--config <name>`
  (env `GL_CONFIG`).
- File mode 0600; directory mode 0700.

Profile schema:

```yaml
host: gitlab.company.com     # default: gitlab.com
token: glpat-xxxxxxxx
```

Resolution:

- `BaseURL()` → `https://<host>/api/v4`.
- Env overrides (take precedence over file): `GL_TOKEN` (fallback
  `GITLAB_TOKEN`), `GL_HOST` (fallback `GITLAB_HOST`).
- `--host` flag overrides the profile host (ad-hoc, highest precedence).
- Auth header: `PRIVATE-TOKEN: <token>`.
- `Validate()` errors when token is missing, with a hint to run
  `gl config init` or set `GL_TOKEN`.

`gl config init` prompts for host (default `gitlab.com`) and token, writes to the
selected profile (`~/.gl/<name>/config.yaml`). It requires `--config <name>` (so
it knows which profile to write) but skips token validation in
`PersistentPreRunE`.

## Project context

Always explicit. Every project-scoped command takes:

- `--project group/sub/repo`, or
- `--project 12345` (numeric ID).

The path form is URL-encoded for the API (`group/repo` → `group%2Frepo`). When a
project-scoped command is run without `--project`, it exits 1 with a hint.

## Command surface (v1)

### Plumbing (always)

| Command | Notes |
|---|---|
| `gl config init` | Write host + token to profile |
| `gl api <METHOD> <path>` | Raw REST v4. `-f key=val` form fields, `--data` raw body |
| `gl api graphql -f query=...` | GraphQL escape hatch |
| `gl me` | Current user (`/user`) |
| `gl skill` | Print embedded skill.md |

### Merge Requests

| Command | Key flags |
|---|---|
| `gl mr list` | `--state opened\|merged\|closed\|all`, `--author`, `--label`, `--limit` |
| `gl mr view <iid>` | `--comments` (include notes) |
| `gl mr create` | `--source`, `--target`, `--title`, `--description`, `--draft` |
| `gl mr update <iid>` | `--title`, `--description`, `--state`, `--label`, `--target` |
| `gl mr merge <iid>` | `--yes` (required), `--squash`, `--remove-source-branch` |
| `gl mr approve <iid>` | — |
| `gl mr note <iid> <text>` | Add a comment |
| `gl mr diff <iid>` | Show changes |

### Pipelines & CI

| Command | Key flags |
|---|---|
| `gl pipeline list` | `--ref`, `--status` |
| `gl pipeline status [id]` | Latest for ref when id omitted |
| `gl job list <pipeline-id>` | — |
| `gl job trace <job-id>` | Print job log |
| `gl job retry <job-id>` | — |
| `gl job cancel <job-id>` | `--yes` (required) |
| `gl ci lint [file]` | Lint `.gitlab-ci.yml` (default: file in cwd) |

### Repo & files

| Command | Key flags |
|---|---|
| `gl file get <path>` | `--ref` (branch/tag/sha, default default-branch) |
| `gl branch list\|create\|delete` | delete requires `--yes` |
| `gl commit list\|view` | `--ref`, `<sha>` for view |
| `gl tag list\|create\|delete` | delete requires `--yes` |
| `gl release list\|view\|create` | — |
| `gl project search <q>\|view` | view uses `--project` |

## Persistent flags

| Flag | Description |
|---|---|
| `--config <name>` | **Required.** Use `~/.gl/<name>/` profile (env `GL_CONFIG`); no default profile |
| `--host <host>` | Override profile host |
| `--project <path-or-id>` | Project for project-scoped commands |
| `--format table\|json\|csv\|tsv` | Output format (default rendered) |
| `--json` | Alias for `--format json` |
| `--verbose` | Dump API request/response JSON to stderr |
| `--yes` | Confirm destructive operations |

## Client / behavior

- HTTP client: base `https://<host>/api/v4`, `PRIVATE-TOKEN` header.
- Retry on HTTP 429 with backoff (mirror fibery's retry approach).
- Pagination helper: `--limit` caps total; client requests pages
  (`per_page`/`page`) until limit or exhaustion.
- `SilenceUsage: true` on root — error messages carry recovery hints; usage
  output is noise for agents.
- Not-found → exit 1 (not silent success).
- List commands print a stderr signal when results hit `--limit` (more may exist).
- Destructive commands (`mr merge`, `branch delete`, `tag delete`, `job cancel`)
  require `--yes`.
- `--verbose` dumps request and response JSON to stderr.

## Render

Port `internal/render` from fibery:

- Default: rendered table for lists, KV for single objects (token-lean).
- `--json` / `--format json`: raw JSON passthrough.
- `--format csv` / `--format tsv`: tabular export.

Field projection (added 2026-06-09, post-implementation): GitLab REST objects
carry 50+ fields, so list/view commands project to a **curated default column
set** before rendering table/csv/tsv. `--fields a,b,author.username` overrides
the set (dotted paths flatten one level of a nested object); `--json` always
emits the full raw object. `render` decodes numbers with `UseNumber` (integer
IDs print verbatim, not `5.9e+06`) and sanitizes table cells (collapse
newlines, escape `|`). `gl mr diff` prints unified patches, not a table.

## Skill + docs

- `gl skill` prints embedded `cmd/skill.md`. Frontmatter:
  `name: gitlab-cli`, `description: Use when interacting with GitLab via the gl CLI — merge requests, pipelines/CI, jobs, repo files, branches, tags, releases.`
- `README.md`: install, setup, multi-profile, command reference, flags, notes.
- cobra `--help` on every command and subcommand.

## Testing

- Per-command `_test.go` in `cmd/` (cobra command wiring, flag parsing).
- `internal/*` unit tests; client tested against `httptest.Server`.
- `go test ./...`, `go vet ./...`.
- `Makefile`: `build`, `install` (symlink to `~/.local/bin/gl`), `uninstall`,
  `test`, `vet`.

## Install / replacing glab

```bash
make install     # symlink ~/.local/bin/gl -> ./gl
```

`gl` and `glab` do not collide on PATH. To stop using `glab`, the user removes it
manually (e.g. `brew uninstall glab`); out of scope for this project.
