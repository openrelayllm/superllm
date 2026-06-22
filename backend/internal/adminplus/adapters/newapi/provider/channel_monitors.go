package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"hash/fnv"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const codexAPISpeedPulseEndpoint = "https://speed.codexapis.com/api/pulse"

func (c *Client) ReadChannelMonitors(ctx context.Context, in ports.SessionProbeInput) (*ports.ReadChannelMonitorsResult, error) {
	if c == nil || c.httpClient == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_INTERNAL_ERROR", "new api provider adapter is not configured")
	}
	apiBaseURL, origin, err := normalizeBaseURLs(firstNonEmpty(in.APIBaseURL, stringValueAt(in.Bundle, "context", "api_base_url"), stringValue(in.Bundle, "api_base_url")), firstNonEmpty(in.Origin, stringValue(in.Bundle, "origin")))
	if err != nil {
		return nil, err
	}
	endpoints := newAPIChannelMonitorEndpoints(apiBaseURL, origin, in.Bundle)
	if len(endpoints) == 0 {
		return nil, infraerrors.New(http.StatusConflict, "SUPPLIER_CHANNEL_MONITOR_CAPABILITY_MISSING", "new api channel monitor endpoint is not configured")
	}

	var lastErr error
	for _, endpoint := range endpoints {
		body, err := c.doPublicJSON(ctx, endpoint)
		if err != nil {
			lastErr = err
			continue
		}
		items, capturedAt, err := parsePulseChannelMonitors(body, endpoint)
		if err != nil {
			lastErr = infraerrors.New(http.StatusBadGateway, "SUPPLIER_CHANNEL_MONITORS_RESPONSE_INVALID", "new api pulse response is invalid").WithCause(err)
			continue
		}
		return &ports.ReadChannelMonitorsResult{
			SupplierID: in.SupplierID,
			SystemType: "new_api",
			Origin:     origin,
			APIBaseURL: endpoint,
			Items:      items,
			CapturedAt: capturedAt,
		}, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_CHANNEL_MONITORS_RESPONSE_INVALID", "new api pulse response is invalid")
}

func (c *Client) doPublicJSON(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	applyBrowserCompatHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_SESSION_REQUEST_FAILED", "failed to request new api pulse endpoint").WithCause(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, infraerrors.New(http.StatusBadGateway, "SUPPLIER_SESSION_BAD_STATUS", "new api pulse endpoint returned non-success status")
	}
	return data, nil
}

func newAPIChannelMonitorEndpoints(apiBaseURL string, origin string, bundle map[string]any) []string {
	contextValue := mapValue(bundle, "context")
	rawCandidates := []string{
		stringValue(bundle, "channel_monitor_url"),
		stringValue(bundle, "channel_status_url"),
		stringValue(bundle, "pulse_url"),
		stringValue(bundle, "speed_url"),
		stringValue(contextValue, "channel_monitor_url"),
		stringValue(contextValue, "channel_status_url"),
		stringValue(contextValue, "pulse_url"),
		stringValue(contextValue, "speed_url"),
	}
	candidates := make([]string, 0, 4)
	seen := map[string]bool{}
	add := func(raw string) {
		endpoint := normalizePulseEndpoint(raw)
		if endpoint == "" || seen[endpoint] {
			return
		}
		seen[endpoint] = true
		candidates = append(candidates, endpoint)
	}
	for _, raw := range rawCandidates {
		add(raw)
	}
	if isCodexAPISite(apiBaseURL) || isCodexAPISite(origin) {
		add(codexAPISpeedPulseEndpoint)
	}
	add(apiBaseURL)
	add(origin)
	return candidates
}

