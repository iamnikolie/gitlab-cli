package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var (
	mrState    string
	mrAuthor   string
	mrLabel    string
	mrLimit    int
	mrComments bool

	mrSource      string
	mrTarget      string
	mrTitle       string
	mrDescription string
	mrDraft       bool

	mrUpdateState  string
	mrSquash       bool
	mrRemoveSource bool
	mrRebaseSkipCI bool
)

// Curated default field sets (token-lean). --json shows the full object;
// --fields overrides these.
var (
	mrListFields     = []string{"iid", "state", "draft", "title", "author.username", "source_branch", "target_branch", "web_url"}
	mrViewFields     = []string{"iid", "state", "draft", "title", "author.username", "source_branch", "target_branch", "merge_status", "has_conflicts", "description", "web_url"}
	mrApprovalFields = []string{"iid", "approvals_required", "approvals_left", "user_has_approved", "approved"}
	noteFields       = []string{"id", "author.username", "created_at", "system", "body"}
)

// draftTitle prefixes "Draft: " when draft is set and not already prefixed.
func draftTitle(title string, draft bool) string {
	if !draft {
		return title
	}
	if strings.HasPrefix(title, "Draft: ") {
		return title
	}
	return "Draft: " + title
}

// stateEvent maps a desired MR state to the GitLab state_event verb.
func stateEvent(state string) string {
	switch state {
	case "closed":
		return "close"
	case "opened":
		return "reopen"
	default:
		return ""
	}
}

// iidString extracts an MR's iid as a string from a decoded object.
func iidString(m map[string]any) string {
	switch v := m["iid"].(type) {
	case json.Number:
		return v.String()
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	}
	return ""
}

// pickMRIID chooses an iid from a merge_requests array: an opened MR if present,
// else the first (results are ordered most-recent-first).
func pickMRIID(data json.RawMessage, ref string) (string, error) {
	items, err := decodeArray(data)
	if err != nil {
		return "", fmt.Errorf("resolve MR for branch %q: %w", ref, err)
	}
	if len(items) == 0 {
		return "", fmt.Errorf("no merge request found for branch %q", ref)
	}
	for _, m := range items {
		if s, _ := m["state"].(string); s == "opened" {
			if id := iidString(m); id != "" {
				return id, nil
			}
		}
	}
	if id := iidString(items[0]); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("resolve MR for branch %q: no iid in response", ref)
}

// resolveMRIID returns the MR iid for a reference. Numeric refs pass through;
// anything else is treated as a source branch and looked up via the API.
func resolveMRIID(cmd *cobra.Command, project, ref string) (string, error) {
	if isNumeric(ref) {
		return ref, nil
	}
	q := url.Values{}
	q.Set("source_branch", ref)
	q.Set("order_by", "updated_at")
	q.Set("sort", "desc")
	data, _, err := cli.GetPaginated(cmd.Context(), "/projects/"+project+"/merge_requests", q, 20)
	if err != nil {
		return "", err
	}
	return pickMRIID(data, ref)
}

// printMRDiff renders the /diffs array as readable unified patches.
func printMRDiff(data json.RawMessage) error {
	var files []struct {
		OldPath     string `json:"old_path"`
		NewPath     string `json:"new_path"`
		NewFile     bool   `json:"new_file"`
		DeletedFile bool   `json:"deleted_file"`
		RenamedFile bool   `json:"renamed_file"`
		Diff        string `json:"diff"`
	}
	if err := json.Unmarshal(data, &files); err != nil {
		return fmt.Errorf("mr diff: %w", err)
	}
	if len(files) == 0 {
		fmt.Println("_No changes._")
		return nil
	}
	for _, f := range files {
		fmt.Printf("diff --git a/%s b/%s\n", f.OldPath, f.NewPath)
		switch {
		case f.NewFile:
			fmt.Printf("new file %s\n", f.NewPath)
		case f.DeletedFile:
			fmt.Printf("deleted file %s\n", f.OldPath)
		case f.RenamedFile:
			fmt.Printf("renamed %s -> %s\n", f.OldPath, f.NewPath)
		}
		fmt.Print(f.Diff)
		if !strings.HasSuffix(f.Diff, "\n") {
			fmt.Println()
		}
	}
	return nil
}

var mrCmd = &cobra.Command{
	Use:   "mr",
	Short: "Manage merge requests",
}

