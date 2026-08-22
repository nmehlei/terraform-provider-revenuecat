package revenuecat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestListFollowsCursorAcrossPages(t *testing.T) {
	var calls atomic.Int32
	var seenCursors []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCursors = append(seenCursors, r.URL.Query().Get("starting_after"))
		switch calls.Add(1) {
		case 1:
			writeJSON(t, w, http.StatusOK, listResponse[Entitlement]{
				Object:   "list",
				Items:    []Entitlement{{ID: "entl1"}, {ID: "entl2"}},
				NextPage: "/v2/projects/proj1/entitlements?starting_after=entl2",
			})
		default:
			writeJSON(t, w, http.StatusOK, listResponse[Entitlement]{
				Object: "list",
				Items:  []Entitlement{{ID: "entl3"}},
			})
		}
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	got, err := c.ListEntitlements(context.Background(), "proj1")
	if err != nil {
		t.Fatalf("ListEntitlements: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("got %d entitlements, want 3", len(got))
	}
	for i, want := range []string{"entl1", "entl2", "entl3"} {
		if got[i].ID != want {
			t.Errorf("item %d = %q, want %q (order must be preserved)", i, got[i].ID, want)
		}
	}
	if len(seenCursors) != 2 || seenCursors[0] != "" || seenCursors[1] != "entl2" {
		t.Errorf("cursors sent = %v, want [\"\", \"entl2\"]", seenCursors)
	}
}

func TestListSinglePageIssuesOneRequest(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		writeJSON(t, w, http.StatusOK, listResponse[Entitlement]{
			Object: "list",
			Items:  []Entitlement{{ID: "entl1"}},
		})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	got, err := c.ListEntitlements(context.Background(), "proj1")
	if err != nil {
		t.Fatalf("ListEntitlements: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d entitlements, want 1", len(got))
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want exactly 1", calls.Load())
	}
}

func TestListStopsOnEndlessCursor(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		writeJSON(t, w, http.StatusOK, listResponse[Entitlement]{
			Object:   "list",
			Items:    []Entitlement{{ID: fmt.Sprintf("entl%d", n)}},
			NextPage: fmt.Sprintf("/v2/projects/proj1/entitlements?starting_after=entl%d", n),
		})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	_, err := c.ListEntitlements(context.Background(), "proj1")
	if err == nil {
		t.Fatal("ListEntitlements succeeded; want an error rather than paging forever")
	}
	if !strings.Contains(err.Error(), "exceeded") {
		t.Errorf("error = %v, want it to report exceeding the page bound", err)
	}
	if int(calls.Load()) != maxPages {
		t.Errorf("calls = %d, want the %d page bound", calls.Load(), maxPages)
	}
}

func TestNextPageCursor(t *testing.T) {
	tests := []struct {
		name     string
		nextPage string
		want     string
	}{
		{"empty", "", ""},
		{"path with cursor", "/v2/projects/p1/apps?starting_after=app9", "app9"},
		{"absolute url with cursor", "https://api.revenuecat.com/v2/projects/p1/apps?starting_after=app9", "app9"},
		{"bare cursor", "app9", "app9"},
		{"path without cursor", "/v2/projects/p1/apps", ""},
		{"query without cursor", "/v2/projects/p1/apps?limit=20", ""},
		{"whitespace", "   ", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := nextPageCursor(tc.nextPage); got != tc.want {
				t.Errorf("nextPageCursor(%q) = %q, want %q", tc.nextPage, got, tc.want)
			}
		})
	}
}

func TestListQueryValuesAreNotMutatedAcrossCalls(t *testing.T) {
	// A caller-supplied query map must survive pagination unmodified, or a
	// second call would start from the first call's last cursor.
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			writeJSON(t, w, http.StatusOK, listResponse[App]{
				Items:    []App{{ID: "app1"}},
				NextPage: "?starting_after=app1",
			})
			return
		}
		writeJSON(t, w, http.StatusOK, listResponse[App]{Items: []App{{ID: "app2"}}})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	shared := map[string][]string{"limit": {"20"}}
	if _, err := listAll[App](context.Background(), c, "/projects/p1/apps", shared); err != nil {
		t.Fatalf("listAll: %v", err)
	}
	if _, ok := shared["starting_after"]; ok {
		t.Error("listAll mutated the caller's query values by adding starting_after")
	}
}
