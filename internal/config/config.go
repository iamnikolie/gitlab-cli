package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// defaultHost is used when neither file, env, nor flag set a host.
const defaultHost = "gitlab.com"

type Config struct {
	Host  string `yaml:"host"`
	Token string `yaml:"token"`
}

// BaseURL returns the REST v4 API root, e.g. https://gitlab.com/api/v4.
func (c *Config) BaseURL() string {
	host := c.Host
	if host == "" {
		host = defaultHost
	}
	return fmt.Sprintf("https://%s/api/v4", host)
}

// Validate errors when no token is configured.
func (c *Config) Validate() error {
	if c.Token == "" {
		return fmt.Errorf("token not set: run 'gl config init' or set GL_TOKEN")
	}
	return nil
}

// firstEnv returns the first non-empty environment variable among names.
func firstEnv(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}

// glHome returns the config directory for the given profile.
// Empty profile → ~/.gl (or GL_HOME for tests).
// Non-empty profile → ~/.gl/<profile>.
func glHome(profile string) (string, error) {
	var base string
	if h := os.Getenv("GL_HOME"); h != "" {
		base = h
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".gl")
	}
	if profile != "" {
		return filepath.Join(base, profile), nil
	}
	return base, nil
}

// Load reads config from the profile's directory. Env vars
// GL_TOKEN (fallback GITLAB_TOKEN) and GL_HOST (fallback GITLAB_HOST)
// override file values.
func Load(profile string) (*Config, error) {
	cfg := &Config{
		Host:  firstEnv("GL_HOST", "GITLAB_HOST"),
		Token: firstEnv("GL_TOKEN", "GITLAB_TOKEN"),
	}

	dir, err := glHome(profile)
	if err != nil {
		return nil, fmt.Errorf("config.Load: %w", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("config.Load: %w", err)
	}
	if err == nil {
		var fileCfg Config
		if parseErr := yaml.Unmarshal(data, &fileCfg); parseErr != nil {
			return nil, fmt.Errorf("config.Load: parse: %w", parseErr)
		}
		if cfg.Token == "" {
			cfg.Token = fileCfg.Token
		}
		if cfg.Host == "" {
			cfg.Host = fileCfg.Host
		}
	}

	return cfg, nil
}

// Save writes config to the profile's directory (~/.gl/<profile>/config.yaml).
// Directory mode 0700, file mode 0600.
func Save(host, token, profile string) error {
	dir, err := glHome(profile)
	if err != nil {
		return fmt.Errorf("config.Save: %w", err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("config.Save: mkdir: %w", err)
	}
	data, err := yaml.Marshal(Config{Host: host, Token: token})
	if err != nil {
		return fmt.Errorf("config.Save: marshal: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0600)
}
