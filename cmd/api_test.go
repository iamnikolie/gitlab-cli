package cmd

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFields(t *testing.T) {
	v, err := parseFields([]string{"a=1", "b=hello world", "c="})
	require.NoError(t, err)
	assert.Equal(t, "1", v.Get("a"))
	assert.Equal(t, "hello world", v.Get("b"))
	assert.Equal(t, "", v.Get("c"))
}

func TestParseFields_NoEquals(t *testing.T) {
	_, err := parseFields([]string{"bad"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key=value")
}

func TestParseFields_RepeatedKey(t *testing.T) {
	v, err := parseFields([]string{"labels=a", "labels=b"})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, v["labels"])
}

func TestSplitPathQuery(t *testing.T) {
	path, q := splitPathQuery("/projects/1/issues?state=opened&x=2")
	assert.Equal(t, "/projects/1/issues", path)
	assert.Equal(t, "opened", q.Get("state"))
	assert.Equal(t, "2", q.Get("x"))
}

func TestSplitPathQuery_NoQuery(t *testing.T) {
	path, q := splitPathQuery("/user")
	assert.Equal(t, "/user", path)
	assert.Equal(t, url.Values{}, q)
}

func TestNormalizeAPIPath(t *testing.T) {
	assert.Equal(t, "/user", normalizeAPIPath("user"))
	assert.Equal(t, "/user", normalizeAPIPath("/user"))
}

// A bracketed -f key is the shape that makes GitLab answer 201 while dropping
// the field — an inline note becomes a plain comment. Refuse it loudly.
func TestCheckFormFields_RejectsBracketedKeys(t *testing.T) {
	v := url.Values{}
	v.Set("body", "x")
	v.Set("position[new_line]", "49")
	v.Set("position[position_type]", "text")
	err := checkFormFields(v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "position[new_line], position[position_type]")
	assert.Contains(t, err.Error(), "--data-file")
}

func TestCheckFormFields_AllowsPlainKeys(t *testing.T) {
	v := url.Values{}
	v.Set("body", "x")
	v.Set("labels", "bug,urgent")
	require.NoError(t, checkFormFields(v))
}
