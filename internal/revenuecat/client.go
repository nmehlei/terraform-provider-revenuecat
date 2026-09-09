// Package revenuecat implements a typed client for the RevenueCat REST API v2.
//
// Every fact about RevenueCat's wire format — endpoint paths, JSON field names,
// pagination and error shapes — is confined to this package so that a
// correction touches one place rather than every resource in the provider.
package revenuecat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the RevenueCat REST API v2 endpoint.
	DefaultBaseURL = "https://api.revenuecat.com/v2"

	// DefaultMaxRetries is how many times a retryable response is retried
	// before the error is surfaced.
	DefaultMaxRetries = 3

	// DefaultTimeout bounds a single HTTP request.
	DefaultTimeout = 30 * time.Second

	// maxErrorBodyExcerpt bounds how much of an undecodable error body is
	// copied into the returned error.
	maxErrorBodyExcerpt = 512

	// maxPages bounds cursor pagination so a server that always returns a
	// next-page cursor cannot make a list call run forever.
	maxPages = 1000

	// baseBackoff is the first retry delay; each subsequent attempt doubles it.
	baseBackoff = 500 * time.Millisecond

	// maxBackoff caps the exponential backoff delay.
	maxBackoff = 30 * time.Second
)

// Client talks to the RevenueCat REST API v2.
type Client struct {
	baseURL    *url.URL
	apiKey     string
	httpClient *http.Client
	userAgent  string
	maxRetries int

	// sleep is the delay function used between retries. Tests replace it to
	// avoid real waiting.
	sleep func(ctx context.Context, d time.Duration) error
}

// Option customizes a Client.
type Option func(*Client)

// WithHTTPClient sets the underlying HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithMaxRetries sets how many times a retryable response is retried. Negative
// values are clamped to zero.
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		if n < 0 {
			n = 0
		}
		c.maxRetries = n
	}
}

// WithTimeout bounds a single HTTP request.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.httpClient.Timeout = d
		}
	}
}

// WithUserAgent sets the User-Agent header sent with every request.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		if ua != "" {
			c.userAgent = ua
		}
	}
}

// New returns a Client authenticating with the given API v2 secret key. An
// empty baseURL selects DefaultBaseURL.
func New(apiKey, baseURL string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("revenuecat: api key must not be empty")
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	parsed, err := ParseBaseURL(baseURL)
	if err != nil {
		return nil, err
	}

	c := &Client{
		baseURL:    parsed,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: DefaultTimeout},
		userAgent:  "terraform-provider-revenuecat",
		maxRetries: DefaultMaxRetries,
		sleep:      sleepCtx,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// ParseBaseURL validates that raw is an absolute http or https URL usable as an
// API base. It is exported so the provider can report a malformed base_url as a
// configuration diagnostic before constructing a client.
func ParseBaseURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("revenuecat: invalid base URL %q: %w", raw, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("revenuecat: invalid base URL %q: scheme must be http or https", raw)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("revenuecat: invalid base URL %q: missing host", raw)
	}
	return parsed, nil
}

// BaseURL returns the configured base URL as a string.
func (c *Client) BaseURL() string {
	return strings.TrimSuffix(c.baseURL.String(), "/")
}

// resolve joins a relative API path onto the base URL, preserving any path
// prefix the base URL carries. A base of http://host/v2 and a path of
// /projects/p1/apps resolves to http://host/v2/projects/p1/apps.
func (c *Client) resolve(path string, query url.Values) string {
	resolved := *c.baseURL
	resolved.Path = strings.TrimSuffix(c.baseURL.Path, "/") + "/" + strings.TrimPrefix(path, "/")
	if len(query) > 0 {
		resolved.RawQuery = query.Encode()
	} else {
		resolved.RawQuery = ""
	}
	return resolved.String()
}

