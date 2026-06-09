package cmd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestPickMRIID_PrefersOpened(t *testing.T) {
	data := []byte(`[{"iid":5,"state":"merged"},{"iid":7,"state":"opened"}]`)
	iid, err := pickMRIID(data, "feature")
	require.NoError(t, err)
	assert.Equal(t, "7", iid)
}

func TestPickMRIID_FallsBackToFirst(t *testing.T) {
	data := []byte(`[{"iid":5,"state":"merged"},{"iid":3,"state":"closed"}]`)
	iid, err := pickMRIID(data, "feature")
	require.NoError(t, err)
	assert.Equal(t, "5", iid)
}

func TestPickMRIID_NoneFound(t *testing.T) {
	_, err := pickMRIID([]byte(`[]`), "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
}

func TestFlattenDiscussions_SkipsSystemByDefault(t *testing.T) {
	data := []byte(`[
		{"id":"sys1","notes":[{"body":"assigned to @x","system":true}]},
		{"id":"abc","notes":[
			{"body":"please fix","system":false,"resolvable":true,"resolved":false,"author":{"username":"alice"}},
			{"body":"done","system":false}
		]}
	]`)
	out, err := flattenDiscussions(data, false)
	require.NoError(t, err)
	rows, err := decodeArray(out)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "abc", rows[0]["discussion_id"])
	assert.Equal(t, "alice", rows[0]["author"])
	assert.Equal(t, "please fix", rows[0]["body"])
	assert.Equal(t, true, rows[0]["resolvable"])
	assert.Equal(t, json.Number("2"), rows[0]["notes"])
}

func TestFlattenDiscussions_IncludeSystem(t *testing.T) {
	data := []byte(`[{"id":"sys1","notes":[{"body":"assigned","system":true}]}]`)
	out, err := flattenDiscussions(data, true)
	require.NoError(t, err)
	rows, err := decodeArray(out)
	require.NoError(t, err)
	assert.Len(t, rows, 1)
}
