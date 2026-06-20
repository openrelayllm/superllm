package provider

import (
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	"github.com/google/wire"
)

func ProvideHTTPClient() *http.Client {
	return &http.Client{Timeout: 8 * time.Second}
}

var ProviderSet = wire.NewSet(
	ProvideHTTPClient,
	NewSessionProfileClient,
	wire.Bind(new(ports.SessionProbeAdapter), new(*SessionProfileClient)),
	wire.Bind(new(ports.SessionGroupAdapter), new(*SessionProfileClient)),
	wire.Bind(new(ports.SessionRateAdapter), new(*SessionProfileClient)),
	wire.Bind(new(ports.SessionKeyAdapter), new(*SessionProfileClient)),
)
