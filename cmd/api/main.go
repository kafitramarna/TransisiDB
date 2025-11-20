package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kafitramarna/TransisiDB/internal/api"
	"github.com/kafitramarna/TransisiDB/internal/config"
	"github.com/kafitramarna/TransisiDB/internal/logger"
)

var (
	configPath = flag.String("config", "config.yaml", "Path to configuration file")
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger.Init("INFO")
	logger.Info("TransisiDB Management API starting", "version", "dev")
	logger.Info("Configuration loaded", "path", *configPath)

	// Initialize Redis store
	redisStore, err := config.NewRedisStore(&cfg.Redis)
	if err != nil {
		logger.Warn("Redis connection failed", "error", err)
		logger.Info("API will start but config operations will be limited")
	} else {
		logger.Info("Redis connection established")

		// Save current config to Redis if needed
		ctx := context.Background()
		if err := redisStore.SaveConfig(ctx, cfg); err != nil {
			logger.Warn("Failed to save config to Redis", "error", err)
		}

		// Sync table configurations from config.yaml to Redis
		if err := redisStore.SyncTablesFromConfig(ctx, cfg); err != nil {
			logger.Warn("Failed to sync tables to Redis", "error", err)
		} else {
			logger.Info("Table configurations synced to Redis successfully")
		}
	}

	// Create API server (without backfill worker for now)
	server := api.NewServer(&cfg.API, redisStore, nil)

	// Start server in goroutine
	go func() {
		logger.Info("Starting API server", "host", cfg.API.Host, "port", cfg.API.Port)
		if err := server.Start(); err != nil {
			logger.Error("API server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("\nShutdown signal received, gracefully stopping...")

	// Shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Error during shutdown", "error", err)
	}

	if redisStore != nil {
		if err := redisStore.Close(); err != nil {
			logger.Error("Error closing Redis", "error", err)
		}
	}

	logger.Info("Server stopped cleanly")
}
