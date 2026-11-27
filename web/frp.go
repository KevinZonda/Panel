package web

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (w *Web) frpPage(c *gin.Context) {
	data := gin.H{"Title": "frpc/frps Management"}
	w.fillFrpPageData(c, data)
	w.render(c, http.StatusOK, "frp.html", data)
}

func (w *Web) fillFrpPageData(c *gin.Context, data gin.H) {
	services, err := w.frpSvc.ListServices(c.Request.Context())
	if err != nil {
		data["ServiceError"] = errorMessage(err)
	} else {
		data["Services"] = services.Items
	}
	if isAdmin(c) {
		configs, err := w.frpSvc.ListConfigs()
		if err != nil {
			data["ConfigError"] = errorMessage(err)
		} else {
			data["Configs"] = configs.Items
		}
	}
}

func (w *Web) createFrpConfig(c *gin.Context) {
	configType := strings.TrimSpace(c.PostForm("type"))
	name := strings.TrimSpace(c.PostForm("name"))
	content := c.PostForm("content")
	validate := c.PostForm("validate") == "on"

	if _, err := w.frpSvc.CreateConfig(c.Request.Context(), configType, name, content, validate); err != nil {
		data := gin.H{
			"Title":       "frpc/frps Management",
			"Error":       errorMessage(err),
			"FormType":    configType,
			"FormName":    name,
			"FormContent": content,
		}
		w.fillFrpPageData(c, data)
		w.render(c, http.StatusOK, "frp.html", data)
		return
	}
	redirectMsg(c, "/ui/frp", "frp config created")
}

func (w *Web) frpConfigPage(c *gin.Context) {
	configType := c.Param("type")
	name := c.Param("name")
	config, err := w.frpSvc.GetConfig(configType, name)
	if err != nil {
		w.renderError(c, http.StatusNotFound, errorMessage(err))
		return
	}
	w.render(c, http.StatusOK, "frp_config.html", gin.H{
		"Title":   "frp Config " + name,
		"Config":  config,
		"Content": config.Content,
	})
}

func (w *Web) updateFrpConfig(c *gin.Context) {
	configType := c.Param("type")
	name := c.Param("name")
	content := c.PostForm("content")
	validate := c.PostForm("validate") == "on"
	expectedSHA := c.PostForm("expected_sha256")

	_, err := w.frpSvc.UpdateConfig(c.Request.Context(), configType, name, content, validate, expectedSHA)
	if err != nil {
		data := gin.H{
			"Title":   "frp Config " + name,
			"Error":   errorMessage(err),
			"Content": content,
		}
		if config, getErr := w.frpSvc.GetConfig(configType, name); getErr == nil {
			data["Config"] = config
		}
		w.render(c, http.StatusOK, "frp_config.html", data)
		return
	}
	redirectMsg(c, "/ui/frp/configs/"+configType+"/"+name, "frp config updated")
}

func (w *Web) deleteFrpConfigPage(c *gin.Context) {
	configType := c.Param("type")
	name := c.Param("name")
	config, err := w.frpSvc.GetConfig(configType, name)
	if err != nil {
		w.renderError(c, http.StatusNotFound, errorMessage(err))
		return
	}
	w.render(c, http.StatusOK, "frp_config_delete.html", gin.H{
		"Title":  "Delete frp Config " + name,
		"Config": config,
	})
}

func (w *Web) deleteFrpConfig(c *gin.Context) {
	configType := c.Param("type")
	name := c.Param("name")
	if err := w.frpSvc.DeleteConfig(configType, name); err != nil {
		w.renderError(c, http.StatusInternalServerError, errorMessage(err))
		return
	}
	redirectMsg(c, "/ui/frp", "frp config deleted")
}

func (w *Web) frpServiceActionConfirmPage(c *gin.Context) {
	name := c.Param("name")
	action := c.Param("action")
	if action != "start" && action != "stop" && action != "restart" {
		w.renderError(c, http.StatusNotFound, "unknown action")
		return
	}
	w.render(c, http.StatusOK, "frp_action_confirm.html", gin.H{
		"Title":   "Confirm frp service " + action,
		"Service": name,
		"Action":  action,
	})
}

func (w *Web) frpServiceAction(c *gin.Context) {
	name := c.Param("name")
	action := c.PostForm("action")
	resp, err := w.frpSvc.ServiceAction(c.Request.Context(), name, action)
	if err != nil {
		w.render(c, http.StatusOK, "frp_action_result.html", gin.H{
			"Title":   "frp Service Action",
			"Service": name,
			"Action":  action,
			"Error":   errorMessage(err),
		})
		return
	}
	w.render(c, http.StatusOK, "frp_action_result.html", gin.H{
		"Title":  "frp Service Action",
		"Result": resp,
	})
}
