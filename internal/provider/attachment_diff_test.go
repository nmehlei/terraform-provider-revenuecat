package provider

import (
	"reflect"
	"testing"
)

func TestDiffProductIDs(t *testing.T) {
	tests := []struct {
		name       string
		current    []string
		desired    []string
		wantAttach []string
		wantDetach []string
	}{
		{
			name:       "add only",
			current:    []string{"a"},
			desired:    []string{"a", "b"},
			wantAttach: []string{"b"},
		},
		{
			name:       "remove only",
			current:    []string{"a", "b"},
			desired:    []string{"a"},
			wantDetach: []string{"b"},
		},
		{
			// The case that motivates sending a difference rather than
			// replacing the whole set: b must not be touched.
			name:       "add and remove together",
			current:    []string{"a", "b"},
			desired:    []string{"b", "c"},
			wantAttach: []string{"c"},
			wantDetach: []string{"a"},
		},
		{
			name:    "reordered but equal",
			current: []string{"a", "b", "c"},
			desired: []string{"c", "a", "b"},
		},
		{
			name:    "identical",
			current: []string{"a"},
			desired: []string{"a"},
		},
		{
			name:       "from empty",
			current:    nil,
			desired:    []string{"a", "b"},
			wantAttach: []string{"a", "b"},
		},
		{
			name:       "to empty",
			current:    []string{"a", "b"},
			desired:    nil,
			wantDetach: []string{"a", "b"},
		},
		{
			name:       "duplicates collapse",
			current:    []string{"a", "a"},
			desired:    []string{"a", "b", "b"},
			wantAttach: []string{"b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotAttach, gotDetach := diffProductIDs(tc.current, tc.desired)

			if !reflect.DeepEqual(gotAttach, tc.wantAttach) {
				t.Errorf("attach = %v, want %v", gotAttach, tc.wantAttach)
			}
			if !reflect.DeepEqual(gotDetach, tc.wantDetach) {
				t.Errorf("detach = %v, want %v", gotDetach, tc.wantDetach)
			}
		})
	}
}

func TestDiffProductIDsLeavesUnchangedProductsAlone(t *testing.T) {
	attach, detach := diffProductIDs([]string{"a", "b"}, []string{"b", "c"})

	for _, id := range append(append([]string{}, attach...), detach...) {
		if id == "b" {
			t.Fatalf("product b appears in attach %v or detach %v; an unchanged product must not be touched", attach, detach)
		}
	}
}

func TestDiffPackageProducts(t *testing.T) {
	tests := []struct {
		name       string
		current    []packageProductSpec
		desired    []packageProductSpec
		wantAttach []packageProductSpec
		wantDetach []string
	}{
		{
			name:       "new product attached",
			current:    []packageProductSpec{{ProductID: "a", EligibilityCriteria: "all"}},
			desired:    []packageProductSpec{{ProductID: "a", EligibilityCriteria: "all"}, {ProductID: "b"}},
			wantAttach: []packageProductSpec{{ProductID: "b"}},
		},
		{
			// Re-attach rather than detach-then-attach: attaching an already
			// attached product updates its criteria in place.
			name:       "criteria changed",
			current:    []packageProductSpec{{ProductID: "a", EligibilityCriteria: "all"}},
			desired:    []packageProductSpec{{ProductID: "a", EligibilityCriteria: "google_sdk_lt_6"}},
			wantAttach: []packageProductSpec{{ProductID: "a", EligibilityCriteria: "google_sdk_lt_6"}},
		},
		{
			name:    "unchanged",
			current: []packageProductSpec{{ProductID: "a", EligibilityCriteria: "all"}},
			desired: []packageProductSpec{{ProductID: "a", EligibilityCriteria: "all"}},
		},
		{
			name:       "product removed",
			current:    []packageProductSpec{{ProductID: "a"}, {ProductID: "b"}},
			desired:    []packageProductSpec{{ProductID: "a"}},
			wantDetach: []string{"b"},
		},
		{
			name:       "add remove and re-criteria at once",
			current:    []packageProductSpec{{ProductID: "a", EligibilityCriteria: "all"}, {ProductID: "b"}},
			desired:    []packageProductSpec{{ProductID: "a", EligibilityCriteria: "new"}, {ProductID: "c"}},
			wantAttach: []packageProductSpec{{ProductID: "a", EligibilityCriteria: "new"}, {ProductID: "c"}},
			wantDetach: []string{"b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotAttach, gotDetach := diffPackageProducts(tc.current, tc.desired)

			if !reflect.DeepEqual(gotAttach, tc.wantAttach) {
				t.Errorf("attach = %#v, want %#v", gotAttach, tc.wantAttach)
			}
			if !reflect.DeepEqual(gotDetach, tc.wantDetach) {
				t.Errorf("detach = %v, want %v", gotDetach, tc.wantDetach)
			}
		})
	}
}

func TestDiffPackageProductsDoesNotDetachOnCriteriaChange(t *testing.T) {
	_, detach := diffPackageProducts(
		[]packageProductSpec{{ProductID: "a", EligibilityCriteria: "all"}},
		[]packageProductSpec{{ProductID: "a", EligibilityCriteria: "changed"}},
	)
	if len(detach) != 0 {
		t.Errorf("detach = %v, want none; a criteria change must not detach the product", detach)
	}
}
