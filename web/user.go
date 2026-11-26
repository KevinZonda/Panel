package web

import (
	"net/http"
	"strconv"

	"github.com/KevinZonda/Panel/lib/authctx"
	"github.com/KevinZonda/Panel/models/apimod"
	"github.com/gin-gonic/gin"
)

func (w *Web) userListPage(c *gin.Context) {
	list, err := w.userSvc.ListUsers(1, 100, "")
	data := gin.H{"Title": "Users"}
	if err != nil {
		data["Error"] = errorMessage(err)
	} else {
		data["Users"] = list.Items
	}
	w.render(c, http.StatusOK, "users.html", data)
}

func (w *Web) createUser(c *gin.Context) {
	req := apimod.CreateUserRequest{
		Username: c.PostForm("username"),
		Password: c.PostForm("password"),
		Role:     c.PostForm("role"),
		Status:   c.PostForm("status"),
	}
	if len(req.Password) < 8 {
		w.renderUserListWithError(c, "password must be at least 8 characters")
		return
	}
	if _, err := w.userSvc.CreateUser(req); err != nil {
		w.renderUserListWithError(c, errorMessage(err))
		return
	}
	redirectMsg(c, "/ui/users", "user created")
}

func (w *Web) renderUserListWithError(c *gin.Context, message string) {
	list, err := w.userSvc.ListUsers(1, 100, "")
	data := gin.H{"Title": "Users", "Error": message}
	if err == nil {
		data["Users"] = list.Items
	}
	w.render(c, http.StatusOK, "users.html", data)
}

func (w *Web) userPage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		w.renderError(c, http.StatusBadRequest, "invalid user id")
		return
	}
	user, err := w.userSvc.GetUser(uint(id))
	if err != nil {
		w.renderError(c, http.StatusNotFound, errorMessage(err))
		return
	}
	w.render(c, http.StatusOK, "user.html", gin.H{
		"Title":  "User " + user.Username,
		"Target": user,
	})
}

func (w *Web) updateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		w.renderError(c, http.StatusBadRequest, "invalid user id")
		return
	}
	actor := authctx.MustUser(c)
	if actor == nil {
		c.Redirect(http.StatusFound, "/ui/login")
		return
	}

	username := c.PostForm("username")
	role := c.PostForm("role")
	status := c.PostForm("status")
	req := apimod.UpdateUserRequest{
		Username: &username,
		Role:     &role,
		Status:   &status,
	}

	if _, err := w.userSvc.UpdateUser(actor.ID, uint(id), req); err != nil {
		user, getErr := w.userSvc.GetUser(uint(id))
		data := gin.H{
			"Title": "User",
			"Error": errorMessage(err),
		}
		if getErr == nil {
			data["Target"] = user
			data["Title"] = "User " + user.Username
		}
		w.render(c, http.StatusOK, "user.html", data)
		return
	}
	redirectMsg(c, "/ui/users/"+c.Param("id"), "user updated")
}

func (w *Web) resetUserPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		w.renderError(c, http.StatusBadRequest, "invalid user id")
		return
	}
	newPassword := c.PostForm("new_password")
	if len(newPassword) < 8 {
		w.renderUserPageWithError(c, uint(id), "new password must be at least 8 characters")
		return
	}
	if err := w.userSvc.ResetPassword(uint(id), newPassword); err != nil {
		w.renderUserPageWithError(c, uint(id), errorMessage(err))
		return
	}
	redirectMsg(c, "/ui/users/"+c.Param("id"), "password reset; user sessions revoked")
}

func (w *Web) renderUserPageWithError(c *gin.Context, id uint, message string) {
	user, err := w.userSvc.GetUser(id)
	data := gin.H{"Title": "User", "Error": message}
	if err == nil {
		data["Target"] = user
		data["Title"] = "User " + user.Username
	}
	w.render(c, http.StatusOK, "user.html", data)
}

func (w *Web) deleteUserPage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		w.renderError(c, http.StatusBadRequest, "invalid user id")
		return
	}
	user, err := w.userSvc.GetUser(uint(id))
	if err != nil {
		w.renderError(c, http.StatusNotFound, errorMessage(err))
		return
	}
	w.render(c, http.StatusOK, "user_delete.html", gin.H{
		"Title":  "Delete User " + user.Username,
		"Target": user,
	})
}

func (w *Web) deleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		w.renderError(c, http.StatusBadRequest, "invalid user id")
		return
	}
	actor := authctx.MustUser(c)
	if actor == nil {
		c.Redirect(http.StatusFound, "/ui/login")
		return
	}
	if err := w.userSvc.DeleteUser(actor.ID, uint(id)); err != nil {
		w.renderError(c, http.StatusConflict, errorMessage(err))
		return
	}
	redirectMsg(c, "/ui/users", "user deleted")
}
