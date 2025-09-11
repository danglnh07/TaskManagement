package main

import (
	"log/slog"
	"os"

	"github.com/danglnh07/TaskManagement/db"
	"github.com/danglnh07/TaskManagement/service/security"
)

func main() {
	// Initialize logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Load config
	config := security.LoadConfig(".env")

	// Initialize queries and connect database
	queries := db.NewQueries(config)
	if err := queries.ConnectDB(); err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}

	// Run auto migration
	if err := queries.AutoMigration(); err != nil {
		logger.Error("Failed to run database migration", "error", err)
		os.Exit(1)
	}

	logger.Info("Initialize database successfully")
}
