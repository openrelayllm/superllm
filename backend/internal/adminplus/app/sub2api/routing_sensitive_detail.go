package sub2api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/bizlogs"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	routingSensitiveUnavailableNotRecorded = "not_recorded"
	maxRoutingSensitiveReasonLength        = 500
)

var defaultRoutingSensitiveFailureFields = []string{
	"error_message",
	"error_body",
	"upstream_error_message",
	"upstream_error_detail",
	"provider_error_code",
	"provider_error_type",
	"network_error_type",
	"error_source",
	"inbound_endpoint",
	"upstream_endpoint",
	"requested_model",
	"upstream_model",
	"retry_after_seconds",
	"request_body",
	"request_headers",
}

var allowedRoutingSensitiveFailureFields = map[string]string{
	"error_message":          "error_message",
	"error_body":             "error_body",
	"upstream_error_message": "upstream_error_message",
	"upstream_error_detail":  "upstream_error_detail",
	"provider_error_code":    "provider_error_code",
	"provider_error_type":    "provider_error_type",
	"network_error_type":     "network_error_type",
	"error_source":           "error_source",
	"inbound_endpoint":       "inbound_endpoint",
	"upstream_endpoint":      "upstream_endpoint",
	"requested_model":        "requested_model",
	"upstream_model":         "upstream_model",
	"retry_after_seconds":    "retry_after_seconds",
	"request_body":           "request_body",
	"request_headers":        "request_headers",
	"headers":                "request_headers",
}

type RoutingSensitiveFailureDetailInput struct {
	FailureID    int64    `json:"failure_id"`
	LocalGroupID int64    `json:"local_group_id"`
	Reason       string   `json:"reason"`
	Fields       []string `json:"fields,omitempty"`
	RequestedBy  int64    `json:"requested_by,omitempty"`
}

type RoutingSensitiveFailureField struct {
	Name              string `json:"name"`
	Available         bool   `json:"available"`
	Value             string `json:"value,omitempty"`
	UnavailableReason string `json:"unavailable_reason,omitempty"`
	Redacted          bool   `json:"redacted,omitempty"`
	Truncated         bool   `json:"truncated,omitempty"`
}

type RoutingSensitiveFailureDetail struct {
	ID                 int64                          `json:"id"`
	LocalGroupID       int64                          `json:"local_group_id"`
	RequestID          string                         `json:"request_id,omitempty"`
	APIKeyID           int64                          `json:"api_key_id,omitempty"`
	APIKeyName         string                         `json:"api_key_name,omitempty"`
	APIKeyPreview      string                         `json:"api_key_preview,omitempty"`
	UserID             int64                          `json:"user_id,omitempty"`
	AccountID          int64                          `json:"account_id,omitempty"`
	Model              string                         `json:"model,omitempty"`
	StatusCode         int                            `json:"status_code,omitempty"`
	UpstreamStatusCode int                            `json:"upstream_status_code,omitempty"`
	ErrorOwner         string                         `json:"error_owner,omitempty"`
	ErrorType          string                         `json:"error_type,omitempty"`
	CreatedAt          time.Time                      `json:"created_at"`
	Available          bool                           `json:"available"`
	UnavailableReason  string                         `json:"unavailable_reason,omitempty"`
	Fields             []RoutingSensitiveFailureField `json:"fields"`
}

func (s *Service) GetRoutingFailureSensitiveDetail(ctx context.Context, input RoutingSensitiveFailureDetailInput) (*RoutingSensitiveFailureDetail, error) {
	if s == nil || s.repo == nil {
		return nil, internalError("sub2api service is not configured")
	}
	normalized, err := normalizeRoutingSensitiveFailureDetailInput(input)
	if err != nil {
		return nil, err
	}
	result, err := s.repo.GetRoutingFailureSensitiveDetail(ctx, normalized)
	if err != nil {
		s.recordRoutingSensitiveFailureDetailAccessFailure(ctx, normalized, err)
		return nil, err
	}
	s.recordRoutingSensitiveFailureDetailAccess(ctx, normalized, result)
	return result, nil
}