// errorEnvelope covers the shapes RevenueCat uses to report an error, both a
// top-level object and one nested under "error".
type errorEnvelope struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Error   *struct {
		Type    string `json:"type"`
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// do executes a single API request with retries, decoding a successful JSON
// response into out. A nil body sends no payload; a nil out discards the
// response body.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("revenuecat: encoding request body for %s %s: %w", method, path, err)
		}
	}

	target := c.resolve(path, query)

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := backoffDelay(attempt, lastErr)
			if err := c.sleep(ctx, delay); err != nil {
				return err
			}
		}

		var reader io.Reader
		if encoded != nil {
			reader = bytes.NewReader(encoded)
		}

		req, err := http.NewRequestWithContext(ctx, method, target, reader)
		if err != nil {
			return fmt.Errorf("revenuecat: building request %s %s: %w", method, path, err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", c.userAgent)
		if encoded != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			// A canceled context is the caller's decision, not a transient
			// failure: stop rather than burning through the retry budget.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return fmt.Errorf("revenuecat: %s %s: %w", method, path, ctxErr)
			}
			lastErr = fmt.Errorf("revenuecat: %s %s: %w", method, path, err)
			continue
		}

		result, retryable := c.handleResponse(resp, method, path, out)
		if retryable {
			lastErr = result
			continue
		}
		return result
	}

	return lastErr
}

// handleResponse consumes resp, reporting whether the failure it represents is
// worth retrying.
func (c *Client) handleResponse(resp *http.Response, method, path string, out any) (err error, retryable bool) {
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out == nil || resp.StatusCode == http.StatusNoContent {
			return nil, false
		}
		if decodeErr := json.NewDecoder(resp.Body).Decode(out); decodeErr != nil {
			return fmt.Errorf("revenuecat: decoding response for %s %s: %w", method, path, decodeErr), false
		}
		return nil, false
	}

	apiErr := c.decodeError(resp, method, path)
	return apiErr, isRetryableStatus(resp.StatusCode)
}

func (c *Client) decodeError(resp *http.Response, method, path string) *APIError {
	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		Method:     method,
		Path:       path,
	}

	// Retry-After is only readable while the response is in hand, and it must
	// be captured even when the body is empty or unreadable.
	if after := parseRetryAfter(resp.Header.Get("Retry-After")); after > 0 {
		apiErr.retryAfter = after
	}

	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyExcerpt*4))
	if readErr != nil || len(raw) == 0 {
		return apiErr
	}

	var envelope errorEnvelope
	if json.Unmarshal(raw, &envelope) == nil {
		if envelope.Error != nil {
			apiErr.Code = firstNonEmpty(envelope.Error.Code, envelope.Error.Type)
			apiErr.Message = envelope.Error.Message
		}
		if apiErr.Code == "" {
			apiErr.Code = firstNonEmpty(envelope.Code, envelope.Type)
		}
		if apiErr.Message == "" {
			apiErr.Message = envelope.Message
		}
	}

	if apiErr.Message == "" {
		apiErr.Body = truncate(strings.TrimSpace(string(raw)), maxErrorBodyExcerpt)
	}

	return apiErr
}

func isRetryableStatus(status int) bool {
	// 423 Locked: the API returns this for a mutation on a package whose
	// offering has another mutation in flight (its own concurrency control,
	// not a client error) — observed destroying one package while updating
	// another in the same offering, which a plain Terraform apply does
	// concurrently by default since the two are independent in the resource
	// graph. Retrying is the correct response, the same as 429 and 5xx.
	return status == http.StatusTooManyRequests || status == http.StatusLocked || status >= 500
}

// backoffDelay returns the wait before the given attempt (1-based), honoring a
// Retry-After hint carried by the previous error.
func backoffDelay(attempt int, lastErr error) time.Duration {
	var apiErr *APIError
	if errors.As(lastErr, &apiErr) && apiErr.retryAfter > 0 {
		return apiErr.retryAfter
	}

	shift := attempt - 1
	if shift < 0 {
		shift = 0
	}
	if shift > 20 {
		shift = 20
	}
	delay := time.Duration(float64(baseBackoff) * math.Pow(2, float64(shift)))
	if delay > maxBackoff {
		delay = maxBackoff
	}
	return delay
}

func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil {
		if seconds < 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(header); err == nil {
		if d := time.Until(when); d > 0 {
			return d
		}
	}
	return 0
}

// sleepCtx waits for d, returning early if ctx is canceled so a canceled apply
// does not sit out the remaining backoff.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
