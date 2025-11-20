package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/transisidb/transisidb/internal/config"
)

var (
	configPath = flag.String("config", "config.yaml", "Path to configuration file")
	version    = "dev"
	buildTime  = "unknown"
)

func main() {
	flag.Parse()

	// Print banner
	printBanner()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Configuration loaded from: %s", *configPath)
	log.Printf("Database: %s@%s:%d/%s", cfg.Database.User, cfg.Database.Host, cfg.Database.Port, cfg.Database.Database)
	log.Printf("Proxy listening on: %s:%d", cfg.Proxy.Host, cfg.Proxy.Port)
	log.Printf("API listening on: %s:%d", cfg.API.Host, cfg.API.Port)
	log.Printf("Conversion ratio: 1:%d (IDR to IDN)", cfg.Conversion.Ratio)
	log.Printf("Rounding strategy: %s (precision: %d)", cfg.Conversion.RoundingStrategy, cfg.Conversion.Precision)

	// Create context with cancellation (will be used when implementing full proxy logic)
	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// TODO: Initialize components
	// - Redis connection
	// - Database connection pool
	// - Proxy server
	// - API server
	// - Metrics exporter

	log.Println("TransisiDB Proxy started successfully")
	log.Println("Press Ctrl+C to shutdown...")

	// Wait for shutdown signal
	<-sigChan
	log.Println("\nReceived shutdown signal, gracefully shutting down...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// TODO: Graceful shutdown
	// - Stop accepting new connections
	// - Wait for in-flight requests to complete
	// - Close database connections
	// - Close Redis connections
	// - Flush metrics

	<-shutdownCtx.Done()
	log.Println("Shutdown complete")
}

func printBanner() {
	banner := `
╔════════════════════════════════════════════════════════════════╗
║                        TransisiDB Proxy                        ║
║         Intelligent Database Proxy for Currency Migration     ║
║                                                                ║
║  Version: %-20s Build: %-20s ║
╚════════════════════════════════════════════════════════════════╝
`
	fmt.Printf(banner, version, buildTime)
}
