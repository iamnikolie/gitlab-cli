package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Client is the GitLab REST v4 HTTP client. All requests carry the
// PRIVATE-TOKEN header and retry on HTTP 429 with exponential backoff.
type Client struct {
	token   string
	baseURL string // e.g. https://gitlab.com/api/v4
	http    *http.Client
	Verbose bool
	// RetryWait is the initial 429 backoff. Defaults to time.Second;
	// tests set it to a tiny value.
	RetryWait time.Duration
}

func New(token, baseURL string) *Client {
	return &Client{
		token:     token,
		baseURL:   strings.TrimRight(baseURL, "/"),
		http:      &http.Client{Timeout: 30 * time.Second},
		RetryWait: time.Second,
	}
}

// do performs a single request to fullURL with 429 retry. Returns the
// response body and status code. Non-2xx (other than retried 429) returns
// an error whose message embeds the status and body.
func (c *Client) do(ctx context.Context, method, fullURL string, body []byte, contentType string) ([]byte, int, error) {
	const maxRetries = 3
	wait := c.RetryWait
	if wait <= 0 {
		wait = time.Second
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		var br io.Reader
		if body != nil {
			br = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, fullURL, br)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("PRIVATE-TOKEN", c.token)
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}

		if c.Verbose {
			fmt.Fprintf(os.Stderr, "→ %s %s\n", method, fullURL)
			if body != nil {
				fmt.Fprintf(os.Stderr, "%s\n", string(body))
			}
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, 0, err
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if c.Verbose {
			fmt.Fprintf(os.Stderr, "← HTTP %d\n%s\n", resp.StatusCode, string(b))
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt == maxRetries {
				return nil, resp.StatusCode, fmt.Errorf("HTTP 429: rate limited after %d retries", maxRetries)
			}
			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			case <-time.After(wait):
			}
			wait *= 2
			continue
		}
		if resp.StatusCode >= 400 {
			return b, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
		}
		return b, resp.StatusCode, nil
	}
	return nil, 0, fmt.Errorf("unreachable")
}

// url builds baseURL + path + encoded query.
func (c *Client) url(path string, query url.Values) string {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return u
}

// Get issues a GET request and returns the response body.
func (c *Client) Get(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	b, _, err := c.do(ctx, http.MethodGet, c.url(path, query), nil, "")
	return b, err
}

// Send issues an arbitrary-method request with an optional body.
func (c *Client) Send(ctx context.Context, method, path string, query url.Values, body []byte, contentType string) (json.RawMessage, error) {
	b, _, err := c.do(ctx, method, c.url(path, query), body, contentType)
	return b, err
}

// GetPaginated requests pages via per_page/page until `limit` items are
// collected or a short page signals exhaustion. Returns the combined JSON
// array (capped at limit) and whether the limit was reached (more may exist).
func (c *Client) GetPaginated(ctx context.Context, path string, query url.Values, limit int) (json.RawMessage, bool, error) {
	if limit <= 0 {
		limit = 1
	}
	base := url.Values{}
	for k, vs := range query {
		for _, v := range vs {
			base.Add(k, v)
		}
	}

	var all []json.RawMessage
	page := 1
	for len(all) < limit {
		perPage := min(limit-len(all), 100)
		q := url.Values{}
		for k, vs := range base {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		q.Set("per_page", strconv.Itoa(perPage))
		q.Set("page", strconv.Itoa(page))

		b, _, err := c.do(ctx, http.MethodGet, c.url(path, q), nil, "")
		if err != nil {
			return nil, false, err
		}
		var items []json.RawMessage
		if err := json.Unmarshal(b, &items); err != nil {
			return nil, false, fmt.Errorf("client.GetPaginated: decode: %w", err)
		}
		all = append(all, items...)
		if len(items) < perPage {
			break // exhausted
		}
		page++
	}

	hitLimit := len(all) >= limit
	if len(all) > limit {
		all = all[:limit]
	}
	out, err := json.Marshal(all)
	if err != nil {
		return nil, false, fmt.Errorf("client.GetPaginated: marshal: %w", err)
	}
	return out, hitLimit, nil
}

// GraphQL POSTs a query to the host-level /api/graphql endpoint. The endpoint
// is derived by trimming the /api/v4 suffix from baseURL.
func (c *Client) GraphQL(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error) {
	endpoint := strings.TrimSuffix(c.baseURL, "/api/v4") + "/api/graphql"
	payload := map[string]any{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("client.GraphQL: marshal: %w", err)
	}
	b, _, err := c.do(ctx, http.MethodPost, endpoint, body, "application/json")
	return b, err
}
