package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (w *Web) nginxPage(c *gin.Context) {
	status, err := w.nginxSvc.Status(c.Request.Context())
	data := gin.H{"Title": "Nginx"}
	if err != nil {
		data["Error"] = errorMessage(err)
	} else {
		data["Nginx"] = status
	}
	w.render(c, http.StatusOK, "nginx.html", data)
}

func (w *Web) nginxTestConfig(c *gin.Context) {
	result := w.nginxSvc.TestConfig(c.Request.Context())
	if result.OK {
		redirectMsg(c, "/ui/nginx", "config test passed")
	} else {
		redirectMsg(c, "/ui/nginx", "config test failed, see output below")
	}
}

func (w *Web) nginxActionConfirmPage(c *gin.Context) {
	action := c.Param("action")
	if action != "start" && action != "stop" && action != "restart" {
		w.renderError(c, http.StatusNotFound, "unknown action")
		return
	}
	w.render(c, http.StatusOK, "nginx_action_confirm.html", gin.H{
		"Title":  "Confirm nginx " + action,
		"Action": action,
	})
}

func (w *Web) nginxAction(c *gin.Context) {
	action := c.PostForm("action")
	resp, err := w.nginxSvc.Action(c.Request.Context(), action)
	data := gin.H{
		"Title":  "Nginx Action",
		"Action": action,
	}
	if err != nil {
		data["Error"] = errorMessage(err)
	} else {
		data["Result"] = resp
	}
	id, err := w.actionResults.put("nginx", data)
	if err != nil {
		w.renderError(c, http.StatusInternalServerError, "failed to store action result")
		return
	}
	c.Redirect(http.StatusSeeOther, "/ui/nginx/action-results/"+id)
}

func (w *Web) nginxActionResultPage(c *gin.Context) {
	data, ok := w.actionResults.get("nginx", c.Param("id"))
	if !ok {
		w.renderError(c, http.StatusNotFound, "nginx action result not found or expired")
		return
	}
	w.render(c, http.StatusOK, "nginx_action_result.html", data)
}

func (w *Web) siteListPage(c *gin.Context) {
	list, err := w.nginxSvc.ListSites()
	data := gin.H{"Title": "Nginx Sites"}
	if err != nil {
		data["Error"] = errorMessage(err)
	} else {
		data["Sites"] = list.Items
	}
	w.render(c, http.StatusOK, "sites.html", data)
}

func (w *Web) createSite(c *gin.Context) {
	name := c.PostForm("name")
	content := c.PostForm("content")
	validate := c.PostForm("validate") == "on"

	_, err := w.nginxSvc.CreateSite(c.Request.Context(), name, content, validate)
	if err != nil {
		list, listErr := w.nginxSvc.ListSites()
		data := gin.H{
			"Title":       "Nginx Sites",
			"Error":       errorMessage(err),
			"FormName":    name,
			"FormContent": content,
		}
		if listErr == nil {
			data["Sites"] = list.Items
		}
		w.render(c, http.StatusOK, "sites.html", data)
		return
	}
	redirectMsg(c, "/ui/nginx/sites", "site created")
}

func (w *Web) sitePage(c *gin.Context) {
	name := c.Param("name")
	site, err := w.nginxSvc.GetSite(name)
	if err != nil {
		w.renderError(c, http.StatusNotFound, errorMessage(err))
		return
	}
	w.render(c, http.StatusOK, "site.html", gin.H{
		"Title": "Site " + name,
		"Site":  site,
	})
}

func (w *Web) updateSite(c *gin.Context) {
	name := c.Param("name")
	content := c.PostForm("content")
	validate := c.PostForm("validate") == "on"
	expectedSHA := c.PostForm("expected_sha256")

	result, err := w.nginxSvc.UpdateSite(c.Request.Context(), name, content, validate, expectedSHA)
	if err != nil {
		site, getErr := w.nginxSvc.GetSite(name)
		data := gin.H{
			"Title":       "Site " + name,
			"Error":       errorMessage(err),
			"FormContent": content,
		}
		if getErr == nil {
			data["Site"] = site
		}
		w.render(c, http.StatusOK, "site.html", data)
		return
	}
	message := "site updated"
	if validate && result.Validation.OK {
		message = "site updated, validation ok"
	}
	redirectMsg(c, "/ui/nginx/sites/"+name, message)
}

func (w *Web) deleteSitePage(c *gin.Context) {
	name := c.Param("name")
	site, err := w.nginxSvc.GetSite(name)
	if err != nil {
		w.renderError(c, http.StatusNotFound, errorMessage(err))
		return
	}
	w.render(c, http.StatusOK, "site_delete.html", gin.H{
		"Title": "Delete Site " + name,
		"Site":  site,
	})
}

func (w *Web) deleteSite(c *gin.Context) {
	name := c.Param("name")
	if err := w.nginxSvc.DeleteSite(name); err != nil {
		w.renderError(c, http.StatusInternalServerError, errorMessage(err))
		return
	}
	redirectMsg(c, "/ui/nginx/sites", "site deleted")
}
