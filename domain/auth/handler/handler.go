package handler

import (
	"net/http"

	"github.com/KevinZonda/Panel/domain/auth/svc"
	"github.com/KevinZonda/Panel/lib/authctx"
	"github.com/KevinZonda/Panel/lib/httpx"
	"github.com/KevinZonda/Panel/models/apimod"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *svc.Service
}

func New(svc *svc.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Login(c *gin.Context) {
	var req apimod.LoginRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	resp, err := h.svc.Login(req.Username, req.Password)
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Logout(c *gin.Context) {
	session := authctx.MustSession(c)
	if session == nil {
		httpx.Unauthorized(c, "authentication required")
		return
	}
	if err := h.svc.Logout(session.ID); err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Me(c *gin.Context) {
	user := authctx.MustUser(c)
	if user == nil {
		httpx.Unauthorized(c, "authentication required")
		return
	}
	resp := apimod.UserResponse(user)
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req apimod.ChangePasswordRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	user := authctx.MustUser(c)
	session := authctx.MustSession(c)
	if user == nil || session == nil {
		httpx.Unauthorized(c, "authentication required")
		return
	}
	if err := h.svc.ChangePassword(user.ID, session.ID, req.OldPassword, req.NewPassword); err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
