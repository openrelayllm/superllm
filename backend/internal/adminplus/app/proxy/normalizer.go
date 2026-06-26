package proxy

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"gopkg.in/yaml.v3"
)

type NormalizedConfig struct {
	ProviderName  string
	ConfigVersion string
	Nodes         []NormalizedNode
	MihomoYAML    []byte
	GeneratedAt   time.Time
}

type NormalizedNode struct {
	Name     string
	Protocol string
	Server   string
	Port     int
	Region   string
	RawHash  string
	Metadata map[string]any
	Proxy    map[string]any
}

type SubscriptionNormalizer struct {
	now func() time.Time
}

func NewSubscriptionNormalizer() *SubscriptionNormalizer {
	return &SubscriptionNormalizer{now: time.Now}
}

func (n *SubscriptionNormalizer) Normalize(subscriptionType adminplusdomain.ProxySubscriptionType, providerName string, content []byte) (*NormalizedConfig, error) {
	if n == nil {
		n = NewSubscriptionNormalizer()
	}
	providerName = strings.TrimSpace(providerName)
	if providerName == "" {
		providerName = "proxy-provider"
	}
	switch subscriptionType {
	case adminplusdomain.ProxySubscriptionClash:
		return n.normalizeClash(providerName, content)
	case adminplusdomain.ProxySubscriptionShadowrocket, adminplusdomain.ProxySubscriptionV2RaySS:
		return n.normalizeURIList(providerName, content)
	default:
		return nil, invalidInput("PROXY_SUBSCRIPTION_TYPE_UNSUPPORTED", "unsupported proxy subscription type")
	}
}

func (n *SubscriptionNormalizer) normalizeClash(providerName string, content []byte) (*NormalizedConfig, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, invalidInput("PROXY_SUBSCRIPTION_PARSE_FAILED", "invalid Clash subscription yaml").WithCause(err)
	}
	rawProxies, ok := doc["proxies"]
	if !ok {
		return nil, invalidInput("PROXY_SUBSCRIPTION_NO_PROXIES", "Clash subscription does not contain proxies")
	}
	proxies, ok := rawProxies.([]any)
	if !ok {
		return nil, invalidInput("PROXY_SUBSCRIPTION_NO_PROXIES", "Clash proxies must be a list")
	}
	nodes := make([]NormalizedNode, 0, len(proxies))
	for _, raw := range proxies {
		proxyMap, ok := normalizeYAMLMap(raw).(map[string]any)
		if !ok {
			continue
		}
		name := stringFromMap(proxyMap, "name")
		server := stringFromMap(proxyMap, "server")
		protocol := strings.ToLower(stringFromMap(proxyMap, "type"))
		port := intFromMap(proxyMap, "port")
		if name == "" || protocol == "" {
			continue
		}
		nodes = append(nodes, NormalizedNode{
			Name:     name,
			Protocol: protocol,
			Server:   server,
			Port:     port,
			Region:   inferRegion(name),
			RawHash:  stableHash(proxyMap),
			Metadata: safeNodeMetadata(proxyMap),
			Proxy:    proxyMap,
		})
	}
	if len(nodes) == 0 {
		return nil, invalidInput("PROXY_SUBSCRIPTION_NO_USABLE_NODE", "subscription does not contain usable proxy nodes")
	}
	normalizedDoc := map[string]any{
		"mixed-port":          7890,
		"allow-lan":           false,
		"mode":                "rule",
		"log-level":           "warning",
		"external-controller": "127.0.0.1:9090",
		"secret":              "",
		"proxies":             proxyMaps(nodes),
		"proxy-groups": []map[string]any{
			{
				"name":    "GLOBAL",
				"type":    "select",
				"proxies": proxyNames(nodes),
			},
		},
		"rules": []string{"MATCH,GLOBAL"},
	}
	if rules, ok := doc["rules"]; ok {
		normalizedDoc["rules"] = rules
	}
	mihomoYAML, err := yaml.Marshal(normalizedDoc)
	if err != nil {
		return nil, err
	}
	return &NormalizedConfig{
		ProviderName:  providerName,
		ConfigVersion: configVersion(content, mihomoYAML),
		Nodes:         nodes,
		MihomoYAML:    mihomoYAML,
		GeneratedAt:   n.now(),
	}, nil
}

