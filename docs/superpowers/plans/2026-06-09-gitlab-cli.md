# gitlab-cli (`gl`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `gl`, an agent-facing GitLab CLI (Go + cobra) modeled 1:1 on `fibery-cli`, covering plumbing (config/api/me/skill), merge requests, pipelines/CI, and repo/files, with token-lean rendered output and named profiles.

**Architecture:** Flat `cmd/` package (one file per domain, cobra parent/child for subcommands) over three `internal/` packages: `config` (named-profile YAML load/save + env/flag overrides), `client` (HTTP client for GitLab REST v4 with `PRIVATE-TOKEN` auth, 429 retry, pagination, verbose dump, GraphQL escape hatch), `render` (table/KV/json/csv/tsv, ported verbatim from fibery). No schema cache (GitLab REST is statically typed).

**Tech Stack:** Go 1.26, `github.com/spf13/cobra`, `gopkg.in/yaml.v3`, `github.com/stretchr/testify`. Module path `github.com/langgerone/gitlab-cli`.

**Spec:** `docs/superpowers/specs/2026-06-09-gitlab-cli-design.md`

**Reference codebase:** `/Users/mako/development/influ2/fibery-cli` — port `internal/render` verbatim; mirror `internal/config`, `internal/client`, `cmd/root.go`, `cmd/config.go`, `cmd/skill.go` patterns.

**Testing strategy (mirrors fibery-cli):** Unit-test the logic-bearing code — `internal/config` (load/save/override resolution), `internal/client` (HTTP behavior against `httptest.Server`), `internal/render` (formatting), and pure `cmd/` helpers (project/path encoding, `-f` field parsing, draft/state mapping, pagination hint). Cobra command bodies that only wire flags to client calls are verified via `go build` + `go vet` + manual smoke, exactly as fibery does (fibery has no live-server command tests). Every task ends green on `go test ./...`.

---

## File Structure

```
main.go                       -> cmd.Execute()
go.mod                        module github.com/langgerone/gitlab-cli
Makefile                      build / install / uninstall / test / vet
README.md                     install, setup, multi-profile, command reference
.gitignore                    /gl, session (already present)
cmd/
  root.go                     root cmd, persistent flags, PersistentPreRunE, outputJSON, project/encode helpers
  root_test.go                tests for projectRef, encodePath helpers
  config.go                   gl config init
  api.go                      gl api <METHOD> <path>  +  gl api graphql
  api_test.go                 tests for parseFields, splitPathQuery
  me.go                       gl me
  mr.go                       gl mr list|view|create|update|merge|approve|note|diff
  mr_test.go                  tests for draftTitle, stateEvent helpers
  ci.go                       gl pipeline list|status ; gl job list|trace|retry|cancel ; gl ci lint
  ci_test.go                  tests for ci lint file resolution
  repo.go                     gl file get ; gl branch ; gl commit ; gl tag ; gl release ; gl project
  repo_test.go                tests for encodeFilePath
  skill.go                    gl skill (//go:embed skill.md)
  skill.md                    embedded skill document
internal/
  config/
    config.go                 Config{Host,Token}, BaseURL, Validate, Load, Save, glHome
    config_test.go
  client/
    client.go                 Client, do/Get/GetPaginated/Send/GraphQL, 429 retry, verbose
    client_test.go
  render/
    render.go                 JSON/List/KV/CSV/TSV (verbatim from fibery)
    render_test.go
```

Dropped from fibery: `internal/cache` and all schema-sync logic. `cmd/root.go` has NO schema-cache check in `PersistentPreRunE`.

---

## Task 1: Project scaffold

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `Makefile`
- Modify: `.gitignore` (already contains `/gl` and `session` — verify, no change needed)

- [ ] **Step 1: Initialize the Go module**

Run from repo root:

```bash
cd /Users/mako/development/langgerone/gitlab-cli
go mod init github.com/langgerone/gitlab-cli
go get github.com/spf13/cobra@v1.10.2
go get gopkg.in/yaml.v3@v3.0.1
go get github.com/stretchr/testify@v1.11.1
```

Expected: `go.mod` created with module `github.com/langgerone/gitlab-cli` and the three requires; `go.sum` populated.

- [ ] **Step 2: Write `main.go`**

```go
package main

import "github.com/langgerone/gitlab-cli/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 3: Write `Makefile`**

```makefile
.PHONY: build install uninstall test vet

BIN := gl
PREFIX ?= $(HOME)/.local

build:
	go build -o $(BIN) .

install: build
	mkdir -p $(PREFIX)/bin
	ln -sf $(CURDIR)/$(BIN) $(PREFIX)/bin/$(BIN)

uninstall:
	rm -f $(PREFIX)/bin/$(BIN)

test:
	go test ./...

vet:
	go vet ./...
```

- [ ] **Step 4: Verify `.gitignore`**

Run: `cat .gitignore`
Expected: contains `/gl` and `session`. If `/gl` is absent, add it (the build output). Already present per repo state — no change.

- [ ] **Step 5: Commit**

Note: `go build` fails until `cmd/` exists — that's expected; we commit scaffold only.

```bash
git add go.mod go.sum main.go Makefile .gitignore
git commit -m "[NO-TASK] Scaffold gitlab-cli Go module"
```

---

## Task 2: internal/config — named profiles

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

Config holds `host` + `token`. Resolution order for each field: env override → file. `--host` flag override is applied by the caller (root.go), not here. Env: `GL_TOKEN` (fallback `GITLAB_TOKEN`), `GL_HOST` (fallback `GITLAB_HOST`). Default host `gitlab.com`.

- [ ] **Step 1: Write the failing tests**

`internal/config/config_test.go`:

```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/langgerone/gitlab-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearEnv(t *testing.T) {
	t.Setenv("GL_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GL_HOST", "")
	t.Setenv("GITLAB_HOST", "")
}

func TestLoad_FromEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv("GL_TOKEN", "glpat-env")
	t.Setenv("GL_HOST", "gl.example.com")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, "glpat-env", cfg.Token)
	assert.Equal(t, "gl.example.com", cfg.Host)
}

func TestLoad_FallbackEnvNames(t *testing.T) {
	clearEnv(t)
	t.Setenv("GITLAB_TOKEN", "glpat-fallback")
	t.Setenv("GITLAB_HOST", "fallback.example.com")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, "glpat-fallback", cfg.Token)
	assert.Equal(t, "fallback.example.com", cfg.Host)
}

func TestLoad_PrimaryEnvWinsOverFallback(t *testing.T) {
	clearEnv(t)
	t.Setenv("GL_TOKEN", "primary")
	t.Setenv("GITLAB_TOKEN", "fallback")
	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, "primary", cfg.Token)
}

func TestLoad_FromFile(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	t.Setenv("GL_HOME", dir)

	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "config.yaml"),
		[]byte("host: file.example.com\ntoken: glpat-file\n"),
		0600,
	))

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, "glpat-file", cfg.Token)
	assert.Equal(t, "file.example.com", cfg.Host)
}

func TestLoad_EnvOverFile(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	t.Setenv("GL_HOME", dir)
	t.Setenv("GL_TOKEN", "envtoken")

	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "config.yaml"),
		[]byte("host: file.example.com\ntoken: glpat-file\n"),
		0600,
	))

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, "envtoken", cfg.Token)            // env wins
	assert.Equal(t, "file.example.com", cfg.Host)     // file fills in
}

func TestBaseURL_DefaultHost(t *testing.T) {
	cfg := &config.Config{Token: "x"}
	assert.Equal(t, "https://gitlab.com/api/v4", cfg.BaseURL())
}

func TestBaseURL_CustomHost(t *testing.T) {
	cfg := &config.Config{Host: "gl.example.com", Token: "x"}
	assert.Equal(t, "https://gl.example.com/api/v4", cfg.BaseURL())
}

func TestValidate(t *testing.T) {
	assert.Error(t, (&config.Config{}).Validate())
	assert.NoError(t, (&config.Config{Token: "x"}).Validate())
}

