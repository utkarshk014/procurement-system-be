package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/utkarshk014/procurement-system-be/src/delivery/rest"
	"github.com/utkarshk014/procurement-system-be/src/internal/repository"
	"github.com/utkarshk014/procurement-system-be/src/internal/usecase"
	"github.com/utkarshk014/procurement-system-be/src/packages/database"
	"github.com/utkarshk014/procurement-system-be/src/packages/logger"
	"github.com/utkarshk014/procurement-system-be/src/utils"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := utils.NewConfig()
	if err != nil {
		return err
	}
	log := logger.New(cfg.AppEnv, cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbm, err := database.NewManager(ctx, cfg.ControlDatabaseURL, cfg.TenantDatabases)
	if err != nil {
		return err
	}
	defer dbm.Close()

	repos := repository.NewRepositories(dbm)
	usecases := usecase.NewUsecases(cfg, repos, dbm, log)
	server := rest.NewRestDelivery(cfg, usecases, dbm.Control(), log)

	go cleanupRefreshTokens(ctx, repos, log)

	errCh := make(chan error, 1)
	go func() { errCh <- server.Start() }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

// cleanupRefreshTokens deletes expired refresh tokens once an hour.
func cleanupRefreshTokens(ctx context.Context, repos *repository.Repositories, log *slog.Logger) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := repos.Auth.DeleteExpiredRefreshTokens(ctx)
			if err != nil {
				log.Warn("refresh token cleanup failed", "error", err)
				continue
			}
			if n > 0 {
				log.Info("deleted expired refresh tokens", "count", n)
			}
		}
	}
}
