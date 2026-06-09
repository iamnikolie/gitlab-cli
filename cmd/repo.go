package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/langgerone/gitlab-cli/internal/render"
	"github.com/spf13/cobra"
)

var (
	fileRef     string
	branchRef   string
	commitRef   string
	commitLimit int
	branchLimit int
	tagLimit    int
	tagRef      string
	releaseRef  string
	releaseName string
	releaseDesc string
	listLimit   int
)

// releasePayload builds the JSON body for `release create`.
func releasePayload(tag string) []byte {
	m := map[string]any{"tag_name": tag}
	if releaseName != "" {
		m["name"] = releaseName
	}
	if releaseDesc != "" {
		m["description"] = releaseDesc
	}
	if releaseRef != "" {
		m["ref"] = releaseRef
	}
	b, _ := json.Marshal(m)
	return b
}

// --- file ---

var fileCmd = &cobra.Command{
	Use:   "file",
	Short: "Repository files",
}

var fileGetCmd = &cobra.Command{
	Use:   "get <path>",
	Short: "Get a file's raw content",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		q := url.Values{}
		if fileRef != "" {
			q.Set("ref", fileRef)
		}
		result, err := cli.Get(cmd.Context(),
			"/projects/"+p+"/repository/files/"+encodePath(args[0])+"/raw", q)
		if err != nil {
			return err
		}
		os.Stdout.Write(result)
		return nil
	},
}

// --- branch ---

var branchCmd = &cobra.Command{
	Use:   "branch",
	Short: "Manage branches",
}

var branchListCmd = &cobra.Command{
	Use:   "list",
	Short: "List branches",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, hitLimit, err := cli.GetPaginated(cmd.Context(),
			"/projects/"+p+"/repository/branches", nil, branchLimit)
		if err != nil {
			return err
		}
		paginationHint(os.Stderr, hitLimit, branchLimit)
		return outputJSON(result, func() error {
			return render.List(os.Stdout, result)
		})
	},
}

var branchCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a branch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		if branchRef == "" {
			return fmt.Errorf("--ref is required (source branch/tag/sha)")
		}
		q := url.Values{}
		q.Set("branch", args[0])
		q.Set("ref", branchRef)
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/repository/branches", q, nil, "")
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var branchDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a branch (requires --yes)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		if !assumeYes {
			return fmt.Errorf("deleting a branch is destructive — add --yes to confirm")
		}
		if _, err := cli.Send(cmd.Context(), "DELETE",
			"/projects/"+p+"/repository/branches/"+encodePath(args[0]), nil, nil, ""); err != nil {
			return err
		}
		fmt.Printf("Deleted branch %s\n", args[0])
		return nil
	},
}

// --- commit ---

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Inspect commits",
}

var commitListCmd = &cobra.Command{
	Use:   "list",
	Short: "List commits",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		q := url.Values{}
		if commitRef != "" {
			q.Set("ref_name", commitRef)
		}
		result, hitLimit, err := cli.GetPaginated(cmd.Context(),
			"/projects/"+p+"/repository/commits", q, commitLimit)
		if err != nil {
			return err
		}
		paginationHint(os.Stderr, hitLimit, commitLimit)
		return outputJSON(result, func() error {
			return render.List(os.Stdout, result)
		})
	},
}

var commitViewCmd = &cobra.Command{
	Use:   "view <sha>",
	Short: "View a commit",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, err := cli.Get(cmd.Context(),
			"/projects/"+p+"/repository/commits/"+encodePath(args[0]), nil)
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

// --- tag ---

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manage tags",
}

var tagListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tags",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, hitLimit, err := cli.GetPaginated(cmd.Context(),
			"/projects/"+p+"/repository/tags", nil, tagLimit)
		if err != nil {
			return err
		}
		paginationHint(os.Stderr, hitLimit, tagLimit)
		return outputJSON(result, func() error {
			return render.List(os.Stdout, result)
		})
	},
}

var tagCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a tag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		if tagRef == "" {
			return fmt.Errorf("--ref is required (branch/sha to tag)")
		}
		q := url.Values{}
		q.Set("tag_name", args[0])
		q.Set("ref", tagRef)
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/repository/tags", q, nil, "")
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var tagDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a tag (requires --yes)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		if !assumeYes {
			return fmt.Errorf("deleting a tag is destructive — add --yes to confirm")
		}
		if _, err := cli.Send(cmd.Context(), "DELETE",
			"/projects/"+p+"/repository/tags/"+encodePath(args[0]), nil, nil, ""); err != nil {
			return err
		}
		fmt.Printf("Deleted tag %s\n", args[0])
		return nil
	},
}

// --- release ---

var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Manage releases",
}

var releaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "List releases",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, hitLimit, err := cli.GetPaginated(cmd.Context(),
			"/projects/"+p+"/releases", nil, listLimit)
		if err != nil {
			return err
		}
		paginationHint(os.Stderr, hitLimit, listLimit)
		return outputJSON(result, func() error {
			return render.List(os.Stdout, result)
		})
	},
}

var releaseViewCmd = &cobra.Command{
	Use:   "view <tag>",
	Short: "View a release",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, err := cli.Get(cmd.Context(),
			"/projects/"+p+"/releases/"+encodePath(args[0]), nil)
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

var releaseCreateCmd = &cobra.Command{
	Use:   "create <tag>",
	Short: "Create a release",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, err := cli.Send(cmd.Context(), "POST",
			"/projects/"+p+"/releases", nil, releasePayload(args[0]), "application/json")
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

// --- project ---

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Search and view projects",
}

var projectSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search projects by name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		q := url.Values{}
		q.Set("search", args[0])
		result, hitLimit, err := cli.GetPaginated(cmd.Context(), "/projects", q, listLimit)
		if err != nil {
			return err
		}
		paginationHint(os.Stderr, hitLimit, listLimit)
		return outputJSON(result, func() error {
			return render.List(os.Stdout, result)
		})
	},
}

var projectViewCmd = &cobra.Command{
	Use:   "view",
	Short: "View the --project project",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := projectRef()
		if err != nil {
			return err
		}
		result, err := cli.Get(cmd.Context(), "/projects/"+p, nil)
		if err != nil {
			return err
		}
		return outputJSON(result, func() error {
			return render.KV(os.Stdout, result)
		})
	},
}

func init() {
	fileGetCmd.Flags().StringVar(&fileRef, "ref", "", "branch/tag/sha (default: default branch)")
	fileCmd.AddCommand(fileGetCmd)

	branchListCmd.Flags().IntVar(&branchLimit, "limit", 50, "max results")
	branchCreateCmd.Flags().StringVar(&branchRef, "ref", "", "source branch/tag/sha")
	branchCmd.AddCommand(branchListCmd, branchCreateCmd, branchDeleteCmd)

	commitListCmd.Flags().StringVar(&commitRef, "ref", "", "branch/tag/sha")
	commitListCmd.Flags().IntVar(&commitLimit, "limit", 50, "max results")
	commitCmd.AddCommand(commitListCmd, commitViewCmd)

	tagListCmd.Flags().IntVar(&tagLimit, "limit", 50, "max results")
	tagCreateCmd.Flags().StringVar(&tagRef, "ref", "", "branch/sha to tag")
	tagCmd.AddCommand(tagListCmd, tagCreateCmd, tagDeleteCmd)

	releaseListCmd.Flags().IntVar(&listLimit, "limit", 50, "max results")
	releaseCreateCmd.Flags().StringVar(&releaseName, "name", "", "release name")
	releaseCreateCmd.Flags().StringVar(&releaseDesc, "description", "", "release description")
	releaseCreateCmd.Flags().StringVar(&releaseRef, "ref", "", "ref to create the tag from (if it doesn't exist)")
	releaseCmd.AddCommand(releaseListCmd, releaseViewCmd, releaseCreateCmd)

	projectSearchCmd.Flags().IntVar(&listLimit, "limit", 20, "max results")
	projectCmd.AddCommand(projectSearchCmd, projectViewCmd)

	rootCmd.AddCommand(fileCmd, branchCmd, commitCmd, tagCmd, releaseCmd, projectCmd)
}
