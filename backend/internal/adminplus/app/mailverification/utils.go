package mailverification

import (
	"strings"
	"time"
)

func cloneTimePtr(in *time.Time) *time.Time {
	if in == nil {
		return nil
	}
	out := in.UTC()
	return &out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeScopes(scopes []string, raw string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(scopes)+4)
	add := func(value string) {
		for _, part := range strings.Fields(strings.ReplaceAll(value, ",", " ")) {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if _, ok := seen[part]; ok {
				continue
			}
			seen[part] = struct{}{}
			out = append(out, part)
		}
	}
	for _, scope := range scopes {
		add(scope)
	}
	add(raw)
	return out
}

func hasScope(scopes []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, scope := range scopes {
		if strings.TrimSpace(scope) == target {
			return true
		}
	}
	return false
}

func scopesToString(scopes []string) string {
	return strings.Join(normalizeScopes(scopes, ""), " ")
}

func maskEmail(email string) string {
	email = strings.TrimSpace(email)
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	local := parts[0]
	domain := parts[1]
	if local == "" || domain == "" {
		return ""
	}
	maskedLocal := local[:1] + "***"
	domainParts := strings.SplitN(domain, ".", 2)
	if len(domainParts) == 2 && domainParts[0] != "" {
		return maskedLocal + "@" + domainParts[0][:1] + "***." + domainParts[1]
	}
	return maskedLocal + "@***"
}

func trimLimit(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}
