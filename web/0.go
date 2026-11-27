package web

import (
	"fmt"
	"net/http"

	authsvc "github.com/KevinZonda/Panel/domain/auth/svc"
	certsvc "github.com/KevinZonda/Panel/domain/cert/svc"
	frpsvc "github.com/KevinZonda/Panel/domain/frp/svc"
	nginxsvc "github.com/KevinZonda/Panel/domain/nginx/svc"
	systemsvc "github.com/KevinZonda/Panel/domain/system/svc"
	terminalsvc "github.com/KevinZonda/Panel/domain/terminal/svc"
	usersvc "github.com/KevinZonda/Panel/domain/user/svc"
	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	ginlimiter "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

type Web struct {
	authSvc       *authsvc.Service
	userSvc       *usersvc.Service
	systemSvc     *systemsvc.Service
	nginxSvc      *nginxsvc.Service
	certSvc       *certsvc.Service
	frpSvc        *frpsvc.Service
	terminal      *terminalsvc.Service
	actionResults *actionResultStore
}

func New(
	authSvc *authsvc.Service,
	userSvc *usersvc.Service,
	systemSvc *systemsvc.Service,
	nginxSvc *nginxsvc.Service,
	certSvc *certsvc.Service,
	frpSvc *frpsvc.Service,
	terminal *terminalsvc.Service,
) *Web {
	return &Web{
		authSvc:       authSvc,
		userSvc:       userSvc,
		systemSvc:     systemSvc,
		nginxSvc:      nginxSvc,
		certSvc:       certSvc,
		frpSvc:        frpSvc,
		terminal:      terminal,
		actionResults: newActionResultStore(),
	}
}

func (w *Web) InitRoutes(engine *gin.Engine) error {
	if w.actionResults == nil {
		w.actionResults = newActionResultStore()
	}
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
	ui.Use(csrfCheck())

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
	if w.terminal != nil {
		if err := w.initTerminalRoutes(ui, authn, admin); err != nil {
			return err
		}
	}

	ui.GET("/nginx", authn, w.nginxPage)
	ui.GET("/nginx/actions/:action", authn, admin, w.nginxActionConfirmPage)
	ui.POST("/nginx/actions", authn, admin, w.nginxAction)
	ui.GET("/nginx/action-results/:id", authn, admin, w.nginxActionResultPage)
	ui.POST("/nginx/test", authn, w.nginxTestConfig)

	ui.GET("/nginx/sites", authn, w.siteListPage)
	ui.POST("/nginx/sites", authn, admin, w.createSite)
	ui.GET("/nginx/sites/:name", authn, w.sitePage)
	ui.POST("/nginx/sites/:name", authn, admin, w.updateSite)
	ui.GET("/nginx/sites/:name/delete", authn, admin, w.deleteSitePage)
	ui.POST("/nginx/sites/:name/delete", authn, admin, w.deleteSite)

	ui.GET("/frp", authn, w.frpPage)
	ui.POST("/frp/configs", authn, admin, w.createFrpConfig)
	ui.GET("/frp/configs/:type/:name", authn, admin, w.frpConfigPage)
	ui.POST("/frp/configs/:type/:name", authn, admin, w.updateFrpConfig)
	ui.GET("/frp/configs/:type/:name/delete", authn, admin, w.deleteFrpConfigPage)
	ui.POST("/frp/configs/:type/:name/delete", authn, admin, w.deleteFrpConfig)
	ui.GET("/frp/services/:name/actions/:action", authn, admin, w.frpServiceActionConfirmPage)
	ui.POST("/frp/services/:name/actions", authn, admin, w.frpServiceAction)
	ui.GET("/frp/action-results/:id", authn, admin, w.frpServiceActionResultPage)

	ui.GET("/certs", authn, w.certListPage)
	ui.GET("/certs/public", authn, admin, w.uploadPublicPage)
	ui.POST("/certs/public", authn, admin, w.uploadPublic)
	ui.GET("/certs/selfsign", authn, admin, w.generateSelfSignPage)
	ui.POST("/certs/selfsign", authn, admin, w.generateSelfSign)
	// gin's router panics when a static segment and a wildcard share a
	// position, so certificate types are registered as literal segments
	// instead of a :type parameter; the type is passed through context.
	for _, certType := range []string{"public", "selfsign"} {
		certType := certType
		withType := func(h gin.HandlerFunc) gin.HandlerFunc {
			return func(c *gin.Context) {
				c.Set("certType", certType)
				h(c)
			}
		}
		ui.GET("/certs/"+certType+"/:name", authn, withType(w.certPage))
		ui.POST("/certs/"+certType+"/:name/files/:file", authn, admin, withType(w.uploadCertFile))
		ui.GET("/certs/"+certType+"/:name/delete", authn, admin, withType(w.deleteCertPage))
		ui.POST("/certs/"+certType+"/:name/delete", authn, admin, withType(w.deleteCert))
	}

	ui.GET("/users", authn, admin, w.userListPage)
	ui.POST("/users", authn, admin, w.createUser)
	ui.GET("/users/:id", authn, admin, w.userPage)
	ui.POST("/users/:id", authn, admin, w.updateUser)
	ui.POST("/users/:id/reset-password", authn, admin, w.resetUserPassword)
	ui.GET("/users/:id/delete", authn, admin, w.deleteUserPage)
	ui.POST("/users/:id/delete", authn, admin, w.deleteUser)
	return nil
}
