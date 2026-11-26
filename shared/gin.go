package shared

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func initGin() error {
	if cfg == nil {
		return fmt.Errorf("config is not initialized")
	}

	gin.SetMode(cfg.GinMode)
	r := gin.New()
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return fmt.Errorf("configure trusted proxies: %w", err)
	}
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2*1024*1024)
		c.Next()
	})

	allowOrigins := cfg.CorsAllowedOrigins
	if len(allowOrigins) == 0 {
		allowOrigins = []string{"*"}
	}
	r.Use(cors.New(cors.Config{
		AllowOrigins: allowOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Authorization"},
		MaxAge:       12 * time.Hour,
	}))

	engine = r
	return nil
}
