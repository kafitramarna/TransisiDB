package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/transisidb/transisidb/internal/api"
	"github.com/transisidb/transisidb/internal/config"
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

	log.Println("TransisiDB Management API")
	log.Printf("Configuration loaded from: %s", *configPath)

	// Initialize Redis store
	redisStore, err := config.NewRedisStore(&cfg.Redis)
	if err != nil {
		log.Printf("Warning: Redis connection failed: %v", err)
		log.Println("API will start but config operations will be limited")
	} else {
		log.Println("Redis connection established")

		// Save current config to Redis if needed
		ctx := context.Background()
		if err := redisStore.SaveConfig(ctx, cfg); err != nil {
			log.Printf("Warning: Failed to save config to Redis: %v", err)
		}

		// Sync table configurations from config.yaml to Redis
		if err := redisStore.SyncTablesFromConfig(ctx, cfg); err != nil {
			log.Printf("Warning: Failed to sync tables to Redis: %v", err)
		} else {
			log.Println("Table configurations synced to Redis successfully")
		}
	}

	// Create API server (without backfill worker for now)
	server := api.NewServer(&cfg.API, redisStore, nil)

	// Start server in goroutine
	go func() {
		log.Printf("Starting API server on %s:%d", cfg.API.Host, cfg.API.Port)
		if err := server.Start(); err != nil {
			log.Fatalf("API server error: %v", err)
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
		log.Printf("Error during shutdown: %v", err)
	}

	if redisStore != nil {
		if err := redisStore.Close(); err != nil {
			log.Printf("Error closing Redis: %v", err)
		}
	}

	log.Println("Server stopped cleanly")
}
