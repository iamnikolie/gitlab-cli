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
