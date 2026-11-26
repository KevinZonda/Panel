package nginx

import "github.com/gin-gonic/gin"

func (d *Domain) InitHttp(api *gin.RouterGroup, authn gin.HandlerFunc, admin gin.HandlerFunc) error {
	api.GET("/nginx/status", authn, d.handler.Status)
	api.POST("/nginx/actions", authn, admin, d.handler.Action)
	api.POST("/nginx/config/test", authn, d.handler.TestConfig)

	api.GET("/nginx/sites", authn, d.handler.ListSites)
	api.POST("/nginx/sites", authn, admin, d.handler.CreateSite)
	api.GET("/nginx/sites/:name", authn, d.handler.GetSite)
	api.PUT("/nginx/sites/:name", authn, admin, d.handler.UpdateSite)
	api.DELETE("/nginx/sites/:name", authn, admin, d.handler.DeleteSite)
	return nil
}
