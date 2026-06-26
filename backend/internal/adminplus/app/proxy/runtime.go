package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"gopkg.in/yaml.v3"
)

const (
	defaultProxyRuntimeDir         = "runtime/proxy"
	defaultProxyBaseMixedPort      = 17890
	defaultProxyBaseControllerPort = 19090
	defaultProxyMaxSlots           = 4
	defaultProxyControllerGroup    = "GLOBAL"
	defaultProxyEgressCheckURL     = "https://api.ipify.org?format=json"
)

type RuntimeConfig struct {
	BinaryPath         string
	RuntimeDir         string
	BaseMixedPort      int
	BaseControllerPort int
	MaxSlots           int
	EgressCheckURL     string
}

type Runtime interface {
	ConfigureSlot(ctx context.Context, slot *adminplusdomain.ProxyRuntimeSlot, node *adminplusdomain.ProxyNode, mihomoYAML []byte, controllerSecret string) (*RuntimeSlotResult, error)
	SwitchNode(ctx context.Context, slot *adminplusdomain.ProxyRuntimeSlot, nodeName string, controllerSecret string) error
	VerifyEgress(ctx context.Context, mixedPort int) (string, error)
	RestartSlot(ctx context.Context, slot *adminplusdomain.ProxyRuntimeSlot) error
}

type RuntimeSlotResult struct {
	ConfigPath string
	ProcessID  *int
	StartedAt  *time.Time
}

type LocalMihomoRuntime struct {
	cfg    RuntimeConfig
	client *http.Client
}

func RuntimeConfigFromConfig(cfg *config.Config) RuntimeConfig {
	out := RuntimeConfig{
		RuntimeDir:         defaultProxyRuntimeDir,
		BaseMixedPort:      defaultProxyBaseMixedPort,
		BaseControllerPort: defaultProxyBaseControllerPort,
		MaxSlots:           defaultProxyMaxSlots,
		EgressCheckURL:     defaultProxyEgressCheckURL,
	}
	if cfg != nil {
		if strings.TrimSpace(cfg.AdminPlus.ProxyMihomoBinaryPath) != "" {
			out.BinaryPath = strings.TrimSpace(cfg.AdminPlus.ProxyMihomoBinaryPath)
		}
		if strings.TrimSpace(cfg.AdminPlus.ProxyRuntimeDir) != "" {
			out.RuntimeDir = strings.TrimSpace(cfg.AdminPlus.ProxyRuntimeDir)
		}
		if cfg.AdminPlus.ProxyBaseMixedPort > 0 {
			out.BaseMixedPort = cfg.AdminPlus.ProxyBaseMixedPort
		}
		if cfg.AdminPlus.ProxyBaseControllerPort > 0 {
			out.BaseControllerPort = cfg.AdminPlus.ProxyBaseControllerPort
		}
		if cfg.AdminPlus.ProxyMaxSlots > 0 {
			out.MaxSlots = cfg.AdminPlus.ProxyMaxSlots
		}
		if strings.TrimSpace(cfg.AdminPlus.ProxyEgressCheckURL) != "" {
			out.EgressCheckURL = strings.TrimSpace(cfg.AdminPlus.ProxyEgressCheckURL)
		}
	}
	return out
}

