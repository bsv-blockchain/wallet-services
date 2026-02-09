package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bsv-blockchain/arcade/config"
	"github.com/bsv-blockchain/arcade/logging"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		slog.Error("Failed to load configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log := logging.NewLogger(cfg.GetLogLevel())
	log.Info("Starting Wallet Services")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := run(ctx, cfg, log); err != nil {
		log.Error("Application error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	services, err := cfg.Initialize(ctx, log, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to initialize services: %w", err)
	}
	defer func() {
		if err := services.Close(); err != nil {
			log.Error("Error closing services", slog.String("error", err.Error()))
		}
	}()

	app := setupServer(ctx, services, log)

	address := cfg.Server.Address
	if address == "" {
		address = ":3011"
	}
	log.Info("Starting HTTP server", slog.String("address", address))

	errCh := make(chan error, 1)
	go func() {
		if err := app.Listen(address); err != nil {
			errCh <- fmt.Errorf("server error: %w", err)
		}
	}()

	log.Info("Wallet Services started",
		slog.String("arcade", "/arcade/*"),
		slog.String("chaintracks", "/chaintracks/*"),
	)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigCh:
		log.Info("Received shutdown signal")
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("Context canceled")
	}

	log.Info("Shutting down gracefully")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Error("Error during server shutdown", slog.String("error", err.Error()))
	}

	log.Info("Shutdown complete")
	return nil
}
