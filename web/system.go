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
	hostInfo, hostErr := w.systemSvc.Host(c.Request.Context())
	cpuStatus, cpuErr := w.systemSvc.CPU(c.Request.Context(), 500*time.Millisecond)
	memStatus, memErr := w.systemSvc.Memory(c.Request.Context())

	data := gin.H{"Title": "System"}
	if hostErr != nil {
		data["HostError"] = errorMessage(hostErr)
	} else {
		data["Host"] = hostInfo
	}
	if cpuErr != nil {
		data["CPUError"] = errorMessage(cpuErr)
	} else {
		data["CPU"] = cpuStatus
		if len(cpuStatus.PerCorePercent) > 0 {
			rows, cols := chunkCores(cpuStatus.PerCorePercent)
			data["CoreRows"] = rows
			data["CoreCols"] = cols
		}
	}
	if memErr != nil {
		data["MemoryError"] = errorMessage(memErr)
	} else {
		data["Memory"] = memStatus
	}
	w.render(c, http.StatusOK, "system.html", data)
}

type coreCell struct {
	Index   int
	Percent float64
}

// chunkCores arranges per-core usage into table rows. Column count scales
// with core count so the table never grows beyond roughly 8 rows:
// up to 32 cores use 4 columns, up to 64 use 8, beyond that 16.
func chunkCores(perCore []float64) (rows [][]coreCell, cols int) {
	cols = 4
	switch {
	case len(perCore) > 64:
		cols = 16
	case len(perCore) > 32:
		cols = 8
	}

	for i := 0; i < len(perCore); i += cols {
		end := i + cols
		if end > len(perCore) {
			end = len(perCore)
		}
		row := make([]coreCell, 0, cols)
		for j := i; j < end; j++ {
			row = append(row, coreCell{Index: j, Percent: perCore[j]})
		}
		rows = append(rows, row)
	}
	return rows, cols
}
