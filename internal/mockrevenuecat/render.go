package mockrevenuecat

// This file maps between request bodies, stored fields, and API
// representations. It mirrors the JSON contract encoded in
// internal/revenuecat; a correction there needs a matching one here, and the
// end-to-end tests fail loudly if the two drift apart.

// copyFields lifts the named keys from a request body into stored fields,
// skipping keys the caller did not supply so an update leaves them untouched.
func copyFields(body map[string]any, names ...string) map[string]any {
	fields := map[string]any{}
	for _, name := range names {
		if value, ok := body[name]; ok {
			fields[name] = value
		}
	}
	return fields
}

// base renders the attributes every object carries.
func base(obj *object, objectType string) map[string]any {
	rendered := map[string]any{
		"object":     objectType,
		"id":         obj.ID,
		"created_at": obj.CreatedAt,
	}
	if obj.ProjectID != "" {
		rendered["project_id"] = obj.ProjectID
	}
	return rendered
}

// withFields merges stored fields into a rendered object, defaulting any name
// the object does not carry so responses have a stable shape.
func withFields(rendered map[string]any, obj *object, defaults map[string]any) map[string]any {
	for name, fallback := range defaults {
		if value, ok := obj.Fields[name]; ok {
			rendered[name] = value
			continue
		}
		if fallback != nil {
			rendered[name] = fallback
		}
	}
	return rendered
}

func renderProject(obj *object) map[string]any {
	return withFields(base(obj, "project"), obj, map[string]any{"name": ""})
}

func renderGeneric(obj *object) map[string]any {
	rendered := base(obj, "object")
	for name, value := range obj.Fields {
		rendered[name] = value
	}
	return rendered
}

func appFields(body map[string]any) map[string]any {
	return copyFields(body, "name", "type", "play_store")
}

// appUpdateFields is narrower than appFields: an app's type is part of its
// identity, so an update must not be able to change it even if one is sent.
func appUpdateFields(body map[string]any) map[string]any {
	return copyFields(body, "name")
}

func renderApp(obj *object) map[string]any {
	rendered := withFields(base(obj, "app"), obj, map[string]any{
		"name": "",
		"type": "",
	})
	if playStore, ok := obj.Fields["play_store"]; ok {
		rendered["play_store"] = playStore
	}
	return rendered
}

// validateAppCreate mirrors the real API's discriminated union: "type" alone
// is not enough, the type-specific object must be present too (this provider
// only implements "play_store" so far — see revenuecat.ImplementedAppTypes).
func validateAppCreate(body map[string]any) *validationError {
	if appType, _ := body["type"].(string); appType == "play_store" {
		if _, ok := body["play_store"]; !ok {
			return &validationError{code: "parameter_error", message: "'play_store' is a required property"}
		}
	}
	return nil
}

func productFields(body map[string]any) map[string]any {
	return copyFields(body, "store_identifier", "app_id", "type", "display_name")
}

func renderProduct(obj *object) map[string]any {
	return withFields(base(obj, "product"), obj, map[string]any{
		"store_identifier": "",
		"app_id":           "",
		"type":             "",
		"display_name":     "",
	})
}

func entitlementFields(body map[string]any) map[string]any {
	return copyFields(body, "lookup_key", "display_name")
}

func renderEntitlement(obj *object) map[string]any {
	return withFields(base(obj, "entitlement"), obj, map[string]any{
		"lookup_key":   "",
		"display_name": "",
	})
}

// validateOfferingCreate mirrors the real API rejecting "is_current" on
// create ("Additional properties are not allowed") — an offering can only be
// marked current through a later update, never at creation.
func validateOfferingCreate(body map[string]any) *validationError {
	if _, ok := body["is_current"]; ok {
		return &validationError{code: "parameter_error", message: "Additional properties are not allowed ('is_current' was unexpected)"}
	}
	return nil
}

func offeringFields(body map[string]any) map[string]any {
	fields := copyFields(body, "lookup_key", "display_name", "is_current")

	// Metadata arrives as a JSON object; store it as a string map so it
	// round-trips as the provider's map[string]string.
	if raw, ok := body["metadata"]; ok {
		if nested, ok := raw.(map[string]any); ok {
			metadata := make(map[string]string, len(nested))
			for key, value := range nested {
				if text, ok := value.(string); ok {
					metadata[key] = text
				}
			}
			fields["metadata"] = metadata
		}
	}
	return fields
}

func renderOffering(obj *object) map[string]any {
	rendered := withFields(base(obj, "offering"), obj, map[string]any{
		"lookup_key":   "",
		"display_name": "",
		"is_current":   false,
	})

	if metadata, ok := obj.Fields["metadata"].(map[string]string); ok {
		rendered["metadata"] = metadata
	}
	return rendered
}

func packageFields(body map[string]any) map[string]any {
	return copyFields(body, "lookup_key", "display_name", "position")
}

func renderPackage(obj *object) map[string]any {
	rendered := withFields(base(obj, "package"), obj, map[string]any{
		"lookup_key":   "",
		"display_name": "",
		"offering_id":  "",
	})

	// Position is genuinely optional: rendering a default would make an unset
	// position read back as 0 and produce a permanent diff.
	if position, ok := obj.Fields["position"]; ok {
		rendered["position"] = position
	}
	return rendered
}

// attachmentsFromBody reads either shape of attach payload: the entitlement
// form carrying product_ids, and the package form carrying products with
// per-product eligibility criteria.
func attachmentsFromBody(body map[string]any) []attachment {
	var out []attachment

	if raw, ok := body["product_ids"].([]any); ok {
		for _, value := range raw {
			if id, ok := value.(string); ok {
				out = append(out, attachment{ProductID: id})
			}
		}
	}

	if raw, ok := body["products"].([]any); ok {
		for _, value := range raw {
			entry, ok := value.(map[string]any)
			if !ok {
				continue
			}
			id, _ := entry["product_id"].(string)
			if id == "" {
				continue
			}
			criteria, _ := entry["eligibility_criteria"].(string)
			out = append(out, attachment{ProductID: id, EligibilityCriteria: criteria})
		}
	}

	return out
}

func productIDsFromBody(body map[string]any) []string {
	var out []string
	if raw, ok := body["product_ids"].([]any); ok {
		for _, value := range raw {
			if id, ok := value.(string); ok {
				out = append(out, id)
			}
		}
	}
	return out
}
