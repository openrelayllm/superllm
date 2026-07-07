package importexport

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewService,
)
