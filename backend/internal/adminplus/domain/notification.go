package domain

import "time"

type NotificationChannel string

const (
	NotificationChannelFeishu NotificationChannel = "feishu"
)

type NotificationStatus string

const (
	NotificationStatusSending   NotificationStatus = "sending"
	NotificationStatusSucceeded NotificationStatus = "succeeded"
	NotificationStatusFailed    NotificationStatus = "failed"
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
