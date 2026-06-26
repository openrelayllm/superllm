package bizlogs

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRecorderRecordWritesSanitizedSystemLog(t *testing.T) {
	writer := &fakeWriter{}
	recorder := NewRecorder(writer)
	recorder.now = func() time.Time {
		return time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)
	}

	recorder.Record(context.Background(), Event{
		Level:        "warning",
		Category:     CategoryLogin,
		Action:       "direct-login",
		Outcome:      OutcomeFailed,
		Message:      "login failed password=secret",
		SupplierID:   7,
		SupplierName: "Relay",
		ProviderType: "sub2api",
		Reason:       "LOGIN_CREDENTIAL_INVALID",
		Endpoint:     "https://supplier.example/api/v1/auth/login",
		StatusCode:   401,
		BodyExcerpt:  `{"password":"secret","message":"invalid"}`,
		Metadata: map[string]any{
			"password":     "secret",
			"access_token": "token",
			"target_email": "o***@example.com",
		},
	})

	require.Len(t, writer.inputs, 1)
	input := writer.inputs[0]
	require.Equal(t, "warn", input.Level)
	require.Equal(t, "admin_plus.login", input.Component)
	require.NotContains(t, input.Message, "secret")
	require.Contains(t, input.Message, "password=***")

	var extra map[string]any
	require.NoError(t, json.Unmarshal([]byte(input.ExtraJSON), &extra))
	require.Equal(t, "direct_login", extra["action"])
	require.Equal(t, "failed", extra["outcome"])
	require.Equal(t, "LOGIN_CREDENTIAL_INVALID", extra["reason"])
	require.Equal(t, float64(401), extra["status_code"])
	require.NotContains(t, input.ExtraJSON, "access_token")
	require.NotContains(t, input.ExtraJSON, "secret")
	require.Equal(t, "o***@example.com", extra["target_email"])
}

func TestEventFromErrorExtractsHTTPDiagnostics(t *testing.T) {
	err := infraerrors.New(http.StatusUnauthorized, "LOGIN_CREDENTIAL_INVALID", "supplier direct login credential is invalid").
		WithMetadata(map[string]string{
			"endpoint":     "https://supplier.example/api/v1/auth/login",
			"status_code":  "401",
			"content_type": "application/json",
			"body_type":    "json",
			"body_excerpt": `{"code":401,"message":"invalid email or password"}`,
		})

	event := EventFromError(Event{Category: CategoryLogin, Action: "direct_login"}, err)

	require.Equal(t, OutcomeFailed, event.Outcome)
	require.Equal(t, LevelWarn, event.Level)
	require.Equal(t, "LOGIN_CREDENTIAL_INVALID", event.Reason)
	require.Equal(t, "https://supplier.example/api/v1/auth/login", event.Endpoint)
	require.Equal(t, 401, event.StatusCode)
	require.Equal(t, "json", event.BodyType)
	require.True(t, strings.Contains(event.BodyExcerpt, "invalid email or password"))
}

type fakeWriter struct {
	inputs []*service.OpsInsertSystemLogInput
}

func (w *fakeWriter) BatchInsertSystemLogs(_ context.Context, inputs []*service.OpsInsertSystemLogInput) (int64, error) {
	w.inputs = append(w.inputs, inputs...)
	return int64(len(inputs)), nil
}