func TestSave_DefaultProfile(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	t.Setenv("GL_HOME", dir)

	require.NoError(t, config.Save("gl.example.com", "glpat-1", ""))

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "glpat-1")
	assert.Contains(t, string(data), "gl.example.com")

	info, err := os.Stat(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestSave_NamedProfileIsolated(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	t.Setenv("GL_HOME", dir)

	require.NoError(t, config.Save("default.example.com", "deftok", ""))
	require.NoError(t, config.Save("work.example.com", "worktok", "work"))

	def, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, "deftok", def.Token)
	assert.Equal(t, "default.example.com", def.Host)

	work, err := config.Load("work")
	require.NoError(t, err)
	assert.Equal(t, "worktok", work.Token)
	assert.Equal(t, "work.example.com", work.Host)

	_, err = os.Stat(filepath.Join(dir, "work", "config.yaml"))
	require.NoError(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/`
Expected: FAIL — `config` package / symbols undefined.

- [ ] **Step 3: Write `internal/config/config.go`**

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// defaultHost is used when neither file, env, nor flag set a host.
const defaultHost = "gitlab.com"

type Config struct {
	Host  string `yaml:"host"`
	Token string `yaml:"token"`
}

// BaseURL returns the REST v4 API root, e.g. https://gitlab.com/api/v4.
func (c *Config) BaseURL() string {
	host := c.Host
	if host == "" {
		host = defaultHost
	}
	return fmt.Sprintf("https://%s/api/v4", host)
}

// Validate errors when no token is configured.
func (c *Config) Validate() error {
	if c.Token == "" {
		return fmt.Errorf("token not set: run 'gl config init' or set GL_TOKEN")
	}
	return nil
}

// firstEnv returns the first non-empty environment variable among names.
func firstEnv(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}

// glHome returns the config directory for the given profile.
// Empty profile → ~/.gl (or GL_HOME for tests).
// Non-empty profile → ~/.gl/<profile>.
func glHome(profile string) (string, error) {
	var base string
	if h := os.Getenv("GL_HOME"); h != "" {
		base = h
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".gl")
	}
	if profile != "" {
		return filepath.Join(base, profile), nil
	}
	return base, nil
}

// Load reads config from the profile's directory. Env vars
// GL_TOKEN (fallback GITLAB_TOKEN) and GL_HOST (fallback GITLAB_HOST)
// override file values.
func Load(profile string) (*Config, error) {
	cfg := &Config{
		Host:  firstEnv("GL_HOST", "GITLAB_HOST"),
		Token: firstEnv("GL_TOKEN", "GITLAB_TOKEN"),
	}

	dir, err := glHome(profile)
	if err != nil {
		return nil, fmt.Errorf("config.Load: %w", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("config.Load: %w", err)
	}
	if err == nil {
		var fileCfg Config
		if parseErr := yaml.Unmarshal(data, &fileCfg); parseErr != nil {
			return nil, fmt.Errorf("config.Load: parse: %w", parseErr)
		}
		if cfg.Token == "" {
			cfg.Token = fileCfg.Token
		}
		if cfg.Host == "" {
			cfg.Host = fileCfg.Host
		}
	}

	return cfg, nil
}

// Save writes config to the profile's directory (~/.gl/<profile>/config.yaml).
// Directory mode 0700, file mode 0600.
func Save(host, token, profile string) error {
	dir, err := glHome(profile)
	if err != nil {
		return fmt.Errorf("config.Save: %w", err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("config.Save: mkdir: %w", err)
	}
	data, err := yaml.Marshal(Config{Host: host, Token: token})
	if err != nil {
		return fmt.Errorf("config.Save: marshal: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0600)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/`
Expected: PASS (all tests).

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "[NO-TASK] Add config package with named profiles"
```

---

## Task 3: internal/render — port from fibery

**Files:**
- Create: `internal/render/render.go`
- Test: `internal/render/render_test.go`

Ported verbatim from `/Users/mako/development/influ2/fibery-cli/internal/render/render.go`.

- [ ] **Step 1: Write the failing tests**

`internal/render/render_test.go`:

```go
package render_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/langgerone/gitlab-cli/internal/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestList_Table(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, render.List(&buf, json.RawMessage(`[{"id":1,"title":"a"},{"id":2,"title":"b"}]`)))
	out := buf.String()
	assert.Contains(t, out, "| id | title |")
	assert.Contains(t, out, "| 1 | a |")
	assert.Contains(t, out, "| 2 | b |")
}

func TestList_Empty(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, render.List(&buf, json.RawMessage(`[]`)))
	assert.Contains(t, buf.String(), "_No results._")
}

func TestKV(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, render.KV(&buf, json.RawMessage(`{"id":7,"title":"hi"}`)))
	out := buf.String()
	assert.Contains(t, out, "**id:** 7")
	assert.Contains(t, out, "**title:** hi")
}

func TestJSON(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, render.JSON(&buf, json.RawMessage(`{"a":1}`)))
	assert.Contains(t, buf.String(), "\"a\": 1")
}

func TestCSV(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, render.CSV(&buf, json.RawMessage(`[{"a":"x","b":"y,z"}]`)))
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Equal(t, "a,b", lines[0])
	assert.Equal(t, `x,"y,z"`, lines[1])
}

func TestTSV(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, render.TSV(&buf, json.RawMessage(`[{"a":"x","b":"y"}]`)))
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Equal(t, "a\tb", lines[0])
	assert.Equal(t, "x\ty", lines[1])
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render/`
Expected: FAIL — `render` package undefined.

- [ ] **Step 3: Write `internal/render/render.go`**

Copy the file verbatim from fibery (it has no fibery-specific logic):

```go
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// JSON pretty-prints raw JSON to w.
func JSON(w io.Writer, data json.RawMessage) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("render.JSON: %w", err)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// List renders a JSON array as a Markdown table.
func List(w io.Writer, data json.RawMessage) error {
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("render.List: %w", err)
	}
	if len(items) == 0 {
		fmt.Fprintln(w, "_No results._")
		return nil
	}

	keySet := map[string]bool{}
	for _, item := range items {
		for k := range item {
			keySet[k] = true
		}
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	sep := make([]string, len(keys))
	for i := range sep {
		sep[i] = "---"
	}
	fmt.Fprintf(w, "| %s |\n", strings.Join(keys, " | "))
	fmt.Fprintf(w, "| %s |\n", strings.Join(sep, " | "))

	for _, item := range items {
		cells := make([]string, len(keys))
		for i, k := range keys {
			v := item[k]
			if v == nil {
				cells[i] = ""
			} else {
				cells[i] = fmt.Sprintf("%v", v)
			}
		}
		fmt.Fprintf(w, "| %s |\n", strings.Join(cells, " | "))
	}
	return nil
}

// KV renders a JSON object as Markdown key-value pairs.
func KV(w io.Writer, data json.RawMessage) error {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("render.KV: %w", err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "**%s:** %v\n", k, m[k])
	}
	return nil
}

// CSV renders a JSON array as comma-separated values with a header row.
func CSV(w io.Writer, data json.RawMessage) error {
	return separatedValues(w, data, ",")
}

// TSV renders a JSON array as tab-separated values with a header row.
func TSV(w io.Writer, data json.RawMessage) error {
	return separatedValues(w, data, "\t")
}

func separatedValues(w io.Writer, data json.RawMessage, sep string) error {
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	if len(items) == 0 {
		return nil
	}
	keySet := map[string]bool{}
	for _, item := range items {
		for k := range item {
			keySet[k] = true
		}
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Fprintln(w, strings.Join(keys, sep))
	for _, item := range items {
		cells := make([]string, len(keys))
		for i, k := range keys {
			v := item[k]
			s := ""
			if v != nil {
				s = fmt.Sprintf("%v", v)
			}
			if sep == "," && strings.ContainsAny(s, ",\"\n") {
				s = `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
			}
			cells[i] = s
		}
		fmt.Fprintln(w, strings.Join(cells, sep))
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/render/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/render/
git commit -m "[NO-TASK] Port render package from fibery-cli"
```

---

## Task 4: internal/client — GitLab REST v4 HTTP client

**Files:**
- Create: `internal/client/client.go`
- Test: `internal/client/client_test.go`

The client targets GitLab REST v4. Auth header `PRIVATE-TOKEN`. Methods:
- `do` — single request with 429 retry + verbose dump (private).
- `Get(ctx, path, query)` — GET, returns body, errors on non-2xx.
- `GetPaginated(ctx, path, query, limit)` — pages via `per_page`/`page` until `limit` collected or a short page signals exhaustion; returns the combined JSON array and whether the limit was reached.
- `Send(ctx, method, path, query, body, contentType)` — mutations; returns body (may be empty on 204), errors on non-2xx.
- `GraphQL(ctx, query, variables)` — POSTs to the host-level `/api/graphql` endpoint (derived by trimming `/api/v4` from baseURL).

- [ ] **Step 1: Write the failing tests**

`internal/client/client_test.go`:

```go
package client_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/langgerone/gitlab-cli/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGet_SetsPrivateTokenHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "glpat-x", r.Header.Get("PRIVATE-TOKEN"))
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/user", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"username":"alice"}`)
	}))
	defer srv.Close()

	c := client.New("glpat-x", srv.URL)
	body, err := c.Get(context.Background(), "/user", nil)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(body, &m))
	assert.Equal(t, "alice", m["username"])
}

