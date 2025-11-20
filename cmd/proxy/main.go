package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kafitramarna/TransisiDB/internal/config"
	"github.com/kafitramarna/TransisiDB/internal/logger"
	"github.com/kafitramarna/TransisiDB/internal/proxy"
)

var (
	configPath = flag.String("config", "config.yaml", "Path to configuration file")
	version    = "dev"
	buildTime  = "unknown"
)

func main() {
	flag.Parse()

	printBanner()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	logger.Info("TransisiDB Proxy starting", "version", version)
	logger.Info("Configuration loaded", "path", *configPath)

	// Initialize and start proxy server
	server := proxy.NewServer(cfg)

	// Handle shutdown signals
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		logger.Info("Shutting down proxy server...")
		server.Stop()
		os.Exit(0)
	}()

	if err := server.Start(); err != nil {
		logger.Error("Proxy server failed", "error", err)
		os.Exit(1)
	}
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
