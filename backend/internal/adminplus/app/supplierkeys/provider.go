package supplierkeys

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/wire"
)

func UseLocalAccountService(admin service.AdminService) LocalAccountService {
	return admin
}

var ProviderSet = wire.NewSet(
	NewSQLRepository,
	wire.Bind(new(Repository), new(*SQLRepository)),
	UseLocalAccountService,
	NewService,
)
