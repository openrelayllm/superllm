package service

import "testing"

func TestNormalizeProductSiteNameMigratesLegacyDefaults(t *testing.T) {
	for _, input := range []string{"", "Sub2API", "Sub2API Admin", "Sub2API Admin Plus", "sub2api-admin-plus"} {
		if got := normalizeProductSiteName(input); got != defaultProductSiteName {
			t.Fatalf("normalizeProductSiteName(%q) = %q, want %q", input, got, defaultProductSiteName)
		}
	}
	if got := normalizeProductSiteName("Custom Console"); got != "Custom Console" {
		t.Fatalf("custom site name changed to %q", got)
	}
}
