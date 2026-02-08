package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/sagerenn/mdict/internal/config"
	"github.com/sagerenn/mdict/internal/dict/loader"
	"github.com/sagerenn/mdict/internal/dict/registry"
	"github.com/sagerenn/mdict/internal/httpx"
	"github.com/sagerenn/mdict/internal/observability"
	"github.com/sagerenn/mdict/internal/service"
)

func main() {
	cfgPath := flag.String("config", "./configs/gdapi.json", "path to JSON config")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fatal("config", err)
	}

	log := observability.New(cfg.Log.Level)
	loadRes := loader.LoadAll(cfg)
	if len(loadRes.Errs) > 0 {
		for _, e := range loadRes.Errs {
			log.Error("dictionary load error", "error", e)
		}
	}

	reg := registry.New()
	if err := reg.MustAddAll(loadRes.Dicts); err != nil {
		fatal("registry", err)
	}

	svc := service.New(reg)
	h := httpx.NewRouter(svc, log, cfg.URLBasePath)

	srv := &http.Server{
		Addr:         cfg.Listen,
		Handler:      h,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	go func() {
		log.Info("server listening", "addr", cfg.Listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	<-shutdown

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Info("server stopped")
}

func fatal(stage string, err error) {
	_, _ = os.Stderr.WriteString(stage + ": " + err.Error() + "\n")
	os.Exit(1)
}
