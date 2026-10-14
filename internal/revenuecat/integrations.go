package revenuecat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// Environment values a webhook integration or a public API key can carry.
const (
	EnvironmentProduction = "production"
	EnvironmentSandbox    = "sandbox"
)

// WebhookEnvironments lists every accepted value of a webhook integration's
// environment. Null (no environment) means events from both are sent.
var WebhookEnvironments = []string{EnvironmentProduction, EnvironmentSandbox}

// WebhookIntegration is a webhook RevenueCat calls for a project's events.
//
// It deliberately has no field for the authorization header or the signing
// secret: neither is returned on an ordinary read, and a struct with nowhere to
// put them is what guarantees a response can never carry one into state.
type WebhookIntegration struct {
	ID          string   `json:"id"`
	ProjectID   string   `json:"project_id"`
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	Environment *string  `json:"environment"`
	EventTypes  []string `json:"event_types"`
	AppID       *string  `json:"app_id"`
	CreatedAt   int64    `json:"created_at"`
}

// CreateWebhookIntegrationRequest is the payload for creating a webhook
// integration. Unset optional fields are omitted, which the API reads as
// "all environments", "all event types" and "all apps".
type CreateWebhookIntegrationRequest struct {
	Name                string   `json:"name"`
	URL                 string   `json:"url"`
	AuthorizationHeader *string  `json:"authorization_header,omitempty"`
	Environment         *string  `json:"environment,omitempty"`
	EventTypes          []string `json:"event_types,omitempty"`
	AppID               *string  `json:"app_id,omitempty"`
}

// UpdateWebhookIntegrationRequest is the payload for updating a webhook
// integration. See MarshalJSON for why its fields are not plain json tags.
type UpdateWebhookIntegrationRequest struct {
	Name        string
	URL         string
	Environment *string
	EventTypes  []string
	AppID       *string

	// RotateAuthorizationHeader sends AuthorizationHeader (null when it is
	// nil). Without it the header is omitted and RevenueCat keeps the one it
	// holds.
	RotateAuthorizationHeader bool
	AuthorizationHeader       *string
}

// MarshalJSON writes the scope fields as explicit nulls when unset, because the
// API treats an omitted field as "unchanged": removing an environment or app
// scope from configuration would otherwise never reach RevenueCat. The header
// is the reverse — omitted unless rotated, so a rename never sends the secret.
func (r UpdateWebhookIntegrationRequest) MarshalJSON() ([]byte, error) {
	body := map[string]any{
		"name":        r.Name,
		"url":         r.URL,
		"environment": r.Environment,
		"app_id":      r.AppID,
		"event_types": nil,
	}
	if len(r.EventTypes) > 0 {
		body["event_types"] = r.EventTypes
	}
	if r.RotateAuthorizationHeader {
		body["authorization_header"] = r.AuthorizationHeader
	}
	return json.Marshal(body)
}

// PublicAPIKey is an SDK key RevenueCat generates for an app. It is public by
// design: it ships inside the app.
type PublicAPIKey struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Environment string `json:"environment"`
	AppID       string `json:"app_id"`
	CreatedAt   int64  `json:"created_at"`
}

// CreateWebhookIntegration creates a webhook integration in a project.
func (c *Client) CreateWebhookIntegration(ctx context.Context, projectID string, req CreateWebhookIntegrationRequest) (*WebhookIntegration, error) {
	var out WebhookIntegration
	if err := c.do(ctx, "POST", webhooksPath(projectID), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetWebhookIntegration reads a single webhook integration.
func (c *Client) GetWebhookIntegration(ctx context.Context, projectID, webhookID string) (*WebhookIntegration, error) {
	var out WebhookIntegration
	if err := c.do(ctx, "GET", webhooksPath(projectID)+"/"+url.PathEscape(webhookID), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateWebhookIntegration updates a webhook integration in place.
func (c *Client) UpdateWebhookIntegration(ctx context.Context, projectID, webhookID string, req UpdateWebhookIntegrationRequest) (*WebhookIntegration, error) {
	var out WebhookIntegration
	if err := c.do(ctx, "POST", webhooksPath(projectID)+"/"+url.PathEscape(webhookID), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteWebhookIntegration removes a webhook integration.
func (c *Client) DeleteWebhookIntegration(ctx context.Context, projectID, webhookID string) error {
	return c.do(ctx, "DELETE", webhooksPath(projectID)+"/"+url.PathEscape(webhookID), nil, nil, nil)
}

// ListAppPublicAPIKeys returns the public SDK keys of an app.
func (c *Client) ListAppPublicAPIKeys(ctx context.Context, projectID, appID string) ([]PublicAPIKey, error) {
	path := fmt.Sprintf("/projects/%s/apps/%s/public_api_keys", url.PathEscape(projectID), url.PathEscape(appID))
	return listAll[PublicAPIKey](ctx, c, path, nil)
}

func webhooksPath(projectID string) string {
	return fmt.Sprintf("/projects/%s/integrations/webhooks", url.PathEscape(projectID))
}
