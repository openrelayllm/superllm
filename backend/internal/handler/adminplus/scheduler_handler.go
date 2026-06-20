package adminplus

import (
	"strings"

	schedulerapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/scheduler"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type SchedulerHandler struct {
	service *schedulerapp.Service
}

func NewSchedulerHandler(service *schedulerapp.Service) *SchedulerHandler {
	return &SchedulerHandler{service: service}
}

type runSchedulerRequest struct {
	Mode          string   `json:"mode"`
	SupplierID    int64    `json:"supplier_id"`
	TaskTypes     []string `json:"task_types"`
	WindowMinutes int      `json:"window_minutes"`
	DryRun        bool     `json:"dry_run"`
}

func (h *SchedulerHandler) Run(c *gin.Context) {
	var req runSchedulerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	taskTypes := make([]adminplusdomain.ExtensionTaskType, 0, len(req.TaskTypes))
	for _, raw := range req.TaskTypes {
		taskTypes = append(taskTypes, adminplusdomain.ExtensionTaskType(strings.TrimSpace(raw)))
	}
	run, err := h.service.Run(c.Request.Context(), schedulerapp.RunInput{
		Mode:          req.Mode,
		SupplierID:    req.SupplierID,
		TaskTypes:     taskTypes,
		WindowMinutes: req.WindowMinutes,
		DryRun:        req.DryRun,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, run)
}

func (h *SchedulerHandler) Status(c *gin.Context) {
	response.Success(c, h.service.Status())
}
