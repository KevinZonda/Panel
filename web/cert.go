package web

import (
	"net/http"
	"strconv"
	"strings"

	certsvc "github.com/KevinZonda/Panel/domain/cert/svc"
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

func (w *Web) uploadPublicPage(c *gin.Context) {
	w.render(c, http.StatusOK, "cert_upload.html", gin.H{
		"Title": "Add External Certificate",
	})
}

func (w *Web) uploadPublic(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	pub := c.PostForm("pub")
	priv := c.PostForm("priv")

	if _, err := w.certSvc.CreatePublicCert(name, pub, priv); err != nil {
		w.render(c, http.StatusOK, "cert_upload.html", gin.H{
			"Title": "Add External Certificate",
			"Error": errorMessage(err),
			"Name":  name,
			"Pub":   pub,
		})
		return
	}
	redirectMsg(c, "/ui/certs", "external certificate added")
}

func (w *Web) generateSelfSignPage(c *gin.Context) {
	w.render(c, http.StatusOK, "cert_generate.html", gin.H{
		"Title":   "Generate Self-Signed Certificate",
		"Subject": defaultSubject(),
		"Days":    certsvc.DefaultValidDays,
	})
}

func defaultSubject() certsvc.SelfSignSubject {
	return certsvc.SelfSignSubject{
		Country:      certsvc.DefaultCountry,
		Province:     certsvc.DefaultProvince,
		Locality:     certsvc.DefaultLocality,
		Organization: certsvc.DefaultOrganization,
		OrgUnit:      certsvc.DefaultOrgUnit,
		CACommonName: certsvc.DefaultCACommonName,
	}
}

func (w *Web) generateSelfSign(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	dnsNames := splitCommaList(c.PostForm("dns_names"))
	days, err := strconv.Atoi(strings.TrimSpace(c.PostForm("days")))
	if err != nil {
		days = certsvc.DefaultValidDays
	}
	subject := certsvc.SelfSignSubject{
		Country:      strings.TrimSpace(c.PostForm("country")),
		Province:     strings.TrimSpace(c.PostForm("province")),
		Locality:     strings.TrimSpace(c.PostForm("locality")),
		Organization: strings.TrimSpace(c.PostForm("organization")),
		OrgUnit:      strings.TrimSpace(c.PostForm("org_unit")),
		CACommonName: strings.TrimSpace(c.PostForm("ca_common_name")),
	}

	if _, err := w.certSvc.GenerateSelfSign(name, dnsNames, days, subject); err != nil {
		w.render(c, http.StatusOK, "cert_generate.html", gin.H{
			"Title":    "Generate Self-Signed Certificate",
			"Error":    errorMessage(err),
			"Subject":  subject,
			"Days":     days,
			"Name":     name,
			"DNSNames": c.PostForm("dns_names"),
		})
		return
	}
	redirectMsg(c, "/ui/certs", "self-signed certificate generated")
}

func certType(c *gin.Context) string {
	return c.GetString("certType")
}

func (w *Web) certPage(c *gin.Context) {
	certType := certType(c)
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
		if !certsvc.IsPublicFile(certType, file) && !isAdmin(c) {
			data["Error"] = "permission denied: admin role required to view this certificate file"
		} else if content, _, err := w.certSvc.ReadFile(certType, name, file); err != nil {
			data["Error"] = errorMessage(err)
		} else {
			data["FileName"] = file
			data["FileContent"] = string(content)
		}
	}
	w.render(c, http.StatusOK, "cert.html", data)
}

func (w *Web) uploadCertFile(c *gin.Context) {
	certType := certType(c)
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
	certType := certType(c)
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
	certType := certType(c)
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
