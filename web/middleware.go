package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/KevinZonda/Panel/lib/authctx"
	"github.com/gin-gonic/gin"
)

const cookieName = "panel_token"

// authRequired authenticates UI requests using the session cookie.
// Unauthenticated requests are redirected to the login page.
func (w *Web) authRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(cookieName)
		if err != nil || token == "" {
			c.Redirect(http.StatusFound, "/ui/login")
			c.Abort()
			return
		}

		user, session, err := w.authSvc.ValidateToken(token)
		if err != nil {
			w.clearCookie(c)
			c.Redirect(http.StatusFound, "/ui/login")
			c.Abort()
			return
		}

		authctx.SetUser(c, user)
		authctx.SetSession(c, session)
		c.Next()
	}
}

func (w *Web) adminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := authctx.MustUser(c)
		if user == nil {
			c.Redirect(http.StatusFound, "/ui/login")
			c.Abort()
			return
		}
		if user.Role != "admin" {
			w.renderError(c, http.StatusForbidden, "permission denied: admin role required")
			c.Abort()
			return
		}
		c.Next()
	}
}

// originCheck rejects cross-site POST requests as a CSRF defense.
// Combined with SameSite=Strict cookies this covers modern browsers.
func originCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}
		u, err := url.Parse(origin)
		if err != nil || !strings.EqualFold(u.Host, c.Request.Host) {
			c.String(http.StatusForbidden, "cross-site request rejected")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (w *Web) setCookie(c *gin.Context, token string, maxAge int) {
	secure := c.Request.TLS != nil
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (w *Web) clearCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}
