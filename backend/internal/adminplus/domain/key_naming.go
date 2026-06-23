package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var keyNameUnsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

type SupplierGroupNamingInput struct {
	SupplierName   string
	OfficialName   string
	GroupName      string
	Description    string
	ProviderFamily string
	RawPayload     map[string]any
	RateMultiplier float64
	UpdatedAt      time.Time
}

func ApplySupplierGroupNaming(group *SupplierGroup, supplierName string, updatedAt time.Time) {
	if group == nil {
		return
	}
	officialName := strings.TrimSpace(group.OfficialName)
	if officialName == "" {
		officialName = strings.TrimSpace(group.Name)
	}
	naming := BuildSupplierGroupNaming(SupplierGroupNamingInput{
		SupplierName:   supplierName,
		OfficialName:   officialName,
		GroupName:      group.Name,
		Description:    group.Description,
		ProviderFamily: group.ProviderFamily,
		RawPayload:     group.RawPayload,
		RateMultiplier: effectiveGroupRate(group),
		UpdatedAt:      updatedAt,
	})
	group.OfficialName = naming.OfficialName
	group.ModelFamily = naming.ModelFamily
	group.ModelSpec = naming.ModelSpec
	group.StandardKeyName = naming.StandardKeyName
	if !updatedAt.IsZero() {
		t := updatedAt.UTC()
		group.NamingUpdatedAt = &t
	}
}

type SupplierGroupNaming struct {
	OfficialName    string
	ModelFamily     string
	ModelSpec       string
	StandardKeyName string
}

func BuildSupplierGroupNaming(in SupplierGroupNamingInput) SupplierGroupNaming {
	officialName := firstText(in.OfficialName, in.GroupName)
	modelFamily := inferModelFamily(in.ProviderFamily, officialName, in.Description, in.RawPayload)
	modelSpec := inferModelSpec(modelFamily, officialName, in.Description, in.RawPayload)
	if modelSpec == "" {
		modelSpec = modelFamily
	}
	standardName := strings.Join([]string{
		keyNamePart(in.SupplierName, "supplier"),
		keyNamePart(modelFamily, "Model"),
		keyNamePart(modelSpec, "Spec"),
		formatRateMultiplier(in.RateMultiplier),
	}, "-")
	return SupplierGroupNaming{
		OfficialName:    officialName,
		ModelFamily:     modelFamily,
		ModelSpec:       modelSpec,
		StandardKeyName: trimRunes(standardName, 160),
	}
}

func effectiveGroupRate(group *SupplierGroup) float64 {
	if group == nil {
		return 1
	}
	if group.EffectiveRateMultiplier > 0 {
		return group.EffectiveRateMultiplier
	}
	if group.UserRateMultiplier != nil && *group.UserRateMultiplier > 0 {
		return *group.UserRateMultiplier
	}
	if group.RateMultiplier > 0 {
		return group.RateMultiplier
	}
	return 1
}

func inferModelFamily(providerFamily string, officialName string, description string, raw map[string]any) string {
	haystack := strings.ToLower(strings.Join([]string{
		providerFamily,
		officialName,
		description,
		stringFromRaw(raw, "provider"),
		stringFromRaw(raw, "provider_family"),
		stringFromRaw(raw, "platform"),
		stringFromRaw(raw, "model_family"),
	}, " "))
	switch {
	case strings.Contains(haystack, "claude") || strings.Contains(haystack, "anthropic"):
		return "Claude"
	case strings.Contains(haystack, "gemini") || strings.Contains(haystack, "google"):
		return "Gemini"
	case strings.Contains(haystack, "antigravity"):
		return "Antigravity"
	case strings.Contains(haystack, "openai") || strings.Contains(haystack, "gpt") || strings.Contains(haystack, "codex") || strings.Contains(haystack, "pro") || strings.Contains(haystack, "plus"):
		return "OpenAI"
	default:
		return titleToken(firstText(providerFamily, "mixed"))
	}
}

func inferModelSpec(modelFamily string, officialName string, description string, raw map[string]any) string {
	if value := firstText(
		stringFromRaw(raw, "model_spec"),
		stringFromRaw(raw, "modelSpec"),
		stringFromRaw(raw, "spec"),
		stringFromRaw(raw, "tier"),
		stringFromRaw(raw, "plan"),
	); value != "" {
		return keyNamePart(value, modelFamily)
	}
	text := firstText(officialName, description)
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "claudemax"):
		return compactDisplayToken(text)
	case strings.Contains(lower, "claude"):
		return firstNonEmptySpec(compactDisplayToken(text), "Claude")
	case strings.Contains(lower, "gemini"):
		return firstNonEmptySpec(compactDisplayToken(text), "Gemini")
	case strings.Contains(lower, "pro"):
		return "Pro"
	case strings.Contains(lower, "plus"):
		return "Plus"
	case strings.Contains(lower, "codex"):
		return "Codex"
	default:
		return keyNamePart(text, modelFamily)
	}
}

func firstNonEmptySpec(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func keyNamePart(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	value = strings.ReplaceAll(value, "/", "-")
	value = strings.ReplaceAll(value, "\\", "-")
	value = strings.ReplaceAll(value, "×", "x")
	value = strings.ReplaceAll(value, "倍率", "")
	value = strings.Join(strings.Fields(value), "-")
	value = keyNameUnsafeChars.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-._")
	if value == "" {
		value = fallback
	}
	return trimRunes(value, 80)
}

func compactDisplayToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, titleToken(part))
	}
	return keyNamePart(strings.Join(out, ""), "")
}

func titleToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	switch lower {
	case "openai":
		return "OpenAI"
	case "new_api", "new-api", "newapi":
		return "OpenAI"
	case "api":
		return "API"
	case "pro":
		return "Pro"
	case "plus":
		return "Plus"
	case "claude":
		return "Claude"
	case "gemini":
		return "Gemini"
	case "antigravity":
		return "Antigravity"
	case "codex":
		return "Codex"
	default:
		rs := []rune(lower)
		if len(rs) == 0 {
			return ""
		}
		rs[0] = unicode.ToUpper(rs[0])
		return string(rs)
	}
}

func formatRateMultiplier(value float64) string {
	if value <= 0 {
		value = 1
	}
	text := strconv.FormatFloat(value, 'f', 4, 64)
	text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	if text == "" {
		text = "1"
	}
	return fmt.Sprintf("%sx", text)
}

func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringFromRaw(raw map[string]any, key string) string {
	if len(raw) == 0 {
		return ""
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func trimRunes(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return value
	}
	rs := []rune(value)
	if len(rs) <= limit {
		return value
	}
	return string(rs[:limit])
}
