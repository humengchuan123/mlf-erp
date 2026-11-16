package main

import (
	"errors"
	"flag"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/joho/godotenv"
	"github.com/mlf-erp/mlf-erp/db/migrations"
	"github.com/mlf-erp/mlf-erp/internal/platform/config"
)

func main() {
	_ = godotenv.Load()

	direction := flag.String("direction", "up", "migration direction: up|down")
	steps := flag.Int("steps", 1, "number of steps for down direction")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		slog.Error("load migrations", "error", err)
		os.Exit(1)
	}

	migrator, err := migrate.NewWithSourceInstance("iofs", source, cfg.DB.DSN)
	if err != nil {
		slog.Error("create migrate instance", "error", err)
		os.Exit(1)
	}
	defer func() {
		sourceErr, databaseErr := migrator.Close()
		if sourceErr != nil || databaseErr != nil {
			slog.Warn("close migrate", "source_error", sourceErr, "database_error", databaseErr)
		}
	}()

	switch *direction {
	case "up":
		err = migrator.Up()
	case "down":
		if *steps <= 0 {
			slog.Error("steps must be greater than zero")
			os.Exit(1)
		}
		err = migrator.Steps(-*steps)
	default:
		slog.Error("invalid direction", "direction", *direction)
		os.Exit(1)
	}

	switch {
	case errors.Is(err, migrate.ErrNoChange):
		slog.Info("no migration change")
	case err != nil:
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	default:
		slog.Info("migration completed", "direction", *direction)
	}
}
