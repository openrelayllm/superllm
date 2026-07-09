package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPatchMihomoRuntimeConfigOverridesProviderRules(t *testing.T) {
	raw := []byte(`
mixed-port: 7890
allow-lan: true
mode: rule
external-controller: 127.0.0.1:9090
secret: old
proxies:
  - name: "香港Y01"
    type: ss
    server: hk.example.com
    port: 443
    cipher: aes-128-gcm
    password: secret
proxy-groups:
  - name: GLOBAL
    type: select
    proxies:
      - "香港Y01"
rules:
  - DOMAIN,example.com,missing-policy
`)

	patched, err := patchMihomoRuntimeConfig(raw, 17890, 19090, "runtime-secret", "香港Y01")
	require.NoError(t, err)

	var generated map[string]any
	require.NoError(t, yaml.Unmarshal(patched, &generated))
	require.Equal(t, 17890, generated["mixed-port"])
	require.Equal(t, false, generated["allow-lan"])
	require.Equal(t, "127.0.0.1:19090", generated["external-controller"])
	require.Equal(t, "runtime-secret", generated["secret"])
	require.Equal(t, []any{"MATCH,GLOBAL"}, generated["rules"])
	dns, ok := generated["dns"].(map[string]any)
	require.True(t, ok)
	require.NotEmpty(t, dns["proxy-server-nameserver"])
}

func TestConfigureSlotUsesAbsoluteConfigPath(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(wd))
	})

	runtime := &LocalMihomoRuntime{cfg: RuntimeConfig{RuntimeDir: filepath.Join("runtime", "proxy")}}
	result, err := runtime.ConfigureSlot(
		t.Context(),
		&adminplusdomain.ProxyRuntimeSlot{ID: 1, SlotKey: "proxy-slot-001"},
		&adminplusdomain.ProxyNode{DisplayName: "香港Y01"},
		[]byte(`
proxies:
  - name: "香港Y01"
    type: ss
    server: hk.example.com
    port: 443
    cipher: aes-128-gcm
    password: secret
proxy-groups:
  - name: GLOBAL
    type: select
    proxies:
      - "香港Y01"
rules:
  - MATCH,GLOBAL
`),
		"runtime-secret",
	)
	require.NoError(t, err)
	require.True(t, filepath.IsAbs(result.ConfigPath))
	require.FileExists(t, result.ConfigPath)
	require.True(t, filepath.IsAbs(result.LogPath))
}

func TestConfigureSlotPassesWritableMihomoHomeAndConfigDir(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "args.log")
	binaryPath := filepath.Join(tmpDir, "mihomo-shim")
	script := "#!/usr/bin/env sh\n" +
		"printf 'args=%s\\n' \"$*\" > " + shellQuote(logPath) + "\n" +
		"printf 'home=%s\\nxdg=%s\\n' \"$HOME\" \"$XDG_CONFIG_HOME\" >> " + shellQuote(logPath) + "\n"
	require.NoError(t, os.WriteFile(binaryPath, []byte(script), 0o755))

	runtime := NewLocalMihomoRuntime(RuntimeConfig{BinaryPath: binaryPath, RuntimeDir: filepath.Join(tmpDir, "runtime")})
	result, err := runtime.ConfigureSlot(
		t.Context(),
		&adminplusdomain.ProxyRuntimeSlot{ID: 1, SlotKey: "proxy-slot-001"},
		&adminplusdomain.ProxyNode{DisplayName: "香港Y01"},
		[]byte(`
proxies:
  - name: "香港Y01"
    type: ss
    server: hk.example.com
    port: 443
    cipher: aes-128-gcm
    password: secret
proxy-groups:
  - name: GLOBAL
    type: select
    proxies:
      - "香港Y01"
`),
		"runtime-secret",
	)
	require.NoError(t, err)
	slotDir := filepath.Dir(result.ConfigPath)
	require.Eventually(t, func() bool {
		content, err := os.ReadFile(logPath)
		if err != nil {
			return false
		}
		text := string(content)
		return strings.Contains(text, "-d "+slotDir) &&
			strings.Contains(text, "home="+slotDir) &&
			strings.Contains(text, "xdg="+slotDir)
	}, 3*time.Second, 10*time.Millisecond)
}

func TestMihomoLogSummaryReturnsLastWarningOrError(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "mihomo.log")
	require.NoError(t, os.WriteFile(logPath, []byte(`
time="2026-06-26T19:00:00+08:00" level=info msg="started"
time="2026-06-26T19:00:01+08:00" level=warning msg="[TCP] dial GLOBAL error: dial tcp 127.127.127.5:19273: i/o timeout"
time="2026-06-26T19:00:02+08:00" level=info msg="shutdown"
`), 0o600))

	require.Contains(t, mihomoLogSummary(logPath), "127.127.127.5:19273")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