func normalizePulseEndpoint(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := parseSafeURL(raw, "SUPPLIER_SESSION_API_BASE_URL_INVALID")
	if err != nil {
		return ""
	}
	path := strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(path, "/api/pulse") {
		parsed.Path = path
		parsed.RawQuery = ""
		parsed.Fragment = ""
		return parsed.String()
	}
	parsed.Path = strings.TrimRight(path, "/") + "/api/pulse"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func isCodexAPISite(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "codexapis.com" || strings.HasSuffix(host, ".codexapis.com")
}

func parsePulseChannelMonitors(data []byte, endpoint string) ([]ports.ChannelMonitorView, time.Time, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return nil, time.Time{}, err
	}
	capturedAt := timeFromUnixMillis(int64FromAny(root["generated_ms"]))
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}
	windowSeconds := int64FromAny(root["window_seconds"])
	if windowSeconds <= 0 {
		windowSeconds = 60
	}
	models, _ := root["models"].([]any)
	items := make([]ports.ChannelMonitorView, 0, len(models))
	for _, raw := range models {
		model, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(stringFromAny(model["model"]))
		if name == "" {
			continue
		}
		status := normalizePulseHealth(stringFromAny(model["health"]), float64FromAny(model["success_rate"]))
		latency := int64PtrFromFloat(model["avg_ttft_ms"])
		responseMS := int64PtrFromSeconds(model["avg_resp_sec"])
		items = append(items, ports.ChannelMonitorView{
			ID:                   stableMonitorID(name),
			Name:                 name,
			Provider:             providerFromModelName(name),
			GroupName:            pulseGroupName(endpoint, windowSeconds, int64FromAny(model["req_count"])),
			PrimaryModel:         name,
			PrimaryStatus:        status,
			PrimaryLatencyMS:     latency,
			PrimaryPingLatencyMS: responseMS,
			Availability7D:       clampPercent(float64FromAny(model["success_rate"])),
			ExtraModels:          []ports.ChannelMonitorExtraModel{},
			Timeline: []ports.ChannelMonitorTimelinePoint{
				{
					Status:        status,
					LatencyMS:     latency,
					PingLatencyMS: responseMS,
					CheckedAt:     capturedAt.Format(time.RFC3339),
				},
			},
		})
	}
	return items, capturedAt, nil
}

func timeFromUnixMillis(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value).UTC()
}

func stableMonitorID(value string) int64 {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(value))
	return int64(hash.Sum32())
}

func int64PtrFromFloat(value any) *int64 {
	number, ok := float64Value(value)
	if !ok || number <= 0 {
		return nil
	}
	out := int64(math.Round(number))
	return &out
}

func int64PtrFromSeconds(value any) *int64 {
	number, ok := float64Value(value)
	if !ok || number <= 0 {
		return nil
	}
	out := int64(math.Round(number * 1000))
	return &out
}

func normalizePulseHealth(health string, successRate float64) string {
	switch strings.ToLower(strings.TrimSpace(health)) {
	case "good", "ok", "healthy", "success", "operational":
		return "operational"
	case "warn", "warning", "slow", "degraded":
		return "degraded"
	case "bad", "down", "failed", "fail":
		return "failed"
	case "error":
		return "error"
	}
	switch {
	case successRate >= 99:
		return "operational"
	case successRate >= 95:
		return "degraded"
	case successRate > 0:
		return "failed"
	default:
		return "empty"
	}
}

func providerFromModelName(model string) string {
	value := strings.ToLower(model)
	switch {
	case strings.Contains(value, "claude") || strings.Contains(value, "anthropic"):
		return "anthropic"
	case strings.Contains(value, "gemini") || strings.Contains(value, "google"):
		return "gemini"
	case strings.Contains(value, "gpt") || strings.Contains(value, "openai") || strings.Contains(value, "o1") || strings.Contains(value, "o3") || strings.Contains(value, "o4"):
		return "openai"
	default:
		return "new_api"
	}
}

func pulseGroupName(endpoint string, windowSeconds int64, requestCount int64) string {
	host := ""
	if parsed, err := url.Parse(endpoint); err == nil {
		host = parsed.Hostname()
	}
	parts := make([]string, 0, 3)
	if host != "" {
		parts = append(parts, host)
	}
	parts = append(parts, formatPulseWindow(windowSeconds))
	if requestCount > 0 {
		parts = append(parts, strconv.FormatInt(requestCount, 10)+" req")
	}
	return strings.Join(parts, " · ")
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func formatPulseWindow(windowSeconds int64) string {
	if windowSeconds <= 0 {
		return "60s"
	}
	return strconv.FormatInt(windowSeconds, 10) + "s"
}
