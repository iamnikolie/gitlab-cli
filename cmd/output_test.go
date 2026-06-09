package cmd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldString(t *testing.T) {
	data := json.RawMessage(`{"iid":42,"web_url":"https://x/1","nil":null}`)
	v, err := fieldString(data, "iid")
	require.NoError(t, err)
	assert.Equal(t, "42", v) // integer, not 4.2e+01

	v, err = fieldString(data, "web_url")
	require.NoError(t, err)
	assert.Equal(t, "https://x/1", v)

	v, err = fieldString(data, "nil")
	require.NoError(t, err)
	assert.Equal(t, "", v)
}

func TestFieldString_Missing(t *testing.T) {
	_, err := fieldString(json.RawMessage(`{"a":1}`), "iid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "iid")
}

func TestMaskToken(t *testing.T) {
	assert.Equal(t, "not set", maskToken(""))
	assert.Equal(t, "set", maskToken("abc"))
	assert.Equal(t, "set (…cdef)", maskToken("glpat-abcdef"))
}
