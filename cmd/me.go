package cmd

import (
	"os"

	"github.com/langgerone/gitlab-cli/internal/render"
	"github.com/spf13/cobra"
)

var meCmd = &cobra.Command{
	Use:   "me",
	Short: "Show the current user (/user)",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := cli.Get(cmd.Context(), "/user", nil)
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

func init() {
	rootCmd.AddCommand(meCmd)
}
