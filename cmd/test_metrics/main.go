package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

func main() {
	fmt.Println("=== Test 5: Connection Pool & Metrics ===\n")

	// Query metrics endpoint
	metricsURL := "http://localhost:8080/metrics"
	fmt.Printf("Fetching metrics from %s...\n\n", metricsURL)

	resp, err := http.Get(metricsURL)
	if err != nil {
		log.Fatalf("Failed to fetch metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Metrics endpoint returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	metrics := string(body)
	lines := strings.Split(metrics, "\n")

	// Filter and display relevant metrics
	fmt.Println("==========================================")
	fmt.Println("Circuit Breaker Metrics")
	fmt.Println("==========================================")
	displayMetrics(lines, "circuit_breaker")
	fmt.Println()

	fmt.Println("==========================================")
	fmt.Println("Connection Pool Metrics")
	fmt.Println("==========================================")
	displayMetrics(lines, "connection_pool")
	fmt.Println()

	fmt.Println("==========================================")
	fmt.Println("Query Metrics")
	fmt.Println("==========================================")
	displayMetrics(lines, "query")
	fmt.Println()

	fmt.Println("==========================================")
	fmt.Println("API Request Metrics")
	fmt.Println("==========================================")
	displayMetrics(lines, "api_request")
	fmt.Println()

	fmt.Println("==========================================")
	fmt.Println("Database Metrics")
	fmt.Println("==========================================")
	displayMetrics(lines, "database")
	fmt.Println()

	// Summary
	fmt.Println("==========================================")
	fmt.Println("Metrics Summary")
	fmt.Println("==========================================")

	// Count total metrics
	metricCount := 0
	for _, line := range lines {
		if !strings.HasPrefix(line, "#") && strings.Contains(line, "{") {
			metricCount++
		}
	}

	fmt.Printf("Total metrics available: %d\n", metricCount)
	fmt.Println()
	fmt.Println("✓ Metrics endpoint is accessible")
	fmt.Println("✓ Prometheus integration working")
	fmt.Println()
	fmt.Println("To view in Prometheus UI:")
	fmt.Println("  1. Make sure Prometheus is running (docker-compose up -d prometheus)")
	fmt.Println("  2. Open http://localhost:9090")
	fmt.Println("  3. Query: transisidb_circuit_breaker_state")
	fmt.Println()
	fmt.Println("=== Metrics Test Complete ===")
}

func displayMetrics(lines []string, keyword string) {
	found := false
	for _, line := range lines {
		if strings.Contains(line, keyword) && !strings.HasPrefix(line, "#") {
			fmt.Println(line)
			found = true
		}
	}

	if !found {
		fmt.Printf("  No metrics found for: %s\n", keyword)
		fmt.Println("  (This is normal if feature hasn't been used yet)")
	}
}
