package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/langgerone/gitlab-cli/internal/render"
	"github.com/spf13/cobra"
)

var (
	apiFields   []string
	apiData     string
	apiDataFile string
	apiPaginate bool
)

// apiMaxPaginate caps --paginate so a runaway endpoint can't loop forever.
const apiMaxPaginate = 100_000

// parseFields turns ["k=v", ...] into url.Values. Repeated keys accumulate.
func parseFields(fields []string) (url.Values, error) {
	v := url.Values{}
	for _, f := range fields {
		k, val, ok := strings.Cut(f, "=")
		if !ok {
			return nil, fmt.Errorf("invalid -f %q: expected key=value", f)
		}
		v.Add(k, val)
	}
	return v, nil
}

// checkFormFields rejects bracketed keys in a form-encoded body. GitLab does
// not reassemble `position[new_line]=49` into a nested object: it ignores the
// field, answers 2xx anyway, and the resource is created without it — an
// inline diff note silently becomes a plain comment. Nested structures have to
// go through --data/--data-file as JSON. Query strings are unaffected (Rails
// parses bracketed query params), so this only guards request bodies.
func checkFormFields(fields url.Values) error {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		if strings.ContainsAny(k, "[]") {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)
	return fmt.Errorf("-f %s: bracketed keys are dropped by GitLab in a form body — the request still returns 2xx with the field missing. Send the whole body as JSON instead: --data-file <file> (or --data '{\"position\":{...}}')", strings.Join(keys, ", "))
}

// splitPathQuery splits a user-supplied path into its path and query parts.
func splitPathQuery(p string) (string, url.Values) {
	path, rawQuery, ok := strings.Cut(p, "?")
	if !ok {
		return path, url.Values{}
	}
	q, err := url.ParseQuery(rawQuery)
	if err != nil {
		return path, url.Values{}
	}
	return path, q
}

// normalizeAPIPath ensures the path begins with a single leading slash.
func normalizeAPIPath(p string) string {
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

// printAPIResult renders an API response: pretty JSON when it parses as JSON,
// otherwise the raw text.
func printAPIResult(data json.RawMessage) error {
	if outputFormat == "json" || jsonOutput {
		os.Stdout.Write(data)
		os.Stdout.Write([]byte("\n"))
		return nil
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		fmt.Println("OK")
		return nil
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return render.JSON(os.Stdout, data)
	}
	fmt.Println(trimmed)
	return nil
}

var apiCmd = &cobra.Command{
	Use:   "api <METHOD> <path>",
	Short: "Raw GitLab REST v4 request",
	Long: `Send a raw request to the GitLab REST v4 API.

Examples:
  gl api GET /user
  gl api GET "/projects/123/issues?state=opened"
  gl api POST /projects/123/labels -f name=bug -f color=#ff0000
  gl api PUT /projects/123/merge_requests/5 --data '{"title":"New"}'
  gl api POST /projects/123/merge_requests/5/discussions --data-file note.json

Nested objects (an inline note's "position") must go through --data/--data-file:
-f sends a form body and GitLab drops bracketed keys without failing.

GraphQL escape hatch:
  gl api graphql -f query='{ currentUser { name } }'`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := dataArg(apiData, apiDataFile)
		if err != nil {
			return err
		}

		// GraphQL form: `gl api graphql -f query=...`
		if strings.EqualFold(args[0], "graphql") {
			fields, err := parseFields(apiFields)
			if err != nil {
				return err
			}
			query := fields.Get("query")
			if query == "" && data != "" {
				query = data
			}
			if query == "" {
				return fmt.Errorf("graphql requires -f query=... or --data")
			}
			result, err := cli.GraphQL(cmd.Context(), query, nil)
			if err != nil {
				return err
			}
			return printAPIResult(result)
		}

		if len(args) < 2 {
			return fmt.Errorf("usage: gl api <METHOD> <path>")
		}
		method := strings.ToUpper(args[0])
		path, query := splitPathQuery(normalizeAPIPath(args[1]))

		fields, err := parseFields(apiFields)
		if err != nil {
			return err
		}

		var body []byte
		var contentType string
		switch {
		case data != "" && len(fields) > 0:
			return fmt.Errorf("use either -f or --data/--data-file, not both")
		case data != "":
			body = []byte(data)
			contentType = "application/json"
		case len(fields) > 0 && method == "GET":
			for k, vs := range fields {
				for _, v := range vs {
					query.Add(k, v)
				}
			}
		case len(fields) > 0:
			if err := checkFormFields(fields); err != nil {
				return err
			}
			body = []byte(fields.Encode())
			contentType = "application/x-www-form-urlencoded"
		}

		if apiPaginate && method == "GET" {
			result, hitLimit, err := cli.GetPaginated(cmd.Context(), path, query, apiMaxPaginate)
			if err != nil {
				return err
			}
			if hitLimit {
				fmt.Fprintf(stderr, "(stopped at %d items; endpoint may have more)\n", apiMaxPaginate)
			}
			return printAPIResult(result)
		}

		result, err := cli.Send(cmd.Context(), method, path, query, body, contentType)
		if err != nil {
			return err
		}
		return printAPIResult(result)
	},
}

func init() {
	apiCmd.Flags().StringArrayVarP(&apiFields, "field", "f", nil, "form field key=value (repeatable)")
	apiCmd.Flags().StringVar(&apiData, "data", "", "raw JSON request body")
	apiCmd.Flags().StringVar(&apiDataFile, "data-file", "", "read the raw JSON request body from a file (- for stdin)")
	apiCmd.Flags().BoolVar(&apiPaginate, "paginate", false, "fetch all pages and merge into one array (GET only)")
	rootCmd.AddCommand(apiCmd)
}
