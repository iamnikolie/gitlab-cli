---
name: gitlab-cli
description: Use when interacting with GitLab via the gl CLI — merge requests, pipelines/CI, jobs, repo files, branches, tags, releases.
---

# gl — GitLab CLI

Agent-facing GitLab CLI. Runs from Bash; prints token-lean rendered output by
default, `--json`/`--format` for machines.

## Setup (one-time per profile)

```bash
gl config init          # prompts for host (default gitlab.com) + token → ~/.gl/config.yaml
```

Multiple profiles use subdirectories:
```bash
gl --config work config init
gl --config work mr list --project group/repo
```
Env alt: `GL_CONFIG=work gl ...`. Token/host env: `GL_TOKEN` (fallback
`GITLAB_TOKEN`), `GL_HOST` (fallback `GITLAB_HOST`). `--host` overrides ad-hoc.

## Project context

Every project-scoped command needs `--project`:
- `--project group/sub/repo` (path, URL-encoded automatically), or
- `--project 12345` (numeric ID).

Without `--project`, project-scoped commands exit 1 with a hint.

## Command reference

### Plumbing
| Command | Description |
|---|---|
| `gl config init` | Write host + token to profile |
| `gl api <METHOD> <path>` | Raw REST v4. `-f key=val` form fields, `--data` raw JSON body |
| `gl api graphql -f query=...` | GraphQL escape hatch |
| `gl me` | Current user |
| `gl skill` | Print this reference |

### Merge requests
| Command | Key flags |
|---|---|
| `gl mr list` | `--state opened\|merged\|closed\|all`, `--author`, `--label`, `--limit` |
| `gl mr view <iid>` | `--comments` |
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
| `gl pipeline status [id]` | Latest for `--ref` when id omitted |
| `gl job list <pipeline-id>` | — |
| `gl job trace <job-id>` | Print job log |
| `gl job retry <job-id>` | — |
| `gl job cancel <job-id>` | `--yes` (required) |
| `gl ci lint [file]` | Lint `.gitlab-ci.yml` (default: file in cwd) |

### Repo & files
| Command | Key flags |
|---|---|
| `gl file get <path>` | `--ref` (default: default branch) |
| `gl branch list` | — |
| `gl branch create <name>` | `--ref` (source, default default branch) |
| `gl branch delete <name>` | `--yes` (required) |
| `gl commit list` | `--ref` |
| `gl commit view <sha>` | — |
| `gl tag list` | — |
| `gl tag create <name>` | `--ref` (required) |
| `gl tag delete <name>` | `--yes` (required) |
| `gl release list` | — |
| `gl release view <tag>` | — |
| `gl release create <tag>` | `--name`, `--description`, `--ref` |
| `gl project search <q>` | `--limit` |
| `gl project view` | uses `--project` |

## Global flags
| Flag | Description |
|---|---|
| `--config <name>` | Use `~/.gl/<name>/` profile (env `GL_CONFIG`) |
| `--host <host>` | Override profile host |
| `--project <path-or-id>` | Project for project-scoped commands |
| `--format table\|json\|csv\|tsv` | Output format (default rendered) |
| `--json` | Alias for `--format json` |
| `--verbose` | Dump API request/response to stderr |
| `--yes` | Confirm destructive operations |

## Destructive operations

`mr merge`, `branch delete`, `tag delete`, `job cancel` require `--yes`:
```bash
gl branch delete old-feature --project group/repo --yes
gl mr merge 42 --project group/repo --yes --squash --remove-source-branch
```

## Pagination signal

List commands print a stderr note when results hit `--limit`:
```
(showing 50 results — limit reached; pass --limit 100 for more)
```

## Typical workflows

**Review a merge request:**
```bash
gl mr list --project group/repo --state opened
gl mr view 42 --project group/repo --comments
gl mr diff 42 --project group/repo
```

**Open and merge an MR:**
```bash
gl mr create --project group/repo --source feature --target main --title "Add X" --draft
gl mr update 42 --project group/repo --title "Add X (ready)"
gl mr approve 42 --project group/repo
gl mr merge 42 --project group/repo --yes --squash --remove-source-branch
```

**Inspect CI:**
```bash
gl pipeline status --project group/repo --ref main
gl job list 9999 --project group/repo
gl job trace 12345 --project group/repo
gl ci lint --project group/repo            # lints .gitlab-ci.yml in cwd
```

**Read repo content:**
```bash
gl file get src/main.go --project group/repo --ref main
gl branch list --project group/repo
gl commit list --project group/repo --ref main
gl tag list --project group/repo
gl release list --project group/repo
```

**Raw API / GraphQL:**
```bash
gl api GET "/projects/123/issues?state=opened"
gl api POST /projects/123/labels -f name=bug -f color=#ff0000
gl api graphql -f query='{ currentUser { name } }'
```

**Output formats / debug:**
```bash
gl mr list --project group/repo --format csv
gl --verbose mr view 42 --project group/repo    # request + response to stderr
```
