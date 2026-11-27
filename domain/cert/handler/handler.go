package handler

import (
	"net/http"

	"github.com/KevinZonda/Panel/domain/cert/svc"
	"github.com/KevinZonda/Panel/lib/authctx"
	"github.com/KevinZonda/Panel/lib/httpx"
	"github.com/KevinZonda/Panel/models"
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
	resp, err := h.svc.List()
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Get(c *gin.Context) {
	resp, err := h.svc.Get(c.Param("type"), c.Param("name"))
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ReadFile(c *gin.Context) {
	certType := c.Param("type")
	file := c.Param("file")
	if !svc.IsPublicFile(certType, file) {
		user := authctx.MustUser(c)
		if user == nil || user.Role != models.RoleAdmin {
			httpx.Forbidden(c, "permission denied: admin role required to read this certificate file")
			return
		}
	}
	content, _, err := h.svc.ReadFile(certType, c.Param("name"), file)
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"content": string(content)})
}

func (h *Handler) UploadFile(c *gin.Context) {
	var req apimod.UploadCertFileRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if err := h.svc.UploadFile(c.Param("type"), c.Param("name"), c.Param("file"), req.Content); err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) CreatePublic(c *gin.Context) {
	var req apimod.CreatePublicCertRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	resp, err := h.svc.CreatePublicCert(req.Name, req.Pub, req.Priv)
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) GenerateSelfSign(c *gin.Context) {
	var req apimod.GenerateSelfSignRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	resp, err := h.svc.GenerateSelfSign(req.Name, req.DNSNames, req.Days, svc.SelfSignSubject{
		Country:      req.Country,
		Province:     req.Province,
		Locality:     req.Locality,
		Organization: req.Organization,
		OrgUnit:      req.OrgUnit,
		CACommonName: req.CACommonName,
	})
	if err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Param("type"), c.Param("name")); err != nil {
		httpx.ServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
