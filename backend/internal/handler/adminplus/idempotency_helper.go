package adminplus

import (
	"context"
	"net/http"
	"strconv"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func executeAdminPlusIdempotentJSON(
	c *gin.Context,
	scope string,
	payload any,
	ttl time.Duration,
	successStatus int,
	execute func(context.Context) (any, error),
) {
	executeAdminPlusIdempotentJSONWithReplay(c, scope, payload, ttl, successStatus, execute, nil)
}

func executeAdminPlusIdempotentJSONWithReplay(
	c *gin.Context,
	scope string,
	payload any,
	ttl time.Duration,
	successStatus int,
	execute func(context.Context) (any, error),
	onReplay func(context.Context) error,
) {
	coordinator := service.DefaultIdempotencyCoordinator()
	if coordinator == nil {
		data, err := execute(c.Request.Context())
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		writeAdminPlusIdempotentSuccess(c, successStatus, data)
		return
	}

	actorScope := "admin:0"
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		actorScope = "admin:" + strconv.FormatInt(subject.UserID, 10)
	}

	result, err := coordinator.Execute(c.Request.Context(), service.IdempotencyExecuteOptions{
		Scope:          scope,
		ActorScope:     actorScope,
		Method:         c.Request.Method,
		Route:          c.FullPath(),
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
		Payload:        payload,
		RequireKey:     true,
		TTL:            ttl,
	}, execute)
	if err != nil {
		if infraerrors.Code(err) == infraerrors.Code(service.ErrIdempotencyStoreUnavail) {
			service.RecordIdempotencyStoreUnavailable(c.FullPath(), scope, "adminplus_handler_fail_close")
			logger.LegacyPrintf("handler.adminplus.idempotency", "[Idempotency] store unavailable: method=%s route=%s scope=%s strategy=fail_close", c.Request.Method, c.FullPath(), scope)
		}
		if retryAfter := service.RetryAfterSecondsFromError(err); retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		response.ErrorFrom(c, err)
		return
	}
	if result != nil && result.Replayed {
		c.Header("X-Idempotency-Replayed", "true")
		if onReplay != nil {
			if replayErr := onReplay(c.Request.Context()); replayErr != nil {
				logger.LegacyPrintf("handler.adminplus.idempotency", "[Idempotency] replay hook failed: method=%s route=%s scope=%s err=%v", c.Request.Method, c.FullPath(), scope, replayErr)
			}
		}
	}
	if result == nil {
		writeAdminPlusIdempotentSuccess(c, successStatus, nil)
		return
	}
	writeAdminPlusIdempotentSuccess(c, successStatus, result.Data)
}

func writeAdminPlusIdempotentSuccess(c *gin.Context, status int, data any) {
	if status <= 0 {
		status = http.StatusOK
	}
	c.JSON(status, response.Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}
