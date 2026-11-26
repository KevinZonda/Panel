package user

import "github.com/gin-gonic/gin"

func (d *Domain) InitHttp(api *gin.RouterGroup, authn gin.HandlerFunc, admin gin.HandlerFunc) error {
	api.GET("/users", authn, admin, d.handler.List)
	api.POST("/users", authn, admin, d.handler.Create)
	api.GET("/users/:id", authn, admin, d.handler.Get)
	api.PATCH("/users/:id", authn, admin, d.handler.Update)
	api.DELETE("/users/:id", authn, admin, d.handler.Delete)
	api.POST("/users/:id/reset-password", authn, admin, d.handler.ResetPassword)
	return nil
}
