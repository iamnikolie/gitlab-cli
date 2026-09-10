package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	pipelineRef    string
	pipelineStatus string
	pipelineLimit  int
	jobListLimit   int
	jobFollow      bool
	ciRunRef       string
	ciRunVars      []string
	ciRunIDOnly    bool
)

// jobPollInterval is how often `job trace --follow` re-fetches the log/status.
var jobPollInterval = 2 * time.Second

// isTerminalJobStatus reports whether a job has stopped progressing on its own.
func isTerminalJobStatus(s string) bool {
	switch s {
	case "success", "failed", "canceled", "skipped", "manual":
		return true
	}
	return false
}

// pipelineVariables turns ["K=V", ...] into the GitLab pipeline variables shape.
func pipelineVariables(kvs []string) ([]map[string]string, error) {
	var out []map[string]string
	for _, kv := range kvs {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --var %q: expected KEY=VALUE", kv)
		}
		out = append(out, map[string]string{"key": k, "value": v})
	}
	return out, nil
}

// followTrace polls a job's log and status, streaming new output until the job
// reaches a terminal state. Returns a non-nil error when the job failed.
func followTrace(cmd *cobra.Command, project, jobID string) error {
	ctx := cmd.Context()
	tracePath := "/projects/" + project + "/jobs/" + jobID + "/trace"
	jobPath := "/projects/" + project + "/jobs/" + jobID
	printed := 0
	emit := func() error {
		trace, err := cli.Get(ctx, tracePath, nil)
		if err != nil {
			return err
		}
		if len(trace) > printed {
			os.Stdout.Write(trace[printed:])
			printed = len(trace)
		}
		return nil
	}
	for {
		if err := emit(); err != nil {
			return err
		}
		jobRaw, err := cli.Get(ctx, jobPath, nil)
		if err != nil {
			return err
		}
		var job struct {
			Status string `json:"status"`
		}
		_ = json.Unmarshal(jobRaw, &job)
		if isTerminalJobStatus(job.Status) {
			if err := emit(); err != nil { // catch the tail
				return err
			}
			fmt.Fprintf(stderr, "\n[job %s: %s]\n", jobID, job.Status)
			if job.Status == "failed" {
				return fmt.Errorf("job %s failed", jobID)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(jobPollInterval):
		}
	}
}

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
	Short: "Print a job's log (--follow to stream until the job finishes)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		if jobFollow {
			return followTrace(cmd, p, args[0])
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

var ciRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Create and run a new pipeline on a ref",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		if ciRunRef == "" {
			return fmt.Errorf("--ref is required (branch/tag to run)")
		}
		vars, err := pipelineVariables(ciRunVars)
		if err != nil {
			return err
		}
		payload := map[string]any{"ref": ciRunRef}
		if len(vars) > 0 {
			payload["variables"] = vars
		}
		body, _ := json.Marshal(payload)
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/pipeline", nil, body, "application/json")
		if err != nil {
			return err
		}
		if ciRunIDOnly {
			return printIDOnly(result, "id")
		}
		return emitObj(result, pipelineStatusFields)
	},
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
		if err := emitObj(result, ciLintFields); err != nil {
			return err
		}
		// Exit non-zero when the config is invalid so agents catch it by code.
		var lint struct {
			Valid bool `json:"valid"`
		}
		if json.Unmarshal(result, &lint) == nil && !lint.Valid {
			return fmt.Errorf("CI configuration is invalid")
		}
		return nil
	},
}

func init() {
	pipelineListCmd.Flags().StringVar(&pipelineRef, "ref", "", "filter by ref (branch/tag)")
	pipelineListCmd.Flags().StringVar(&pipelineStatus, "status", "", "filter by status")
	pipelineListCmd.Flags().IntVar(&pipelineLimit, "limit", 20, "max results")
	pipelineStatusCmd.Flags().StringVar(&pipelineRef, "ref", "", "ref for the latest pipeline")
	pipelineCmd.AddCommand(pipelineListCmd, pipelineStatusCmd)

	jobListCmd.Flags().IntVar(&jobListLimit, "limit", 50, "max results")
	jobTraceCmd.Flags().BoolVar(&jobFollow, "follow", false, "stream the log until the job finishes (exit 1 if it fails)")
	jobCmd.AddCommand(jobListCmd, jobTraceCmd, jobRetryCmd, jobCancelCmd)

	ciRunCmd.Flags().StringVar(&ciRunRef, "ref", "", "branch or tag to run the pipeline on")
	ciRunCmd.Flags().StringArrayVar(&ciRunVars, "var", nil, "pipeline variable KEY=VALUE (repeatable)")
	ciRunCmd.Flags().BoolVar(&ciRunIDOnly, "id-only", false, "print only the new pipeline id")
	ciCmd.AddCommand(ciLintCmd, ciRunCmd)

	rootCmd.AddCommand(pipelineCmd, jobCmd, ciCmd)
}
