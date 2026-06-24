package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/lib/pq"
)

type Repository interface {
	CreateDelivery(ctx context.Context, delivery *adminplusdomain.NotificationDelivery) (*adminplusdomain.NotificationDelivery, bool, error)
	GetDelivery(ctx context.Context, id int64) (*adminplusdomain.NotificationDelivery, error)
	ListDeliveries(ctx context.Context, filter DeliveryFilter) ([]*adminplusdomain.NotificationDelivery, error)
	MarkDeliverySucceeded(ctx context.Context, id int64) error
	MarkDeliveryFailed(ctx context.Context, id int64, message string) error
	IncrementDeliveryAttempt(ctx context.Context, id int64) (*adminplusdomain.NotificationDelivery, error)
	LoadSettings(ctx context.Context) (*adminplusdomain.NotificationSettings, error)
	SaveSettings(ctx context.Context, settings adminplusdomain.NotificationSettings) error
}

type DeliveryFilter struct {
	SupplierID int64
	Channel    adminplusdomain.NotificationChannel
	Status     adminplusdomain.NotificationStatus
	EventType  string
	Limit      int
}

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) CreateDelivery(ctx context.Context, delivery *adminplusdomain.NotificationDelivery) (*adminplusdomain.NotificationDelivery, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, infraerrors.New(http.StatusInternalServerError, "NOTIFICATION_REPOSITORY_NOT_CONFIGURED", "notification repository is not configured")
	}
	payload, err := marshalPayload(delivery.Payload)
	if err != nil {
		return nil, false, err
	}
	status := delivery.Status
	if status == "" {
		status = adminplusdomain.NotificationStatusSending
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO admin_plus_notification_deliveries (
			channel,
			event_type,
			event_id,
			supplier_id,
			dedupe_key,
			status,
			attempts,
			last_error,
			payload
		)
		VALUES ($1, $2, $3, $4, $5, $6, 1, $7, $8)
		RETURNING id, channel, event_type, event_id, supplier_id, dedupe_key, status, attempts, last_error, payload, sent_at, created_at, updated_at
	`,
		delivery.Channel,
		delivery.EventType,
		delivery.EventID,
		delivery.SupplierID,
		delivery.DedupeKey,
		status,
		truncateError(delivery.LastError),
		payload,
	)
	created, err := scanDelivery(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return created, true, nil
}

func (r *SQLRepository) GetDelivery(ctx context.Context, id int64) (*adminplusdomain.NotificationDelivery, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "NOTIFICATION_REPOSITORY_NOT_CONFIGURED", "notification repository is not configured")
	}
	if id <= 0 {
		return nil, infraerrors.New(http.StatusBadRequest, "NOTIFICATION_DELIVERY_ID_INVALID", "invalid notification delivery id")
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, channel, event_type, event_id, supplier_id, dedupe_key, status, attempts, last_error, payload, sent_at, created_at, updated_at
		FROM admin_plus_notification_deliveries
		WHERE id = $1
	`, id)
	return scanDelivery(row)
}

func (r *SQLRepository) ListDeliveries(ctx context.Context, filter DeliveryFilter) ([]*adminplusdomain.NotificationDelivery, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "NOTIFICATION_REPOSITORY_NOT_CONFIGURED", "notification repository is not configured")
	}
	query := `
		SELECT id, channel, event_type, event_id, supplier_id, dedupe_key, status, attempts, last_error, payload, sent_at, created_at, updated_at
		FROM admin_plus_notification_deliveries
		WHERE 1=1`
	args := make([]any, 0, 5)
	if filter.SupplierID > 0 {
		args = append(args, filter.SupplierID)
		query += " AND supplier_id = $" + itoa(len(args))
	}
	if filter.Channel != "" {
		args = append(args, filter.Channel)
		query += " AND channel = $" + itoa(len(args))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		query += " AND status = $" + itoa(len(args))
	}
	eventType := strings.TrimSpace(filter.EventType)
	if eventType != "" {
		args = append(args, eventType)
		query += " AND event_type = $" + itoa(len(args))
	}
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	args = append(args, limit)
	query += " ORDER BY created_at DESC, id DESC LIMIT $" + itoa(len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]*adminplusdomain.NotificationDelivery, 0)
	for rows.Next() {
		item, err := scanDelivery(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *SQLRepository) MarkDeliverySucceeded(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return infraerrors.New(http.StatusInternalServerError, "NOTIFICATION_REPOSITORY_NOT_CONFIGURED", "notification repository is not configured")
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE admin_plus_notification_deliveries
		SET status = $2,
			last_error = '',
			sent_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`, id, adminplusdomain.NotificationStatusSucceeded)
	if err != nil {
		return err
	}
	return requireAffected(result, "NOTIFICATION_DELIVERY_NOT_FOUND", "notification delivery not found")
}

func (r *SQLRepository) MarkDeliveryFailed(ctx context.Context, id int64, message string) error {
	if r == nil || r.db == nil {
		return infraerrors.New(http.StatusInternalServerError, "NOTIFICATION_REPOSITORY_NOT_CONFIGURED", "notification repository is not configured")
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE admin_plus_notification_deliveries
		SET status = $2,
			last_error = $3,
			updated_at = NOW()
		WHERE id = $1
	`, id, adminplusdomain.NotificationStatusFailed, truncateError(message))
	if err != nil {
		return err
	}
	return requireAffected(result, "NOTIFICATION_DELIVERY_NOT_FOUND", "notification delivery not found")
}