func (n *SubscriptionNormalizer) normalizeURIList(providerName string, content []byte) (*NormalizedConfig, error) {
	lines := decodeSubscriptionLines(content)
	nodes := make([]NormalizedNode, 0, len(lines))
	for _, line := range lines {
		node, err := parseNodeURI(line)
		if err != nil || node == nil {
			continue
		}
		nodes = append(nodes, *node)
	}
	if len(nodes) == 0 {
		return nil, invalidInput("PROXY_SUBSCRIPTION_NO_USABLE_NODE", "subscription does not contain usable proxy nodes")
	}
	doc := map[string]any{
		"mixed-port":          7890,
		"allow-lan":           false,
		"mode":                "rule",
		"log-level":           "warning",
		"external-controller": "127.0.0.1:9090",
		"secret":              "",
		"proxies":             proxyMaps(nodes),
		"proxy-groups": []map[string]any{
			{
				"name":    "GLOBAL",
				"type":    "select",
				"proxies": proxyNames(nodes),
			},
		},
		"rules": []string{"MATCH,GLOBAL"},
	}
	mihomoYAML, err := yaml.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return &NormalizedConfig{
		ProviderName:  providerName,
		ConfigVersion: configVersion(content, mihomoYAML),
		Nodes:         nodes,
		MihomoYAML:    mihomoYAML,
		GeneratedAt:   n.now(),
	}, nil
}

func decodeSubscriptionLines(content []byte) []string {
	text := strings.TrimSpace(string(content))
	if decoded, err := base64.StdEncoding.DecodeString(text); err == nil && strings.Contains(string(decoded), "://") {
		text = string(decoded)
	}
	lines := strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "://") {
			out = append(out, line)
		}
	}
	return out
}

func parseNodeURI(raw string) (*NormalizedNode, error) {
	if strings.HasPrefix(raw, "ss://") {
		return parseShadowsocksURI(raw)
	}
	if strings.HasPrefix(raw, "trojan://") {
		return parseSimpleAuthURI(raw, "trojan")
	}
	if strings.HasPrefix(raw, "vless://") {
		return parseSimpleAuthURI(raw, "vless")
	}
	if strings.HasPrefix(raw, "vmess://") {
		return parseVMessURI(raw)
	}
	return nil, fmt.Errorf("unsupported proxy uri")
}

func parseShadowsocksURI(raw string) (*NormalizedNode, error) {
	trimmed := strings.TrimPrefix(raw, "ss://")
	name := ""
	if idx := strings.LastIndex(trimmed, "#"); idx >= 0 {
		name, _ = url.QueryUnescape(trimmed[idx+1:])
		trimmed = trimmed[:idx]
	}
	if strings.Contains(trimmed, "@") {
		u, err := url.Parse("ss://" + trimmed)
		if err != nil {
			return nil, err
		}
		method := u.User.Username()
		password, _ := u.User.Password()
		port := parsePort(u.Port())
		if name == "" {
			name = u.Hostname()
		}
		return normalizedURIProxy(name, "ss", u.Hostname(), port, map[string]any{
			"type":     "ss",
			"name":     name,
			"server":   u.Hostname(),
			"port":     port,
			"cipher":   method,
			"password": password,
		}), nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(trimmed)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(trimmed)
	}
	if err != nil {
		return nil, err
	}
	u, err := url.Parse("ss://" + string(decoded))
	if err != nil {
		return nil, err
	}
	method := u.User.Username()
	password, _ := u.User.Password()
	port := parsePort(u.Port())
	if name == "" {
		name = u.Hostname()
	}
	return normalizedURIProxy(name, "ss", u.Hostname(), port, map[string]any{
		"type":     "ss",
		"name":     name,
		"server":   u.Hostname(),
		"port":     port,
		"cipher":   method,
		"password": password,
	}), nil
}

func parseSimpleAuthURI(raw string, protocol string) (*NormalizedNode, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	name, _ := url.QueryUnescape(u.Fragment)
	if name == "" {
		name = u.Hostname()
	}
	port := parsePort(u.Port())
	proxyMap := map[string]any{
		"type":   protocol,
		"name":   name,
		"server": u.Hostname(),
		"port":   port,
	}
	if protocol == "trojan" {
		proxyMap["password"] = u.User.Username()
	} else {
		proxyMap["uuid"] = u.User.Username()
	}
	if security := u.Query().Get("security"); security == "tls" || security == "reality" || u.Query().Get("tls") == "1" {
		proxyMap["tls"] = true
	}
	if network := u.Query().Get("type"); network != "" {
		proxyMap["network"] = network
	}
	return normalizedURIProxy(name, protocol, u.Hostname(), port, proxyMap), nil
}

