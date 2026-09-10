package cmd

import (
	"github.com/spf13/cobra"
)

// meFields is the curated default column set for `gl me`.
var meFields = []string{"id", "username", "name", "state", "web_url"}

var meCmd = &cobra.Command{
	Use:   "me",
	Short: "Show the current user (/user)",
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := cli.Get(cmd.Context(), "/user", nil)
		if err != nil {
			return err
		}
		return emitObj(result, meFields)
	},
}

func init() {
	rootCmd.AddCommand(meCmd)
}
