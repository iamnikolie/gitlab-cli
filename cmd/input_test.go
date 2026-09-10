package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFileArg(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "d.md")
	require.NoError(t, os.WriteFile(f, []byte("## Title\nbody"), 0600))
	got, err := readFileArg(f)
	require.NoError(t, err)
	assert.Equal(t, "## Title\nbody", got)
}

func TestReadFileArg_Missing(t *testing.T) {
	_, err := readFileArg("/no/such/file.md")
	require.Error(t, err)
}

func TestDescArg_Inline(t *testing.T) {
	text, provided, err := descArg("hello", "")
	require.NoError(t, err)
	assert.True(t, provided)
	assert.Equal(t, "hello", text)
}

func TestDescArg_File(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "d.md")
	require.NoError(t, os.WriteFile(f, []byte("from file"), 0600))
	text, provided, err := descArg("", f)
	require.NoError(t, err)
	assert.True(t, provided)
	assert.Equal(t, "from file", text)
}

func TestDescArg_Both(t *testing.T) {
	_, _, err := descArg("x", "f.md")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not both")
}

func TestDescArg_Neither(t *testing.T) {
	text, provided, err := descArg("", "")
	require.NoError(t, err)
	assert.False(t, provided)
	assert.Equal(t, "", text)
}

func TestTextFromArgOrFile_Arg(t *testing.T) {
	got, err := textFromArgOrFile("inline", true, "")
	require.NoError(t, err)
	assert.Equal(t, "inline", got)
}

func TestTextFromArgOrFile_File(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "b.md")
	require.NoError(t, os.WriteFile(f, []byte("body text"), 0600))
	got, err := textFromArgOrFile("", false, f)
	require.NoError(t, err)
	assert.Equal(t, "body text", got)
}

func TestTextFromArgOrFile_Both(t *testing.T) {
	_, err := textFromArgOrFile("inline", true, "f.md")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not both")
}

func TestTextFromArgOrFile_Neither(t *testing.T) {
	_, err := textFromArgOrFile("", false, "")
	require.Error(t, err)
}

func TestDataArg_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"body":"hi"}`), 0o600))

	got, err := dataArg("", path)
	require.NoError(t, err)
	assert.Equal(t, `{"body":"hi"}`, got)
}

func TestDataArg_InlineAndEmpty(t *testing.T) {
	got, err := dataArg(`{"a":1}`, "")
	require.NoError(t, err)
	assert.Equal(t, `{"a":1}`, got)

	got, err = dataArg("", "")
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestDataArg_BothIsAnError(t *testing.T) {
	_, err := dataArg(`{"a":1}`, "note.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not both")
}
