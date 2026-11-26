package handler

import (
	"net/http"
	"strconv"

	"github.com/KevinZonda/Panel/domain/user/svc"
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

func (h *Handler) List(c *gin.Context) {
	page, ok := queryInt(c, "page", 1)
	if !ok {
		return
	}
	pageSize, ok := queryInt(c, "page_size", 20)
	if !ok {
		return
	}

	resp, err := h.svc.ListUsers(page, pageSize, c.Query("keyword"))
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Create(c *gin.Context) {
	var req apimod.CreateUserRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	user, err := h.svc.CreateUser(req)
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, apimod.UserResponse(user))
}

func (h *Handler) Get(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	user, err := h.svc.GetUser(id)
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, apimod.UserResponse(user))
}

func (h *Handler) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req apimod.UpdateUserRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	actor := authctx.MustUser(c)
	if actor == nil {
		httpx.Unauthorized(c, "authentication required")
		return
	}
	user, err := h.svc.UpdateUser(actor.ID, id, req)
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, apimod.UserResponse(user))
}

func (h *Handler) ResetPassword(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req apimod.ResetPasswordRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if err := h.svc.ResetPassword(id, req.NewPassword); err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Delete(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	actor := authctx.MustUser(c)
	if actor == nil {
		httpx.Unauthorized(c, "authentication required")
		return
	}
	if err := h.svc.DeleteUser(actor.ID, id); err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func pathID(c *gin.Context) (uint, bool) {
	n, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || n == 0 {
		httpx.BadRequest(c, "invalid user id", map[string]any{"field": "id"})
		return 0, false
	}
	return uint(n), true
}

func queryInt(c *gin.Context, key string, fallback int) (int, bool) {
	raw := c.Query(key)
	if raw == "" {
		return fallback, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		httpx.BadRequest(c, "invalid query parameter", map[string]any{"field": key})
		return 0, false
	}
	return n, true
}
