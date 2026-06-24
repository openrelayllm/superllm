package notifications

import (
	"context"
	"net/http"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type MemoryRepository struct {
	items    []*adminplusdomain.NotificationDelivery
	next     int64
	settings *adminplusdomain.NotificationSettings
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{next: 1}
}

func (r *MemoryRepository) CreateDelivery(_ context.Context, delivery *adminplusdomain.NotificationDelivery) (*adminplusdomain.NotificationDelivery, bool, error) {
	if r == nil {
		return nil, false, infraerrors.New(http.StatusInternalServerError, "NOTIFICATION_REPOSITORY_NOT_CONFIGURED", "notification repository is not configured")
	}
	for _, existing := range r.items {
		if existing.DedupeKey == delivery.DedupeKey {
			return nil, false, nil
		}
	}
	now := time.Now().UTC()
	item := cloneDelivery(delivery)
	item.ID = r.next
	r.next++
	item.CreatedAt = now
	item.UpdatedAt = now
	if item.Status == "" {
		item.Status = adminplusdomain.NotificationStatusSending
	}
	if item.Attempts <= 0 {
		item.Attempts = 1
	}
	r.items = append(r.items, item)
	return cloneDelivery(item), true, nil
}

func (r *MemoryRepository) GetDelivery(_ context.Context, id int64) (*adminplusdomain.NotificationDelivery, error) {
	if r == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "NOTIFICATION_REPOSITORY_NOT_CONFIGURED", "notification repository is not configured")
	}
	for _, item := range r.items {
		if item.ID == id {
			return cloneDelivery(item), nil
		}
	}
	return nil, infraerrors.New(http.StatusNotFound, "NOTIFICATION_DELIVERY_NOT_FOUND", "notification delivery not found")
}

func (r *MemoryRepository) ListDeliveries(_ context.Context, filter DeliveryFilter) ([]*adminplusdomain.NotificationDelivery, error) {
	if r == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "NOTIFICATION_REPOSITORY_NOT_CONFIGURED", "notification repository is not configured")
	}
	items := make([]*adminplusdomain.NotificationDelivery, 0)
	for i := len(r.items) - 1; i >= 0; i-- {
		item := r.items[i]
		if filter.SupplierID > 0 && item.SupplierID != filter.SupplierID {
			continue
		}
		if filter.Channel != "" && item.Channel != filter.Channel {
			continue
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		if strings.TrimSpace(filter.EventType) != "" && item.EventType != strings.TrimSpace(filter.EventType) {
			continue
		}
		items = append(items, cloneDelivery(item))
		limit := filter.Limit
		if limit <= 0 || limit > 200 {
			limit = 100
		}
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

func (r *MemoryRepository) MarkDeliverySucceeded(_ context.Context, id int64) error {
	return r.updateStatus(id, adminplusdomain.NotificationStatusSucceeded, "")
}

func (r *MemoryRepository) MarkDeliveryFailed(_ context.Context, id int64, message string) error {
	return r.updateStatus(id, adminplusdomain.NotificationStatusFailed, message)
}

func (r *MemoryRepository) IncrementDeliveryAttempt(_ context.Context, id int64) (*adminplusdomain.NotificationDelivery, error) {
	if r == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "NOTIFICATION_REPOSITORY_NOT_CONFIGURED", "notification repository is not configured")
	}
	for _, item := range r.items {
		if item.ID == id {
			item.Attempts++
			item.Status = adminplusdomain.NotificationStatusSending
			item.LastError = ""
			item.UpdatedAt = time.Now().UTC()
			return cloneDelivery(item), nil
		}
	}
	return nil, infraerrors.New(http.StatusNotFound, "NOTIFICATION_DELIVERY_NOT_FOUND", "notification delivery not found")
}

func (r *MemoryRepository) LoadSettings(_ context.Context) (*adminplusdomain.NotificationSettings, error) {
	if r == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "NOTIFICATION_REPOSITORY_NOT_CONFIGURED", "notification repository is not configured")
	}
	if r.settings == nil {
		return nil, nil
	}
	copied := cloneSettings(*r.settings)
	return &copied, nil
}

func (r *MemoryRepository) SaveSettings(_ context.Context, settings adminplusdomain.NotificationSettings) error {
	if r == nil {
		return infraerrors.New(http.StatusInternalServerError, "NOTIFICATION_REPOSITORY_NOT_CONFIGURED", "notification repository is not configured")
	}
	copied := cloneSettings(settings)
	r.settings = &copied
	return nil
}

func (r *MemoryRepository) updateStatus(id int64, status adminplusdomain.NotificationStatus, message string) error {
	for _, item := range r.items {
		if item.ID == id {
			now := time.Now().UTC()
			item.Status = status
			item.LastError = message
			item.UpdatedAt = now
			if status == adminplusdomain.NotificationStatusSucceeded {
				item.SentAt = &now
			}
			return nil
		}
	}
	return infraerrors.New(http.StatusNotFound, "NOTIFICATION_DELIVERY_NOT_FOUND", "notification delivery not found")
}

func cloneSettings(in adminplusdomain.NotificationSettings) adminplusdomain.NotificationSettings {
	out := in
	if in.Rules != nil {
		out.Rules = append([]adminplusdomain.NotificationRule{}, in.Rules...)
	}
	return out
}

func cloneDelivery(in *adminplusdomain.NotificationDelivery) *adminplusdomain.NotificationDelivery {
	if in == nil {
		return nil
	}
	out := *in
	if in.Payload != nil {
		out.Payload = make(map[string]any, len(in.Payload))
		for k, v := range in.Payload {
			out.Payload[k] = v
		}
	}
	return &out
}
