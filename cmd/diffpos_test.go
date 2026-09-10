package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleDiff changes lines 10-14 of the new file: one line replaced, one added.
const sampleDiff = "@@ -10,5 +10,6 @@ func Foo() {\n" +
	" \ta := 1\n" +
	"-\tb := 2\n" +
	"+\tb := 3\n" +
	"+\tc := 4\n" +
	" \treturn\n" +
	" }\n"

func TestParseHunks_ResolvesLineNumbers(t *testing.T) {
	lines := parseHunks(sampleDiff)
	require.Len(t, lines, 6)
	assert.Equal(t, diffLine{kind: "context", oldLine: 10, newLine: 10}, lines[0])
	assert.Equal(t, diffLine{kind: "removed", oldLine: 11}, lines[1])
	assert.Equal(t, diffLine{kind: "added", newLine: 11}, lines[2])
	assert.Equal(t, diffLine{kind: "added", newLine: 12}, lines[3])
	assert.Equal(t, diffLine{kind: "context", oldLine: 12, newLine: 13}, lines[4])
	assert.Equal(t, diffLine{kind: "context", oldLine: 13, newLine: 14}, lines[5])
}

func TestParseHunks_MultipleHunksAndNoNewline(t *testing.T) {
	diff := "@@ -1,2 +1,2 @@\n-old\n+new\n\\ No newline at end of file\n@@ -20,2 +20,3 @@\n ctx\n+added\n"
	lines := parseHunks(diff)
	require.Len(t, lines, 4)
	assert.Equal(t, diffLine{kind: "removed", oldLine: 1}, lines[0])
	assert.Equal(t, diffLine{kind: "added", newLine: 1}, lines[1])
	assert.Equal(t, diffLine{kind: "context", oldLine: 20, newLine: 20}, lines[2])
	assert.Equal(t, diffLine{kind: "added", newLine: 21}, lines[3])
}

// An empty context line reaches us as "" when trailing whitespace was
// stripped; counting it is what keeps later line numbers correct.
func TestParseHunks_EmptyContextLineStillCounts(t *testing.T) {
	lines := parseHunks("@@ -1,3 +1,3 @@\n a\n\n+b\n")
	require.Len(t, lines, 3)
	assert.Equal(t, diffLine{kind: "context", oldLine: 2, newLine: 2}, lines[1])
	assert.Equal(t, diffLine{kind: "added", newLine: 3}, lines[2])
}

func TestAnchorLine_AddedLineCarriesNewLineOnly(t *testing.T) {
	oldLine, newLine, err := anchorLine(parseHunks(sampleDiff), "x.go", 11, 0)
	require.NoError(t, err)
	assert.Nil(t, oldLine)
	require.NotNil(t, newLine)
	assert.Equal(t, 11, *newLine)
}

func TestAnchorLine_ContextLineCarriesBoth(t *testing.T) {
	oldLine, newLine, err := anchorLine(parseHunks(sampleDiff), "x.go", 13, 0)
	require.NoError(t, err)
	require.NotNil(t, oldLine)
	require.NotNil(t, newLine)
	assert.Equal(t, 12, *oldLine)
	assert.Equal(t, 13, *newLine)
}

func TestAnchorLine_DeletedLineCarriesOldLineOnly(t *testing.T) {
	oldLine, newLine, err := anchorLine(parseHunks(sampleDiff), "x.go", 0, 11)
	require.NoError(t, err)
	assert.Nil(t, newLine)
	require.NotNil(t, oldLine)
	assert.Equal(t, 11, *oldLine)
}

func TestAnchorLine_LineOutsideDiffListsWhatIsAvailable(t *testing.T) {
	_, _, err := anchorLine(parseHunks(sampleDiff), "x.go", 99, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "line 99 is not part of the diff for x.go")
	assert.Contains(t, err.Error(), "10-14")
	assert.Contains(t, err.Error(), "10-13")
}

