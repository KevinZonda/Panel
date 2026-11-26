package handler

import (
	"net/http"

	"github.com/KevinZonda/Panel/domain/nginx/svc"
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

func (h *Handler) Status(c *gin.Context) {
	resp, err := h.svc.Status(c.Request.Context())
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) TestConfig(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.TestConfig(c.Request.Context()))
}

func (h *Handler) Action(c *gin.Context) {
	var req apimod.NginxActionRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	resp, err := h.svc.Action(c.Request.Context(), req.Action)
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListSites(c *gin.Context) {
	resp, err := h.svc.ListSites()
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetSite(c *gin.Context) {
	resp, err := h.svc.GetSite(c.Param("name"))
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) CreateSite(c *gin.Context) {
	var req apimod.CreateNginxSiteRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	resp, err := h.svc.CreateSite(c.Request.Context(), req.Name, req.Content, boolDefault(req.Validate, true))
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) UpdateSite(c *gin.Context) {
	var req apimod.UpdateNginxSiteRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	resp, err := h.svc.UpdateSite(c.Request.Context(), c.Param("name"), req.Content, boolDefault(req.Validate, true), req.ExpectedSHA256)
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) DeleteSite(c *gin.Context) {
	if err := h.svc.DeleteSite(c.Param("name")); err != nil {
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
