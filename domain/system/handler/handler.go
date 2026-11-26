package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/KevinZonda/Panel/domain/system/svc"
	"github.com/KevinZonda/Panel/lib/httpx"
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

func (h *Handler) CPU(c *gin.Context) {
	intervalMS := 500
	if raw := c.Query("interval_ms"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 100 || n > 5000 {
			httpx.BadRequest(c, "invalid interval_ms", map[string]any{"field": "interval_ms"})
			return
		}
		intervalMS = n
	}

	resp, err := h.svc.CPU(c.Request.Context(), time.Duration(intervalMS)*time.Millisecond)
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Memory(c *gin.Context) {
	resp, err := h.svc.Memory(c.Request.Context())
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
