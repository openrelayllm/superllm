package supplierkeys

import (
	"net/http"
	"os"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/wire"
)

const sub2APIEmbeddedGatewayFallbackEnv = "ADMIN_PLUS_ALLOW_EMBEDDED_SUB2API_GATEWAY"

func UseSub2APIGateway(admin service.AdminService, client *http.Client, cfg *config.Config) Sub2APIGateway {
	if ShouldUseSub2APIHTTPGatewayFromConfig(cfg) {
		gateway, err := NewSub2APIHTTPGatewayFromConfig(cfg, client)
		if err == nil {
			return gateway
		}
		if admin == nil || !ShouldAllowEmbeddedSub2APIGatewayFallbackFromConfig(cfg) {
			return NewFailingSub2APIGateway(err)
		}
		return admin
	}
	if admin == nil {
		_, err := NewSub2APIHTTPGatewayFromConfig(cfg, client)
		return NewFailingSub2APIGateway(err)
	}
	return admin
}

func UseLocalAccountService(admin service.AdminService) LocalAccountService {
	return admin
}

func UseLegacyAccountService(admin service.AdminService) LegacyAccountService {
	return admin
}

func ShouldAllowEmbeddedSub2APIGatewayFallbackFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(sub2APIEmbeddedGatewayFallbackEnv))) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func ShouldAllowEmbeddedSub2APIGatewayFallbackFromConfig(cfg *config.Config) bool {
	if cfg == nil {
		return ShouldAllowEmbeddedSub2APIGatewayFallbackFromEnv()
	}
	return cfg.AdminPlus.AllowEmbeddedSub2APIGateway
}

var ProviderSet = wire.NewSet(
	NewSQLRepository,
	wire.Bind(new(Repository), new(*SQLRepository)),
	UseSub2APIGateway,
	UseLegacyAccountService,
	NewServiceWithLegacy,
)
