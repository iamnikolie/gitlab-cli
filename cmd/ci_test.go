package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveCIFile_Default(t *testing.T) {
	assert.Equal(t, ".gitlab-ci.yml", resolveCIFile(nil))
	assert.Equal(t, ".gitlab-ci.yml", resolveCIFile([]string{}))
}

func TestResolveCIFile_Explicit(t *testing.T) {
	assert.Equal(t, "ci/custom.yml", resolveCIFile([]string{"ci/custom.yml"}))
}
