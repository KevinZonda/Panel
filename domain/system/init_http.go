package system

import "github.com/gin-gonic/gin"

func (d *Domain) InitHttp(api *gin.RouterGroup, authn gin.HandlerFunc) error {
	api.GET("/system/status", authn, d.handler.Status)
	api.GET("/system/cpu", authn, d.handler.CPU)
	api.GET("/system/memory", authn, d.handler.Memory)
	return nil
}
