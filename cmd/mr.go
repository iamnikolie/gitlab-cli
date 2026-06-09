package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/langgerone/gitlab-cli/internal/render"
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
		paginationHint(os.Stderr, hitLimit, mrLimit)
		return outputJSON(result, func() error {
			return render.List(os.Stdout, result)
		})
	},
}

var mrViewCmd = &cobra.Command{
	Use:   "view <iid>",
	Short: "View a merge request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		base := "/projects/" + p + "/merge_requests/" + args[0]
		result, err := cli.Get(cmd.Context(), base, nil)
		if err != nil {
			return err
		}
		if err := outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		}); err != nil {
			return err
		}
		if mrComments {
			notes, err := cli.Get(cmd.Context(), base+"/notes", nil)
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, "\n## Notes")
			return outputJSON(notes, func() error {
				return render.List(os.Stdout, notes)
			})
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
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var mrUpdateCmd = &cobra.Command{
	Use:   "update <iid>",
	Short: "Update a merge request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
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
			"/projects/"+p+"/merge_requests/"+args[0], nil, body, "application/json")
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var mrMergeCmd = &cobra.Command{
	Use:   "merge <iid>",
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
		payload := map[string]any{}
		if mrSquash {
			payload["squash"] = true
		}
		if mrRemoveSource {
			payload["should_remove_source_branch"] = true
		}
		body, _ := json.Marshal(payload)
		result, err := cli.Send(cmd.Context(), "PUT",
			"/projects/"+p+"/merge_requests/"+args[0]+"/merge", nil, body, "application/json")
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var mrApproveCmd = &cobra.Command{
	Use:   "approve <iid>",
	Short: "Approve a merge request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/merge_requests/"+args[0]+"/approve", nil, nil, "")
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var mrNoteCmd = &cobra.Command{
	Use:   "note <iid> <text>",
	Short: "Add a comment to a merge request",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		body, _ := json.Marshal(map[string]any{"body": args[1]})
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/merge_requests/"+args[0]+"/notes", nil, body, "application/json")
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var mrDiffCmd = &cobra.Command{
	Use:   "diff <iid>",
	Short: "Show a merge request's changes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, err := cli.Get(cmd.Context(),
			"/projects/"+p+"/merge_requests/"+args[0]+"/diffs", nil)
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.List(os.Stdout, result)
		})
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

	mrCmd.AddCommand(mrListCmd, mrViewCmd, mrCreateCmd, mrUpdateCmd, mrMergeCmd, mrApproveCmd, mrNoteCmd, mrDiffCmd)
	rootCmd.AddCommand(mrCmd)
}
