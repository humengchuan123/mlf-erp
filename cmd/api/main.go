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

	"github.com/joho/godotenv"
	masterapp "github.com/mlf-erp/mlf-erp/internal/modules/masterdata/app"
	masterdomain "github.com/mlf-erp/mlf-erp/internal/modules/masterdata/domain"
	masterhttp "github.com/mlf-erp/mlf-erp/internal/modules/masterdata/http"
	masterpostgres "github.com/mlf-erp/mlf-erp/internal/modules/masterdata/infra/postgres"
	"github.com/mlf-erp/mlf-erp/internal/modules/purchase/app"
	purchasehttp "github.com/mlf-erp/mlf-erp/internal/modules/purchase/http"
	"github.com/mlf-erp/mlf-erp/internal/modules/purchase/infra"
	systemapp "github.com/mlf-erp/mlf-erp/internal/modules/system/app"
	systemhttp "github.com/mlf-erp/mlf-erp/internal/modules/system/http"
	systempostgres "github.com/mlf-erp/mlf-erp/internal/modules/system/infra/postgres"
	"github.com/mlf-erp/mlf-erp/internal/platform/authmw"
	"github.com/mlf-erp/mlf-erp/internal/platform/config"
	"github.com/mlf-erp/mlf-erp/internal/platform/database"
	"github.com/mlf-erp/mlf-erp/internal/platform/httpserver"
	"github.com/mlf-erp/mlf-erp/internal/platform/logger"
	"github.com/mlf-erp/mlf-erp/internal/platform/security"
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

	if !cfg.DB.Enabled {
		log.Error("database is required for stage 1; set DB_ENABLED=true")
		os.Exit(1)
	}

	connectCtx, cancelConnect := context.WithTimeout(context.Background(), cfg.DB.ConnectTimeout)
	db, connectErr := database.NewPostgres(connectCtx, cfg.DB)
	cancelConnect()
	if connectErr != nil {
		log.Error("connect database", "error", connectErr)
		os.Exit(1)
	}
	defer db.Close()

	engine := httpserver.New(cfg, log, db)

	userRepository := systempostgres.NewUserRepository(db)
	roleRepository := systempostgres.NewRoleRepository(db)
	permissionRepository := systempostgres.NewPermissionRepository(db)
	orgRepository := systempostgres.NewOrgRepository(db)
	refreshTokenRepository := systempostgres.NewRefreshTokenRepository(db)
	auditRepository := systempostgres.NewAuditRepository(db)
	loginLogRepository := systempostgres.NewLoginLogRepository(db)
	codeRuleRepository := systempostgres.NewCodeRuleRepository(db)

	tokenService := security.NewTokenService(cfg.Auth.JWTSecret, cfg.Auth.JWTIssuer, cfg.Auth.AccessTokenTTL)
	authService := systemapp.NewAuthService(
		userRepository,
		roleRepository,
		refreshTokenRepository,
		loginLogRepository,
		tokenService,
		cfg.Auth.RefreshTokenTTL,
	)
	userService := systemapp.NewUserService(userRepository, roleRepository, refreshTokenRepository)
	roleService := systemapp.NewRoleService(roleRepository)
	permissionService := systemapp.NewPermissionService(permissionRepository)
	orgService := systemapp.NewOrgService(orgRepository)
	auditService := systemapp.NewAuditService(auditRepository)
	codeRuleService := systemapp.NewCodeRuleService(codeRuleRepository)

	authHandler := systemhttp.NewAuthHandler(authService, cfg.App.DefaultTenantID, log)
	userHandler := systemhttp.NewUserHandler(userService, auditService, log)
	roleHandler := systemhttp.NewRoleHandler(roleService, permissionService, auditService, log)
	orgHandler := systemhttp.NewOrgHandler(orgService, auditService, log)
	auditHandler := systemhttp.NewAuditHandler(auditService, log)
	codeRuleHandler := systemhttp.NewCodeRuleHandler(codeRuleService, auditService, log)

	partnerRepository := masterpostgres.NewRepository[masterdomain.Partner](db)
	materialRepository := masterpostgres.NewRepository[masterdomain.Material](db)
	warehouseRepository := masterpostgres.NewRepository[masterdomain.Warehouse](db)
	unitRepository := masterpostgres.NewRepository[masterdomain.Unit](db)
	currencyRepository := masterpostgres.NewRepository[masterdomain.Currency](db)
	taxRateRepository := masterpostgres.NewRepository[masterdomain.TaxRate](db)

	partnerService := masterapp.NewPartnerService(partnerRepository, codeRuleService)
	materialService := masterapp.NewMaterialService(materialRepository, unitRepository, taxRateRepository, codeRuleService)
	warehouseService := masterapp.NewWarehouseService(warehouseRepository, codeRuleService)
	unitService := masterapp.NewUnitService(unitRepository)
	currencyService := masterapp.NewCurrencyService(currencyRepository)
	taxRateService := masterapp.NewTaxRateService(taxRateRepository)

	partnerHandler := masterhttp.NewPartnerHandler(partnerService, auditService, log)
	materialHandler := masterhttp.NewMaterialHandler(materialService, auditService, log)
	warehouseHandler := masterhttp.NewWarehouseHandler(warehouseService, auditService, log)
	unitHandler := masterhttp.NewUnitHandler(unitService, auditService, log)
	currencyHandler := masterhttp.NewCurrencyHandler(currencyService, auditService, log)
	taxRateHandler := masterhttp.NewTaxRateHandler(taxRateService, auditService, log)

	api := engine.Group("/api/v1")
	authHandler.RegisterPublic(api)

	protected := api.Group("", authmw.Auth(tokenService, userRepository, roleRepository, orgRepository))
	authHandler.RegisterProtected(protected)
	userHandler.Register(protected)
	roleHandler.Register(protected)
	orgHandler.Register(protected)
	auditHandler.Register(protected)
	codeRuleHandler.Register(protected)
	partnerHandler.Register(protected)
	materialHandler.Register(protected)
	warehouseHandler.Register(protected)
	unitHandler.Register(protected)
	currencyHandler.Register(protected)
	taxRateHandler.Register(protected)

	purchaseRepository := infra.NewMemoryRepository()
	purchaseService := app.NewService(purchaseRepository)
	purchasehttp.Register(protected, purchaseService, log)

	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
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
