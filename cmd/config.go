package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/langgerone/gitlab-cli/internal/config"
	"github.com/spf13/cobra"
)

// maskToken hides all but the last 4 chars of a token.
func maskToken(t string) string {
	if t == "" {
		return "not set"
	}
	if len(t) <= 4 {
		return "set"
	}
	return "set (…" + t[len(t)-4:] + ")"
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage gl configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Set host and token (writes ~/.gl/config.yaml)",
	RunE: func(cmd *cobra.Command, args []string) error {
		r := bufio.NewReader(os.Stdin)

		fmt.Print("GitLab host [gitlab.com]: ")
		host, _ := r.ReadString('\n')
		host = strings.TrimSpace(host)
		if host == "" {
			host = "gitlab.com"
		}

		fmt.Print("Token (glpat-...): ")
		tok, _ := r.ReadString('\n')
		tok = strings.TrimSpace(tok)

		if err := config.Save(host, tok, profile); err != nil {
			return err
		}
		fmt.Printf("Saved to ~/.gl/%s/config.yaml\n", profile)
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the active profile's host and token state",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(profile)
		if err != nil {
			return err
		}
		host := cfg.Host
		if host == "" {
			host = "gitlab.com"
		}
		if hostFlag != "" {
			host = hostFlag
		}
		row := map[string]any{
			"profile":  profile,
			"host":     host,
			"base_url": "https://" + host + "/api/v4",
			"token":    maskToken(cfg.Token),
		}
		b, _ := json.Marshal(row)
		return emitObj(b, []string{"profile", "host", "base_url", "token"})
	},
}

func init() {
	configCmd.AddCommand(configInitCmd, configShowCmd)
	rootCmd.AddCommand(configCmd)
}
