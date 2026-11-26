package middleware

import (
	"errors"
	"strings"

	"github.com/KevinZonda/Panel/domain/auth/svc"
	"github.com/KevinZonda/Panel/lib/apperr"
	"github.com/KevinZonda/Panel/lib/authctx"
	"github.com/KevinZonda/Panel/lib/httpx"
	"github.com/gin-gonic/gin"
)

type Middleware struct {
	svc *svc.Service
}

func New(svc *svc.Service) *Middleware {
	return &Middleware{svc: svc}
}

func (m *Middleware) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" {
			httpx.Unauthorized(c, "missing or malformed bearer token")
			return
		}

		user, session, err := m.svc.ValidateToken(token)
		if err != nil {
			if errors.Is(err, apperr.ErrUserDisabled) {
				httpx.Forbidden(c, "user is disabled")
			} else {
				httpx.Unauthorized(c, "invalid or expired token")
			}
			return
		}

		authctx.SetUser(c, user)
		authctx.SetSession(c, session)
		c.Next()
	}
}

func (m *Middleware) RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := authctx.MustUser(c)
		if user == nil {
			httpx.Unauthorized(c, "authentication required")
			return
		}
		if user.Role != role {
			httpx.Forbidden(c, "permission denied")
			return
		}
		c.Next()
	}
}

func bearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
