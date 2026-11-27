package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	terminalsvc "github.com/KevinZonda/Panel/domain/terminal/svc"
	"github.com/KevinZonda/Panel/lib/authctx"
	"github.com/KevinZonda/Panel/models"
	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const (
	terminalIOTimeout = 5 * time.Second
	terminalIdleLimit = 15 * time.Minute
)

//go:embed terminalassets
var terminalAssets embed.FS

func (w *Web) initTerminalRoutes(ui *gin.RouterGroup, authn, admin gin.HandlerFunc) error {
	assets, err := fs.Sub(terminalAssets, "terminalassets")
	if err != nil {
		return fmt.Errorf("load terminal assets: %w", err)
	}
	ui.GET("/terminal", authn, admin, w.terminalPage)
	ui.GET("/terminal/ws", authn, admin, w.terminalSocket)
	ui.StaticFS("/terminal/assets", http.FS(assets))
	return nil
}

func (w *Web) terminalPage(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
	w.render(c, http.StatusOK, "terminal.html", gin.H{
		"Title":        "Terminal",
		"TerminalPage": true,
		"LoginAs":      w.terminal.AccountName(),
	})
}

func (w *Web) terminalSocket(c *gin.Context) {
	if !terminalOriginAllowed(c.Request) {
		c.String(http.StatusForbidden, "terminal requires a same-origin HTTPS connection")
		return
	}
	user := authctx.MustUser(c)
	session := authctx.MustSession(c)
	token, _ := c.Cookie(cookieName)
	// The WebSocket outlives the HTTP request after the connection is hijacked.
	parent, cancel := context.WithDeadline(context.Background(), session.ExpiresAt)
	defer cancel()
	ctx, release, err := w.terminal.Reserve(parent, session.ID)
	if err != nil {
		c.String(http.StatusServiceUnavailable, "terminal unavailable")
		return
	}
	defer release()

	conn, err := websocket.Accept(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(64 * 1024)

	initCtx, cancelInit := context.WithTimeout(ctx, terminalIOTimeout)
	messageType, data, err := conn.Read(initCtx)
	cancelInit()
	if err != nil {
		return
	}
	size, err := terminalSize(messageType, data)
	if err != nil {
		_ = conn.Close(websocket.StatusInvalidFramePayloadData, "Invalid terminal size")
		return
	}
	if ctx.Err() != nil || !w.terminalAuthorized(token) {
		_ = conn.Close(websocket.StatusPolicyViolation, "Panel session ended")
		return
	}

	audit := logrus.WithFields(logrus.Fields{
		"panel_user":      user.Username,
		"auth_session_id": session.ID,
		"system_user":     w.terminal.AccountName(),
	})
	shell, err := w.terminal.Start(size)
	if err != nil {
		audit.WithError(err).Warn("local terminal failed to start")
		_ = conn.Close(websocket.StatusInternalError, "Could not start Bash. Check terminal.login_as and the Panel log.")
		return
	}
	audit.Info("local terminal opened")
	end := w.runTerminal(ctx, conn, shell, token)
	audit.WithField("reason", end.reason).Info("local terminal closed")
}

func terminalOriginAllowed(req *http.Request) bool {
	origins := req.Header.Values("Origin")
	if len(origins) != 1 {
		return false
	}
	origin, err := url.Parse(origins[0])
	if err != nil || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.ForceQuery ||
		origin.Fragment != "" || origin.Host == "" || !strings.EqualFold(origin.Host, req.Host) {
		return false
	}
	if origin.Scheme == "https" {
		return true
	}
	host := origin.Hostname()
	return origin.Scheme == "http" && req.TLS == nil &&
		(strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback())
}

func terminalSize(messageType websocket.MessageType, data []byte) (terminalsvc.Size, error) {
	var size terminalsvc.Size
	if messageType != websocket.MessageText || len(data) > 128 || json.Unmarshal(data, &size) != nil || !size.Valid() {
		return size, fmt.Errorf("invalid terminal size")
	}
	return size, nil
}

type terminalEnd struct {
	code   websocket.StatusCode
	reason string
}

func (w *Web) runTerminal(ctx context.Context, conn *websocket.Conn, shell *terminalsvc.Session, token string) (end terminalEnd) {
	ioCtx, cancel := context.WithCancel(ctx)
	var pumps sync.WaitGroup
	defer func() {
		shell.Close()
		if ctx.Err() == nil {
			_ = conn.Close(end.code, end.reason)
		}
		cancel()
		_ = conn.CloseNow()
		pumps.Wait()
	}()
	if err := writeTerminalMessage(ioCtx, conn, websocket.MessageText, []byte("ready")); err != nil {
		return terminalEnd{websocket.StatusGoingAway, "Connection ended"}
	}

	var lastInput atomic.Int64
	lastInput.Store(time.Now().Unix())
	ended := make(chan terminalEnd, 2)
	pumps.Go(func() { ended <- readTerminalInput(ioCtx, conn, shell, &lastInput) })
	pumps.Go(func() { ended <- writeTerminalOutput(ioCtx, conn, shell) })

	checks := time.NewTicker(5 * time.Second)
	defer checks.Stop()
	keepAlive := time.NewTicker(25 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-ctx.Done():
			return terminalEnd{websocket.StatusGoingAway, "Panel session ended or server stopped"}
		case end := <-ended:
			return end
		case <-checks.C:
			if !w.terminalAuthorized(token) {
				return terminalEnd{websocket.StatusPolicyViolation, "Panel session ended"}
			}
			if time.Since(time.Unix(lastInput.Load(), 0)) >= terminalIdleLimit {
				return terminalEnd{websocket.StatusPolicyViolation, "Terminal idle timeout"}
			}
		case <-keepAlive.C:
			pingCtx, cancelPing := context.WithTimeout(ioCtx, terminalIOTimeout)
			err := conn.Ping(pingCtx)
			cancelPing()
			if err != nil {
				return terminalEnd{websocket.StatusGoingAway, "Connection ended"}
			}
		}
	}
}

