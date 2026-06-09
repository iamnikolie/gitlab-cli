package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/langgerone/gitlab-cli/internal/config"
	"github.com/spf13/cobra"
)

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

func init() {
	configCmd.AddCommand(configInitCmd)
	rootCmd.AddCommand(configCmd)
}
