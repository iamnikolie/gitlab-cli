package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

var (
	pipelineRef    string
	pipelineStatus string
	pipelineLimit  int
	jobListLimit   int
)

var (
	pipelineListFields   = []string{"id", "status", "ref", "sha", "source", "created_at", "web_url"}
	pipelineStatusFields = []string{"id", "status", "ref", "sha", "source", "duration", "created_at", "updated_at", "web_url"}
	jobListFields        = []string{"id", "name", "status", "stage", "ref", "allow_failure", "web_url"}
	jobFields            = []string{"id", "name", "status", "stage", "ref", "web_url"}
	ciLintFields         = []string{"valid", "errors", "warnings"}
)

// resolveCIFile returns the lint target path; defaults to .gitlab-ci.yml.
func resolveCIFile(args []string) string {
	if len(args) > 0 && args[0] != "" {
		return args[0]
	}
	return ".gitlab-ci.yml"
}

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Manage pipelines",
}

var pipelineListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pipelines",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		q := url.Values{}
		if pipelineRef != "" {
			q.Set("ref", pipelineRef)
		}
		if pipelineStatus != "" {
			q.Set("status", pipelineStatus)
		}
		result, hitLimit, err := cli.GetPaginated(cmd.Context(),
			"/projects/"+p+"/pipelines", q, pipelineLimit)
		if err != nil {
			return err
		}
		paginationHint(stderr, hitLimit, pipelineLimit)
		return emitList(result, pipelineListFields)
	},
}

var pipelineStatusCmd = &cobra.Command{
	Use:   "status [id]",
	Short: "Show a pipeline (latest for --ref when id omitted)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		var path string
		var q url.Values
		if len(args) == 1 {
			path = "/projects/" + p + "/pipelines/" + args[0]
		} else {
			path = "/projects/" + p + "/pipelines/latest"
			q = url.Values{}
			if pipelineRef != "" {
				q.Set("ref", pipelineRef)
			}
		}
		result, err := cli.Get(cmd.Context(), path, q)
		if err != nil {
			return err
		}
		return emitObj(result, pipelineStatusFields)
	},
}

var jobCmd = &cobra.Command{
	Use:   "job",
	Short: "Manage CI jobs",
}

var jobListCmd = &cobra.Command{
	Use:   "list <pipeline-id>",
	Short: "List jobs in a pipeline",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, hitLimit, err := cli.GetPaginated(cmd.Context(),
			"/projects/"+p+"/pipelines/"+args[0]+"/jobs", nil, jobListLimit)
		if err != nil {
			return err
		}
		paginationHint(stderr, hitLimit, jobListLimit)
		return emitList(result, jobListFields)
	},
}

var jobTraceCmd = &cobra.Command{
	Use:   "trace <job-id>",
	Short: "Print a job's log",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, err := cli.Get(cmd.Context(), "/projects/"+p+"/jobs/"+args[0]+"/trace", nil)
		if err != nil {
			return err
		}
		os.Stdout.Write(result)
		return nil
	},
}

var jobRetryCmd = &cobra.Command{
	Use:   "retry <job-id>",
	Short: "Retry a job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/jobs/"+args[0]+"/retry", nil, nil, "")
		if err != nil {
			return err
		}
		return emitObj(result, jobFields)
	},
}

var jobCancelCmd = &cobra.Command{
	Use:   "cancel <job-id>",
	Short: "Cancel a job (requires --yes)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		if !assumeYes {
			return fmt.Errorf("cancelling is destructive — add --yes to confirm")
		}
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/jobs/"+args[0]+"/cancel", nil, nil, "")
		if err != nil {
			return err
		}
		return emitObj(result, jobFields)
	},
}

var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "CI helpers",
}

var ciLintCmd = &cobra.Command{
	Use:   "lint [file]",
	Short: "Lint a .gitlab-ci.yml file (default: file in cwd)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		file := resolveCIFile(args)
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read %s: %w", file, err)
		}
		body, _ := json.Marshal(map[string]any{"content": string(content)})
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/ci/lint", nil, body, "application/json")
		if err != nil {
			return err
		}
		return emitObj(result, ciLintFields)
	},
}

func init() {
	pipelineListCmd.Flags().StringVar(&pipelineRef, "ref", "", "filter by ref (branch/tag)")
	pipelineListCmd.Flags().StringVar(&pipelineStatus, "status", "", "filter by status")
	pipelineListCmd.Flags().IntVar(&pipelineLimit, "limit", 20, "max results")
	pipelineStatusCmd.Flags().StringVar(&pipelineRef, "ref", "", "ref for the latest pipeline")
	pipelineCmd.AddCommand(pipelineListCmd, pipelineStatusCmd)

	jobListCmd.Flags().IntVar(&jobListLimit, "limit", 50, "max results")
	jobCmd.AddCommand(jobListCmd, jobTraceCmd, jobRetryCmd, jobCancelCmd)

	ciCmd.AddCommand(ciLintCmd)

	rootCmd.AddCommand(pipelineCmd, jobCmd, ciCmd)
}
