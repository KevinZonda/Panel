package web

import (
	"fmt"
	"net/http"

	authsvc "github.com/KevinZonda/Panel/domain/auth/svc"
	nginxsvc "github.com/KevinZonda/Panel/domain/nginx/svc"
	systemsvc "github.com/KevinZonda/Panel/domain/system/svc"
	usersvc "github.com/KevinZonda/Panel/domain/user/svc"
	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	ginlimiter "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

type Web struct {
	authSvc   *authsvc.Service
	userSvc   *usersvc.Service
	systemSvc *systemsvc.Service
	nginxSvc  *nginxsvc.Service
}

func New(
	authSvc *authsvc.Service,
	userSvc *usersvc.Service,
	systemSvc *systemsvc.Service,
	nginxSvc *nginxsvc.Service,
) *Web {
	return &Web{
		authSvc:   authSvc,
		userSvc:   userSvc,
		systemSvc: systemSvc,
		nginxSvc:  nginxSvc,
	}
}

func (w *Web) InitRoutes(engine *gin.Engine) error {
	tmpl, err := loadTemplates()
	if err != nil {
		return fmt.Errorf("load web templates: %w", err)
	}
	engine.SetHTMLTemplate(tmpl)

	rate, err := limiter.NewRateFromFormatted("10-M")
	if err != nil {
		return err
	}
	loginLimiter := ginlimiter.NewMiddleware(
		limiter.New(memory.NewStore(), rate),
		ginlimiter.WithLimitReachedHandler(func(c *gin.Context) {
			w.renderError(c, http.StatusTooManyRequests, "too many login attempts, try again later")
		}),
	)

	engine.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/ui")
	})

	ui := engine.Group("/ui")
	ui.Use(originCheck())

	ui.GET("/login", w.loginPage)
	ui.POST("/login", loginLimiter, w.login)

	authn := w.authRequired()
	admin := w.adminRequired()

	ui.GET("/", authn, w.dashboard)
	ui.GET("/logout", authn, w.logoutPage)
	ui.POST("/logout", authn, w.logout)
	ui.GET("/password", authn, w.passwordPage)
	ui.POST("/password", authn, w.changePassword)

	ui.GET("/system", authn, w.systemPage)

	ui.GET("/nginx", authn, w.nginxPage)
	ui.POST("/nginx/actions", authn, admin, w.nginxAction)

	ui.GET("/nginx/sites", authn, w.siteListPage)
	ui.POST("/nginx/sites", authn, admin, w.createSite)
	ui.GET("/nginx/sites/:name", authn, w.sitePage)
	ui.POST("/nginx/sites/:name", authn, admin, w.updateSite)
	ui.GET("/nginx/sites/:name/delete", authn, admin, w.deleteSitePage)
	ui.POST("/nginx/sites/:name/delete", authn, admin, w.deleteSite)

	ui.GET("/users", authn, admin, w.userListPage)
	ui.POST("/users", authn, admin, w.createUser)
	ui.GET("/users/:id", authn, admin, w.userPage)
	ui.POST("/users/:id", authn, admin, w.updateUser)
	ui.POST("/users/:id/reset-password", authn, admin, w.resetUserPassword)
	ui.GET("/users/:id/delete", authn, admin, w.deleteUserPage)
	ui.POST("/users/:id/delete", authn, admin, w.deleteUser)
	return nil
}
