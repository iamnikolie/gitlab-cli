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