func parseVMessURI(raw string) (*NormalizedNode, error) {
	encoded := strings.TrimPrefix(raw, "vmess://")
	decoded, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(encoded)
	}
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, err
	}
	name := stringFromMap(payload, "ps")
	server := stringFromMap(payload, "add")
	port := intFromAny(payload["port"])
	if name == "" {
		name = server
	}
	proxyMap := map[string]any{
		"type":    "vmess",
		"name":    name,
		"server":  server,
		"port":    port,
		"uuid":    stringFromMap(payload, "id"),
		"alterId": intFromAny(payload["aid"]),
		"cipher":  "auto",
	}
	if network := stringFromMap(payload, "net"); network != "" {
		proxyMap["network"] = network
	}
	if tls := stringFromMap(payload, "tls"); tls != "" {
		proxyMap["tls"] = tls == "tls"
	}
	if host := stringFromMap(payload, "host"); host != "" {
		proxyMap["ws-opts"] = map[string]any{"headers": map[string]any{"Host": host}}
	}
	if path := stringFromMap(payload, "path"); path != "" {
		opts, _ := proxyMap["ws-opts"].(map[string]any)
		if opts == nil {
			opts = map[string]any{}
		}
		opts["path"] = path
		proxyMap["ws-opts"] = opts
	}
	return normalizedURIProxy(name, "vmess", server, port, proxyMap), nil
}

func normalizedURIProxy(name, protocol, server string, port int, proxyMap map[string]any) *NormalizedNode {
	if strings.TrimSpace(name) == "" {
		name = server
	}
	proxyMap["name"] = name
	return &NormalizedNode{
		Name:     name,
		Protocol: protocol,
		Server:   server,
		Port:     port,
		Region:   inferRegion(name),
		RawHash:  stableHash(proxyMap),
		Metadata: safeNodeMetadata(proxyMap),
		Proxy:    proxyMap,
	}
}

func normalizeYAMLMap(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for k, v := range typed {
			out[k] = normalizeYAMLMap(v)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(typed))
		for k, v := range typed {
			out[fmt.Sprint(k)] = normalizeYAMLMap(v)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeYAMLMap(item))
		}
		return out
	default:
		return value
	}
}

func proxyMaps(nodes []NormalizedNode) []map[string]any {
	out := make([]map[string]any, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, node.Proxy)
	}
	return out
}

func proxyNames(nodes []NormalizedNode) []string {
	out := make([]string, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, node.Name)
	}
	return out
}

func safeNodeMetadata(proxyMap map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"name", "type", "server", "port", "network", "tls", "udp"} {
		if value, ok := proxyMap[key]; ok {
			out[key] = value
		}
	}
	return out
}

func configVersion(parts ...[]byte) string {
	h := sha256.New()
	for _, part := range parts {
		_, _ = h.Write(part)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func stableHash(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func publicHash(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func serverHash(server string) string {
	server = strings.TrimSpace(strings.ToLower(server))
	if server == "" {
		return ""
	}
	return publicHash(server)
}

func stringFromMap(value map[string]any, key string) string {
	raw, ok := value[key]
	if !ok || raw == nil {
		return ""
	}
	switch typed := raw.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func intFromMap(value map[string]any, key string) int {
	return intFromAny(value[key])
}

func intFromAny(raw any) int {
	switch typed := raw.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		return parsePort(typed)
	default:
		return 0
	}
}

func parsePort(value string) int {
	port, _ := strconv.Atoi(strings.TrimSpace(value))
	return port
}

func inferRegion(name string) string {
	name = strings.ToUpper(strings.TrimSpace(name))
	switch {
	case strings.Contains(name, "香港") || strings.Contains(name, "HK"):
		return "HK"
	case strings.Contains(name, "日本") || strings.Contains(name, "JP"):
		return "JP"
	case strings.Contains(name, "美国") || strings.Contains(name, "US"):
		return "US"
	case strings.Contains(name, "新加坡") || strings.Contains(name, "SG"):
		return "SG"
	case strings.Contains(name, "台湾") || strings.Contains(name, "TW"):
		return "TW"
	case strings.Contains(name, "韩国") || strings.Contains(name, "KR"):
		return "KR"
	case strings.Contains(name, "德国") || strings.Contains(name, "DE"):
		return "DE"
	case strings.Contains(name, "法国") || strings.Contains(name, "FR"):
		return "FR"
	case strings.Contains(name, "英国") || strings.Contains(name, "UK") || strings.Contains(name, "GB"):
		return "GB"
	default:
		return ""
	}
}

func hostMatchesPolicy(target, policy string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	policy = strings.ToLower(strings.TrimSpace(policy))
	if target == "" || policy == "" {
		return false
	}
	if target == policy {
		return true
	}
	if strings.HasPrefix(policy, "*.") {
		suffix := strings.TrimPrefix(policy, "*")
		return strings.HasSuffix(target, suffix)
	}
	return false
}

func canonicalHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err == nil {
			raw = parsed.Host
		}
	}
	host := raw
	if h, _, err := net.SplitHostPort(raw); err == nil {
		host = h
	}
	return strings.ToLower(strings.Trim(host, "[] "))
}
