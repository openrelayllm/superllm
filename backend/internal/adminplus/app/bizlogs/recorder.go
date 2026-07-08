package bizlogs

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
)

const (
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"

	CategoryLogin        = "login"
	CategoryBalance      = "balance"
	CategoryMail         = "mail"
	CategoryRegistration = "registration"
	CategoryExtension    = "extension"
	CategorySub2API      = "sub2api"

	OutcomeSucceeded = "succeeded"
	OutcomeFailed    = "failed"
	OutcomeBlocked   = "blocked"

	maxMessageLength     = 500
	maxExtraStringLength = 500
	componentPrefix      = "admin_plus."
)

type Writer interface {
	BatchInsertSystemLogs(ctx context.Context, inputs []*service.OpsInsertSystemLogInput) (int64, error)
}

type Recorder struct {
	writer Writer
	now    func() time.Time
}

type Event struct {
	Level        string
	Category     string
	Action       string
	Outcome      string
	Message      string
	SupplierID   int64
	SupplierName string
	ProviderType string

	Reason      string
	Endpoint    string
	StatusCode  int
	ContentType string
	BodyType    string
	BodyExcerpt string

	Metadata map[string]any
	At       time.Time
}

func NewRecorder(writer Writer) *Recorder {
	return &Recorder{
		writer: writer,
		now:    time.Now,
	}
}

func (r *Recorder) Record(ctx context.Context, event Event) {
	if r == nil || r.writer == nil {
		return
	}
	input := r.buildInput(event)
	if input == nil {
		return
	}
	if ctx == nil || ctx.Err() != nil {
		ctx = context.Background()
	}
	writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if _, err := r.writer.BatchInsertSystemLogs(writeCtx, []*service.OpsInsertSystemLogInput{input}); err != nil {
		slog.Warn("admin plus business log write failed", "component", input.Component, "err", err)
	}
}

func (r *Recorder) buildInput(event Event) *service.OpsInsertSystemLogInput {
	category := normalizeToken(event.Category)
	if category == "" {
		return nil
	}
	level := normalizeLevel(event.Level)
	action := normalizeToken(event.Action)
	outcome := normalizeToken(event.Outcome)
	if outcome == "" {
		outcome = OutcomeSucceeded
	}

	extra := sanitizeMap(event.Metadata)
	putString(extra, "category", category)
	putString(extra, "action", action)
	putString(extra, "outcome", outcome)
	putInt64(extra, "supplier_id", event.SupplierID)
	putString(extra, "supplier_name", event.SupplierName)
	putString(extra, "provider_type", event.ProviderType)
	putString(extra, "reason", event.Reason)
	putString(extra, "endpoint", event.Endpoint)
	putInt(extra, "status_code", event.StatusCode)
	putString(extra, "content_type", event.ContentType)
	putString(extra, "body_type", event.BodyType)
	putString(extra, "body_excerpt", event.BodyExcerpt)

	createdAt := event.At.UTC()
	if createdAt.IsZero() {
		createdAt = r.now().UTC()
	}
	message := trimLimit(event.Message, maxMessageLength)
	if message == "" {
		message = defaultMessage(category, action, outcome)
	}
	extraJSONBytes, _ := json.Marshal(logredact.RedactMap(extra, "token", "cookie", "cookies", "verification_code", "mail_body", "email_body"))
	return &service.OpsInsertSystemLogInput{
		CreatedAt: createdAt,
		Level:     level,
		Component: componentPrefix + category,
		Message:   logredact.RedactText(message, "token", "cookie", "verification_code"),
		ExtraJSON: string(extraJSONBytes),
	}
}

func FromError(err error) map[string]any {
	out := map[string]any{}
	if err == nil {
		return out
	}
	appErr := infraerrors.FromError(err)
	if appErr != nil {
		putString(out, "reason", appErr.Reason)
		putInt(out, "status_code", int(appErr.Code))
		putString(out, "error_message", appErr.Message)
		for key, value := range appErr.Metadata {
			putString(out, key, value)
		}
		return out
	}
	putString(out, "error_message", err.Error())
	return out
}

func EventFromError(event Event, err error) Event {
	if err == nil {
		return event
	}
	appErr := infraerrors.FromError(err)
	event.Outcome = OutcomeFailed
	if event.Level == "" {
		event.Level = LevelWarn
	}
	if event.Reason == "" {
		event.Reason = appErr.Reason
	}
	if event.Message == "" {
		event.Message = appErr.Message
	}
	if event.Metadata == nil {
		event.Metadata = map[string]any{}
	}
	event.Metadata["error_message"] = appErr.Message
	for key, value := range appErr.Metadata {
		event.Metadata[key] = value
	}
	if event.Endpoint == "" {
		event.Endpoint = appErr.Metadata["endpoint"]
	}
	if event.StatusCode == 0 {
		if n, err := strconv.Atoi(appErr.Metadata["status_code"]); err == nil {
			event.StatusCode = n
		} else if appErr.Code > 0 {
			event.StatusCode = int(appErr.Code)
		}
	}
	if event.ContentType == "" {
		event.ContentType = appErr.Metadata["content_type"]
	}
	if event.BodyType == "" {
		event.BodyType = appErr.Metadata["body_type"]
	}
	if event.BodyExcerpt == "" {
		event.BodyExcerpt = appErr.Metadata["body_excerpt"]
	}
	return event
}

func sanitizeMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		normalizedKey := normalizeToken(key)
		if normalizedKey == "" {
			continue
		}
		if isSensitiveKey(normalizedKey) {
			continue
		}
		out[normalizedKey] = sanitizeValue(value)
	}
	return out
}

func sanitizeValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		return trimLimit(logredact.RedactText(v, "token", "cookie", "verification_code"), maxExtraStringLength)
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			item = trimLimit(logredact.RedactText(item, "token", "cookie", "verification_code"), maxExtraStringLength)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case map[string]any:
		return sanitizeMap(v)
	default:
		return v
	}
}

func isSensitiveKey(key string) bool {
	switch key {
	case "password", "access_token", "refresh_token", "id_token", "client_secret", "authorization", "cookie", "cookies", "code", "verification_code", "mail_body", "email_body", "body", "text", "html":
		return true
	default:
		return strings.Contains(key, "password") || strings.Contains(key, "token") || strings.Contains(key, "secret") || strings.Contains(key, "cookie")
	}
}

func putString(out map[string]any, key string, value string) {
	value = trimLimit(logredact.RedactText(strings.TrimSpace(value), "token", "cookie", "verification_code"), maxExtraStringLength)
	if value != "" {
		out[key] = value
	}
}

func putInt(out map[string]any, key string, value int) {
	if value > 0 {
		out[key] = value
	}
}

func putInt64(out map[string]any, key string, value int64) {
	if value > 0 {
		out[key] = value
	}
}

func normalizeLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case LevelInfo:
		return LevelInfo
	case "warning", LevelWarn:
		return LevelWarn
	case LevelError:
		return LevelError
	default:
		return LevelInfo
	}
}

func normalizeToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func defaultMessage(category string, action string, outcome string) string {
	parts := []string{"admin plus", category}
	if action != "" {
		parts = append(parts, action)
	}
	if outcome != "" {
		parts = append(parts, outcome)
	}
	return strings.Join(parts, " ")
}

func trimLimit(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}
