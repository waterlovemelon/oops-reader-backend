package reading

import "testing"

func TestCatalogShelfBookKeyUsesTheProgressKeyForm(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"bare catalog id", "remote_walden", "catalog:remote_walden"},
		{"already canonical", "catalog:remote_walden", "catalog:remote_walden"},
		{"surrounding spaces", "  catalog:remote_walden\t", "catalog:remote_walden"},
		{"empty", "", ""},
		{"spaces only", "   ", ""},
		{"prefix only", "catalog:", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CatalogShelfBookKey(test.input); got != test.want {
				t.Fatalf("CatalogShelfBookKey(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

// The shelf key must round trip through CatalogBookKey, otherwise the shelf and
// reading progress address the same online book by different identifiers.
func TestCatalogShelfBookKeyRoundTripsThroughCatalogBookKey(t *testing.T) {
	for _, id := range []string{"remote_walden", "book-with-dashes", "book_1"} {
		shelfKey := CatalogShelfBookKey(id)
		if got := CatalogBookKey(shelfKey); got != id {
			t.Fatalf("CatalogBookKey(CatalogShelfBookKey(%q)) = %q, want %q", id, got, id)
		}
	}
}
