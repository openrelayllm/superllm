package proxy

import (
	"testing"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSubscriptionNormalizerClash(t *testing.T) {
	normalizer := NewSubscriptionNormalizer()
	config, err := normalizer.Normalize(adminplusdomain.ProxySubscriptionClash, "test", []byte(`
proxies:
  - name: "香港Y01"
    type: ss
    server: hk.example.com
    port: 443
    cipher: aes-128-gcm
    password: secret
rules:
  - MATCH,GLOBAL
`))
	require.NoError(t, err)
	require.Len(t, config.Nodes, 1)
	require.Equal(t, "香港Y01", config.Nodes[0].Name)
	require.Equal(t, "ss", config.Nodes[0].Protocol)
	require.Equal(t, "HK", config.Nodes[0].Region)
	require.NotEmpty(t, config.ConfigVersion)

	var generated map[string]any
	require.NoError(t, yaml.Unmarshal(config.MihomoYAML, &generated))
	require.Equal(t, false, generated["allow-lan"])
	require.Equal(t, "127.0.0.1:9090", generated["external-controller"])
	require.Contains(t, generated, "proxy-groups")
}

func TestSubscriptionNormalizerURIList(t *testing.T) {
	normalizer := NewSubscriptionNormalizer()
	config, err := normalizer.Normalize(adminplusdomain.ProxySubscriptionV2RaySS, "uri", []byte("trojan://pass@jp.example.com:443?security=tls#日本Y01"))
	require.NoError(t, err)
	require.Len(t, config.Nodes, 1)
	require.Equal(t, "日本Y01", config.Nodes[0].Name)
	require.Equal(t, "trojan", config.Nodes[0].Protocol)
	require.Equal(t, "JP", config.Nodes[0].Region)
	require.Equal(t, "jp.example.com", config.Nodes[0].Server)
	require.Equal(t, 443, config.Nodes[0].Port)
}

func TestHostMatchesPolicy(t *testing.T) {
	require.True(t, hostMatchesPolicy("api.example.com", "*.example.com"))
	require.True(t, hostMatchesPolicy("example.com", "example.com"))
	require.False(t, hostMatchesPolicy("evil-example.com", "*.example.com"))
	require.False(t, hostMatchesPolicy("api.example.org", "*.example.com"))
}
