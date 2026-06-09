package client_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/langgerone/gitlab-cli/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGet_SetsPrivateTokenHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "glpat-x", r.Header.Get("PRIVATE-TOKEN"))
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/user", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"username":"alice"}`)
	}))
	defer srv.Close()

	c := client.New("glpat-x", srv.URL)
	body, err := c.Get(context.Background(), "/user", nil)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(body, &m))
	assert.Equal(t, "alice", m["username"])
}

func TestGet_HTTPErrorIncludesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"404 Project Not Found"}`)
	}))
	defer srv.Close()

	c := client.New("t", srv.URL)
	_, err := c.Get(context.Background(), "/projects/x", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "Not Found")
}

func TestGet_RetriesOn429(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	c := client.New("t", srv.URL)
	c.RetryWait = 1 // 1ns backoff so the test is fast
	body, err := c.Get(context.Background(), "/x", nil)
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	assert.JSONEq(t, `{"ok":true}`, string(body))
}

func TestGetPaginated_StopsAtLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		// Server has effectively unlimited items; always return a full page.
		items := make([]map[string]int, perPage)
		for i := range items {
			items[i] = map[string]int{"id": (page-1)*perPage + i}
		}
		_ = json.NewEncoder(w).Encode(items)
	}))
	defer srv.Close()

	c := client.New("t", srv.URL)
	body, hitLimit, err := c.GetPaginated(context.Background(), "/items", nil, 5)
	require.NoError(t, err)
	assert.True(t, hitLimit)
	var items []map[string]int
	require.NoError(t, json.Unmarshal(body, &items))
	assert.Len(t, items, 5)
	assert.Equal(t, 0, items[0]["id"])
	assert.Equal(t, 4, items[4]["id"])
}

func TestGetPaginated_StopsOnExhaustion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page == 1 {
			fmt.Fprint(w, `[{"id":1},{"id":2}]`) // fewer than per_page → exhausted
			return
		}
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	c := client.New("t", srv.URL)
	body, hitLimit, err := c.GetPaginated(context.Background(), "/items", nil, 50)
	require.NoError(t, err)
	assert.False(t, hitLimit)
	var items []map[string]int
	require.NoError(t, json.Unmarshal(body, &items))
	assert.Len(t, items, 2)
}

func TestGetPaginated_MergesQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "opened", r.URL.Query().Get("state"))
		assert.NotEmpty(t, r.URL.Query().Get("per_page"))
		fmt.Fprint(w, `[{"id":1}]`)
	}))
	defer srv.Close()

	c := client.New("t", srv.URL)
	q := url.Values{}
	q.Set("state", "opened")
	_, _, err := c.GetPaginated(context.Background(), "/mrs", q, 10)
	require.NoError(t, err)
}

func TestSend_PostBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var m map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&m))
		assert.Equal(t, "hi", m["title"])
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"iid":7}`)
	}))
	defer srv.Close()

	c := client.New("t", srv.URL)
	body, err := c.Send(context.Background(), http.MethodPost, "/mrs", nil,
		[]byte(`{"title":"hi"}`), "application/json")
	require.NoError(t, err)
	assert.JSONEq(t, `{"iid":7}`, string(body))
}

func TestSend_NoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := client.New("t", srv.URL)
	body, err := c.Send(context.Background(), http.MethodDelete, "/branches/x", nil, nil, "")
	require.NoError(t, err)
	assert.Empty(t, body)
}

func TestGraphQL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/graphql", r.URL.Path)
		var m map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&m))
		assert.Equal(t, "{ currentUser { name } }", m["query"])
		fmt.Fprint(w, `{"data":{"currentUser":{"name":"Alice"}}}`)
	}))
	defer srv.Close()

	c := client.New("t", srv.URL+"/api/v4")
	body, err := c.GraphQL(context.Background(), "{ currentUser { name } }", nil)
	require.NoError(t, err)
	assert.Contains(t, string(body), "Alice")
}

func TestVerboseDefault(t *testing.T) {
	c := client.New("t", "https://gitlab.com/api/v4")
	assert.False(t, c.Verbose)
}
