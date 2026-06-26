package proxy

import (
	"net/http"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func invalidInput(reason, message string) *infraerrors.ApplicationError {
	return infraerrors.New(http.StatusBadRequest, reason, message)
}

func notFound(reason, message string) *infraerrors.ApplicationError {
	return infraerrors.New(http.StatusNotFound, reason, message)
}

func conflict(reason, message string) *infraerrors.ApplicationError {
	return infraerrors.New(http.StatusConflict, reason, message)
}

func forbidden(reason, message string) *infraerrors.ApplicationError {
	return infraerrors.New(http.StatusForbidden, reason, message)
}

func unavailable(reason, message string) *infraerrors.ApplicationError {
	return infraerrors.New(http.StatusServiceUnavailable, reason, message)
}

func internalError(message string) *infraerrors.ApplicationError {
	return infraerrors.New(http.StatusInternalServerError, "ADMIN_PLUS_PROXY_INTERNAL_ERROR", message)
}
