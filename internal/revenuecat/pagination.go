package revenuecat

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// listResponse is the envelope every v2 list endpoint returns.
type listResponse[T any] struct {
	Object   string `json:"object"`
	Items    []T    `json:"items"`
	NextPage string `json:"next_page"`
	URL      string `json:"url"`
}

// listAll walks cursor pagination from path, returning the concatenation of
// every page in order. It stops with an error past maxPages so a server that
// always returns a cursor cannot make the call run forever.
func listAll[T any](ctx context.Context, c *Client, path string, query url.Values) ([]T, error) {
	if query == nil {
		query = url.Values{}
	} else {
		// Copy so repeated calls with the same values are not mutated by the
		// cursor we add per page.
		copied := url.Values{}
		for k, v := range query {
			copied[k] = append([]string(nil), v...)
		}
		query = copied
	}

	var all []T
	for page := 0; ; page++ {
		if page >= maxPages {
			return nil, fmt.Errorf("revenuecat: listing %s exceeded %d pages; the API kept returning a next-page cursor", path, maxPages)
		}

		var resp listResponse[T]
		if err := c.do(ctx, "GET", path, query, nil, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Items...)

		cursor := nextPageCursor(resp.NextPage)
		if cursor == "" {
			return all, nil
		}
		query.Set("starting_after", cursor)
	}
}

// nextPageCursor extracts the pagination cursor from a next_page value. The API
// returns a URL or path carrying a starting_after query parameter; a bare
// cursor value is accepted too.
func nextPageCursor(nextPage string) string {
	nextPage = strings.TrimSpace(nextPage)
	if nextPage == "" {
		return ""
	}

	if parsed, err := url.Parse(nextPage); err == nil {
		if cursor := parsed.Query().Get("starting_after"); cursor != "" {
			return cursor
		}
		// A parseable value with no cursor parameter carries no way to advance;
		// treating it as a bare cursor would loop on the same page forever.
		if parsed.RawQuery != "" || strings.Contains(nextPage, "/") {
			return ""
		}
	}

	return nextPage
}
