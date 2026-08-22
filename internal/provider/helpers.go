package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/revenuecat"
)

// configureResourceClient pulls the configured client out of the provider data
// passed to a resource. Provider data is nil during validation, which is not an
// error.
func configureResourceClient(req resource.ConfigureRequest, resp *resource.ConfigureResponse) *revenuecat.Client {
	if req.ProviderData == nil {
		return nil
	}

	client, ok := req.ProviderData.(*revenuecat.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *revenuecat.Client, got %T. This is a bug in the provider.", req.ProviderData),
		)
		return nil
	}
	return client
}

// configureDataSourceClient is configureResourceClient for data sources.
func configureDataSourceClient(req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) *revenuecat.Client {
	if req.ProviderData == nil {
		return nil
	}

	client, ok := req.ProviderData.(*revenuecat.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *revenuecat.Client, got %T. This is a bug in the provider.", req.ProviderData),
		)
		return nil
	}
	return client
}

// parseImportID splits a colon-delimited import identifier into exactly the
// expected number of segments, rejecting empty segments. The names describe
// each segment so a malformed identifier produces an error stating the format.
func parseImportID(id string, names ...string) ([]string, error) {
	parts := strings.Split(id, ":")
	format := strings.Join(names, ":")

	if len(parts) != len(names) {
		return nil, fmt.Errorf(
			"expected an import identifier of the form %q, got %q (%d segments, want %d)",
			format, id, len(parts), len(names),
		)
	}

	for i, part := range parts {
		if strings.TrimSpace(part) == "" {
			return nil, fmt.Errorf(
				"expected an import identifier of the form %q, got %q (the %s segment is empty)",
				format, id, names[i],
			)
		}
	}
	return parts, nil
}

// importIDError renders a parse failure as an import diagnostic.
func importIDError(diags *diag.Diagnostics, err error) {
	diags.AddError("Invalid import identifier", err.Error())
}

// optionalString converts a framework string into a pointer suitable for an
// omitempty request field: a null or unknown value becomes nil, so an unset
// attribute is left out of the payload rather than sent as an empty string.
func optionalString(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	v := value.ValueString()
	return &v
}

// optionalBool is optionalString for booleans.
func optionalBool(value types.Bool) *bool {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	v := value.ValueBool()
	return &v
}

// optionalInt64 is optionalString for integers.
func optionalInt64(value types.Int64) *int64 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	v := value.ValueInt64()
	return &v
}

// stringOrNull maps an API string onto a framework value, preserving null for
// an absent value so an unset optional attribute does not read back as "".
func stringOrNull(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

// int64OrNull maps an optional API integer onto a framework value.
func int64OrNull(value *int64) types.Int64 {
	if value == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*value)
}

// metadataToFramework converts an API metadata map into a framework map,
// preserving null for an absent map.
func metadataToFramework(ctx context.Context, metadata map[string]string, diags *diag.Diagnostics) types.Map {
	if metadata == nil {
		return types.MapNull(types.StringType)
	}
	value, mapDiags := types.MapValueFrom(ctx, types.StringType, metadata)
	diags.Append(mapDiags...)
	return value
}

// metadataFromFramework converts a framework map into an API metadata map,
// returning nil for a null or unknown map.
func metadataFromFramework(ctx context.Context, value types.Map, diags *diag.Diagnostics) map[string]string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	metadata := map[string]string{}
	diags.Append(value.ElementsAs(ctx, &metadata, false)...)
	return metadata
}

// stringSetFromFramework converts a framework set of strings into a Go slice.
func stringSetFromFramework(ctx context.Context, value types.Set, diags *diag.Diagnostics) []string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	var out []string
	diags.Append(value.ElementsAs(ctx, &out, false)...)
	return out
}