func TestGet_HTTPErrorIncludesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"404 Project Not Found"}`)
	}))
	defer srv.Close()

	c := client.New("t", srv.URL)
	_, err := c.Get(context.Background(), "/projects/x", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "Not Found")
}

func TestGet_RetriesOn429(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	c := client.New("t", srv.URL)
	c.RetryWait = 1 // 1ns backoff so the test is fast
	body, err := c.Get(context.Background(), "/x", nil)
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	assert.JSONEq(t, `{"ok":true}`, string(body))
}

func TestGetPaginated_StopsAtLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		// Server has effectively unlimited items; always return a full page.
		items := make([]map[string]int, perPage)
		for i := range items {
			items[i] = map[string]int{"id": (page-1)*perPage + i}
		}
		_ = json.NewEncoder(w).Encode(items)
	}))
	defer srv.Close()

	c := client.New("t", srv.URL)
	body, hitLimit, err := c.GetPaginated(context.Background(), "/items", nil, 5)
	require.NoError(t, err)
	assert.True(t, hitLimit)
	var items []map[string]int
	require.NoError(t, json.Unmarshal(body, &items))
	assert.Len(t, items, 5)
	assert.Equal(t, 0, items[0]["id"])
	assert.Equal(t, 4, items[4]["id"])
}

func TestGetPaginated_StopsOnExhaustion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page == 1 {
			fmt.Fprint(w, `[{"id":1},{"id":2}]`) // fewer than per_page → exhausted
			return
		}
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	c := client.New("t", srv.URL)
	body, hitLimit, err := c.GetPaginated(context.Background(), "/items", nil, 50)
	require.NoError(t, err)
	assert.False(t, hitLimit)
	var items []map[string]int
	require.NoError(t, json.Unmarshal(body, &items))
	assert.Len(t, items, 2)
}

func TestGetPaginated_MergesQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "opened", r.URL.Query().Get("state"))
		assert.NotEmpty(t, r.URL.Query().Get("per_page"))
		fmt.Fprint(w, `[{"id":1}]`)
	}))
	defer srv.Close()

	c := client.New("t", srv.URL)
	q := url.Values{}
	q.Set("state", "opened")
	_, _, err := c.GetPaginated(context.Background(), "/mrs", q, 10)
	require.NoError(t, err)
}

func TestSend_PostBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var m map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&m))
		assert.Equal(t, "hi", m["title"])
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"iid":7}`)
	}))
	defer srv.Close()

	c := client.New("t", srv.URL)
	body, err := c.Send(context.Background(), http.MethodPost, "/mrs", nil,
		[]byte(`{"title":"hi"}`), "application/json")
	require.NoError(t, err)
	assert.JSONEq(t, `{"iid":7}`, string(body))
}

func TestSend_NoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := client.New("t", srv.URL)
	body, err := c.Send(context.Background(), http.MethodDelete, "/branches/x", nil, nil, "")
	require.NoError(t, err)
	assert.Empty(t, body)
}

func TestGraphQL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/graphql", r.URL.Path)
		var m map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&m))
		assert.Equal(t, "{ currentUser { name } }", m["query"])
		fmt.Fprint(w, `{"data":{"currentUser":{"name":"Alice"}}}`)
	}))
	defer srv.Close()

	c := client.New("t", srv.URL+"/api/v4")
	body, err := c.GraphQL(context.Background(), "{ currentUser { name } }", nil)
	require.NoError(t, err)
	assert.Contains(t, string(body), "Alice")
}

func TestVerboseDefault(t *testing.T) {
	c := client.New("t", "https://gitlab.com/api/v4")
	assert.False(t, c.Verbose)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/`
Expected: FAIL — `client` package undefined.

- [ ] **Step 3: Write `internal/client/client.go`**

```go
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Client is the GitLab REST v4 HTTP client. All requests carry the
// PRIVATE-TOKEN header and retry once-per-attempt on HTTP 429.
type Client struct {
	token   string
	baseURL string // e.g. https://gitlab.com/api/v4
	http    *http.Client
	Verbose bool
	// RetryWait is the initial 429 backoff. Defaults to time.Second;
	// tests set it to a tiny value.
	RetryWait time.Duration
}

func New(token, baseURL string) *Client {
	return &Client{
		token:     token,
		baseURL:   strings.TrimRight(baseURL, "/"),
		http:      &http.Client{Timeout: 30 * time.Second},
		RetryWait: time.Second,
	}
}

// do performs a single request to fullURL with 429 retry. Returns the
// response body and status code. Non-2xx (other than retried 429) returns
// an error whose message embeds the status and body.
func (c *Client) do(ctx context.Context, method, fullURL string, body []byte, contentType string) ([]byte, int, error) {
	const maxRetries = 3
	wait := c.RetryWait
	if wait <= 0 {
		wait = time.Second
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		var br io.Reader
		if body != nil {
			br = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, fullURL, br)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("PRIVATE-TOKEN", c.token)
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}

		if c.Verbose {
			fmt.Fprintf(os.Stderr, "→ %s %s\n", method, fullURL)
			if body != nil {
				fmt.Fprintf(os.Stderr, "%s\n", string(body))
			}
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, 0, err
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if c.Verbose {
			fmt.Fprintf(os.Stderr, "← HTTP %d\n%s\n", resp.StatusCode, string(b))
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt == maxRetries {
				return nil, resp.StatusCode, fmt.Errorf("HTTP 429: rate limited after %d retries", maxRetries)
			}
			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			case <-time.After(wait):
			}
			wait *= 2
			continue
		}
		if resp.StatusCode >= 400 {
			return b, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
		}
		return b, resp.StatusCode, nil
	}
	return nil, 0, fmt.Errorf("unreachable")
}

// url builds baseURL + path + encoded query.
func (c *Client) url(path string, query url.Values) string {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return u
}

// Get issues a GET request and returns the response body.
func (c *Client) Get(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	b, _, err := c.do(ctx, http.MethodGet, c.url(path, query), nil, "")
	return b, err
}

// Send issues an arbitrary-method request with an optional body.
func (c *Client) Send(ctx context.Context, method, path string, query url.Values, body []byte, contentType string) (json.RawMessage, error) {
	b, _, err := c.do(ctx, method, c.url(path, query), body, contentType)
	return b, err
}

