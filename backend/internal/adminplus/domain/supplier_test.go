package domain

import "testing"

func TestNormalizeSupplierTypeAliases(t *testing.T) {
	tests := map[string]SupplierType{
		"new api":              SupplierTypeNewAPI,
		"one-api":              SupplierTypeNewAPI,
		"donehub":              SupplierTypeNewAPI,
		"vo-api":               SupplierTypeNewAPI,
		"super-api":            SupplierTypeNewAPI,
		"rix-api":              SupplierTypeNewAPI,
		"neo-api":              SupplierTypeNewAPI,
		"wong-gongyi":          SupplierTypeNewAPI,
		"claude":               SupplierTypeAnthropic,
		"claude_compatible":    SupplierTypeAnthropic,
		"google":               SupplierTypeGemini,
		"google_ai_studio":     SupplierTypeGemini,
		"sub2-api":             SupplierTypeSub2API,
		"sub-api":              SupplierTypeSub2API,
		"browser-only":         SupplierTypeBrowserOnly,
		"unknown-provider":     SupplierType("unknown-provider"),
		" https://example.com": SupplierType("https://example.com"),
	}

	for input, want := range tests {
		if got := NormalizeSupplierType(input); got != want {
			t.Fatalf("NormalizeSupplierType(%q) = %q, want %q", input, got, want)
		}
	}
}
