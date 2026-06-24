package domain

import "time"

type NotificationChannel string

const (
	NotificationChannelFeishu NotificationChannel = "feishu"
)

type NotificationStatus string

const (
	NotificationStatusSending    NotificationStatus = "sending"
	NotificationStatusSucceeded  NotificationStatus = "succeeded"
	NotificationStatusFailed     NotificationStatus = "failed"
	NotificationStatusSuppressed NotificationStatus = "suppressed"
)

type NotificationDelivery struct {
	ID         int64               `json:"id"`
	Channel    NotificationChannel `json:"channel"`
	EventType  string              `json:"event_type"`
	EventID    int64               `json:"event_id"`
	SupplierID int64               `json:"supplier_id"`
	DedupeKey  string              `json:"dedupe_key"`
	Status     NotificationStatus  `json:"status"`
	Attempts   int                 `json:"attempts"`
	LastError  string              `json:"last_error,omitempty"`
	Payload    map[string]any      `json:"payload,omitempty"`
	SentAt     *time.Time          `json:"sent_at,omitempty"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

type NotificationRule struct {
	EventType          string `json:"event_type"`
	Label              string `json:"label"`
	Description        string `json:"description"`
	Enabled            bool   `json:"enabled"`
	Severity           string `json:"severity"`
	QuietWindowMinutes int    `json:"quiet_window_minutes"`
	DedupeScope        string `json:"dedupe_scope"`
	NotifyRecovery     bool   `json:"notify_recovery"`
	Threshold          string `json:"threshold,omitempty"`
}

type NotificationChannelSettings struct {
	Enabled           bool       `json:"enabled"`
	WebhookURL        string     `json:"webhook_url,omitempty"`
	WebhookSecret     string     `json:"webhook_secret,omitempty"`
	WebhookHost       string     `json:"webhook_host,omitempty"`
	WebhookConfigured bool       `json:"webhook_configured"`
	SecretConfigured  bool       `json:"secret_configured"`
	ConfigSource      string     `json:"config_source"`
	LastTestAt        *time.Time `json:"last_test_at,omitempty"`
	LastTestStatus    string     `json:"last_test_status,omitempty"`
	LastTestError     string     `json:"last_test_error,omitempty"`
}

type NotificationSettings struct {
	Feishu NotificationChannelSettings `json:"feishu"`
	Rules  []NotificationRule          `json:"rules"`
}

type NotificationCenterStatus struct {
	FeishuConfigured bool       `json:"feishu_configured"`
	FeishuEnabled    bool       `json:"feishu_enabled"`
	OpenRules        int        `json:"open_rules"`
	TotalRules       int        `json:"total_rules"`
	TotalDeliveries  int        `json:"total_deliveries"`
	Succeeded        int        `json:"succeeded"`
	Failed           int        `json:"failed"`
	Sending          int        `json:"sending"`
	Suppressed       int        `json:"suppressed"`
	LastDeliveryAt   *time.Time `json:"last_delivery_at,omitempty"`
}
