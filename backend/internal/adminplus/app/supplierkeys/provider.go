package supplierkeys

import (
	"net/http"
	"os"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/wire"
)

const sub2APIEmbeddedGatewayFallbackEnv = "ADMIN_PLUS_ALLOW_EMBEDDED_SUB2API_GATEWAY"

func UseSub2APIGateway(admin service.AdminService, client *http.Client) Sub2APIGateway {
	if ShouldUseSub2APIHTTPGatewayFromEnv() {
		gateway, err := NewSub2APIHTTPGatewayFromEnv(client)
		if err == nil {
			return gateway
		}
		if !ShouldAllowEmbeddedSub2APIGatewayFallbackFromEnv() {
			return NewFailingSub2APIGateway(err)
		}
		return admin
	}
	if !ShouldAllowEmbeddedSub2APIGatewayFallbackFromEnv() {
		_, err := NewSub2APIHTTPGatewayFromEnv(client)
		return NewFailingSub2APIGateway(err)
	}
	return admin
}

func UseLocalAccountService(admin service.AdminService) LocalAccountService {
	return UseSub2APIGateway(admin, nil)
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

var ProviderSet = wire.NewSet(
	NewSQLRepository,
	wire.Bind(new(Repository), new(*SQLRepository)),
	UseSub2APIGateway,
	UseLegacyAccountService,
	NewServiceWithLegacy,
)
