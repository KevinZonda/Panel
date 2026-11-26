package auth

import (
	"net/http"

	"github.com/KevinZonda/Panel/lib/httpx"
	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	ginlimiter "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

func (d *Domain) InitHttp(api *gin.RouterGroup) error {
	rate, err := limiter.NewRateFromFormatted("10-M")
	if err != nil {
		return err
	}
	loginLimiter := ginlimiter.NewMiddleware(
		limiter.New(memory.NewStore(), rate),
		ginlimiter.WithLimitReachedHandler(func(c *gin.Context) {
			httpx.Error(c, http.StatusTooManyRequests, httpx.CodeRateLimited, "too many login attempts", nil)
		}),
	)

	authn := d.mw.AuthRequired()
	api.POST("/auth/login", loginLimiter, d.handler.Login)
	api.POST("/auth/logout", authn, d.handler.Logout)
	api.GET("/auth/me", authn, d.handler.Me)
	api.POST("/auth/change-password", authn, d.handler.ChangePassword)
	return nil
}
