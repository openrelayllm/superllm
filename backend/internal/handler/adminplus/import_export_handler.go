package adminplus

import (
	"strings"

	importexportapp "github.com/Wei-Shaw/sub2api/internal/adminplus/app/importexport"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type ImportExportHandler struct {
	service *importexportapp.Service
}

func NewImportExportHandler(service *importexportapp.Service) *ImportExportHandler {
	return &ImportExportHandler{service: service}
}

func (h *ImportExportHandler) Scope(c *gin.Context) {
	if h == nil || h.service == nil {
		response.InternalError(c, "import/export service is not configured")
		return
	}
	response.Success(c, h.service.Scope())
}

func (h *ImportExportHandler) Export(c *gin.Context) {
	if h == nil || h.service == nil {
		response.InternalError(c, "import/export service is not configured")
		return
	}
	archive, err := h.service.Export(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, archive)
}

func (h *ImportExportHandler) Preview(c *gin.Context) {
	if h == nil || h.service == nil {
		response.InternalError(c, "import/export service is not configured")
		return
	}
	var archive importexportapp.Archive
	if err := c.ShouldBindJSON(&archive); err != nil {
		response.BadRequest(c, "invalid archive: "+err.Error())
		return
	}
	preview, err := h.service.Preview(c.Request.Context(), archive)
	if err != nil {
		if importexportapp.IsValidationError(err) {
			response.BadRequest(c, normalizeImportExportError(err))
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, preview)
}

func (h *ImportExportHandler) Import(c *gin.Context) {
	if h == nil || h.service == nil {
		response.InternalError(c, "import/export service is not configured")
		return
	}
	var archive importexportapp.Archive
	if err := c.ShouldBindJSON(&archive); err != nil {
		response.BadRequest(c, "invalid archive: "+err.Error())
		return
	}
	result, err := h.service.Import(c.Request.Context(), archive)
	if err != nil {
		if importexportapp.IsValidationError(err) {
			response.BadRequest(c, normalizeImportExportError(err))
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func normalizeImportExportError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "invalid archive"
	}
	return message
}