func NewLocalMihomoRuntime(cfg RuntimeConfig) *LocalMihomoRuntime {
	if cfg.RuntimeDir == "" {
		cfg.RuntimeDir = defaultProxyRuntimeDir
	}
	if cfg.BaseMixedPort <= 0 {
		cfg.BaseMixedPort = defaultProxyBaseMixedPort
	}
	if cfg.BaseControllerPort <= 0 {
		cfg.BaseControllerPort = defaultProxyBaseControllerPort
	}
	if cfg.MaxSlots <= 0 {
		cfg.MaxSlots = defaultProxyMaxSlots
	}
	if cfg.EgressCheckURL == "" {
		cfg.EgressCheckURL = defaultProxyEgressCheckURL
	}
	return &LocalMihomoRuntime{
		cfg:    cfg,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (r *LocalMihomoRuntime) ConfigureSlot(ctx context.Context, slot *adminplusdomain.ProxyRuntimeSlot, node *adminplusdomain.ProxyNode, mihomoYAML []byte, controllerSecret string) (*RuntimeSlotResult, error) {
	if slot == nil {
		return nil, invalidInput("PROXY_SLOT_REQUIRED", "proxy runtime slot is required")
	}
	if len(mihomoYAML) == 0 {
		return nil, invalidInput("PROXY_CONFIG_REQUIRED", "mihomo config is required")
	}
	if slot.MixedPort <= 0 {
		slot.MixedPort = r.cfg.BaseMixedPort + int(slot.ID)
	}
	if slot.ControllerPort <= 0 {
		slot.ControllerPort = r.cfg.BaseControllerPort + int(slot.ID)
	}
	selectedNodeName := ""
	if node != nil {
		selectedNodeName = node.DisplayName
	}
	configBytes, err := patchMihomoRuntimeConfig(mihomoYAML, slot.MixedPort, slot.ControllerPort, controllerSecret, selectedNodeName)
	if err != nil {
		return nil, err
	}
	slotDir := filepath.Join(r.cfg.RuntimeDir, "slots", slot.SlotKey)
	if err := os.MkdirAll(slotDir, 0o700); err != nil {
		return nil, err
	}
	configPath := filepath.Join(slotDir, "config.yaml")
	if err := os.WriteFile(configPath, configBytes, 0o600); err != nil {
		return nil, err
	}
	result := &RuntimeSlotResult{ConfigPath: configPath}
	if strings.TrimSpace(r.cfg.BinaryPath) == "" {
		return result, nil
	}
	cmd := exec.CommandContext(ctx, r.cfg.BinaryPath, "-f", configPath)
	cmd.Dir = slotDir
	if err := cmd.Start(); err != nil {
		return nil, unavailable("PROXY_MIHOMO_START_FAILED", "failed to start mihomo runtime").WithCause(err)
	}
	pid := cmd.Process.Pid
	startedAt := time.Now()
	result.ProcessID = &pid
	result.StartedAt = &startedAt
	go func() {
		_ = cmd.Wait()
	}()
	_ = node
	return result, nil
}

func (r *LocalMihomoRuntime) SwitchNode(ctx context.Context, slot *adminplusdomain.ProxyRuntimeSlot, nodeName string, controllerSecret string) error {
	if slot == nil || slot.ControllerPort <= 0 {
		return invalidInput("PROXY_SLOT_CONTROLLER_REQUIRED", "runtime slot controller is not configured")
	}
	nodeName = strings.TrimSpace(nodeName)
	if nodeName == "" {
		return invalidInput("PROXY_NODE_NAME_REQUIRED", "proxy node name is required")
	}
	body, _ := json.Marshal(map[string]string{"name": nodeName})
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("http://127.0.0.1:%d/proxies/%s", slot.ControllerPort, defaultProxyControllerGroup), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if controllerSecret != "" {
		req.Header.Set("Authorization", "Bearer "+controllerSecret)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return unavailable("PROXY_NODE_SWITCH_FAILED", "failed to switch proxy node").WithCause(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return unavailable("PROXY_NODE_SWITCH_FAILED", "mihomo controller rejected node switch")
	}
	return nil
}

func (r *LocalMihomoRuntime) VerifyEgress(ctx context.Context, mixedPort int) (string, error) {
	if mixedPort <= 0 {
		return "", invalidInput("PROXY_MIXED_PORT_REQUIRED", "proxy mixed port is required")
	}
	proxyURL, err := url.Parse("http://127.0.0.1:" + strconv.Itoa(mixedPort))
	if err != nil {
		return "", err
	}
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for {
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		client := &http.Client{Transport: transport, Timeout: 4 * time.Second}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.cfg.EgressCheckURL, nil)
		if err != nil {
			return "", err
		}
		resp, err := client.Do(req)
		if err == nil {
			var payload struct {
				IP string `json:"ip"`
			}
			decodeErr := json.NewDecoder(resp.Body).Decode(&payload)
			_ = resp.Body.Close()
			if decodeErr != nil {
				lastErr = decodeErr
			} else if strings.TrimSpace(payload.IP) != "" {
				return strings.TrimSpace(payload.IP), nil
			} else {
				lastErr = fmt.Errorf("proxy egress ip response is empty")
			}
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
	return "", unavailable("PROXY_EGRESS_VERIFY_FAILED", "failed to verify proxy egress ip").WithCause(lastErr)
}

func (r *LocalMihomoRuntime) RestartSlot(ctx context.Context, slot *adminplusdomain.ProxyRuntimeSlot) error {
	if slot == nil {
		return invalidInput("PROXY_SLOT_REQUIRED", "proxy runtime slot is required")
	}
	if slot.ProcessID != nil && *slot.ProcessID > 0 {
		if process, err := os.FindProcess(*slot.ProcessID); err == nil {
			_ = process.Kill()
		}
	}
	_ = ctx
	return nil
}

func patchMihomoRuntimeConfig(raw []byte, mixedPort int, controllerPort int, secret string, selectedNodeName string) ([]byte, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	doc["mixed-port"] = mixedPort
	doc["allow-lan"] = false
	doc["external-controller"] = "127.0.0.1:" + strconv.Itoa(controllerPort)
	doc["secret"] = secret
	reorderMihomoProxyGroup(doc, defaultProxyControllerGroup, selectedNodeName)
	return yaml.Marshal(doc)
}

func reorderMihomoProxyGroup(doc map[string]any, groupName string, selectedNodeName string) {
	selectedNodeName = strings.TrimSpace(selectedNodeName)
	if selectedNodeName == "" {
		return
	}
	rawGroups, ok := doc["proxy-groups"].([]any)
	if !ok {
		return
	}
	for _, rawGroup := range rawGroups {
		group, ok := rawGroup.(map[string]any)
		if !ok || strings.TrimSpace(fmt.Sprint(group["name"])) != groupName {
			continue
		}
		rawProxies, ok := group["proxies"].([]any)
		if !ok {
			continue
		}
		next := make([]any, 0, len(rawProxies))
		next = append(next, selectedNodeName)
		for _, rawProxy := range rawProxies {
			name := strings.TrimSpace(fmt.Sprint(rawProxy))
			if name == "" || name == selectedNodeName {
				continue
			}
			next = append(next, rawProxy)
		}
		group["proxies"] = next
		return
	}
}
