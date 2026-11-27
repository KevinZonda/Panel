package handler

import (
	"net/http"

	"github.com/KevinZonda/Panel/domain/frp/svc"
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

func (h *Handler) ListServices(c *gin.Context) {
	resp, err := h.svc.ListServices(c.Request.Context())
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ServiceAction(c *gin.Context) {
	var req apimod.FrpServiceActionRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	resp, err := h.svc.ServiceAction(c.Request.Context(), c.Param("name"), req.Action)
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListConfigs(c *gin.Context) {
	resp, err := h.svc.ListConfigs()
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetConfig(c *gin.Context) {
	resp, err := h.svc.GetConfig(c.Param("type"), c.Param("name"))
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) CreateConfig(c *gin.Context) {
	var req apimod.CreateFrpConfigRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	resp, err := h.svc.CreateConfig(c.Request.Context(), c.Param("type"), req.Name, req.Content, boolDefault(req.Validate, true))
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) UpdateConfig(c *gin.Context) {
	var req apimod.UpdateFrpConfigRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	resp, err := h.svc.UpdateConfig(c.Request.Context(), c.Param("type"), c.Param("name"), req.Content, boolDefault(req.Validate, true), req.ExpectedSHA256)
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) DeleteConfig(c *gin.Context) {
	if err := h.svc.DeleteConfig(c.Param("type"), c.Param("name")); err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func boolDefault(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}
