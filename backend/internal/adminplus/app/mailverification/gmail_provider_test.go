package mailverification

import (
	"net/http"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestParseGmailAPIErrorIncludesGoogleMetadata(t *testing.T) {
	raw := []byte(`{
		"error": {
			"code": 403,
			"message": "Gmail API has not been used in project 123 before or it is disabled.",
			"status": "PERMISSION_DENIED",
			"details": [
				{
					"@type": "type.googleapis.com/google.rpc.ErrorInfo",
					"reason": "SERVICE_DISABLED",
					"domain": "googleapis.com",
					"metadata": {
						"activationUrl": "https://console.cloud.google.com/apis/library/gmail.googleapis.com?project=123"
					}
				},
				{
					"@type": "type.googleapis.com/google.rpc.Help",
					"links": [
						{
							"description": "Google developers console API activation",
							"url": "https://console.cloud.google.com/apis/library/gmail.googleapis.com?project=123"
						}
					]
				}
			]
		}
	}`)

	err := parseGmailAPIError(http.StatusForbidden, http.Header{}, raw)

	require.Equal(t, "MAIL_GMAIL_PERMISSION_DENIED", infraerrors.Reason(err))
	appErr := infraerrors.FromError(err)
	require.Equal(t, "SERVICE_DISABLED", appErr.Metadata["gmail_reason"])
	require.Equal(t, "PERMISSION_DENIED", appErr.Metadata["gmail_status"])
	require.Equal(t, "googleapis.com", appErr.Metadata["gmail_domain"])
	require.Contains(t, appErr.Metadata["gmail_message"], "Gmail API has not been used")
	require.Contains(t, appErr.Metadata["gmail_activation_url"], "gmail.googleapis.com")
	require.Contains(t, appErr.Metadata["gmail_help_url"], "gmail.googleapis.com")
}
