package supplierkeys

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/wire"
)

func UseSub2APIGateway(admin service.AdminService, client *http.Client) Sub2APIGateway {
	if ShouldUseSub2APIHTTPGatewayFromEnv() {
		gateway, err := NewSub2APIHTTPGatewayFromEnv(client)
		if err == nil {
			return gateway
		}
	}
	return admin
}

func UseLocalAccountService(admin service.AdminService) LocalAccountService {
	return UseSub2APIGateway(admin, nil)
}

func UseLegacyAccountService(admin service.AdminService) LegacyAccountService {
	return admin
}

var ProviderSet = wire.NewSet(
	NewSQLRepository,
	wire.Bind(new(Repository), new(*SQLRepository)),
	UseSub2APIGateway,
	UseLegacyAccountService,
	NewServiceWithLegacy,
)
