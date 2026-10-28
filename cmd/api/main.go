package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/deepsi/agent-service/internal/app"
	"github.com/deepsi/agent-service/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	handlerOptions := &slog.HandlerOptions{Level: cfg.App.LogLevel}
	var handler slog.Handler
	if cfg.IsProduction() {
		handler = slog.NewJSONHandler(os.Stdout, handlerOptions)
	} else {
		handler = slog.NewTextHandler(os.Stdout, handlerOptions)
	}
	log := slog.New(handler)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, cfg, log)
	if err != nil {
		log.Error("initialize application", "error", err)
		os.Exit(1)
	}
	defer application.Close(context.Background())

	if err := application.Run(ctx); err != nil {
		log.Error("run application", "error", err)
		os.Exit(1)
	}
	log.Info("server stopped")
}
