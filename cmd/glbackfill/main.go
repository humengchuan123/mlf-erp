package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
	glpostgres "github.com/mlf-erp/mlf-erp/internal/modules/gl/infra/postgres"
	systemapp "github.com/mlf-erp/mlf-erp/internal/modules/system/app"
	systempostgres "github.com/mlf-erp/mlf-erp/internal/modules/system/infra/postgres"
	"github.com/mlf-erp/mlf-erp/internal/platform/config"
	"github.com/mlf-erp/mlf-erp/internal/platform/database"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	if !cfg.DB.Enabled {
		slog.Error("database is required; set DB_ENABLED=true")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	db, err := database.NewPostgres(ctx, cfg.DB)
	if err != nil {
		slog.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	codeRuleService := systemapp.NewCodeRuleService(systempostgres.NewCodeRuleRepository(db))
	result, err := glpostgres.Backfill(ctx, db, codeRuleService.Next)
	if err != nil {
		slog.Error("backfill general ledger", "error", err)
		os.Exit(1)
	}
	fmt.Printf("gl backfill completed: sales_shipments=%d purchase_receipts=%d payments=%d\n",
		result.SalesShipments, result.PurchaseReceipts, result.Payments)
}
