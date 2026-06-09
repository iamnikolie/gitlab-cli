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
	for _, n := range []string{"gl", "skill", "help", "completion", "bash", "zsh", "fish", "powershell"} {
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