var mrListCmd = &cobra.Command{
	Use:   "list",
	Short: "List merge requests",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		q := url.Values{}
		if mrState != "" && mrState != "all" {
			q.Set("state", mrState)
		}
		if mrAuthor != "" {
			q.Set("author_username", mrAuthor)
		}
		if mrLabel != "" {
			q.Set("labels", mrLabel)
		}
		result, hitLimit, err := cli.GetPaginated(cmd.Context(),
			"/projects/"+p+"/merge_requests", q, mrLimit)
		if err != nil {
			return err
		}
		paginationHint(stderr, hitLimit, mrLimit)
		return emitList(result, mrListFields)
	},
}

var mrViewCmd = &cobra.Command{
	Use:   "view <id|branch>",
	Short: "View a merge request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		iid, err := resolveMRIID(cmd, p, args[0])
		if err != nil {
			return err
		}
		base := "/projects/" + p + "/merge_requests/" + iid
		result, err := cli.Get(cmd.Context(), base, nil)
		if err != nil {
			return err
		}
		if err := emitObj(result, mrViewFields); err != nil {
			return err
		}
		if mrComments {
			notes, err := cli.Get(cmd.Context(), base+"/notes", nil)
			if err != nil {
				return err
			}
			if outputFormat != "json" && !jsonOutput {
				fmt.Println("\n## Notes")
			}
			return emitList(notes, noteFields)
		}
		return nil
	},
}

var mrCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a merge request",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		if mrSource == "" || mrTarget == "" || mrTitle == "" {
			return fmt.Errorf("--source, --target and --title are required")
		}
		payload := map[string]any{
			"source_branch": mrSource,
			"target_branch": mrTarget,
			"title":         draftTitle(mrTitle, mrDraft),
		}
		if mrDescription != "" {
			payload["description"] = mrDescription
		}
		body, _ := json.Marshal(payload)
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/merge_requests", nil, body, "application/json")
		if err != nil {
			return err
		}
		return emitObj(result, mrViewFields)
	},
}

var mrUpdateCmd = &cobra.Command{
	Use:   "update <id|branch>",
	Short: "Update a merge request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		iid, err := resolveMRIID(cmd, p, args[0])
		if err != nil {
			return err
		}
		payload := map[string]any{}
		if mrTitle != "" {
			payload["title"] = mrTitle
		}
		if mrDescription != "" {
			payload["description"] = mrDescription
		}
		if mrLabel != "" {
			payload["labels"] = mrLabel
		}
		if mrTarget != "" {
			payload["target_branch"] = mrTarget
		}
		if ev := stateEvent(mrUpdateState); ev != "" {
			payload["state_event"] = ev
		}
		if len(payload) == 0 {
			return fmt.Errorf("nothing to update: set --title, --description, --label, --target or --state")
		}
		body, _ := json.Marshal(payload)
		result, err := cli.Send(cmd.Context(), "PUT",
			"/projects/"+p+"/merge_requests/"+iid, nil, body, "application/json")
		if err != nil {
			return err
		}
		return emitObj(result, mrViewFields)
	},
}

// mrSetState is the shared body for close/reopen.
func mrSetState(cmd *cobra.Command, ref, event string) error {
	p, err := projectRef()
	if err != nil {
		return err
	}
	iid, err := resolveMRIID(cmd, p, ref)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{"state_event": event})
	result, err := cli.Send(cmd.Context(), "PUT",
		"/projects/"+p+"/merge_requests/"+iid, nil, body, "application/json")
	if err != nil {
		return err
	}
	return emitObj(result, mrViewFields)
}

var mrCloseCmd = &cobra.Command{
	Use:   "close <id|branch>",
	Short: "Close a merge request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return mrSetState(cmd, args[0], "close")
	},
}

var mrReopenCmd = &cobra.Command{
	Use:   "reopen <id|branch>",
	Short: "Reopen a merge request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return mrSetState(cmd, args[0], "reopen")
	},
}

var mrRebaseCmd = &cobra.Command{
	Use:   "rebase <id|branch>",
	Short: "Rebase the source branch against its target",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		iid, err := resolveMRIID(cmd, p, args[0])
		if err != nil {
			return err
		}
		q := url.Values{}
		if mrRebaseSkipCI {
			q.Set("skip_ci", "true")
		}
		result, err := cli.Send(cmd.Context(), "PUT",
			"/projects/"+p+"/merge_requests/"+iid+"/rebase", q, nil, "")
		if err != nil {
			return err
		}
		if outputFormat == "json" || jsonOutput {
			return writeRaw(result)
		}
		if len(strings.TrimSpace(string(result))) == 0 {
			fmt.Printf("Rebase requested for MR !%s\n", iid)
			return nil
		}
		return emitObj(result, []string{"rebase_in_progress", "merge_error"})
	},
}