func (r *SQLRepository) IncrementDeliveryAttempt(ctx context.Context, id int64) (*adminplusdomain.NotificationDelivery, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "NOTIFICATION_REPOSITORY_NOT_CONFIGURED", "notification repository is not configured")
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE admin_plus_notification_deliveries
		SET status = $2,
			attempts = attempts + 1,
			last_error = '',
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, channel, event_type, event_id, supplier_id, dedupe_key, status, attempts, last_error, payload, sent_at, created_at, updated_at
	`, id, adminplusdomain.NotificationStatusSending)
	return scanDelivery(row)
}

func (r *SQLRepository) LoadSettings(ctx context.Context) (*adminplusdomain.NotificationSettings, error) {
	if r == nil || r.db == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "NOTIFICATION_REPOSITORY_NOT_CONFIGURED", "notification repository is not configured")
	}
	var raw []byte
	err := r.db.QueryRowContext(ctx, `SELECT value FROM admin_plus_notification_settings WHERE key = 'global'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var settings adminplusdomain.NotificationSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *SQLRepository) SaveSettings(ctx context.Context, settings adminplusdomain.NotificationSettings) error {
	if r == nil || r.db == nil {
		return infraerrors.New(http.StatusInternalServerError, "NOTIFICATION_REPOSITORY_NOT_CONFIGURED", "notification repository is not configured")
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		return infraerrors.New(http.StatusBadRequest, "NOTIFICATION_SETTINGS_INVALID", "notification settings must be valid JSON")
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO admin_plus_notification_settings (key, value, updated_at)
		VALUES ('global', $1, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`, raw)
	return err
}

func marshalPayload(payload map[string]any) ([]byte, error) {
	if payload == nil {
		return []byte(`{}`), nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadRequest, "NOTIFICATION_PAYLOAD_INVALID", "notification payload must be valid JSON")
	}
	return raw, nil
}

type deliveryScanner interface {
	Scan(dest ...any) error
}

func scanDelivery(row deliveryScanner) (*adminplusdomain.NotificationDelivery, error) {
	delivery := &adminplusdomain.NotificationDelivery{}
	var payload []byte
	err := row.Scan(
		&delivery.ID,
		&delivery.Channel,
		&delivery.EventType,
		&delivery.EventID,
		&delivery.SupplierID,
		&delivery.DedupeKey,
		&delivery.Status,
		&delivery.Attempts,
		&delivery.LastError,
		&payload,
		&delivery.SentAt,
		&delivery.CreatedAt,
		&delivery.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(payload) > 0 {
		var decoded map[string]any
		if err := json.Unmarshal(payload, &decoded); err != nil {
			return nil, err
		}
		delivery.Payload = decoded
	}
	return delivery, nil
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}

func requireAffected(result sql.Result, reason string, message string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return infraerrors.New(http.StatusNotFound, reason, message)
	}
	return nil
}

func truncateError(message string) string {
	v := strings.TrimSpace(message)
	if len(v) > 1000 {
		return v[:1000]
	}
	return v
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
