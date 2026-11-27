package web

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"time"

	"github.com/KevinZonda/Panel/lib/apperr"
	"github.com/KevinZonda/Panel/lib/authctx"
	"github.com/gin-gonic/gin"
)

//go:embed templates
var templatesFS embed.FS

func loadTemplates() (*template.Template, error) {
	funcs := template.FuncMap{
		"fmtBytes":  fmtBytes,
		"fmtTime":   fmtTime,
		"fmtUptime": fmtUptime,
		"derefStr":  derefStr,
		"derefInt":  derefInt,
		"shortSHA":  shortSHA,
		"seq":       seq,
		"yesNo":     yesNo,
		"fmtNow":    fmtNow,
		"buildInfo": buildInfoSummary,
	}
	return template.New("web").Funcs(funcs).ParseFS(templatesFS, "templates/*.html")
}

func (w *Web) render(c *gin.Context, status int, page string, data gin.H) {
	if data == nil {
		data = gin.H{}
	}
	if _, ok := data["User"]; !ok {
		data["User"] = authctx.MustUser(c)
	}
	if _, ok := data["Message"]; !ok {
		if msg := c.Query("msg"); msg != "" {
			data["Message"] = msg
		}
	}
	data["CSRFToken"] = csrfTokenFromCookie(c)
	data["TerminalEnabled"] = w.terminal != nil
	c.HTML(status, page, data)
}

func (w *Web) renderError(c *gin.Context, status int, message string) {
	w.render(c, status, "error.html", gin.H{
		"Title": fmt.Sprintf("Error %d", status),
		"Error": message,
	})
}

func redirectMsg(c *gin.Context, path, msg string) {
	c.Redirect(http.StatusSeeOther, path+"?msg="+url.QueryEscape(msg))
}

func errorHTTPStatus(err error) int {
	switch {
	case errors.Is(err, apperr.ErrValidation):
		return http.StatusBadRequest
	case errors.Is(err, apperr.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, apperr.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, apperr.ErrForbidden):
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

func errorMessage(err error) string {
	switch {
	case errors.Is(err, apperr.ErrInvalidCredentials):
		return "invalid username or password"
	case errors.Is(err, apperr.ErrUserDisabled):
		return "user is disabled"
	case errors.Is(err, apperr.ErrValidation),
		errors.Is(err, apperr.ErrNotFound),
		errors.Is(err, apperr.ErrConflict),
		errors.Is(err, apperr.ErrForbidden),
		errors.Is(err, apperr.ErrDependency):
		return err.Error()
	default:
		return "internal server error"
	}
}

func fmtBytes(v uint64) string {
	const unit = 1024
	if v < unit {
		return fmt.Sprintf("%d B", v)
	}
	div, exp := uint64(unit), 0
	for n := v / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(v)/float64(div), "KMGTPE"[exp])
}

func fmtTime(v any) string {
	var t time.Time
	switch tv := v.(type) {
	case time.Time:
		t = tv
	case *time.Time:
		if tv == nil {
			return "-"
		}
		t = *tv
	default:
		return "-"
	}
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func fmtNow() string {
	return fmtTime(time.Now())
}

func fmtUptime(seconds uint64) string {
	d := time.Duration(seconds) * time.Second
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func derefStr(v *string) string {
	if v == nil {
		return "-"
	}
	return *v
}

func derefInt(v *int) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *v)
}

func shortSHA(v string) string {
	if len(v) > 12 {
		return v[:12]
	}
	return v
}

// seq returns [0, n) for templates that need to repeat a block n times.
func seq(n int) []int {
	if n < 0 {
		n = 0
	}
	items := make([]int, n)
	for i := range items {
		items[i] = i
	}
	return items
}

// yesNo renders a boolean health state as colored text using pure HTML
// presentational markup: green yes when true, red no when false.
func yesNo(ok bool) template.HTML {
	if ok {
		return template.HTML(`<font color="green">yes</font>`)
	}
	return template.HTML(`<font color="red">no</font>`)
}
