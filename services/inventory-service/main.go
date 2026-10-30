package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"

	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/api"
	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/config"
	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/pricing"
	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/store"
	"github.com/clevertechware/observabilite-agents-demo/services/inventory-service/internal/telemetry"
)

const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}

	shutdownTelemetry, err := telemetry.Setup(ctx)
	if err != nil {
		return err
	}
	defer flushTelemetry(shutdownTelemetry)

	if err = runtime.Start(); err != nil {
		return err
	}

	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	repository := store.NewRepository(pool, cfg.FaultSlowQuerySeconds)
	pricingClient := pricing.NewClient(cfg.PricingURL)
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.NewRouter(repository, pricingClient),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() { serverErr <- server.ListenAndServe() }()

	select {
	case err = <-serverErr:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err = server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func flushTelemetry(shutdown func(context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		log.Printf("telemetry shutdown: %v", err)
	}
}
