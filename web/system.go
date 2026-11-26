package web

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (w *Web) dashboard(c *gin.Context) {
	status, sysErr := w.systemSvc.Status(c.Request.Context())
	nginxStatus, nginxErr := w.nginxSvc.Status(c.Request.Context())

	data := gin.H{"Title": "Dashboard"}
	if sysErr != nil {
		data["SystemError"] = errorMessage(sysErr)
	} else {
		data["System"] = status
	}
	if nginxErr != nil {
		data["NginxError"] = errorMessage(nginxErr)
	} else {
		data["Nginx"] = nginxStatus
	}
	w.render(c, http.StatusOK, "dashboard.html", data)
}

func (w *Web) systemPage(c *gin.Context) {
	cpuStatus, cpuErr := w.systemSvc.CPU(c.Request.Context(), 500*time.Millisecond)
	memStatus, memErr := w.systemSvc.Memory(c.Request.Context())

	data := gin.H{"Title": "System"}
	if cpuErr != nil {
		data["CPUError"] = errorMessage(cpuErr)
	} else {
		data["CPU"] = cpuStatus
	}
	if memErr != nil {
		data["MemoryError"] = errorMessage(memErr)
	} else {
		data["Memory"] = memStatus
	}
	w.render(c, http.StatusOK, "system.html", data)
}
