package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveCIFile_Default(t *testing.T) {
	assert.Equal(t, ".gitlab-ci.yml", resolveCIFile(nil))
	assert.Equal(t, ".gitlab-ci.yml", resolveCIFile([]string{}))
}

func TestResolveCIFile_Explicit(t *testing.T) {
	assert.Equal(t, "ci/custom.yml", resolveCIFile([]string{"ci/custom.yml"}))
}

func TestIsTerminalJobStatus(t *testing.T) {
	for _, s := range []string{"success", "failed", "canceled", "skipped", "manual"} {
		assert.True(t, isTerminalJobStatus(s), s)
	}
	for _, s := range []string{"created", "pending", "running", "preparing", ""} {
		assert.False(t, isTerminalJobStatus(s), s)
	}
}

func TestPipelineVariables(t *testing.T) {
	vars, err := pipelineVariables([]string{"FOO=bar", "EMPTY="})
	require.NoError(t, err)
	require.Len(t, vars, 2)
	assert.Equal(t, "FOO", vars[0]["key"])
	assert.Equal(t, "bar", vars[0]["value"])
	assert.Equal(t, "EMPTY", vars[1]["key"])
	assert.Equal(t, "", vars[1]["value"])
}

func TestPipelineVariables_Invalid(t *testing.T) {
	_, err := pipelineVariables([]string{"noequals"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "KEY=VALUE")
}
