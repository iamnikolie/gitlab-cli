package cmd

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"runtime/debug"

	"github.com/langgerone/gitlab-cli/internal/client"
	"github.com/langgerone/gitlab-cli/internal/config"
	"github.com/spf13/cobra"
)

// version is the base CLI version; buildVersion appends the VCS revision.
const version = "0.1.0"

// buildVersion returns the version plus the embedded git revision when present.
func buildVersion() string {
	v := version
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				rev := s.Value
				if len(rev) > 12 {
					rev = rev[:12]
				}
				v += " (" + rev + ")"
			}
		}
	}
	return v
}

var (
	jsonOutput    bool
	profile       string
	hostFlag      string
	projectFlag   string
	verbose       bool
	outputFormat  string
	assumeYes     bool
	projectFields []string

	cfg *config.Config
	cli *client.Client
)

// stderr is the sink for pagination hints and other out-of-band signals.
var stderr io.Writer = os.Stderr

var rootCmd = &cobra.Command{
	Use:   "gl",
	Short: "GitLab CLI — agent-facing GitLab from the terminal",
	Long: `GitLab CLI — agent-facing GitLab from the terminal.

Run 'gl skill' to print the full Claude skill reference (commands, flags, workflows).`,
	// On RunE errors cobra prints the error itself — usage is noise for agents,
	// and error messages already carry recovery hints.
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Meta commands need neither a profile nor auth (skill prints a
		// static doc; help/completion/root just print text).
		if profileExempt(cmd.Name()) {
			return nil
		}
		// Every real command requires a named profile — there is no default.
		if profile == "" {
			return fmt.Errorf("--config <name> is required (or set GL_CONFIG); there is no default profile — run 'gl --config <name> config init'")
		}
		// config init writes the token and config show inspects it: both need
		// the profile name but not a validated token / built client.
		if cmd.Name() == "init" || cmd.Name() == "show" {
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
	rootCmd.PersistentFlags().StringSliceVar(&projectFields, "fields", nil, "comma-separated fields for table/csv/tsv (default: a curated set; --json shows everything)")

	rootCmd.Version = buildVersion()
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the gl version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("gl version %s\n", buildVersion())
		return nil
	},
}

// profileExempt reports whether a command runs without a named profile —
// meta commands that touch neither config nor the API. Everything else
// requires --config (or GL_CONFIG); there is no default profile.
func profileExempt(name string) bool {
	switch name {
	case "gl", "skill", "help", "completion", "version", "bash", "zsh", "fish", "powershell":
		return true
	}
	return false
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
