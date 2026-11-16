package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/mlf-erp/mlf-erp/internal/modules/purchase/app"
	purchasehttp "github.com/mlf-erp/mlf-erp/internal/modules/purchase/http"
	"github.com/mlf-erp/mlf-erp/internal/modules/purchase/infra"
	"github.com/mlf-erp/mlf-erp/internal/platform/config"
	"github.com/mlf-erp/mlf-erp/internal/platform/database"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpserver"
	"github.com/mlf-erp/mlf-erp/internal/platform/logger"
)

var version = "dev"

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Log.Level)
	log.Info("starting api",
		"app", cfg.App.Name,
		"env", cfg.App.Env,
		"version", version,
		"delivery_mode", cfg.App.DeliveryMode,
	)

	var db *pgxpool.Pool
	if cfg.DB.Enabled {
		connectCtx, cancel := context.WithTimeout(context.Background(), cfg.DB.ConnectTimeout)
		pool, connectErr := database.NewPostgres(connectCtx, cfg.DB)
		cancel()

		if connectErr != nil {
			if cfg.DB.Required {
				log.Error("connect database", "error", connectErr)
				os.Exit(1)
			}
			log.Warn("database unavailable, starting in degraded mode", "error", connectErr)
		} else {
			db = pool
			defer db.Close()
		}
	}

	engine := httpserver.New(cfg, log, db)

	purchaseRepository := infra.NewMemoryRepository()
	purchaseService := app.NewService(purchaseRepository)
	purchasehttp.Register(engine.Group("/api/v1"), purchaseService, log)

	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("http server listening", "addr", cfg.HTTP.Addr)
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Error("http server failed", "error", serveErr)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down api")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
	}

	log.Info("api stopped")
}
