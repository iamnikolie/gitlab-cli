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
