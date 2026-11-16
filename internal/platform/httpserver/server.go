package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mlf-erp/mlf-erp/internal/platform/config"
	"github.com/mlf-erp/mlf-erp/internal/platform/entitlement"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpx"
)

func New(cfg *config.Config, log *slog.Logger, db *pgxpool.Pool) *gin.Engine {
	if cfg.App.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	if len(cfg.HTTP.TrustedProxies) == 0 {
		_ = router.SetTrustedProxies(nil)
	} else if err := router.SetTrustedProxies(cfg.HTTP.TrustedProxies); err != nil {
		log.Warn("invalid trusted proxies, disabling proxy trust", "error", err)
		_ = router.SetTrustedProxies(nil)
	}
	router.Use(gin.Recovery())
	router.Use(httpmw.RequestID())
	router.Use(httpmw.Logger(log))
	router.Use(httpmw.CORS(cfg.HTTP.CORSAllowedOrigins))

	provider := entitlement.NewStatic(entitlement.DefaultCapabilities...)

	router.GET("/healthz", func(c *gin.Context) {
		httpx.OK(c, gin.H{
			"status": "ok",
			"app":    cfg.App.Name,
			"env":    cfg.App.Env,
		})
	})

	router.GET("/readyz", func(c *gin.Context) {
		if db == nil {
			httpx.Error(c, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database is not configured or unavailable")
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			httpx.Error(c, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database ping failed")
			return
		}

		httpx.OK(c, gin.H{"status": "ready"})
	})

	router.GET("/api/v1/runtime-config", func(c *gin.Context) {
		httpx.OK(c, gin.H{
			"app":           cfg.App.Name,
			"env":           cfg.App.Env,
			"delivery_mode": cfg.App.DeliveryMode,
			"capabilities":  provider.List(),
		})
	})

	router.NoRoute(func(c *gin.Context) {
		httpx.Error(c, http.StatusNotFound, httpx.CodeNotFound, "route not found")
	})

	return router
}
