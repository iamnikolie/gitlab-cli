package cmd

import (
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
