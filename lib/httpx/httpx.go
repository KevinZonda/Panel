package httpx

import (
	"errors"
	"net/http"

	"github.com/KevinZonda/Panel/lib/apperr"
	"github.com/KevinZonda/Panel/models/apimod"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const (
	CodeInvalidArgument = "INVALID_ARGUMENT"
	CodeUnauthorized    = "UNAUTHORIZED"
	CodeForbidden       = "FORBIDDEN"
	CodeNotFound        = "NOT_FOUND"
	CodeConflict        = "CONFLICT"
	CodeRateLimited     = "RATE_LIMITED"
	CodeInternalError   = "INTERNAL_ERROR"
	CodeDependencyError = "DEPENDENCY_ERROR"
)

func Error(c *gin.Context, status int, code, message string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	c.AbortWithStatusJSON(status, apimod.ErrorResponse{
		Error: apimod.ErrorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

func BadRequest(c *gin.Context, message string, details map[string]any) {
	Error(c, http.StatusBadRequest, CodeInvalidArgument, message, details)
}

func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, CodeUnauthorized, message, nil)
}

func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, CodeForbidden, message, nil)
}

func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, CodeNotFound, message, nil)
}

func Conflict(c *gin.Context, message string, details map[string]any) {
	Error(c, http.StatusConflict, CodeConflict, message, details)
}

func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, CodeInternalError, message, nil)
}

func DependencyError(c *gin.Context, message string, details map[string]any) {
	Error(c, http.StatusBadGateway, CodeDependencyError, message, details)
}

func BindJSON(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		BadRequest(c, "invalid request body", map[string]any{"error": err.Error()})
		return false
	}
	return true
}

func ServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, apperr.ErrInvalidCredentials):
		Unauthorized(c, "invalid username or password")
	case errors.Is(err, apperr.ErrUserDisabled):
		Forbidden(c, "user is disabled")
	case errors.Is(err, apperr.ErrValidation):
		BadRequest(c, err.Error(), nil)
	case errors.Is(err, apperr.ErrNotFound):
		NotFound(c, err.Error())
	case errors.Is(err, apperr.ErrConflict):
		Conflict(c, err.Error(), nil)
	case errors.Is(err, apperr.ErrForbidden):
		Forbidden(c, err.Error())
	case errors.Is(err, apperr.ErrDependency):
		DependencyError(c, err.Error(), nil)
	default:
		logrus.WithError(err).Error("request failed")
		InternalError(c, "internal server error")
	}
}
