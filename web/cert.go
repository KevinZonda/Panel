package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/KevinZonda/Panel/lib/authctx"
	"github.com/KevinZonda/Panel/models"
	"github.com/gin-gonic/gin"
)

func (w *Web) certListPage(c *gin.Context) {
	list, err := w.certSvc.List()
	data := gin.H{"Title": "Certificates"}
	if err != nil {
		data["Error"] = errorMessage(err)
	} else {
		data["Certs"] = list.Items
	}
	w.render(c, http.StatusOK, "certs.html", data)
}

func (w *Web) generateSelfSign(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	dnsNames := splitCommaList(c.PostForm("dns_names"))
	days, err := strconv.Atoi(strings.TrimSpace(c.PostForm("days")))
	if err != nil || days < 1 {
		days = 365
	}

	if _, err := w.certSvc.GenerateSelfSign(name, dnsNames, days); err != nil {
		list, listErr := w.certSvc.List()
		data := gin.H{"Title": "Certificates", "Error": errorMessage(err)}
		if listErr == nil {
			data["Certs"] = list.Items
		}
		w.render(c, http.StatusOK, "certs.html", data)
		return
	}
	redirectMsg(c, "/ui/certs", "self-signed certificate generated")
}

func (w *Web) certPage(c *gin.Context) {
	certType := c.Param("type")
	name := c.Param("name")
	detail, err := w.certSvc.Get(certType, name)
	if err != nil {
		w.renderError(c, http.StatusNotFound, errorMessage(err))
		return
	}
	data := gin.H{
		"Title": "Cert " + name,
		"Cert":  detail,
	}
	// Optionally show a file's content when ?file= is requested; private
	// files are only reachable for admins because this whole branch is
	// guarded in the template and re-checked server-side below.
	if file := c.Query("file"); file != "" {
		content, isPrivate, err := w.certSvc.ReadFile(certType, name, file)
		if err != nil {
			data["Error"] = errorMessage(err)
		} else if isPrivate && !isAdmin(c) {
			data["Error"] = "permission denied: admin role required to view private keys"
		} else {
			data["FileName"] = file
			data["FileContent"] = string(content)
		}
	}
	w.render(c, http.StatusOK, "cert.html", data)
}

func (w *Web) uploadCertFile(c *gin.Context) {
	certType := c.Param("type")
	name := c.Param("name")
	file := c.Param("file")
	content := c.PostForm("content")

	if err := w.certSvc.UploadFile(certType, name, file, content); err != nil {
		detail, getErr := w.certSvc.Get(certType, name)
		data := gin.H{"Title": "Cert " + name, "Error": errorMessage(err)}
		if getErr == nil {
			data["Cert"] = detail
		}
		w.render(c, http.StatusOK, "cert.html", data)
		return
	}
	redirectMsg(c, "/ui/certs/"+certType+"/"+name, "file updated")
}

func (w *Web) deleteCertPage(c *gin.Context) {
	certType := c.Param("type")
	name := c.Param("name")
	detail, err := w.certSvc.Get(certType, name)
	if err != nil {
		w.renderError(c, http.StatusNotFound, errorMessage(err))
		return
	}
	w.render(c, http.StatusOK, "cert_delete.html", gin.H{
		"Title": "Delete Cert " + name,
		"Cert":  detail,
	})
}

func (w *Web) deleteCert(c *gin.Context) {
	certType := c.Param("type")
	name := c.Param("name")
	if err := w.certSvc.Delete(certType, name); err != nil {
		w.renderError(c, http.StatusInternalServerError, errorMessage(err))
		return
	}
	redirectMsg(c, "/ui/certs", "certificate deleted")
}

func splitCommaList(raw string) []string {
	var items []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func isAdmin(c *gin.Context) bool {
	user := authctx.MustUser(c)
	return user != nil && user.Role == models.RoleAdmin
}
