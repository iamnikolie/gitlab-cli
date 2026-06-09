# gitlab-cli (`gl`)

An agent-facing GitLab CLI. Replaces `glab` / GitLab MCP in Claude Code and
agent workflows: the agent runs `gl <command>` from Bash and reads token-lean
output (rendered tables/KV by default, `--json`/`--format` for machines).

Binary name is `gl` — no PATH conflict with `glab`. Removing an existing `glab`
is a separate manual step (e.g. `brew uninstall glab`); this project does not
touch it.

## Install

```bash
git clone git@github.com:langgerone/gitlab-cli.git
cd gitlab-cli
make install        # builds and symlinks ~/.local/bin/gl -> ./gl
```

Or build directly:

```bash
make build          # produces ./gl
```

## Setup

```bash
gl config init
```

Prompts for the GitLab host (default `gitlab.com`) and a personal access token
(`glpat-...`). Config is saved to `~/.gl/config.yaml` (mode 0600).

Alternatively, use env vars:

```bash
export GL_TOKEN=glpat-xxxxxxxx        # fallback: GITLAB_TOKEN
export GL_HOST=gitlab.company.com     # fallback: GITLAB_HOST; default gitlab.com
```

### Multiple profiles

Each profile gets its own subdirectory under `~/.gl/`:

```bash
gl --config work config init
gl --config work mr list --project group/repo
```

Env alternative: `GL_CONFIG=work gl ...`.

`--host gitlab.company.com` overrides the profile host ad-hoc (highest
precedence).

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
- `gl api <METHOD> <path>` — raw REST v4 (`-f key=val` form fields, `--data` raw JSON body).
- `gl api graphql -f query=...` — GraphQL escape hatch.
- `gl me` — current user.
- `gl skill` — print the embedded skill document.

### Merge requests
- `gl mr list` — `--state opened|merged|closed|all`, `--author`, `--label`, `--limit`.
- `gl mr view <iid>` — `--comments`.
- `gl mr create` — `--source`, `--target`, `--title`, `--description`, `--draft`.
- `gl mr update <iid>` — `--title`, `--description`, `--state`, `--label`, `--target`.
- `gl mr merge <iid>` — `--yes` (required), `--squash`, `--remove-source-branch`.
- `gl mr approve <iid>`.
- `gl mr note <iid> <text>`.
- `gl mr diff <iid>`.

### Pipelines & CI
- `gl pipeline list` — `--ref`, `--status`.
- `gl pipeline status [id]` — latest for `--ref` when id omitted.
- `gl job list <pipeline-id>`.
- `gl job trace <job-id>`.
- `gl job retry <job-id>`.
- `gl job cancel <job-id>` — `--yes` (required).
- `gl ci lint [file]` — lints `.gitlab-ci.yml` (default: file in cwd).

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
| `--config <name>` | Use `~/.gl/<name>/` profile (env `GL_CONFIG`) |
| `--host <host>` | Override profile host |
| `--project <path-or-id>` | Project for project-scoped commands |
| `--format table\|json\|csv\|tsv` | Output format (default rendered) |
| `--json` | Alias for `--format json` |
| `--verbose` | Dump API request/response to stderr |
| `--yes` | Confirm destructive operations |

## Notes

- Retries on HTTP 429 with exponential backoff.
- `--limit` caps total results; the client paginates (`per_page`/`page`) until
  the limit or exhaustion. A stderr note prints when the limit is hit.
- Not-found responses exit 1 (not silent success).
- Destructive commands (`mr merge`, `branch delete`, `tag delete`, `job cancel`)
  require `--yes`.
