package revenuecat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient returns a client pointed at srv with retry backoff replaced by
// a recorder, so retry tests assert on the delays without waiting them out.
func newTestClient(t *testing.T, srv *httptest.Server, opts ...Option) (*Client, *[]time.Duration) {
	t.Helper()

	var waits []time.Duration
	c, err := New("test-key", srv.URL+"/v2", opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.sleep = func(ctx context.Context, d time.Duration) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		waits = append(waits, d)
		return nil
	}
	return c, &waits
}

func TestRequestHeaders(t *testing.T) {
	var gotAuth, gotAccept, gotUA, gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotUA = r.Header.Get("User-Agent")
		gotContentType = r.Header.Get("Content-Type")
		writeJSON(t, w, http.StatusOK, Entitlement{ID: "entl1"})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	if _, err := c.CreateEntitlement(context.Background(), "proj1", CreateEntitlementRequest{LookupKey: "pro"}); err != nil {
		t.Fatalf("CreateEntitlement: %v", err)
	}

	if want := "Bearer test-key"; gotAuth != want {
		t.Errorf("Authorization = %q, want %q", gotAuth, want)
	}
	if want := "application/json"; gotAccept != want {
		t.Errorf("Accept = %q, want %q", gotAccept, want)
	}
	if !strings.Contains(gotUA, "terraform-provider-revenuecat") {
		t.Errorf("User-Agent = %q, want it to contain terraform-provider-revenuecat", gotUA)
	}
	if want := "application/json"; gotContentType != want {
		t.Errorf("Content-Type = %q, want %q", gotContentType, want)
	}
}

func TestRequestWithoutBodyOmitsContentType(t *testing.T) {
	var hadContentType bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadContentType = r.Header["Content-Type"]
		writeJSON(t, w, http.StatusOK, Entitlement{ID: "entl1"})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	if _, err := c.GetEntitlement(context.Background(), "proj1", "entl1"); err != nil {
		t.Fatalf("GetEntitlement: %v", err)
	}
	if hadContentType {
		t.Error("GET request sent a Content-Type header; want none")
	}
}

func TestBaseURLPathPrefixPreserved(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		writeJSON(t, w, http.StatusOK, listResponse[Entitlement]{Items: []Entitlement{}})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	if _, err := c.ListEntitlements(context.Background(), "proj1"); err != nil {
		t.Fatalf("ListEntitlements: %v", err)
	}
	if want := "/v2/projects/proj1/entitlements"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

func TestParseBaseURLRejectsInvalid(t *testing.T) {
	for _, raw := range []string{"", "not-a-url", "ftp://example.com", "https://"} {
		if _, err := ParseBaseURL(raw); err == nil {
			t.Errorf("ParseBaseURL(%q) succeeded; want an error", raw)
		}
	}
	if _, err := ParseBaseURL("https://api.revenuecat.com/v2"); err != nil {
		t.Errorf("ParseBaseURL of a valid URL failed: %v", err)
	}
}

func TestNewRejectsEmptyAPIKey(t *testing.T) {
	if _, err := New("", ""); err == nil {
		t.Error("New with an empty API key succeeded; want an error")
	}
}

func TestStructuredErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"type":"parameter_error","code":"invalid_lookup_key","message":"lookup_key is invalid"}`)
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	_, err := c.GetEntitlement(context.Background(), "proj1", "entl1")
	if err == nil {
		t.Fatal("GetEntitlement succeeded; want an error")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error is %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want 400", apiErr.StatusCode)
	}
	if apiErr.Code != "invalid_lookup_key" {
		t.Errorf("Code = %q, want invalid_lookup_key", apiErr.Code)
	}
	if apiErr.Message != "lookup_key is invalid" {
		t.Errorf("Message = %q, want lookup_key is invalid", apiErr.Message)
	}
	if !strings.Contains(apiErr.Error(), "/projects/proj1/entitlements/entl1") {
		t.Errorf("Error() = %q, want it to name the request path", apiErr.Error())
	}
}

func TestNestedErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, `{"error":{"code":"duplicate","message":"already exists"}}`)
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	_, err := c.GetEntitlement(context.Background(), "proj1", "entl1")
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error is %T, want *APIError", err)
	}
	if apiErr.Code != "duplicate" || apiErr.Message != "already exists" {
		t.Errorf("got code %q message %q, want duplicate/already exists", apiErr.Code, apiErr.Message)
	}
}

func TestUnstructuredErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "<html>gateway exploded</html>")
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv, WithMaxRetries(0))
	_, err := c.GetEntitlement(context.Background(), "proj1", "entl1")
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error is %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want 500", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Body, "gateway exploded") {
		t.Errorf("Body = %q, want it to excerpt the response body", apiErr.Body)
	}
}

func TestNotFoundIsRecognizable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"code":"not_found","message":"no such entitlement"}`)
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	_, err := c.GetEntitlement(context.Background(), "proj1", "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound(%v) = false, want true", err)
	}
	if got := StatusCode(err); got != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want 404", got)
	}
	if IsNotFound(fmt.Errorf("some other failure")) {
		t.Error("IsNotFound reported true for a non-API error")
	}
}

func TestRateLimitIsRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeJSON(t, w, http.StatusOK, Entitlement{ID: "entl1", LookupKey: "pro"})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	got, err := c.GetEntitlement(context.Background(), "proj1", "entl1")
	if err != nil {
		t.Fatalf("GetEntitlement: %v", err)
	}
	if got.LookupKey != "pro" {
		t.Errorf("LookupKey = %q, want pro", got.LookupKey)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", calls.Load())
	}
}

// TestResourceLockedIsRetried guards against a real API behavior: mutating
// two packages in the same offering concurrently (as a plain Terraform apply
// does by default, since they're independent in the resource graph) makes
// the API 423 one of them with "currently being updated by a different
// request" — observed in production destroying revenuecat_package.annual
// while revenuecat_package.monthly was being updated in the same apply. That
// is exactly the kind of transient, retry-and-it-clears failure the client's
// retry loop already handles for 429 and 5xx; 423 belongs in the same set.
func TestResourceLockedIsRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusLocked)
			fmt.Fprint(w, `{"code":"resource_locked_error","message":"Packages from offering ofrng1 are currently being updated by a different request"}`)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	err := c.DeletePackage(context.Background(), "proj1", "pkg1")
	if err != nil {
		t.Fatalf("DeletePackage: %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", calls.Load())
	}
}

func TestRetryAfterIsHonored(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeJSON(t, w, http.StatusOK, Entitlement{ID: "entl1"})
	}))
	defer srv.Close()

	c, waits := newTestClient(t, srv)
	if _, err := c.GetEntitlement(context.Background(), "proj1", "entl1"); err != nil {
		t.Fatalf("GetEntitlement: %v", err)
	}
	if len(*waits) != 1 {
		t.Fatalf("recorded %d waits, want 1", len(*waits))
	}
	if (*waits)[0] < 7*time.Second {
		t.Errorf("waited %v, want at least the 7s Retry-After", (*waits)[0])
	}
}

func TestClientErrorIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	if _, err := c.GetEntitlement(context.Background(), "proj1", "entl1"); err == nil {
		t.Fatal("GetEntitlement succeeded; want an error")
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want exactly 1 (422 must not be retried)", calls.Load())
	}
}

func TestRetriesAreExhausted(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c, waits := newTestClient(t, srv, WithMaxRetries(2))
	_, err := c.GetEntitlement(context.Background(), "proj1", "entl1")
	if err == nil {
		t.Fatal("GetEntitlement succeeded; want an error after exhausting retries")
	}
	if StatusCode(err) != http.StatusServiceUnavailable {
		t.Errorf("StatusCode = %d, want 503", StatusCode(err))
	}
	if calls.Load() != 3 {
		t.Errorf("calls = %d, want 3 (initial attempt plus 2 retries)", calls.Load())
	}
	if len(*waits) != 2 {
		t.Errorf("recorded %d waits, want 2", len(*waits))
	}
	// Backoff must grow rather than hammer at a fixed interval.
	if len(*waits) == 2 && (*waits)[1] <= (*waits)[0] {
		t.Errorf("backoff did not grow: %v then %v", (*waits)[0], (*waits)[1])
	}
}

func TestCancellationDuringBackoffStopsRetrying(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	// A real sleep here, with a context that is canceled while it is pending.
	c, err := New("test-key", srv.URL+"/v2", WithMaxRetries(5))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.sleep = func(ctx context.Context, d time.Duration) error {
		cancel()
		return sleepCtx(ctx, d)
	}

	start := time.Now()
	if _, err := c.GetEntitlement(ctx, "proj1", "entl1"); err == nil {
		t.Fatal("GetEntitlement succeeded; want a cancellation error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %v; cancellation should return promptly rather than waiting out the backoff", elapsed)
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want 1 (cancellation must stop further attempts)", calls.Load())
	}
}

func TestTransportErrorIsRetriedThenSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // nothing is listening, so every attempt fails at the transport

	c, waits := newTestClient(t, srv, WithMaxRetries(2))
	if _, err := c.GetEntitlement(context.Background(), "proj1", "entl1"); err == nil {
		t.Fatal("GetEntitlement succeeded; want a transport error")
	}
	if len(*waits) != 2 {
		t.Errorf("recorded %d waits, want 2 retries of a transport error", len(*waits))
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Errorf("encoding test response: %v", err)
	}
}

func readBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("reading request body: %v", err)
	}
	if len(raw) == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding request body %q: %v", raw, err)
	}
	return out
}