func (w *Web) terminalAuthorized(token string) bool {
	user, _, err := w.authSvc.ValidateToken(token)
	return err == nil && user.Role == models.RoleAdmin
}

func readTerminalInput(ctx context.Context, conn *websocket.Conn, shell *terminalsvc.Session, lastInput *atomic.Int64) terminalEnd {
	for {
		messageType, data, err := conn.Read(ctx)
		if err != nil {
			return terminalEnd{websocket.StatusGoingAway, "Connection ended"}
		}
		if messageType == websocket.MessageBinary {
			lastInput.Store(time.Now().Unix())
			if _, err := shell.Write(data); err != nil {
				return terminalEnd{websocket.StatusNormalClosure, "Shell exited"}
			}
			continue
		}
		size, err := terminalSize(messageType, data)
		if err != nil {
			return terminalEnd{websocket.StatusInvalidFramePayloadData, "Invalid terminal size"}
		}
		if err := shell.Resize(size); err != nil {
			return terminalEnd{websocket.StatusInternalError, "Could not resize terminal"}
		}
	}
}

func writeTerminalOutput(ctx context.Context, conn *websocket.Conn, shell *terminalsvc.Session) terminalEnd {
	buffer := make([]byte, 16*1024)
	for {
		n, err := shell.Read(buffer)
		if n > 0 && writeTerminalMessage(ctx, conn, websocket.MessageBinary, buffer[:n]) != nil {
			return terminalEnd{websocket.StatusGoingAway, "Connection ended"}
		}
		if err != nil {
			return terminalEnd{websocket.StatusNormalClosure, "Shell exited"}
		}
	}
}

func writeTerminalMessage(ctx context.Context, conn *websocket.Conn, messageType websocket.MessageType, data []byte) error {
	writeCtx, cancel := context.WithTimeout(ctx, terminalIOTimeout)
	defer cancel()
	return conn.Write(writeCtx, messageType, data)
}
