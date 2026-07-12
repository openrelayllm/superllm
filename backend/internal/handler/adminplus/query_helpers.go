package adminplus

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func parseOptionalBoolQuery(c *gin.Context, name string) *bool {
	value := strings.TrimSpace(c.Query(name))
	if value == "" {
		return nil
	}
	parsed := value == "true" || value == "1"
	return &parsed
}

func parseFloat64Query(c *gin.Context, name string) (float64, bool) {
	raw := c.Query(name)
	if raw == "" {
		return 0, true
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return value, true
}

func parseIDParam(c *gin.Context, name string) (int64, bool) {
	value, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || value <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return value, true
}
