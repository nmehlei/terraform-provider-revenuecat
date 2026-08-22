package provider

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// int64UseStateForUnknown keeps a computed integer attribute stable across
// plans instead of showing it as "(known after apply)" on every change.
func int64UseStateForUnknown() planmodifier.Int64 {
	return int64planmodifier.UseStateForUnknown()
}

// joinBackticked renders a list of allowed values for a Markdown description.
func joinBackticked(values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = v
	}
	return strings.Join(quoted, "`, `")
}
