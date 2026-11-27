package web

import (
	"net/http"
	"time"

	"github.com/KevinZonda/Panel/lib/authctx"
	"github.com/KevinZonda/Panel/lib/passwordx"
	"github.com/gin-gonic/gin"
)

func (w *Web) loginPage(c *gin.Context) {
	if token, err := c.Cookie(cookieName); err == nil && token != "" {
		if _, _, err := w.authSvc.ValidateToken(token); err == nil {
			c.Redirect(http.StatusFound, "/ui")
			return
		}
	}
	w.render(c, http.StatusOK, "login.html", gin.H{"Title": "Login"})
}

func (w *Web) login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	resp, err := w.authSvc.Login(username, password)
	if err != nil {
		w.render(c, http.StatusUnauthorized, "login.html", gin.H{
			"Title": "Login",
			"Error": errorMessage(err),
		})
		return
	}

	w.setCookie(c, resp.Token, int(time.Until(resp.ExpiresAt).Seconds()))
	c.Redirect(http.StatusSeeOther, "/ui")
}

func (w *Web) logoutPage(c *gin.Context) {
	w.render(c, http.StatusOK, "logout.html", gin.H{"Title": "Logout"})
}

func (w *Web) logout(c *gin.Context) {
	session := authctx.MustSession(c)
	if session != nil {
		_ = w.authSvc.Logout(session.ID)
		if w.terminal != nil {
			w.terminal.Disconnect(session.ID)
		}
	}
	w.clearCookie(c)
	c.Redirect(http.StatusSeeOther, "/ui/login")
}

func (w *Web) passwordPage(c *gin.Context) {
	w.render(c, http.StatusOK, "password.html", gin.H{"Title": "Change Password"})
}

func (w *Web) changePassword(c *gin.Context) {
	user := authctx.MustUser(c)
	session := authctx.MustSession(c)
	if user == nil || session == nil {
		c.Redirect(http.StatusFound, "/ui/login")
		return
	}

	oldPassword := c.PostForm("old_password")
	newPassword := c.PostForm("new_password")
	if err := passwordx.ValidatePassword(newPassword); err != nil {
		w.render(c, http.StatusOK, "password.html", gin.H{
			"Title": "Change Password",
			"Error": err.Error(),
		})
		return
	}

	if err := w.authSvc.ChangePassword(user.ID, session.ID, oldPassword, newPassword); err != nil {
		w.render(c, http.StatusOK, "password.html", gin.H{
			"Title": "Change Password",
			"Error": errorMessage(err),
		})
		return
	}
	redirectMsg(c, "/ui/password", "password changed; other sessions were signed out")
}
