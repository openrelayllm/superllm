package provider

import (
	"github.com/Wei-Shaw/sub2api/internal/adminplus/ports"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewSessionProfileClient,
	wire.Bind(new(ports.SessionProbeAdapter), new(*SessionProfileClient)),
	wire.Bind(new(ports.SessionGroupAdapter), new(*SessionProfileClient)),
)
