package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/KevinZonda/Panel/domain"
	terminalsvc "github.com/KevinZonda/Panel/domain/terminal/svc"
	"github.com/KevinZonda/Panel/shared"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			if err := runInit(os.Args[2:], os.Stdin, os.Stdout); err != nil {
				logrus.WithError(err).Fatal("panel init failed")
			}
		case "-h", "--help":
			fmt.Fprintln(os.Stdout, "Usage: panel [init]\nRun panel init --help for setup options.")
		default:
			logrus.Fatalf("unknown command %q, see panel --help", os.Args[1])
		}
		return
	}

	if err := shared.InitCfg(); err != nil {
		logrus.WithError(err).Fatal("init config failed")
	}
	if err := shared.InitSvcs(); err != nil {
		logrus.WithError(err).Fatal("init services failed")
	}

	cfg, err := shared.GetConfig()
	if err != nil {
		logrus.WithError(err).Fatal("get config failed")
	}
	db, err := shared.GetDB()
	if err != nil {
		logrus.WithError(err).Fatal("get database failed")
	}
	engine, err := shared.GetGinEngine()
	if err != nil {
		logrus.WithError(err).Fatal("get gin engine failed")
	}

	var terminal *terminalsvc.Service
	if cfg.TerminalEnabled {
		terminal = terminalsvc.New(cfg.TerminalLoginAs)
	}
	if err := domain.InitRoute(engine, db, cfg, terminal); err != nil {
		logrus.WithError(err).Fatal("init routes failed")
	}

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logrus.WithField("addr", cfg.ListenAddr).Info("panel server started")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logrus.WithError(err).Fatal("panel server failed")
		}
	}()

	stop, stopNotify := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopNotify()
	<-stop.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if terminal != nil {
		if err := terminal.Shutdown(shutdownCtx); err != nil {
			logrus.WithError(err).Error("shutdown terminal sessions failed")
		}
	}
	if err := server.Shutdown(shutdownCtx); err != nil {
		logrus.WithError(err).Error("shutdown server failed")
	}
	if err := shared.CloseDB(); err != nil {
		logrus.WithError(err).Error("close database failed")
	}
	logrus.Info("panel server stopped")
}