// GetPaginated requests pages via per_page/page until `limit` items are
// collected or a short page signals exhaustion. Returns the combined JSON
// array (capped at limit) and whether the limit was reached (more may exist).
func (c *Client) GetPaginated(ctx context.Context, path string, query url.Values, limit int) (json.RawMessage, bool, error) {
	if limit <= 0 {
		limit = 1
	}
	base := url.Values{}
	for k, vs := range query {
		for _, v := range vs {
			base.Add(k, v)
		}
	}

	var all []json.RawMessage
	page := 1
	for len(all) < limit {
		perPage := limit - len(all)
		if perPage > 100 {
			perPage = 100
		}
		q := url.Values{}
		for k, vs := range base {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		q.Set("per_page", strconv.Itoa(perPage))
		q.Set("page", strconv.Itoa(page))

		b, _, err := c.do(ctx, http.MethodGet, c.url(path, q), nil, "")
		if err != nil {
			return nil, false, err
		}
		var items []json.RawMessage
		if err := json.Unmarshal(b, &items); err != nil {
			return nil, false, fmt.Errorf("client.GetPaginated: decode: %w", err)
		}
		all = append(all, items...)
		if len(items) < perPage {
			break // exhausted
		}
		page++
	}

	hitLimit := len(all) >= limit
	if len(all) > limit {
		all = all[:limit]
	}
	out, err := json.Marshal(all)
	if err != nil {
		return nil, false, fmt.Errorf("client.GetPaginated: marshal: %w", err)
	}
	return out, hitLimit, nil
}

// GraphQL POSTs a query to the host-level /api/graphql endpoint. The endpoint
// is derived by trimming the /api/v4 suffix from baseURL.
func (c *Client) GraphQL(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error) {
	endpoint := strings.TrimSuffix(c.baseURL, "/api/v4") + "/api/graphql"
	payload := map[string]any{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("client.GraphQL: marshal: %w", err)
	}
	b, _, err := c.do(ctx, http.MethodPost, endpoint, body, "application/json")
	return b, err
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/client/`
Expected: PASS (all tests).

- [ ] **Step 5: Commit**

```bash
git add internal/client/
git commit -m "[NO-TASK] Add GitLab REST v4 HTTP client"
```

---

## Task 5: cmd/root — root command, persistent flags, helpers

**Files:**
- Create: `cmd/root.go`
- Test: `cmd/root_test.go`

Persistent flags: `--config`, `--host`, `--project`, `--format`, `--json`, `--verbose`, `--yes`. `PersistentPreRunE` loads config (skips for `init`), applies `--host` override, validates, builds client. `SilenceUsage: true`. Helpers: `outputJSON` (format dispatch), `projectRef` (encode `--project` or error), `encodePath` (URL-encode a path segment with slashes), `paginationHint`.

- [ ] **Step 1: Write the failing tests**

`cmd/root_test.go`:

```go
package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectRef_Numeric(t *testing.T) {
	projectFlag = "12345"
	ref, err := projectRef()
	require.NoError(t, err)
	assert.Equal(t, "12345", ref)
}

func TestProjectRef_Path(t *testing.T) {
	projectFlag = "group/sub/repo"
	ref, err := projectRef()
	require.NoError(t, err)
	assert.Equal(t, "group%2Fsub%2Frepo", ref)
}

func TestProjectRef_Missing(t *testing.T) {
	projectFlag = ""
	_, err := projectRef()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--project")
}

func TestEncodePath(t *testing.T) {
	assert.Equal(t, "src%2Fmain.go", encodePath("src/main.go"))
	assert.Equal(t, "README.md", encodePath("README.md"))
}

func TestPaginationHint_AtLimit(t *testing.T) {
	var buf bytes.Buffer
	paginationHint(&buf, true, 50)
	assert.Contains(t, buf.String(), "limit reached")
	assert.Contains(t, buf.String(), "--limit 100")
}

func TestPaginationHint_BelowLimit(t *testing.T) {
	var buf bytes.Buffer
	paginationHint(&buf, false, 50)
	assert.Empty(t, buf.String())
}

func TestIsNumeric(t *testing.T) {
	assert.True(t, isNumeric("12345"))
	assert.False(t, isNumeric("group/repo"))
	assert.False(t, isNumeric(""))
}

func TestOutputJSON_RawWhenJSONFlag(t *testing.T) {
	// Save and restore globals
	defer func() { jsonOutput = false; outputFormat = "" }()
	jsonOutput = true
	var rendered bool
	data := json.RawMessage(`{"a":1}`)
	// outputJSON writes to os.Stdout; we only assert renderFn is NOT called.
	err := outputJSON(data, func() error { rendered = true; return nil })
	require.NoError(t, err)
	assert.False(t, rendered)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/`
Expected: FAIL — `cmd` package / symbols undefined.

- [ ] **Step 3: Write `cmd/root.go`**

```go
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/langgerone/gitlab-cli/internal/client"
	"github.com/langgerone/gitlab-cli/internal/config"
	"github.com/langgerone/gitlab-cli/internal/render"
	"github.com/spf13/cobra"
)

var (
	jsonOutput   bool
	profile      string
	hostFlag     string
	projectFlag  string
	verbose      bool
	outputFormat string
	assumeYes    bool

	cfg *config.Config
	cli *client.Client
)

var rootCmd = &cobra.Command{
	Use:   "gl",
	Short: "GitLab CLI — agent-facing GitLab from the terminal",
	Long: `GitLab CLI — agent-facing GitLab from the terminal.

Run 'gl skill' to print the full Claude skill reference (commands, flags, workflows).`,
	// On RunE errors cobra prints the error itself — usage is noise for agents,
	// and error messages already carry recovery hints.
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// config init doesn't need auth
		if cmd.Name() == "init" {
			return nil
		}
		var err error
		cfg, err = config.Load(profile)
		if err != nil {
			return err
		}
		if hostFlag != "" {
			cfg.Host = hostFlag
		}
		if err := cfg.Validate(); err != nil {
			return err
		}
		cli = client.New(cfg.Token, cfg.BaseURL())
		cli.Verbose = verbose
		return nil
	},
}

func Execute() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output raw JSON (alias for --format json)")
	rootCmd.PersistentFlags().StringVar(&profile, "config", os.Getenv("GL_CONFIG"), "profile to use (subdirectory of ~/.gl/)")
	rootCmd.PersistentFlags().StringVar(&hostFlag, "host", "", "override profile host (e.g. gitlab.company.com)")
	rootCmd.PersistentFlags().StringVar(&projectFlag, "project", "", "project for project-scoped commands (group/repo or numeric ID)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "dump API request/response to stderr")
	rootCmd.PersistentFlags().StringVar(&outputFormat, "format", "", "output format: table (default), json, csv, tsv")
	rootCmd.PersistentFlags().BoolVar(&assumeYes, "yes", false, "confirm destructive operations")
}

// isNumeric reports whether s is a non-empty run of ASCII digits.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// projectRef returns the project segment for /projects/<ref> URLs. Numeric IDs
// pass through; path forms are URL-encoded (group/repo → group%2Frepo). Errors
// when --project is unset.
func projectRef() (string, error) {
	if projectFlag == "" {
		return "", fmt.Errorf("--project is required (e.g. --project group/repo or --project 12345)")
	}
	if isNumeric(projectFlag) {
		return projectFlag, nil
	}
	return url.PathEscape(projectFlag), nil
}

// encodePath URL-encodes a path segment, escaping slashes — for file paths and
// branch/tag names interpolated into REST URLs.
func encodePath(p string) string {
	return url.PathEscape(p)
}

// paginationHint writes a stderr note when a list hit its --limit, signaling
// that more results may exist.
func paginationHint(w io.Writer, hitLimit bool, limit int) {
	if hitLimit {
		fmt.Fprintf(w, "(showing %d results — limit reached; pass --limit %d for more)\n", limit, limit*2)
	}
}

// outputJSON prints raw JSON when --json / --format json is set, dispatches
// csv/tsv, and otherwise calls renderFn (the rendered table/KV path).
func outputJSON(data json.RawMessage, renderFn func() error) error {
	switch outputFormat {
	case "json":
		os.Stdout.Write(data)
		os.Stdout.Write([]byte("\n"))
		return nil
	case "csv":
		return render.CSV(os.Stdout, data)
	case "tsv":
		return render.TSV(os.Stdout, data)
	default:
		if jsonOutput {
			os.Stdout.Write(data)
			os.Stdout.Write([]byte("\n"))
			return nil
		}
		return renderFn()
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/root.go cmd/root_test.go
git commit -m "[NO-TASK] Add root command, persistent flags, shared helpers"
```

---

## Task 6: cmd/config — `gl config init`

**Files:**
- Create: `cmd/config.go`

Prompts for host (default `gitlab.com`) and token; writes to the selected profile.

- [ ] **Step 1: Write `cmd/config.go`**

```go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/langgerone/gitlab-cli/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage gl configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Set host and token (writes ~/.gl/config.yaml)",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := bufio.NewReader(os.Stdin)

		fmt.Print("GitLab host [gitlab.com]: ")
		host, _ := r.ReadString('\n')
		host = strings.TrimSpace(host)
		if host == "" {
			host = "gitlab.com"
		}

		fmt.Print("Token (glpat-...): ")
		tok, _ := r.ReadString('\n')
		tok = strings.TrimSpace(tok)

		if err := config.Save(host, tok, profile); err != nil {
			return err
		}
		if profile == "" {
			fmt.Println("Saved to ~/.gl/config.yaml")
		} else {
			fmt.Printf("Saved to ~/.gl/%s/config.yaml\n", profile)
		}
		return nil
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
	rootCmd.AddCommand(configCmd)
}
```

- [ ] **Step 2: Verify it builds and tests still pass**

Run: `go build ./... && go test ./cmd/`
Expected: builds clean; cmd tests PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/config.go
git commit -m "[NO-TASK] Add gl config init"
```

---

## Task 7: cmd/me — `gl me`

**Files:**
- Create: `cmd/me.go`

GET `/user`, render KV.

- [ ] **Step 1: Write `cmd/me.go`**

```go
package cmd

import (
	"os"

	"github.com/langgerone/gitlab-cli/internal/render"
	"github.com/spf13/cobra"
)

var meCmd = &cobra.Command{
	Use:   "me",
	Short: "Show the current user (/user)",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := cli.Get(cmd.Context(), "/user", nil)
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

func init() {
	rootCmd.AddCommand(meCmd)
}
```

- [ ] **Step 2: Verify build + tests**

Run: `go build ./... && go test ./cmd/`
Expected: builds clean; PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/me.go
git commit -m "[NO-TASK] Add gl me"
```

---

## Task 8: cmd/api — raw REST + GraphQL escape hatch

**Files:**
- Create: `cmd/api.go`
- Test: `cmd/api_test.go`

`gl api <METHOD> <path> [-f key=val ...] [--data raw]`. For GET, `-f` fields go to the query string; for other methods they form an `application/x-www-form-urlencoded` body. `--data` sends a raw `application/json` body (mutually exclusive with `-f`). `gl api graphql -f query=...` POSTs to the GraphQL endpoint. Pure helpers `parseFields` and `splitPathQuery` are unit-tested.

- [ ] **Step 1: Write the failing tests**

`cmd/api_test.go`:

```go
package cmd

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFields(t *testing.T) {
	v, err := parseFields([]string{"a=1", "b=hello world", "c="})
	require.NoError(t, err)
	assert.Equal(t, "1", v.Get("a"))
	assert.Equal(t, "hello world", v.Get("b"))
	assert.Equal(t, "", v.Get("c"))
}

func TestParseFields_NoEquals(t *testing.T) {
	_, err := parseFields([]string{"bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key=value")
}

func TestParseFields_RepeatedKey(t *testing.T) {
	v, err := parseFields([]string{"labels=a", "labels=b"})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, v["labels"])
}

func TestSplitPathQuery(t *testing.T) {
	path, q := splitPathQuery("/projects/1/issues?state=opened&x=2")
	assert.Equal(t, "/projects/1/issues", path)
	assert.Equal(t, "opened", q.Get("state"))
	assert.Equal(t, "2", q.Get("x"))
}

func TestSplitPathQuery_NoQuery(t *testing.T) {
	path, q := splitPathQuery("/user")
	assert.Equal(t, "/user", path)
	assert.Equal(t, url.Values{}, q)
}

func TestNormalizeAPIPath(t *testing.T) {
	assert.Equal(t, "/user", normalizeAPIPath("user"))
	assert.Equal(t, "/user", normalizeAPIPath("/user"))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ -run 'ParseFields|SplitPathQuery|NormalizeAPIPath'`
Expected: FAIL — symbols undefined.

- [ ] **Step 3: Write `cmd/api.go`**

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/langgerone/gitlab-cli/internal/render"
	"github.com/spf13/cobra"
)

var (
	apiFields []string
	apiData   string
)

// parseFields turns ["k=v", ...] into url.Values. Repeated keys accumulate.
func parseFields(fields []string) (url.Values, error) {
	v := url.Values{}
	for _, f := range fields {
		k, val, ok := strings.Cut(f, "=")
		if !ok {
			return nil, fmt.Errorf("invalid -f %q: expected key=value", f)
		}
		v.Add(k, val)
	}
	return v, nil
}

// splitPathQuery splits a user-supplied path into its path and query parts.
func splitPathQuery(p string) (string, url.Values) {
	path, rawQuery, ok := strings.Cut(p, "?")
	if !ok {
		return path, url.Values{}
	}
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		return path, url.Values{}
	}
	return path, q
}

// normalizeAPIPath ensures the path begins with a single leading slash.
func normalizeAPIPath(p string) string {
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

// printAPIResult renders an API response: pretty JSON when it parses as JSON,
// otherwise the raw text.
func printAPIResult(data json.RawMessage) error {
	if outputFormat == "json" || jsonOutput {
		os.Stdout.Write(data)
		os.Stdout.Write([]byte("\n"))
		return nil
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		fmt.Println("OK")
		return nil
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return render.JSON(os.Stdout, data)
	}
	fmt.Println(trimmed)
	return nil
}

var apiCmd = &cobra.Command{
	Use:   "api <METHOD> <path>",
	Short: "Raw GitLab REST v4 request",
	Long: `Send a raw request to the GitLab REST v4 API.

Examples:
  gl api GET /user
  gl api GET "/projects/123/issues?state=opened"
  gl api POST /projects/123/labels -f name=bug -f color=#ff0000
  gl api PUT /projects/123/merge_requests/5 --data '{"title":"New"}'

GraphQL escape hatch:
  gl api graphql -f query='{ currentUser { name } }'`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// GraphQL form: `gl api graphql -f query=...`
		if strings.EqualFold(args[0], "graphql") {
			fields, err := parseFields(apiFields)
			if err != nil {
				return err
			}
			query := fields.Get("query")
			if query == "" && apiData != "" {
				query = apiData
			}
			if query == "" {
				return fmt.Errorf("graphql requires -f query=... or --data")
			}
			result, err := cli.GraphQL(cmd.Context(), query, nil)
			if err != nil {
				return err
			}
			return printAPIResult(result)
		}

		if len(args) < 2 {
			return fmt.Errorf("usage: gl api <METHOD> <path>")
		}
		method := strings.ToUpper(args[0])
		path, query := splitPathQuery(normalizeAPIPath(args[1]))

		fields, err := parseFields(apiFields)
		if err != nil {
			return err
		}

		var body []byte
		var contentType string
		switch {
		case apiData != "" && len(fields) > 0:
			return fmt.Errorf("use either -f or --data, not both")
		case apiData != "":
			body = []byte(apiData)
			contentType = "application/json"
		case len(fields) > 0 && method == "GET":
			for k, vs := range fields {
				for _, v := range vs {
					query.Add(k, v)
				}
			}
		case len(fields) > 0:
			body = []byte(fields.Encode())
			contentType = "application/x-www-form-urlencoded"
		}

		result, err := cli.Send(cmd.Context(), method, path, query, body, contentType)
		if err != nil {
			return err
		}
		return printAPIResult(result)
	},
}

func init() {
	apiCmd.Flags().StringArrayVarP(&apiFields, "field", "f", nil, "form field key=value (repeatable)")
	apiCmd.Flags().StringVar(&apiData, "data", "", "raw JSON request body")
	rootCmd.AddCommand(apiCmd)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/ && go build ./...`
Expected: PASS; builds clean.

- [ ] **Step 5: Commit**

```bash
git add cmd/api.go cmd/api_test.go
git commit -m "[NO-TASK] Add gl api (raw REST + graphql)"
```

---

## Task 9: cmd/skill + skill.md — embedded skill

**Files:**
- Create: `cmd/skill.go`
- Create: `cmd/skill.md`

- [ ] **Step 1: Write `cmd/skill.go`**

```go
package cmd

import (
	_ "embed"
	"fmt"

	"github.com/spf13/cobra"
)

//go:embed skill.md
var skillDoc string

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Print the Claude skill reference for this CLI",
	Long:  "Prints the full gitlab-cli skill document for use as Claude context.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(skillDoc)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(skillCmd)
}
```

- [ ] **Step 2: Write `cmd/skill.md`**

```markdown
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
```

- [ ] **Step 3: Verify build + run**

Run: `go build -o gl . && ./gl skill | head -5`
Expected: prints the frontmatter (`name: gitlab-cli`).

- [ ] **Step 4: Commit**

```bash
git add cmd/skill.go cmd/skill.md
git commit -m "[NO-TASK] Add gl skill with embedded skill.md"
```

---

## Task 10: cmd/mr — merge requests

**Files:**
- Create: `cmd/mr.go`
- Test: `cmd/mr_test.go`

Parent `mr` with children `list`, `view`, `create`, `update`, `merge`, `approve`, `note`, `diff`. Pure helpers `draftTitle` and `stateEvent` are unit-tested.

REST paths (`P` = `projectRef()` result):
- list: GET `/projects/P/merge_requests` (paginated; params `state`, `author_username`, `labels`)
- view: GET `/projects/P/merge_requests/<iid>`; `--comments` → also GET `.../notes`
- create: POST `/projects/P/merge_requests` (JSON: source_branch, target_branch, title, description)
- update: PUT `/projects/P/merge_requests/<iid>` (JSON: title, description, state_event, labels, target_branch)
- merge: PUT `/projects/P/merge_requests/<iid>/merge` (JSON: squash, should_remove_source_branch)
- approve: POST `/projects/P/merge_requests/<iid>/approve`
- note: POST `/projects/P/merge_requests/<iid>/notes` (JSON: body)
- diff: GET `/projects/P/merge_requests/<iid>/diffs`

- [ ] **Step 1: Write the failing tests**

`cmd/mr_test.go`:

```go
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDraftTitle(t *testing.T) {
	assert.Equal(t, "Draft: Add X", draftTitle("Add X", true))
	assert.Equal(t, "Add X", draftTitle("Add X", false))
	// Already prefixed → no double prefix.
	assert.Equal(t, "Draft: Add X", draftTitle("Draft: Add X", true))
}

func TestStateEvent(t *testing.T) {
	assert.Equal(t, "close", stateEvent("closed"))
	assert.Equal(t, "reopen", stateEvent("opened"))
	assert.Equal(t, "", stateEvent(""))
	assert.Equal(t, "", stateEvent("merged"))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ -run 'DraftTitle|StateEvent'`
Expected: FAIL — symbols undefined.

- [ ] **Step 3: Write `cmd/mr.go`**

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/langgerone/gitlab-cli/internal/render"
	"github.com/spf13/cobra"
)

var (
	mrState    string
	mrAuthor   string
	mrLabel    string
	mrLimit    int
	mrComments bool

	mrSource      string
	mrTarget      string
	mrTitle       string
	mrDescription string
	mrDraft       bool

	mrUpdateState  string
	mrSquash       bool
	mrRemoveSource bool
)

// draftTitle prefixes "Draft: " when draft is set and not already prefixed.
func draftTitle(title string, draft bool) string {
	if !draft {
		return title
	}
	if strings.HasPrefix(title, "Draft: ") {
		return title
	}
	return "Draft: " + title
}

// stateEvent maps a desired MR state to the GitLab state_event verb.
func stateEvent(state string) string {
	switch state {
	case "closed":
		return "close"
	case "opened":
		return "reopen"
	default:
		return ""
	}
}

var mrCmd = &cobra.Command{
	Use:   "mr",
	Short: "Manage merge requests",
}

var mrListCmd = &cobra.Command{
	Use:   "list",
	Short: "List merge requests",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		q := url.Values{}
		if mrState != "" && mrState != "all" {
			q.Set("state", mrState)
		}
		if mrAuthor != "" {
			q.Set("author_username", mrAuthor)
		}
		if mrLabel != "" {
			q.Set("labels", mrLabel)
		}
		result, hitLimit, err := cli.GetPaginated(cmd.Context(),
			"/projects/"+p+"/merge_requests", q, mrLimit)
		if err != nil {
			return err
		}
		paginationHint(os.Stderr, hitLimit, mrLimit)
		return outputJSON(result, func() error {
			return render.List(os.Stdout, result)
		})
	},
}

var mrViewCmd = &cobra.Command{
	Use:   "view <iid>",
	Short: "View a merge request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		base := "/projects/" + p + "/merge_requests/" + args[0]
		result, err := cli.Get(cmd.Context(), base, nil)
		if err != nil {
			return err
		}
		if err := outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		}); err != nil {
			return err
		}
		if mrComments {
			notes, err := cli.Get(cmd.Context(), base+"/notes", nil)
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, "\n## Notes")
			return outputJSON(notes, func() error {
				return render.List(os.Stdout, notes)
			})
		}
		return nil
	},
}

var mrCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a merge request",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		if mrSource == "" || mrTarget == "" || mrTitle == "" {
			return fmt.Errorf("--source, --target and --title are required")
		}
		payload := map[string]any{
			"source_branch": mrSource,
			"target_branch": mrTarget,
			"title":         draftTitle(mrTitle, mrDraft),
		}
		if mrDescription != "" {
			payload["description"] = mrDescription
		}
		body, _ := json.Marshal(payload)
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/merge_requests", nil, body, "application/json")
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var mrUpdateCmd = &cobra.Command{
	Use:   "update <iid>",
	Short: "Update a merge request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		payload := map[string]any{}
		if mrTitle != "" {
			payload["title"] = mrTitle
		}
		if mrDescription != "" {
			payload["description"] = mrDescription
		}
		if mrLabel != "" {
			payload["labels"] = mrLabel
		}
		if mrTarget != "" {
			payload["target_branch"] = mrTarget
		}
		if ev := stateEvent(mrUpdateState); ev != "" {
			payload["state_event"] = ev
		}
		if len(payload) == 0 {
			return fmt.Errorf("nothing to update: set --title, --description, --label, --target or --state")
		}
		body, _ := json.Marshal(payload)
		result, err := cli.Send(cmd.Context(), "PUT",
			"/projects/"+p+"/merge_requests/"+args[0], nil, body, "application/json")
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var mrMergeCmd = &cobra.Command{
	Use:   "merge <iid>",
	Short: "Merge a merge request (requires --yes)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		if !assumeYes {
			return fmt.Errorf("merging is destructive — add --yes to confirm")
		}
		payload := map[string]any{}
		if mrSquash {
			payload["squash"] = true
		}
		if mrRemoveSource {
			payload["should_remove_source_branch"] = true
		}
		body, _ := json.Marshal(payload)
		result, err := cli.Send(cmd.Context(), "PUT",
			"/projects/"+p+"/merge_requests/"+args[0]+"/merge", nil, body, "application/json")
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var mrApproveCmd = &cobra.Command{
	Use:   "approve <iid>",
	Short: "Approve a merge request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/merge_requests/"+args[0]+"/approve", nil, nil, "")
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var mrNoteCmd = &cobra.Command{
	Use:   "note <iid> <text>",
	Short: "Add a comment to a merge request",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		body, _ := json.Marshal(map[string]any{"body": args[1]})
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/merge_requests/"+args[0]+"/notes", nil, body, "application/json")
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var mrDiffCmd = &cobra.Command{
	Use:   "diff <iid>",
	Short: "Show a merge request's changes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, err := cli.Get(cmd.Context(),
			"/projects/"+p+"/merge_requests/"+args[0]+"/diffs", nil)
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.List(os.Stdout, result)
		})
	},
}

