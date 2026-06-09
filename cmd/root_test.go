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

func TestProfileExempt(t *testing.T) {
	for _, n := range []string{"gl", "skill", "help", "completion", "version", "bash", "zsh", "fish", "powershell"} {
		assert.True(t, profileExempt(n), "exempt: %s", n)
	}
	for _, n := range []string{"me", "init", "list", "view", "mr", "api"} {
		assert.False(t, profileExempt(n), "not exempt: %s", n)
	}
}

func TestIsNumeric(t *testing.T) {
	assert.True(t, isNumeric("12345"))
	assert.False(t, isNumeric("group/repo"))
	assert.False(t, isNumeric(""))
}

func TestProjectObject_TopLevelAndNested(t *testing.T) {
	m := map[string]any{
		"iid":   json.Number("42"),
		"title": "Add X",
		"extra": "drop me",
		"author": map[string]any{
			"username": "alice",
			"id":       json.Number("7"),
		},
	}
	out := projectObject(m, []string{"iid", "title", "author.username", "missing"})
	assert.Equal(t, json.Number("42"), out["iid"])
	assert.Equal(t, "Add X", out["title"])
	assert.Equal(t, "alice", out["author.username"])
	_, hasExtra := out["extra"]
	assert.False(t, hasExtra)
	_, hasMissing := out["missing"]
	assert.False(t, hasMissing)
}

func TestProjectList_EmptyFieldsPassThrough(t *testing.T) {
	data := json.RawMessage(`[{"a":1,"b":2}]`)
	assert.JSONEq(t, string(data), string(projectList(data, nil)))
}

func TestActiveFields(t *testing.T) {
	defer func() { projectFields = nil }()
	projectFields = nil
	assert.Equal(t, []string{"a", "b"}, activeFields([]string{"a", "b"}))
	projectFields = []string{"x"}
	assert.Equal(t, []string{"x"}, activeFields([]string{"a", "b"}))
}
