package web

import (
	"crypto/subtle"
	"net/http"

	"github.com/KevinZonda/GoX/pkg/cryptox"
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

// csrfToken derives a per-session CSRF token from the session token.
// An attacker cannot know it because the session cookie is HttpOnly.
func csrfToken(sessionToken string) string {
	return cryptox.SHA256Str(sessionToken + ":panel-csrf")
}

func csrfTokenFromCookie(c *gin.Context) string {
	token, err := c.Cookie(cookieName)
	if err != nil || token == "" {
		return ""
	}
	return csrfToken(token)
}

// csrfCheck validates the hidden csrf_token form field on POST requests.
// Unlike an Origin/Host comparison this works behind any reverse proxy.
// Requests without a session cookie pass through: they are either the
// login POST or will be redirected to login by the auth middleware anyway.
func csrfCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}
		expected := csrfTokenFromCookie(c)
		if expected == "" {
			c.Next()
			return
		}
		provided := c.PostForm("csrf_token")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			c.String(http.StatusForbidden, "invalid or missing csrf token")
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