var mrMergeCmd = &cobra.Command{
	Use:   "merge <id|branch>",
	Short: "Merge a merge request (requires --yes)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		if !assumeYes {
			return fmt.Errorf("merging is destructive — add --yes to confirm")
		}
		iid, err := resolveMRIID(cmd, p, args[0])
		if err != nil {
			return err
		}
		payload := map[string]any{}
		if mrSquash {
			payload["squash"] = true
		}
		if mrRemoveSource {
			payload["should_remove_source_branch"] = true
		}
		body, _ := json.Marshal(payload)
		result, err := cli.Send(cmd.Context(), "PUT",
			"/projects/"+p+"/merge_requests/"+iid+"/merge", nil, body, "application/json")
		if err != nil {
			return err
		}
		return emitObj(result, mrViewFields)
	},
}

var mrApproveCmd = &cobra.Command{
	Use:   "approve <id|branch>",
	Short: "Approve a merge request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		iid, err := resolveMRIID(cmd, p, args[0])
		if err != nil {
			return err
		}
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/merge_requests/"+iid+"/approve", nil, nil, "")
		if err != nil {
			return err
		}
		return emitObj(result, mrApprovalFields)
	},
}

var mrNoteCmd = &cobra.Command{
	Use:   "note <id|branch> <text>",
	Short: "Add a comment to a merge request",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		iid, err := resolveMRIID(cmd, p, args[0])
		if err != nil {
			return err
		}
		body, _ := json.Marshal(map[string]any{"body": args[1]})
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/merge_requests/"+iid+"/notes", nil, body, "application/json")
		if err != nil {
			return err
		}
		return emitObj(result, noteFields)
	},
}

var mrDiffCmd = &cobra.Command{
	Use:   "diff <id|branch>",
	Short: "Show a merge request's changes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		iid, err := resolveMRIID(cmd, p, args[0])
		if err != nil {
			return err
		}
		result, err := cli.Get(cmd.Context(),
			"/projects/"+p+"/merge_requests/"+iid+"/diffs", nil)
		if err != nil {
			return err
		}
		if outputFormat == "json" || jsonOutput {
			return writeRaw(result)
		}
		return printMRDiff(result)
	},
}

func init() {
	mrListCmd.Flags().StringVar(&mrState, "state", "opened", "opened|merged|closed|all")
	mrListCmd.Flags().StringVar(&mrAuthor, "author", "", "filter by author username")
	mrListCmd.Flags().StringVar(&mrLabel, "label", "", "filter by label(s), comma-separated")
	mrListCmd.Flags().IntVar(&mrLimit, "limit", 20, "max results")

	mrViewCmd.Flags().BoolVar(&mrComments, "comments", false, "include notes/comments")

	mrCreateCmd.Flags().StringVar(&mrSource, "source", "", "source branch")
	mrCreateCmd.Flags().StringVar(&mrTarget, "target", "", "target branch")
	mrCreateCmd.Flags().StringVar(&mrTitle, "title", "", "MR title")
	mrCreateCmd.Flags().StringVar(&mrDescription, "description", "", "MR description")
	mrCreateCmd.Flags().BoolVar(&mrDraft, "draft", false, "mark as draft")

	mrUpdateCmd.Flags().StringVar(&mrTitle, "title", "", "new title")
	mrUpdateCmd.Flags().StringVar(&mrDescription, "description", "", "new description")
	mrUpdateCmd.Flags().StringVar(&mrUpdateState, "state", "", "opened (reopen) | closed (close)")
	mrUpdateCmd.Flags().StringVar(&mrLabel, "label", "", "set label(s), comma-separated")
	mrUpdateCmd.Flags().StringVar(&mrTarget, "target", "", "new target branch")

	mrMergeCmd.Flags().BoolVar(&mrSquash, "squash", false, "squash commits on merge")
	mrMergeCmd.Flags().BoolVar(&mrRemoveSource, "remove-source-branch", false, "remove source branch after merge")

	mrRebaseCmd.Flags().BoolVar(&mrRebaseSkipCI, "skip-ci", false, "skip the CI pipeline triggered by the rebase")

	mrCmd.AddCommand(mrListCmd, mrViewCmd, mrCreateCmd, mrUpdateCmd, mrCloseCmd, mrReopenCmd, mrRebaseCmd, mrMergeCmd, mrApproveCmd, mrNoteCmd, mrDiffCmd)
	rootCmd.AddCommand(mrCmd)
}
