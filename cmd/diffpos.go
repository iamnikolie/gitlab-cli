package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// diffPosition is the nested position object GitLab requires for an inline
// (diff-anchored) note. It must be sent as JSON: bracketed form fields are
// dropped silently, leaving a plain comment behind a 201 response.
type diffPosition struct {
	PositionType string `json:"position_type"`
	BaseSHA      string `json:"base_sha"`
	StartSHA     string `json:"start_sha"`
	HeadSHA      string `json:"head_sha"`
	OldPath      string `json:"old_path"`
	NewPath      string `json:"new_path"`
	OldLine      *int   `json:"old_line,omitempty"`
	NewLine      *int   `json:"new_line,omitempty"`
}

// diffLine is one body line of a unified diff resolved to its line numbers.
// kind is "added", "removed" or "context"; the line number that does not
// apply to the kind is zero.
type diffLine struct {
	kind    string
	oldLine int
	newLine int
}

// parseRangeStart reads the start of a hunk range ("-12,7" → 12).
func parseRangeStart(s string, sign byte) (int, bool) {
	if s == "" || s[0] != sign {
		return 0, false
	}
	s = s[1:]
	if i := strings.IndexByte(s, ','); i >= 0 {
		s = s[:i]
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return v, true
}

// parseHunkHeader reads "@@ -12,7 +14,9 @@ func x()" → (12, 14).
func parseHunkHeader(s string) (oldStart, newStart int, ok bool) {
	parts := strings.Fields(strings.TrimPrefix(s, "@@"))
	if len(parts) < 2 {
		return 0, 0, false
	}
	o, okOld := parseRangeStart(parts[0], '-')
	n, okNew := parseRangeStart(parts[1], '+')
	return o, n, okOld && okNew
}

// parseHunks walks a unified diff and returns one entry per body line, with
// the old/new line numbers GitLab expects in a position. Lines outside a hunk
// header are ignored.
func parseHunks(diff string) []diffLine {
	rows := strings.Split(diff, "\n")
	// A trailing newline yields one empty element that is not a diff line.
	if n := len(rows); n > 0 && rows[n-1] == "" {
		rows = rows[:n-1]
	}
	var (
		out         []diffLine
		oldN, newN  int
		insideAHunk bool
	)
	for _, raw := range rows {
		switch {
		case strings.HasPrefix(raw, "@@"):
			o, n, ok := parseHunkHeader(raw)
			if !ok {
				continue
			}
			oldN, newN, insideAHunk = o, n, true
		case !insideAHunk:
			// Header noise before the first hunk (---/+++ lines).
		case strings.HasPrefix(raw, "\\"):
			// "\ No newline at end of file" — carries no line number.
		case strings.HasPrefix(raw, "+"):
			out = append(out, diffLine{kind: "added", newLine: newN})
			newN++
		case strings.HasPrefix(raw, "-"):
			out = append(out, diffLine{kind: "removed", oldLine: oldN})
			oldN++
		default:
			// Context line (" foo"); an all-whitespace context line can reach
			// us as "" when trailing space was stripped, and still counts.
			out = append(out, diffLine{kind: "context", oldLine: oldN, newLine: newN})
			oldN++
			newN++
		}
	}
	return out
}

// formatRanges compresses a sorted set of line numbers into "1-4, 9, 20-22".
func formatRanges(nums []int) string {
	if len(nums) == 0 {
		return "none"
	}
	sort.Ints(nums)
	var parts []string
	start, prev := nums[0], nums[0]
	flush := func() {
		if start == prev {
			parts = append(parts, strconv.Itoa(start))
			return
		}
		parts = append(parts, fmt.Sprintf("%d-%d", start, prev))
	}
	for _, n := range nums[1:] {
		if n == prev || n == prev+1 {
			prev = n
			continue
		}
		flush()
		start, prev = n, n
	}
	flush()
	return strings.Join(parts, ", ")
}

// commentableLines summarizes which lines of a parsed diff can be anchored to,
// for the error shown when the requested one cannot.
func commentableLines(lines []diffLine) (newSide, oldSide string) {
	var news, olds []int
	for _, l := range lines {
		switch l.kind {
		case "added":
			news = append(news, l.newLine)
		case "context":
			news = append(news, l.newLine)
			olds = append(olds, l.oldLine)
		case "removed":
			olds = append(olds, l.oldLine)
		}
	}
	return formatRanges(news), formatRanges(olds)
}

// anchorLine resolves a requested line to the old/new pair GitLab accepts:
// an added line carries new_line only, a removed line old_line only, and a
// context line carries both. A line that is not part of the diff is an error
// naming the lines that are — GitLab would otherwise answer 201 and drop the
// anchor, or reject with an opaque 400.
func anchorLine(lines []diffLine, path string, newLine, oldLine int) (*int, *int, error) {
	if newLine <= 0 && oldLine <= 0 {
		return nil, nil, fmt.Errorf("--line is required (the line number in the new file); use --old-line for a deleted line")
	}
	newSide, oldSide := commentableLines(lines)

	if newLine > 0 {
		for _, l := range lines {
			if l.newLine != newLine {
				continue
			}
			switch l.kind {
			case "added":
				if oldLine > 0 {
					return nil, nil, fmt.Errorf("line %d of %s is an added line — drop --old-line (GitLab rejects an old_line on an added line)", newLine, path)
				}
				n := newLine
				return nil, &n, nil
			case "context":
				if oldLine > 0 && oldLine != l.oldLine {
					return nil, nil, fmt.Errorf("line %d of %s is a context line paired with old line %d, not %d", newLine, path, l.oldLine, oldLine)
				}
				o, n := l.oldLine, newLine
				return &o, &n, nil
			}
		}
		return nil, nil, fmt.Errorf("line %d is not part of the diff for %s — commentable new lines: %s (deleted lines, use --old-line: %s)", newLine, path, newSide, oldSide)
	}

	for _, l := range lines {
		if l.oldLine != oldLine {
			continue
		}
		switch l.kind {
		case "removed":
			o := oldLine
			return &o, nil, nil
		case "context":
			o, n := oldLine, l.newLine
			return &o, &n, nil
		}
	}
	return nil, nil, fmt.Errorf("old line %d is not part of the diff for %s — deleted lines: %s (new-file lines, use --line: %s)", oldLine, path, oldSide, newSide)
}

// postedNote is the note as GitLab stored it, read back from the create
// response to prove the anchor survived.
type postedNote struct {
	ID     int64  `json:"id"`
	Type   string `json:"type"`
	Body   string `json:"body"`
	Author struct {
		Username string `json:"username"`
	} `json:"author"`
	Position *struct {
		NewPath string `json:"new_path"`
		OldPath string `json:"old_path"`
		NewLine *int   `json:"new_line"`
		OldLine *int   `json:"old_line"`
		HeadSHA string `json:"head_sha"`
	} `json:"position"`
}

// verifyDiffNote asserts the created discussion is a real inline note anchored
// where it was aimed. GitLab answers 201 even when it drops the position, so
// the response body is the only place that failure is visible; the returned
// error carries the command that removes the stray comment.
func verifyDiffNote(data json.RawMessage, want diffPosition, project, iid string) (string, postedNote, error) {
	var disc struct {
		ID    string       `json:"id"`
		Notes []postedNote `json:"notes"`
	}
	if err := json.Unmarshal(data, &disc); err != nil {
		return "", postedNote{}, fmt.Errorf("parse discussion: %w", err)
	}
	if len(disc.Notes) == 0 {
		return disc.ID, postedNote{}, fmt.Errorf("GitLab returned a discussion with no notes")
	}
	note := disc.Notes[0]
	remedy := fmt.Sprintf("gl mr note-delete %s %d --project %s --yes", iid, note.ID, project)

	if note.Type != "DiffNote" || note.Position == nil {
		return disc.ID, note, fmt.Errorf("note %d landed as a plain comment (type=%q, position dropped), not anchored to %s:%s — remove it with: %s",
			note.ID, note.Type, want.NewPath, lineLabel(want.OldLine, want.NewLine), remedy)
	}
	if note.Position.NewPath != want.NewPath {
		return disc.ID, note, fmt.Errorf("note %d anchored to %s, expected %s — remove it with: %s",
			note.ID, note.Position.NewPath, want.NewPath, remedy)
	}
	if !intPtrEqual(note.Position.NewLine, want.NewLine) || !intPtrEqual(note.Position.OldLine, want.OldLine) {
		return disc.ID, note, fmt.Errorf("note %d anchored to line %s, expected %s — remove it with: %s",
			note.ID, lineLabel(note.Position.OldLine, note.Position.NewLine), lineLabel(want.OldLine, want.NewLine), remedy)
	}
	return disc.ID, note, nil
}

func intPtrEqual(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// shortSHA trims a commit sha to the 8 characters that identify it in output.
func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

// lineLabel renders an old/new line pair for messages ("49", "-12", "12/49").
func lineLabel(oldLine, newLine *int) string {
	switch {
	case oldLine != nil && newLine != nil:
		return fmt.Sprintf("%d/%d", *oldLine, *newLine)
	case newLine != nil:
		return strconv.Itoa(*newLine)
	case oldLine != nil:
		return "-" + strconv.Itoa(*oldLine)
	default:
		return "?"
	}
}
