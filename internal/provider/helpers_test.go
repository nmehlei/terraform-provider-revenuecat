package provider

import (
	"strings"
	"testing"
)

func TestParseImportID(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		names     []string
		want      []string
		wantErr   bool
		errSubstr string
	}{
		{
			name:  "two segments",
			id:    "proj1:entl123",
			names: []string{"project_id", "entitlement_id"},
			want:  []string{"proj1", "entl123"},
		},
		{
			name:  "three segments",
			id:    "proj1:ofrng123:pkg456",
			names: []string{"project_id", "offering_id", "package_id"},
			want:  []string{"proj1", "ofrng123", "pkg456"},
		},
		{
			name:      "too few segments",
			id:        "proj1",
			names:     []string{"project_id", "entitlement_id"},
			wantErr:   true,
			errSubstr: "project_id:entitlement_id",
		},
		{
			name:      "too many segments",
			id:        "proj1:entl123:extra",
			names:     []string{"project_id", "entitlement_id"},
			wantErr:   true,
			errSubstr: "project_id:entitlement_id",
		},
		{
			name:      "empty identifier",
			id:        "",
			names:     []string{"project_id", "entitlement_id"},
			wantErr:   true,
			errSubstr: "project_id:entitlement_id",
		},
		{
			name:      "empty leading segment",
			id:        ":entl123",
			names:     []string{"project_id", "entitlement_id"},
			wantErr:   true,
			errSubstr: "project_id segment is empty",
		},
		{
			name:      "empty trailing segment",
			id:        "proj1:",
			names:     []string{"project_id", "entitlement_id"},
			wantErr:   true,
			errSubstr: "entitlement_id segment is empty",
		},
		{
			name:      "whitespace-only segment",
			id:        "proj1:   ",
			names:     []string{"project_id", "entitlement_id"},
			wantErr:   true,
			errSubstr: "entitlement_id segment is empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseImportID(tc.id, tc.names...)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseImportID(%q) succeeded; want an error", tc.id)
				}
				if !strings.Contains(err.Error(), tc.errSubstr) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), tc.errSubstr)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseImportID(%q): %v", tc.id, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d segments, want %d", len(got), len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("segment %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestRequireExactlyOneSelector(t *testing.T) {
	if err := requireExactlyOneSelector(true, false, "id", "lookup_key"); err != nil {
		t.Errorf("id only: unexpected error %v", err)
	}
	if err := requireExactlyOneSelector(false, true, "id", "lookup_key"); err != nil {
		t.Errorf("key only: unexpected error %v", err)
	}

	err := requireExactlyOneSelector(true, true, "id", "lookup_key")
	if err == nil {
		t.Fatal("both selectors: expected an error")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("both selectors: error = %q, want it to say the selectors are mutually exclusive", err)
	}

	err = requireExactlyOneSelector(false, false, "id", "lookup_key")
	if err == nil {
		t.Fatal("neither selector: expected an error")
	}
	if !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("neither selector: error = %q, want it to require exactly one", err)
	}
}

func TestFindByKeyNamesTheMissingKey(t *testing.T) {
	type item struct{ key string }
	items := []item{{key: "a"}, {key: "b"}}
	keyOf := func(i item) string { return i.key }

	got, err := findByKey(items, "b", keyOf, "entitlement", "lookup_key")
	if err != nil {
		t.Fatalf("findByKey: %v", err)
	}
	if got.key != "b" {
		t.Errorf("got key %q, want b", got.key)
	}

	_, err = findByKey(items, "missing", keyOf, "entitlement", "lookup_key")
	if err == nil {
		t.Fatal("findByKey for an absent key succeeded; want an error")
	}
	if !strings.Contains(err.Error(), `"missing"`) {
		t.Errorf("error = %q, want it to name the key that was searched for", err)
	}
	if !strings.Contains(err.Error(), "lookup_key") {
		t.Errorf("error = %q, want it to name the key attribute", err)
	}
}

func TestJoinBackticked(t *testing.T) {
	if got, want := joinBackticked([]string{"a", "b", "c"}), "a`, `b`, `c"; got != want {
		t.Errorf("joinBackticked = %q, want %q", got, want)
	}
}