func init() {
	mrListCmd.Flags().StringVar(&mrState, "state", "opened", "opened|merged|closed|all")
	mrListCmd.Flags().StringVar(&mrAuthor, "author", "", "filter by author username")
	mrListCmd.Flags().StringVar(&mrLabel, "label", "", "filter by label(s), comma-separated")
	mrListCmd.Flags().IntVar(&mrLimit, "limit", 20, "max results")

	mrViewCmd.Flags().BoolVar(&mrComments, "comments", false, "include notes/comments")

	mrCreateCmd.Flags().StringVar(&mrSource, "source", "", "source branch")
	mrCreateCmd.Flags().StringVar(&mrTarget, "target", "", "target branch")
	mrCreateCmd.Flags().StringVar(&mrTitle, "title", "", "MR title")
	mrCreateCmd.Flags().StringVar(&mrDescription, "description", "", "MR description")
	mrCreateCmd.Flags().BoolVar(&mrDraft, "draft", false, "mark as draft")

	mrUpdateCmd.Flags().StringVar(&mrTitle, "title", "", "new title")
	mrUpdateCmd.Flags().StringVar(&mrDescription, "description", "", "new description")
	mrUpdateCmd.Flags().StringVar(&mrUpdateState, "state", "", "opened (reopen) | closed (close)")
	mrUpdateCmd.Flags().StringVar(&mrLabel, "label", "", "set label(s), comma-separated")
	mrUpdateCmd.Flags().StringVar(&mrTarget, "target", "", "new target branch")

	mrMergeCmd.Flags().BoolVar(&mrSquash, "squash", false, "squash commits on merge")
	mrMergeCmd.Flags().BoolVar(&mrRemoveSource, "remove-source-branch", false, "remove source branch after merge")

	mrCmd.AddCommand(mrListCmd, mrViewCmd, mrCreateCmd, mrUpdateCmd, mrMergeCmd, mrApproveCmd, mrNoteCmd, mrDiffCmd)
	rootCmd.AddCommand(mrCmd)
}
```

- [ ] **Step 4: Run tests + build**

Run: `go test ./cmd/ && go build ./...`
Expected: PASS; builds clean.

- [ ] **Step 5: Commit**

```bash
git add cmd/mr.go cmd/mr_test.go
git commit -m "[NO-TASK] Add gl mr (list/view/create/update/merge/approve/note/diff)"
```

---

## Task 11: cmd/ci — pipelines, jobs, lint

**Files:**
- Create: `cmd/ci.go`
- Test: `cmd/ci_test.go`

Parents `pipeline` and `job`, plus top-level `ci lint`. Pure helper `resolveCIFile` (returns the lint target file path, default `.gitlab-ci.yml`) is unit-tested.

REST paths (`P` = `projectRef()`):
- pipeline list: GET `/projects/P/pipelines` (paginated; params `ref`, `status`)
- pipeline status [id]: id given → GET `/projects/P/pipelines/<id>`; omitted → GET `/projects/P/pipelines/latest` (params `ref`)
- job list <pid>: GET `/projects/P/pipelines/<pid>/jobs` (paginated)
- job trace <jid>: GET `/projects/P/jobs/<jid>/trace` (plain text)
- job retry <jid>: POST `/projects/P/jobs/<jid>/retry`
- job cancel <jid>: POST `/projects/P/jobs/<jid>/cancel` (requires --yes)
- ci lint [file]: POST `/projects/P/ci/lint` (JSON: `{"content": "<yaml>"}`)

- [ ] **Step 1: Write the failing tests**

`cmd/ci_test.go`:

```go
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveCIFile_Default(t *testing.T) {
	assert.Equal(t, ".gitlab-ci.yml", resolveCIFile(nil))
	assert.Equal(t, ".gitlab-ci.yml", resolveCIFile([]string{}))
}

