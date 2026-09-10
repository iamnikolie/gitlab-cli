package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iamnikolie/gitlab-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearEnv(t *testing.T) {
	t.Setenv("GL_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GL_HOST", "")
	t.Setenv("GITLAB_HOST", "")
}

func TestLoad_FromEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv("GL_TOKEN", "glpat-env")
	t.Setenv("GL_HOST", "gl.example.com")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, "glpat-env", cfg.Token)
	assert.Equal(t, "gl.example.com", cfg.Host)
}

func TestLoad_FallbackEnvNames(t *testing.T) {
	clearEnv(t)
	t.Setenv("GITLAB_TOKEN", "glpat-fallback")
	t.Setenv("GITLAB_HOST", "fallback.example.com")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, "glpat-fallback", cfg.Token)
	assert.Equal(t, "fallback.example.com", cfg.Host)
}

func TestLoad_PrimaryEnvWinsOverFallback(t *testing.T) {
	clearEnv(t)
	t.Setenv("GL_TOKEN", "primary")
	t.Setenv("GITLAB_TOKEN", "fallback")
	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, "primary", cfg.Token)
}

func TestLoad_FromFile(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	t.Setenv("GL_HOME", dir)

	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "config.yaml"),
		[]byte("host: file.example.com\ntoken: glpat-file\n"),
		0600,
	))

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, "glpat-file", cfg.Token)
	assert.Equal(t, "file.example.com", cfg.Host)
}

func TestLoad_EnvOverFile(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	t.Setenv("GL_HOME", dir)
	t.Setenv("GL_TOKEN", "envtoken")

	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "config.yaml"),
		[]byte("host: file.example.com\ntoken: glpat-file\n"),
		0600,
	))

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, "envtoken", cfg.Token)        // env wins
	assert.Equal(t, "file.example.com", cfg.Host) // file fills in
}

func TestBaseURL_DefaultHost(t *testing.T) {
	cfg := &config.Config{Token: "x"}
	assert.Equal(t, "https://gitlab.com/api/v4", cfg.BaseURL())
}

func TestBaseURL_CustomHost(t *testing.T) {
	cfg := &config.Config{Host: "gl.example.com", Token: "x"}
	assert.Equal(t, "https://gl.example.com/api/v4", cfg.BaseURL())
}

func TestValidate(t *testing.T) {
	assert.Error(t, (&config.Config{}).Validate())
	assert.NoError(t, (&config.Config{Token: "x"}).Validate())
}

func TestSave_DefaultProfile(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	t.Setenv("GL_HOME", dir)

	require.NoError(t, config.Save("gl.example.com", "glpat-1", ""))

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "glpat-1")
	assert.Contains(t, string(data), "gl.example.com")

	info, err := os.Stat(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestSave_NamedProfileIsolated(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	t.Setenv("GL_HOME", dir)

	require.NoError(t, config.Save("default.example.com", "deftok", ""))
	require.NoError(t, config.Save("work.example.com", "worktok", "work"))

	def, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, "deftok", def.Token)
	assert.Equal(t, "default.example.com", def.Host)

	work, err := config.Load("work")
	require.NoError(t, err)
	assert.Equal(t, "worktok", work.Token)
	assert.Equal(t, "work.example.com", work.Host)

	_, err = os.Stat(filepath.Join(dir, "work", "config.yaml"))
	require.NoError(t, err)
}
