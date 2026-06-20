package supplierkeys

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/wire"
)

func UseLocalAccountCreator(admin service.AdminService) LocalAccountCreator {
	return admin
}

var ProviderSet = wire.NewSet(
	NewSQLRepository,
	wire.Bind(new(Repository), new(*SQLRepository)),
	UseLocalAccountCreator,
	NewService,
)
