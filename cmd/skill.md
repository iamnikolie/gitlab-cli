---
name: gitlab-cli
description: Use when interacting with GitLab via the gl CLI — merge requests, pipelines/CI, jobs, repo files, branches, tags, releases.
---

# gl — GitLab CLI

Agent-facing GitLab CLI. Runs from Bash; prints token-lean rendered output by
default, `--json`/`--format` for machines.

## Setup (one-time per profile)

There is no default profile — every command needs `--config <name>` (or
`GL_CONFIG`). Without it, commands exit 1 with a hint.

```bash
gl --config work config init                       # host (default gitlab.com) + token → ~/.gl/work/config.yaml
gl --config work mr list --project group/repo
```
Env alt: `GL_CONFIG=work gl mr list --project group/repo`. Token/host env:
`GL_TOKEN` (fallback `GITLAB_TOKEN`), `GL_HOST` (fallback `GITLAB_HOST`).
`--host` overrides ad-hoc. (`gl skill` and `gl --help` work without a profile.)

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

Every `mr` command that takes `<id|branch>` accepts either the MR iid (`42`)
or a source branch name (resolved to the open MR for that branch).

| Command | Key flags |
|---|---|
| `gl mr list` | `--state opened\|merged\|closed\|all`, `--author`, `--label`, `--limit` |
| `gl mr view <id\|branch>` | `--comments` |
| `gl mr create` | `--source`, `--target`, `--title`, `--description`, `--draft` |
| `gl mr update <id\|branch>` | `--title`, `--description`, `--state`, `--label`, `--target` |
| `gl mr close <id\|branch>` | Close |
| `gl mr reopen <id\|branch>` | Reopen |
| `gl mr rebase <id\|branch>` | `--skip-ci` |
| `gl mr merge <id\|branch>` | `--yes` (required), `--squash`, `--remove-source-branch` |
| `gl mr approve <id\|branch>` | — |
| `gl mr note <id\|branch> <text>` | Add a comment; `--thread` makes a resolvable thread + returns `discussion_id` |
| `gl mr note-delete <id\|branch> <note-id>...` | Delete one or more comments (requires `--yes`) |
| `gl mr discussions <id\|branch>` | List threads (`discussion_id`, resolvable/resolved, body); `--system` to include system threads |
| `gl mr reply <id\|branch> <discussion-id> <text>` | Reply into a thread |
| `gl mr resolve \| unresolve <id\|branch> <discussion-id>` | Resolve / unresolve a thread |
| `gl mr diff <id\|branch>` | Show changes (unified patch) |

### Pipelines & CI
| Command | Key flags |
|---|---|
| `gl pipeline list` | `--ref`, `--status` |
| `gl pipeline status [id]` | Latest for `--ref` when id omitted |
| `gl ci run` | `--ref` (required), `--var KEY=VAL` (repeatable) — create/run a pipeline |
| `gl job list <pipeline-id>` | — |
| `gl job trace <job-id>` | `--follow` (stream until done; exit 1 if the job fails) |
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
| `--config <name>` | **Required.** Use `~/.gl/<name>/` profile (env `GL_CONFIG`); no default profile |
| `--host <host>` | Override profile host |
| `--project <path-or-id>` | Project for project-scoped commands |
| `--format table\|json\|csv\|tsv` | Output format (default rendered) |
| `--json` | Alias for `--format json` (full raw object) |
| `--fields a,b,...` | Columns for table/csv/tsv (default: a curated set) |
| `--verbose` | Dump API request/response to stderr |
| `--yes` | Confirm destructive operations |

## Destructive operations

`mr merge`, `branch delete`, `tag delete`, `job cancel` require `--yes`:
```bash
gl branch delete old-feature --project group/repo --yes
gl mr merge 42 --project group/repo --yes --squash --remove-source-branch
```

## Token-efficient output

List/view commands print a **curated subset** of columns by default, not every
API field (GitLab objects have 50+ fields). To change the columns:

```bash
gl mr list --project group/repo --fields iid,title,state,author.username
```

- Dotted paths pull one level out of a nested object: `author.username`,
  `commit.short_id`.
- `--fields` applies to `table`, `csv`, and `tsv`.
- `--json` / `--format json` always emits the **full raw object** (escape hatch
  when you need a field not in the curated set).

`gl mr diff` prints readable unified patches (use `--json` for the raw diffs
array).

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

**Work review threads:**
```bash
gl mr discussions 42 --project group/repo                 # list threads + discussion_id
gl mr note 42 --project group/repo --thread "Please fix"  # resolvable thread, returns discussion_id
gl mr reply 42 <discussion-id> "Done" --project group/repo
gl mr resolve 42 <discussion-id> --project group/repo
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

**Run a pipeline and watch it:**
```bash
gl ci run --project group/repo --ref my-branch --var DEPLOY=1
gl pipeline status --project group/repo --ref my-branch      # get the pipeline id
gl job list <pipeline-id> --project group/repo               # get a job id
gl job trace <job-id> --project group/repo --follow          # stream until done; exit 1 if it fails
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
