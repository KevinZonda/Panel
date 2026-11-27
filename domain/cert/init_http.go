package cert

import "github.com/gin-gonic/gin"

func (d *Domain) InitHttp(api *gin.RouterGroup, authn gin.HandlerFunc, admin gin.HandlerFunc) error {
	api.GET("/certs", authn, d.handler.List)
	api.GET("/certs/:type/:name", authn, d.handler.Get)
	api.GET("/certs/:type/:name/files/:file", authn, d.handler.ReadFile)
	api.PUT("/certs/:type/:name/files/:file", authn, admin, d.handler.UploadFile)
	api.POST("/certs/public", authn, admin, d.handler.CreatePublic)
	api.POST("/certs/selfsign", authn, admin, d.handler.GenerateSelfSign)
	api.DELETE("/certs/:type/:name", authn, admin, d.handler.Delete)
	return nil
}