func normalizeRoutingSensitiveFailureDetailInput(input RoutingSensitiveFailureDetailInput) (RoutingSensitiveFailureDetailInput, error) {
	if input.FailureID <= 0 {
		return RoutingSensitiveFailureDetailInput{}, badRequest("ROUTING_FAILURE_ID_INVALID", "invalid failure id")
	}
	if input.LocalGroupID <= 0 {
		return RoutingSensitiveFailureDetailInput{}, badRequest("ROUTING_GROUP_ID_INVALID", "invalid group id")
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return RoutingSensitiveFailureDetailInput{}, badRequest("ROUTING_FAILURE_SENSITIVE_DETAIL_REASON_REQUIRED", "reason is required")
	}
	if len(reason) > maxRoutingSensitiveReasonLength {
		return RoutingSensitiveFailureDetailInput{}, badRequest("ROUTING_FAILURE_SENSITIVE_DETAIL_REASON_TOO_LONG", "reason is too long")
	}
	fields, err := normalizeRoutingSensitiveFailureFields(input.Fields)
	if err != nil {
		return RoutingSensitiveFailureDetailInput{}, err
	}
	return RoutingSensitiveFailureDetailInput{
		FailureID:    input.FailureID,
		LocalGroupID: input.LocalGroupID,
		Reason:       reason,
		Fields:       fields,
		RequestedBy:  input.RequestedBy,
	}, nil
}

func normalizeRoutingSensitiveFailureFields(fields []string) ([]string, error) {
	if len(fields) == 0 {
		return append([]string(nil), defaultRoutingSensitiveFailureFields...), nil
	}
	seen := make(map[string]struct{}, len(fields))
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		normalized := strings.ToLower(strings.TrimSpace(field))
		if normalized == "" {
			continue
		}
		canonical, ok := allowedRoutingSensitiveFailureFields[normalized]
		if !ok {
			return nil, infraerrors.New(http.StatusBadRequest, "ROUTING_FAILURE_SENSITIVE_DETAIL_FIELD_INVALID", "invalid sensitive detail field")
		}
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		out = append(out, canonical)
	}
	if len(out) == 0 {
		return append([]string(nil), defaultRoutingSensitiveFailureFields...), nil
	}
	return out, nil
}

func (s *Service) recordRoutingSensitiveFailureDetailAccess(ctx context.Context, input RoutingSensitiveFailureDetailInput, result *RoutingSensitiveFailureDetail) {
	if s == nil || s.bizlog == nil || result == nil {
		return
	}
	availableFields, unavailableFields := routingSensitiveFieldNames(result.Fields)
	metadata := map[string]any{
		"failure_id":         input.FailureID,
		"local_group_id":     input.LocalGroupID,
		"requested_fields":   input.Fields,
		"available_fields":   availableFields,
		"unavailable_fields": unavailableFields,
		"available":          result.Available,
	}
	if input.RequestedBy > 0 {
		metadata["requested_by"] = input.RequestedBy
	}
	s.bizlog.Record(ctx, bizlogs.Event{
		Level:    bizlogs.LevelWarn,
		Category: bizlogs.CategorySub2API,
		Action:   "routing_failure_sensitive_detail",
		Outcome:  bizlogs.OutcomeSucceeded,
		Message:  "routing failure sensitive detail accessed",
		Reason:   input.Reason,
		Metadata: metadata,
	})
}

func (s *Service) recordRoutingSensitiveFailureDetailAccessFailure(ctx context.Context, input RoutingSensitiveFailureDetailInput, err error) {
	if s == nil || s.bizlog == nil {
		return
	}
	event := bizlogs.EventFromError(bizlogs.Event{
		Level:    bizlogs.LevelWarn,
		Category: bizlogs.CategorySub2API,
		Action:   "routing_failure_sensitive_detail",
		Outcome:  bizlogs.OutcomeFailed,
		Message:  "routing failure sensitive detail access failed",
		Reason:   input.Reason,
		Metadata: map[string]any{
			"failure_id":       input.FailureID,
			"local_group_id":   input.LocalGroupID,
			"requested_fields": input.Fields,
		},
	}, err)
	s.bizlog.Record(ctx, event)
}

func routingSensitiveFieldNames(fields []RoutingSensitiveFailureField) ([]string, []string) {
	available := make([]string, 0, len(fields))
	unavailable := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.Available {
			available = append(available, field.Name)
		} else {
			unavailable = append(unavailable, field.Name)
		}
	}
	return available, unavailable
}
