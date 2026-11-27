package frp

import "github.com/gin-gonic/gin"

func (d *Domain) InitHttp(api *gin.RouterGroup, authn gin.HandlerFunc, admin gin.HandlerFunc) error {
	api.GET("/frp/services", authn, d.handler.ListServices)
	api.POST("/frp/services/:name/actions", authn, admin, d.handler.ServiceAction)

	api.GET("/frp/configs", authn, admin, d.handler.ListConfigs)
	api.POST("/frp/configs/:type", authn, admin, d.handler.CreateConfig)
	api.GET("/frp/configs/:type/:name", authn, admin, d.handler.GetConfig)
	api.PUT("/frp/configs/:type/:name", authn, admin, d.handler.UpdateConfig)
	api.DELETE("/frp/configs/:type/:name", authn, admin, d.handler.DeleteConfig)
	return nil
}