func TestResolveCIFile_Explicit(t *testing.T) {
	assert.Equal(t, "ci/custom.yml", resolveCIFile([]string{"ci/custom.yml"}))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ -run ResolveCIFile`
Expected: FAIL — `resolveCIFile` undefined.

- [ ] **Step 3: Write `cmd/ci.go`**

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/langgerone/gitlab-cli/internal/render"
	"github.com/spf13/cobra"
)

var (
	pipelineRef    string
	pipelineStatus string
	pipelineLimit  int
	jobListLimit   int
)

// resolveCIFile returns the lint target path; defaults to .gitlab-ci.yml.
func resolveCIFile(args []string) string {
	if len(args) > 0 && args[0] != "" {
		return args[0]
	}
	return ".gitlab-ci.yml"
}

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Manage pipelines",
}

var pipelineListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pipelines",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		q := url.Values{}
		if pipelineRef != "" {
			q.Set("ref", pipelineRef)
		}
		if pipelineStatus != "" {
			q.Set("status", pipelineStatus)
		}
		result, hitLimit, err := cli.GetPaginated(cmd.Context(),
			"/projects/"+p+"/pipelines", q, pipelineLimit)
		if err != nil {
			return err
		}
		paginationHint(os.Stderr, hitLimit, pipelineLimit)
		return outputJSON(result, func() error {
			return render.List(os.Stdout, result)
		})
	},
}

var pipelineStatusCmd = &cobra.Command{
	Use:   "status [id]",
	Short: "Show a pipeline (latest for --ref when id omitted)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		var path string
		var q url.Values
		if len(args) == 1 {
			path = "/projects/" + p + "/pipelines/" + args[0]
		} else {
			path = "/projects/" + p + "/pipelines/latest"
			q = url.Values{}
			if pipelineRef != "" {
				q.Set("ref", pipelineRef)
			}
		}
		result, err := cli.Get(cmd.Context(), path, q)
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var jobCmd = &cobra.Command{
	Use:   "job",
	Short: "Manage CI jobs",
}

var jobListCmd = &cobra.Command{
	Use:   "list <pipeline-id>",
	Short: "List jobs in a pipeline",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, hitLimit, err := cli.GetPaginated(cmd.Context(),
			"/projects/"+p+"/pipelines/"+args[0]+"/jobs", nil, jobListLimit)
		if err != nil {
			return err
		}
		paginationHint(os.Stderr, hitLimit, jobListLimit)
		return outputJSON(result, func() error {
			return render.List(os.Stdout, result)
		})
	},
}

var jobTraceCmd = &cobra.Command{
	Use:   "trace <job-id>",
	Short: "Print a job's log",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, err := cli.Get(cmd.Context(), "/projects/"+p+"/jobs/"+args[0]+"/trace", nil)
		if err != nil {
			return err
		}
		os.Stdout.Write(result)
		return nil
	},
}

var jobRetryCmd = &cobra.Command{
	Use:   "retry <job-id>",
	Short: "Retry a job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/jobs/"+args[0]+"/retry", nil, nil, "")
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var jobCancelCmd = &cobra.Command{
	Use:   "cancel <job-id>",
	Short: "Cancel a job (requires --yes)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		if !assumeYes {
			return fmt.Errorf("cancelling is destructive — add --yes to confirm")
		}
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/jobs/"+args[0]+"/cancel", nil, nil, "")
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "CI helpers",
}

var ciLintCmd = &cobra.Command{
	Use:   "lint [file]",
	Short: "Lint a .gitlab-ci.yml file (default: file in cwd)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		file := resolveCIFile(args)
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read %s: %w", file, err)
		}
		body, _ := json.Marshal(map[string]any{"content": string(content)})
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/ci/lint", nil, body, "application/json")
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

func init() {
	pipelineListCmd.Flags().StringVar(&pipelineRef, "ref", "", "filter by ref (branch/tag)")
	pipelineListCmd.Flags().StringVar(&pipelineStatus, "status", "", "filter by status")
	pipelineListCmd.Flags().IntVar(&pipelineLimit, "limit", 20, "max results")
	pipelineStatusCmd.Flags().StringVar(&pipelineRef, "ref", "", "ref for the latest pipeline")
	pipelineCmd.AddCommand(pipelineListCmd, pipelineStatusCmd)

	jobListCmd.Flags().IntVar(&jobListLimit, "limit", 50, "max results")
	jobCmd.AddCommand(jobListCmd, jobTraceCmd, jobRetryCmd, jobCancelCmd)

	ciCmd.AddCommand(ciLintCmd)

	rootCmd.AddCommand(pipelineCmd, jobCmd, ciCmd)
}
```

Note: `pipelineRef` is bound to both `pipeline list --ref` and `pipeline status --ref`; cobra evaluates only the invoked command's flags, so sharing the variable is safe (single-threaded CLI run).

- [ ] **Step 4: Run tests + build**

Run: `go test ./cmd/ && go build ./...`
Expected: PASS; builds clean.

- [ ] **Step 5: Commit**

```bash
git add cmd/ci.go cmd/ci_test.go
git commit -m "[NO-TASK] Add gl pipeline/job/ci commands"
```

---

## Task 12: cmd/repo — files, branches, commits, tags, releases, projects

**Files:**
- Create: `cmd/repo.go`
- Test: `cmd/repo_test.go`

Parents `branch`, `commit`, `tag`, `release`, `project`; plus top-level `file get`. Pure helper for branch/tag URL building is exercised via `encodePath` (already tested in root_test.go); add a focused test for the release-create payload builder `releasePayload`.

REST paths (`P` = `projectRef()`):
- file get <path>: GET `/projects/P/repository/files/<encodePath(path)>/raw` (params `ref`) — raw text
- branch list: GET `/projects/P/repository/branches` (paginated)
- branch create <name>: POST `/projects/P/repository/branches` (params `branch=<name>`, `ref=<--ref or default>`)
- branch delete <name>: DELETE `/projects/P/repository/branches/<encodePath(name)>` (requires --yes)
- commit list: GET `/projects/P/repository/commits` (paginated; params `ref_name`)
- commit view <sha>: GET `/projects/P/repository/commits/<sha>`
- tag list: GET `/projects/P/repository/tags` (paginated)
- tag create <name>: POST `/projects/P/repository/tags` (params `tag_name=<name>`, `ref=<--ref>`)
- tag delete <name>: DELETE `/projects/P/repository/tags/<encodePath(name)>` (requires --yes)
- release list: GET `/projects/P/releases` (paginated)
- release view <tag>: GET `/projects/P/releases/<encodePath(tag)>`
- release create <tag>: POST `/projects/P/releases` (JSON via releasePayload)
- project search <q>: GET `/projects` (paginated; params `search=<q>`)
- project view: GET `/projects/P`

- [ ] **Step 1: Write the failing tests**

`cmd/repo_test.go`:

```go
package cmd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReleasePayload(t *testing.T) {
	releaseName = "v1.0"
	releaseDesc = "First release"
	releaseRef = "main"
	defer func() { releaseName, releaseDesc, releaseRef = "", "", "" }()

	body := releasePayload("v1.0.0")
	var m map[string]any
	require.NoError(t, json.Unmarshal(body, &m))
	assert.Equal(t, "v1.0.0", m["tag_name"])
	assert.Equal(t, "v1.0", m["name"])
	assert.Equal(t, "First release", m["description"])
	assert.Equal(t, "main", m["ref"])
}

func TestReleasePayload_TagOnly(t *testing.T) {
	releaseName, releaseDesc, releaseRef = "", "", ""
	body := releasePayload("v2.0.0")
	var m map[string]any
	require.NoError(t, json.Unmarshal(body, &m))
	assert.Equal(t, "v2.0.0", m["tag_name"])
	_, hasName := m["name"]
	assert.False(t, hasName)
	_, hasRef := m["ref"]
	assert.False(t, hasRef)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/ -run ReleasePayload`
Expected: FAIL — `releasePayload` / vars undefined.

- [ ] **Step 3: Write `cmd/repo.go`**

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/langgerone/gitlab-cli/internal/render"
	"github.com/spf13/cobra"
)

var (
	fileRef     string
	branchRef   string
	commitRef   string
	commitLimit int
	branchLimit int
	tagLimit    int
	tagRef      string
	releaseRef  string
	releaseName string
	releaseDesc string
	listLimit   int
)

// releasePayload builds the JSON body for `release create`.
func releasePayload(tag string) []byte {
	m := map[string]any{"tag_name": tag}
	if releaseName != "" {
		m["name"] = releaseName
	}
	if releaseDesc != "" {
		m["description"] = releaseDesc
	}
	if releaseRef != "" {
		m["ref"] = releaseRef
	}
	b, _ := json.Marshal(m)
	return b
}

// --- file ---

var fileCmd = &cobra.Command{
	Use:   "file",
	Short: "Repository files",
}

var fileGetCmd = &cobra.Command{
	Use:   "get <path>",
	Short: "Get a file's raw content",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		q := url.Values{}
		if fileRef != "" {
			q.Set("ref", fileRef)
		}
		result, err := cli.Get(cmd.Context(),
			"/projects/"+p+"/repository/files/"+encodePath(args[0])+"/raw", q)
		if err != nil {
			return err
		}
		os.Stdout.Write(result)
		return nil
	},
}

// --- branch ---

var branchCmd = &cobra.Command{
	Use:   "branch",
	Short: "Manage branches",
}

var branchListCmd = &cobra.Command{
	Use:   "list",
	Short: "List branches",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, hitLimit, err := cli.GetPaginated(cmd.Context(),
			"/projects/"+p+"/repository/branches", nil, branchLimit)
		if err != nil {
			return err
		}
		paginationHint(os.Stderr, hitLimit, branchLimit)
		return outputJSON(result, func() error {
			return render.List(os.Stdout, result)
		})
	},
}

var branchCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a branch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		if branchRef == "" {
			return fmt.Errorf("--ref is required (source branch/tag/sha)")
		}
		q := url.Values{}
		q.Set("branch", args[0])
		q.Set("ref", branchRef)
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/repository/branches", q, nil, "")
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var branchDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a branch (requires --yes)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		if !assumeYes {
			return fmt.Errorf("deleting a branch is destructive — add --yes to confirm")
		}
		if _, err := cli.Send(cmd.Context(), "DELETE",
			"/projects/"+p+"/repository/branches/"+encodePath(args[0]), nil, nil, ""); err != nil {
			return err
		}
		fmt.Printf("Deleted branch %s\n", args[0])
		return nil
	},
}

// --- commit ---

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Inspect commits",
}

var commitListCmd = &cobra.Command{
	Use:   "list",
	Short: "List commits",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		q := url.Values{}
		if commitRef != "" {
			q.Set("ref_name", commitRef)
		}
		result, hitLimit, err := cli.GetPaginated(cmd.Context(),
			"/projects/"+p+"/repository/commits", q, commitLimit)
		if err != nil {
			return err
		}
		paginationHint(os.Stderr, hitLimit, commitLimit)
		return outputJSON(result, func() error {
			return render.List(os.Stdout, result)
		})
	},
}

var commitViewCmd = &cobra.Command{
	Use:   "view <sha>",
	Short: "View a commit",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, err := cli.Get(cmd.Context(),
			"/projects/"+p+"/repository/commits/"+encodePath(args[0]), nil)
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

// --- tag ---

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manage tags",
}

var tagListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tags",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, hitLimit, err := cli.GetPaginated(cmd.Context(),
			"/projects/"+p+"/repository/tags", nil, tagLimit)
		if err != nil {
			return err
		}
		paginationHint(os.Stderr, hitLimit, tagLimit)
		return outputJSON(result, func() error {
			return render.List(os.Stdout, result)
		})
	},
}

var tagCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a tag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		if tagRef == "" {
			return fmt.Errorf("--ref is required (branch/sha to tag)")
		}
		q := url.Values{}
		q.Set("tag_name", args[0])
		q.Set("ref", tagRef)
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/repository/tags", q, nil, "")
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var tagDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a tag (requires --yes)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		if !assumeYes {
			return fmt.Errorf("deleting a tag is destructive — add --yes to confirm")
		}
		if _, err := cli.Send(cmd.Context(), "DELETE",
			"/projects/"+p+"/repository/tags/"+encodePath(args[0]), nil, nil, ""); err != nil {
			return err
		}
		fmt.Printf("Deleted tag %s\n", args[0])
		return nil
	},
}

// --- release ---

var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Manage releases",
}

var releaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "List releases",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, hitLimit, err := cli.GetPaginated(cmd.Context(),
			"/projects/"+p+"/releases", nil, listLimit)
		if err != nil {
			return err
		}
		paginationHint(os.Stderr, hitLimit, listLimit)
		return outputJSON(result, func() error {
			return render.List(os.Stdout, result)
		})
	},
}

var releaseViewCmd = &cobra.Command{
	Use:   "view <tag>",
	Short: "View a release",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, err := cli.Get(cmd.Context(),
			"/projects/"+p+"/releases/"+encodePath(args[0]), nil)
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var releaseCreateCmd = &cobra.Command{
	Use:   "create <tag>",
	Short: "Create a release",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/releases", nil, releasePayload(args[0]), "application/json")
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

// --- project ---

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Search and view projects",
}

var projectSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search projects by name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		q := url.Values{}
		q.Set("search", args[0])
		result, hitLimit, err := cli.GetPaginated(cmd.Context(), "/projects", q, listLimit)
		if err != nil {
			return err
		}
		paginationHint(os.Stderr, hitLimit, listLimit)
		return outputJSON(result, func() error {
			return render.List(os.Stdout, result)
		})
	},
}

var projectViewCmd = &cobra.Command{
	Use:   "view",
	Short: "View the --project project",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, err := cli.Get(cmd.Context(), "/projects/"+p, nil)
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

func init() {
	fileGetCmd.Flags().StringVar(&fileRef, "ref", "", "branch/tag/sha (default: default branch)")
	fileCmd.AddCommand(fileGetCmd)

	branchListCmd.Flags().IntVar(&branchLimit, "limit", 50, "max results")
	branchCreateCmd.Flags().StringVar(&branchRef, "ref", "", "source branch/tag/sha")
	branchCmd.AddCommand(branchListCmd, branchCreateCmd, branchDeleteCmd)

	commitListCmd.Flags().StringVar(&commitRef, "ref", "", "branch/tag/sha")
	commitListCmd.Flags().IntVar(&commitLimit, "limit", 50, "max results")
	commitCmd.AddCommand(commitListCmd, commitViewCmd)

	tagListCmd.Flags().IntVar(&tagLimit, "limit", 50, "max results")
	tagCreateCmd.Flags().StringVar(&tagRef, "ref", "", "branch/sha to tag")
	tagCmd.AddCommand(tagListCmd, tagCreateCmd, tagDeleteCmd)

	releaseListCmd.Flags().IntVar(&listLimit, "limit", 50, "max results")
	releaseCreateCmd.Flags().StringVar(&releaseName, "name", "", "release name")
	releaseCreateCmd.Flags().StringVar(&releaseDesc, "description", "", "release description")
	releaseCreateCmd.Flags().StringVar(&releaseRef, "ref", "", "ref to create the tag from (if it doesn't exist)")
	releaseCmd.AddCommand(releaseListCmd, releaseViewCmd, releaseCreateCmd)

	projectSearchCmd.Flags().IntVar(&listLimit, "limit", 20, "max results")
	projectCmd.AddCommand(projectSearchCmd, projectViewCmd)

	rootCmd.AddCommand(fileCmd, branchCmd, commitCmd, tagCmd, releaseCmd, projectCmd)
}
```

Note on `listLimit`: it is registered by three different `--limit` flags (`release list`, `project search` — and previously others). Cobra stores the parsed value into the same variable, which is fine because exactly one leaf command runs per invocation. The default differs per command (50 vs 20), and cobra applies the invoked command's default. **Verify this compiles** — registering `IntVar(&listLimit, ...)` on two flag sets is legal; only the executed command's default/value is applied.

- [ ] **Step 4: Run tests + build + vet**

Run: `go test ./cmd/ && go build ./... && go vet ./...`
Expected: PASS; builds clean; vet clean.

- [ ] **Step 5: Commit**

```bash
git add cmd/repo.go cmd/repo_test.go
git commit -m "[NO-TASK] Add gl file/branch/commit/tag/release/project"
```

---

## Task 13: README.md

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write `README.md`**

````markdown
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
````

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "[NO-TASK] Add README"
```

---

## Task 14: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Run the full test suite**

Run: `go test ./...`
Expected: all packages PASS (`ok` for `internal/config`, `internal/client`, `internal/render`, `cmd`).

- [ ] **Step 2: Vet**

Run: `go vet ./...`
Expected: no output (clean).

- [ ] **Step 3: Build the binary**

Run: `go build -o gl . && ./gl --help`
Expected: cobra help lists `api`, `branch`, `ci`, `commit`, `config`, `file`, `job`, `me`, `mr`, `pipeline`, `project`, `release`, `skill`, `tag`.

- [ ] **Step 4: Smoke-test help on subcommands**

Run: `./gl mr --help && ./gl pipeline --help && ./gl skill | head -3`
Expected: subcommand listings render; `gl skill` prints the frontmatter (`name: gitlab-cli`).

- [ ] **Step 5: Smoke-test the missing-project guard**

Run: `./gl mr list; echo "exit=$?"`
Expected: error mentioning `--project`, `exit=1`.

- [ ] **Step 6: Final commit (if anything changed)**

```bash
git add -A
git commit -m "[NO-TASK] gitlab-cli v1 complete" || echo "nothing to commit"
```

---

## Self-Review

**1. Spec coverage** — every spec section maps to a task:
- Architecture / layout → Tasks 1–12 (file structure matches spec exactly; `internal/cache` dropped).
- Auth/config (named profiles, env overrides, `--host`, `BaseURL`, `Validate`, `config init` skips auth) → Tasks 2, 5, 6.
- Project context (`--project` path/id, URL-encode, exit 1 without it) → Task 5 (`projectRef`), used in Tasks 10–12.
- Plumbing (config init, api, api graphql, me, skill) → Tasks 6, 8, 7, 9.
- Merge Requests (list/view/create/update/merge/approve/note/diff) → Task 10.
- Pipelines & CI (pipeline list/status, job list/trace/retry/cancel, ci lint) → Task 11.
- Repo & files (file get, branch, commit, tag, release, project) → Task 12.
- Persistent flags (config, host, project, format, json, verbose, yes) → Task 5.
- Client behavior (PRIVATE-TOKEN, 429 retry, pagination, SilenceUsage, not-found→exit 1, limit signal, `--yes` gating, verbose) → Tasks 4, 5, 10–12.
- Render (table/KV/json/csv/tsv) → Task 3.
- Skill + docs (`gl skill`, frontmatter, README, cobra help) → Tasks 9, 13.
- Testing (per-command `_test.go`, `internal/*` tests, httptest client, `go test`/`go vet`, Makefile targets) → Tasks 1–14.
- Install / Makefile → Tasks 1, 14.

**2. Placeholder scan** — no "TBD"/"handle errors"/"similar to". Every code step has complete code; every run step has an exact command + expected result.

**3. Type consistency** — `projectRef()`, `encodePath()`, `paginationHint(w, hitLimit, limit)`, `outputJSON(data, renderFn)`, `isNumeric()` defined in Task 5 and used consistently in Tasks 10–12. Client methods `Get`, `GetPaginated` (returns `(json.RawMessage, bool, error)`), `Send`, `GraphQL` defined in Task 4 and called with matching signatures. `assumeYes`/`verbose`/`outputFormat`/`jsonOutput`/`projectFlag`/`hostFlag`/`profile` globals declared once in Task 5. The shared `pipelineRef` (Task 11) and `listLimit` (Task 12) variables are each bound to multiple flags — legal in cobra, noted inline.

**Known follow-ups (acceptable for v1, not blockers):**
- `gl mr diff` renders GitLab's `/diffs` array as a table; raw diff text is available via `--json`. Acceptable per spec ("Show changes").
- Shared `--limit` variable defaults: each command registers its own default, applied by the invoked command. Verified legal in Task 12 step 4.
