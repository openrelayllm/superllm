package suppliers

import (
	"net/http"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nonBlankSecret(value string) bool {
	return strings.TrimSpace(value) != ""
}

func trimLimit(value string, limit int) string {
	v := strings.TrimSpace(value)
	if len(v) <= limit {
		return v
	}
	return v[:limit]
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func stringInSlice(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if strings.EqualFold(value, candidate) {
			return true
		}
	}
	return false
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func maskUsername(value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return ""
	}
	if strings.Contains(v, "@") {
		parts := strings.SplitN(v, "@", 2)
		name := parts[0]
		if len(name) <= 2 {
			return name[:1] + "***@" + parts[1]
		}
		return name[:2] + "***@" + parts[1]
	}
	if len(v) <= 4 {
		return v[:1] + "***"
	}
	return v[:2] + "***" + v[len(v)-2:]
}

func badRequest(reason string, message string) error {
	return infraerrors.New(http.StatusBadRequest, reason, message)
}

func internalError(message string) error {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", message)
}
