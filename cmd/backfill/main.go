package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/transisidb/transisidb/internal/backfill"
	"github.com/transisidb/transisidb/internal/config"
	"github.com/transisidb/transisidb/internal/database"
)

var (
	configPath = flag.String("config", "config.yaml", "Path to configuration file")
	tableName  = flag.String("table", "", "Table name to backfill (required)")
	dryRun     = flag.Bool("dry-run", false, "Dry run mode (count rows only)")
)

func main() {
	flag.Parse()

	if *tableName == "" {
		log.Fatal("Error: --table flag is required")
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Check if backfill is enabled
	if !cfg.Backfill.Enabled {
		log.Fatal("Backfill is disabled in configuration")
	}

	// Check if table is configured
	tableConfig, exists := cfg.Tables[*tableName]
	if !exists {
		log.Fatalf("Table '%s' not found in configuration", *tableName)
	}

	if !tableConfig.Enabled {
		log.Fatalf("Table '%s' is not enabled for conversion", *tableName)
	}

	log.Printf("Starting backfill for table: %s", *tableName)
	log.Printf("Batch size: %d", cfg.Backfill.BatchSize)
	log.Printf("Sleep interval: %dms", cfg.Backfill.SleepIntervalMs)
	log.Printf("Conversion ratio: 1:%d", cfg.Conversion.Ratio)
	log.Printf("Rounding strategy: %s", cfg.Conversion.RoundingStrategy)

	// Connect to database
	dbPool, err := database.NewPool(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbPool.Close()

	log.Println("Database connection established")

	// Create backfill worker
	worker := backfill.NewWorker(dbPool.GetDB(), cfg)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start progress reporting in background
	go reportProgress(ctx, worker, 2*time.Second)

	// Handle signals
	go func() {
		<-sigChan
		log.Println("\nReceived shutdown signal, stopping backfill...")
		worker.Stop()
		cancel()
	}()

	// Start backfill
	startTime := time.Now()

	if *dryRun {
		log.Println("DRY RUN MODE: Counting rows only...")
	}

	err = worker.Start(ctx, *tableName, tableConfig)

	duration := time.Since(startTime)

	if err != nil {
		if err == context.Canceled {
			log.Println("Backfill cancelled by user")
		} else {
			log.Fatalf("Backfill failed: %v", err)
		}
	} else {
		snapshot := worker.GetProgress().GetSnapshot()
		log.Println("\n" + strings.Repeat("=", 60))
		log.Println("BACKFILL COMPLETED SUCCESSFULLY")
		log.Println(strings.Repeat("=", 60))
		log.Printf("Table: %s", snapshot.TableName)
		log.Printf("Total rows processed: %d", snapshot.CompletedRows)
		log.Printf("Errors: %d", snapshot.Errors)
		log.Printf("Duration: %s", duration.Round(time.Second))
		log.Printf("Average speed: %.0f rows/second", float64(snapshot.CompletedRows)/duration.Seconds())
		log.Println(strings.Repeat("=", 60))
	}
}

// reportProgress periodically prints progress updates
func reportProgress(ctx context.Context, worker *backfill.Worker, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !worker.IsRunning() {
				continue
			}

			snapshot := worker.GetProgress().GetSnapshot()
			log.Println(snapshot.String())
		}
	}
}
