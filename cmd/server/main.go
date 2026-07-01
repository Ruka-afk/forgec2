package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Initialize structured logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting ForgeC2 Professional C2 Framework")

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("Failed to load config", "err", err)
		os.Exit(1)
	}

	// Initialize database
	database, err := db.InitDB(cfg.Database.Path, slog.LevelInfo)
	if err != nil {
		slog.Error("Failed to initialize database", "err", err)
		os.Exit(1)
	}

	// Create and start server
	srv := server.New(cfg, database)

	// Initialize optimizations
	srv.InitOptimizations(*configPath)

	slog.Info("ForgeC2 ready", "web_ui", fmt.Sprintf("http://localhost:%d", cfg.Server.Port))
	fmt.Println("\n" + `╔════════════════════════════════════════════════════════════╗
║  ForgeC2 v1.0  •  Professional Red Team C2 Framework       ║
║  Web UI: http://your-ip:8080    |   Login with your pass    ║
╚════════════════════════════════════════════════════════════╝`)

	if err := srv.Run(); err != nil {
		slog.Error("Server failed", "err", err)
		os.Exit(1)
	}
}
