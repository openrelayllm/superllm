package supplierkeys

import (
	"context"
	"net/http"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type FailingSub2APIGateway struct {
	err error
}

func NewFailingSub2APIGateway(err error) *FailingSub2APIGateway {
	if err == nil {
		err = infraerrors.New(http.StatusInternalServerError, "SUB2API_GATEWAY_CONFIG_REQUIRED", "sub2api gateway base url and admin api key are required")
	}
	return &FailingSub2APIGateway{err: err}
}

func (g *FailingSub2APIGateway) CreateAccount(context.Context, *service.CreateAccountInput) (*service.Account, error) {
	return nil, g.gatewayError()
}

func (g *FailingSub2APIGateway) GetAccount(context.Context, int64) (*service.Account, error) {
	return nil, g.gatewayError()
}

func (g *FailingSub2APIGateway) UpdateAccount(context.Context, int64, *service.UpdateAccountInput) (*service.Account, error) {
	return nil, g.gatewayError()
}

func (g *FailingSub2APIGateway) CreateGroup(context.Context, *service.CreateGroupInput) (*service.Group, error) {
	return nil, g.gatewayError()
}

func (g *FailingSub2APIGateway) GetAllGroupsIncludingInactive(context.Context) ([]service.Group, error) {
	return nil, g.gatewayError()
}

func (g *FailingSub2APIGateway) FindAccount(context.Context, Sub2APIAccountLookupInput) (*service.Account, error) {
	return nil, g.gatewayError()
}

func (g *FailingSub2APIGateway) gatewayError() error {
	if g == nil || g.err == nil {
		return infraerrors.New(http.StatusInternalServerError, "SUB2API_GATEWAY_CONFIG_REQUIRED", "sub2api gateway base url and admin api key are required")
	}
	return infraerrors.Clone(infraerrors.FromError(g.err))
}

var _ Sub2APIGateway = (*FailingSub2APIGateway)(nil)
var _ Sub2APIAccountFinder = (*FailingSub2APIGateway)(nil)
