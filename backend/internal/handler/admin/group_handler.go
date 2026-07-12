package admin

import (
	"context"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type groupReader interface {
	GetAllGroups(ctx context.Context) ([]service.Group, error)
	GetAllGroupsByPlatform(ctx context.Context, platform string) ([]service.Group, error)
	GetAllGroupsIncludingInactive(ctx context.Context) ([]service.Group, error)
}

// GroupHandler exposes the read-only group surface kept by SuperLLM MVP0.
type GroupHandler struct {
	groups groupReader
}

func NewGroupHandler(groups groupReader, _ ...any) *GroupHandler {
	return &GroupHandler{groups: groups}
}

// GetAll handles getting all groups needed by Ops/SuperLLM filters.
// GET /api/v1/admin/groups/all
func (h *GroupHandler) GetAll(c *gin.Context) {
	if h == nil || h.groups == nil {
		response.Error(c, http.StatusServiceUnavailable, "Group service not available")
		return
	}

	platform := c.Query("platform")
	includeInactive := c.Query("include_inactive") == "true"

	var groups []service.Group
	var err error
	if includeInactive {
		groups, err = h.groups.GetAllGroupsIncludingInactive(c.Request.Context())
	} else if platform != "" {
		groups, err = h.groups.GetAllGroupsByPlatform(c.Request.Context(), platform)
	} else {
		groups, err = h.groups.GetAllGroups(c.Request.Context())
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	outGroups := make([]dto.AdminGroup, 0, len(groups))
	for i := range groups {
		outGroups = append(outGroups, *dto.GroupFromServiceAdmin(&groups[i]))
	}
	response.Success(c, outGroups)
}
