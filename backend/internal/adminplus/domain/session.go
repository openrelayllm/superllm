package domain

import "time"

type SupplierBrowserSession struct {
	SupplierID              int64          `json:"supplier_id"`
	Origin                  string         `json:"origin"`
	APIBaseURL              string         `json:"api_base_url,omitempty"`
	SessionSummary          map[string]any `json:"session_summary,omitempty"`
	SessionBundleCiphertext string         `json:"session_bundle_ciphertext,omitempty"`
	CapturedAt              time.Time      `json:"captured_at"`
	ExpiresAt               *time.Time     `json:"expires_at,omitempty"`
	SourceExtensionTaskID   int64          `json:"source_extension_task_id,omitempty"`
	CreatedAt               time.Time      `json:"created_at"`
	UpdatedAt               time.Time      `json:"updated_at"`
}
