package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/langgerone/gitlab-cli/internal/client"
	"github.com/langgerone/gitlab-cli/internal/config"
	"github.com/langgerone/gitlab-cli/internal/render"
	"github.com/spf13/cobra"
)

var (
	jsonOutput   bool
	profile      string
	hostFlag     string
	projectFlag  string
	verbose      bool
	outputFormat string
	assumeYes    bool

	cfg *config.Config
	cli *client.Client
)

var rootCmd = &cobra.Command{
	Use:   "gl",
	Short: "GitLab CLI — agent-facing GitLab from the terminal",
	Long: `GitLab CLI — agent-facing GitLab from the terminal.

Run 'gl skill' to print the full Claude skill reference (commands, flags, workflows).`,
	// On RunE errors cobra prints the error itself — usage is noise for agents,
	// and error messages already carry recovery hints.
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// config init and skill don't need auth: init writes the token,
		// skill prints a static doc agents read before configuring.
		if cmd.Name() == "init" || cmd.Name() == "skill" {
			return nil
		}
		var err error
		cfg, err = config.Load(profile)
		if err != nil {
			return err
		}
		if hostFlag != "" {
			cfg.Host = hostFlag
		}
		if err := cfg.Validate(); err != nil {
			return err
		}
		cli = client.New(cfg.Token, cfg.BaseURL())
		cli.Verbose = verbose
		return nil
	},
}

func Execute() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output raw JSON (alias for --format json)")
	rootCmd.PersistentFlags().StringVar(&profile, "config", os.Getenv("GL_CONFIG"), "profile to use (subdirectory of ~/.gl/)")
	rootCmd.PersistentFlags().StringVar(&hostFlag, "host", "", "override profile host (e.g. gitlab.company.com)")
	rootCmd.PersistentFlags().StringVar(&projectFlag, "project", "", "project for project-scoped commands (group/repo or numeric ID)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "dump API request/response to stderr")
	rootCmd.PersistentFlags().StringVar(&outputFormat, "format", "", "output format: table (default), json, csv, tsv")
	rootCmd.PersistentFlags().BoolVar(&assumeYes, "yes", false, "confirm destructive operations")
}

// isNumeric reports whether s is a non-empty run of ASCII digits.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// projectRef returns the project segment for /projects/<ref> URLs. Numeric IDs
// pass through; path forms are URL-encoded (group/repo → group%2Frepo). Errors
// when --project is unset.
func projectRef() (string, error) {
	if projectFlag == "" {
		return "", fmt.Errorf("--project is required (e.g. --project group/repo or --project 12345)")
	}
	if isNumeric(projectFlag) {
		return projectFlag, nil
	}
	return url.PathEscape(projectFlag), nil
}

// encodePath URL-encodes a path segment, escaping slashes — for file paths and
// branch/tag names interpolated into REST URLs.
func encodePath(p string) string {
	return url.PathEscape(p)
}

// paginationHint writes a stderr note when a list hit its --limit, signaling
// that more results may exist.
func paginationHint(w io.Writer, hitLimit bool, limit int) {
	if hitLimit {
		fmt.Fprintf(w, "(showing %d results — limit reached; pass --limit %d for more)\n", limit, limit*2)
	}
}

// outputJSON prints raw JSON when --json / --format json is set, dispatches
// csv/tsv, and otherwise calls renderFn (the rendered table/KV path).
func outputJSON(data json.RawMessage, renderFn func() error) error {
	switch outputFormat {
	case "json":
		os.Stdout.Write(data)
		os.Stdout.Write([]byte("\n"))
		return nil
	case "csv":
		return render.CSV(os.Stdout, data)
	case "tsv":
		return render.TSV(os.Stdout, data)
	default:
		if jsonOutput {
			os.Stdout.Write(data)
			os.Stdout.Write([]byte("\n"))
			return nil
		}
		return renderFn()
	}
}