func TestAnchorLine_OldLineOnAddedLineIsRejected(t *testing.T) {
	_, _, err := anchorLine(parseHunks(sampleDiff), "x.go", 11, 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "added line")
}

func TestAnchorLine_NoLineGiven(t *testing.T) {
	_, _, err := anchorLine(parseHunks(sampleDiff), "x.go", 0, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--line is required")
}

func TestFormatRanges(t *testing.T) {
	assert.Equal(t, "none", formatRanges(nil))
	assert.Equal(t, "3", formatRanges([]int{3}))
	assert.Equal(t, "1-4, 9, 20-22", formatRanges([]int{2, 1, 3, 4, 9, 21, 20, 22}))
}

func TestVerifyDiffNote_Accepts(t *testing.T) {
	data := []byte(`{"id":"disc1","notes":[{"id":77,"type":"DiffNote","author":{"username":"mako"},
		"position":{"new_path":"x.go","old_path":"x.go","new_line":49,"head_sha":"f3ab7da2c0"}}]}`)
	line := 49
	want := diffPosition{NewPath: "x.go", NewLine: &line}
	discID, note, err := verifyDiffNote(data, want, "g%2Fr", "42")
	require.NoError(t, err)
	assert.Equal(t, "disc1", discID)
	assert.Equal(t, int64(77), note.ID)
	assert.Equal(t, "f3ab7da2", shortSHA(note.Position.HeadSHA))
}

// The silent failure this whole command exists to catch: 201, well-formed
// note, no position — the comment landed at the bottom of the MR.
func TestVerifyDiffNote_RejectsDroppedPosition(t *testing.T) {
	data := []byte(`{"id":"disc1","notes":[{"id":77,"type":"DiscussionNote","position":null}]}`)
	line := 49
	want := diffPosition{NewPath: "x.go", NewLine: &line}
	_, _, err := verifyDiffNote(data, want, "g%2Fr", "42")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plain comment")
	assert.Contains(t, err.Error(), "gl mr note-delete 42 77 --project g%2Fr --yes")
}

func TestVerifyDiffNote_RejectsWrongLine(t *testing.T) {
	data := []byte(`{"id":"disc1","notes":[{"id":77,"type":"DiffNote","position":{"new_path":"x.go","new_line":12}}]}`)
	line := 49
	want := diffPosition{NewPath: "x.go", NewLine: &line}
	_, _, err := verifyDiffNote(data, want, "g%2Fr", "42")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "anchored to line 12, expected 49")
}

func TestVerifyDiffNote_RejectsWrongPath(t *testing.T) {
	data := []byte(`{"id":"disc1","notes":[{"id":77,"type":"DiffNote","position":{"new_path":"other.go","new_line":49}}]}`)
	line := 49
	want := diffPosition{NewPath: "x.go", NewLine: &line}
	_, _, err := verifyDiffNote(data, want, "g%2Fr", "42")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected x.go")
}

func TestFindDiffFile_MatchesEitherSideOfRename(t *testing.T) {
	files := []mrDiffFile{{OldPath: "a.go", NewPath: "b.go", RenamedFile: true}}
	f, err := findDiffFile(files, "a.go")
	require.NoError(t, err)
	assert.Equal(t, "b.go", f.NewPath)
}

func TestFindDiffFile_UnknownPathListsChangedFiles(t *testing.T) {
	files := []mrDiffFile{{OldPath: "a.go", NewPath: "a.go"}, {OldPath: "b.go", NewPath: "b.go"}}
	_, err := findDiffFile(files, "c.go")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "changed files: a.go, b.go")
}

func TestLineLabel(t *testing.T) {
	o, n := 12, 49
	assert.Equal(t, "12/49", lineLabel(&o, &n))
	assert.Equal(t, "49", lineLabel(nil, &n))
	assert.Equal(t, "-12", lineLabel(&o, nil))
	assert.Equal(t, "?", lineLabel(nil, nil))
}
