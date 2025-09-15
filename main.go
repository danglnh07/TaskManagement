// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
package main

import (
	"log/slog"
	"os"

	"github.com/danglnh07/TaskManagement/api"
	"github.com/danglnh07/TaskManagement/db"
	"github.com/danglnh07/TaskManagement/util"
)

func main() {
	// Initialize logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Load config
	config := util.LoadConfig(".env")

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

	// Initialize server and run
	server := api.NewServer(queries, logger, config)
	if err := server.Start(); err != nil {
		logger.Error("Failed to start server or server shutdown unexpectedly", "error", err)
		os.Exit(1)
	}
}
