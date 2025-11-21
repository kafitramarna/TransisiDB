package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	fmt.Println("=== Test 4: Circuit Breaker Test ===\n")

	proxyDSN := "root:secret@tcp(127.0.0.1:3308)/ecommerce_db?parseTime=true&timeout=5s"

	// Phase 1: Verify normal operation
	fmt.Println("==========================================")
	fmt.Println("Phase 1: Verify Normal Operation")
	fmt.Println("==========================================")

	db, err := sql.Open("mysql", proxyDSN)
	if err != nil {
		log.Fatalf("Failed to create connection: %v", err)
	}

	if err := db.Ping(); err != nil {
		fmt.Printf("✗ Initial connection failed: %v\n", err)
		fmt.Println("Note: Make sure MySQL is running (docker-compose up -d mysql)")
		return
	}
	fmt.Println("✓ Initial connection successful")
	db.Close()
	fmt.Println()

	// Phase 2: Stop MySQL and test circuit breaker
	fmt.Println("==========================================")
	fmt.Println("Phase 2: Backend Failure Simulation")
	fmt.Println("==========================================")
	fmt.Println("Stopping MySQL container...")
	fmt.Println("Run in another terminal: docker-compose stop mysql")
	fmt.Println()
	fmt.Println("Press Enter when MySQL is stopped...")
	fmt.Scanln()

	// Try multiple connections to trigger circuit breaker
	fmt.Println("Attempting connections to trigger circuit breaker...")
	for i := 1; i <= 6; i++ {
		fmt.Printf("Attempt %d: ", i)

		db, err := sql.Open("mysql", proxyDSN)
		if err != nil {
			fmt.Printf("✗ Failed to open: %v\n", err)
			continue
		}

		start := time.Now()
		err = db.Ping()
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("✗ Connection failed in %v - %v\n", duration, err)
		} else {
			fmt.Printf("✓ Connected in %v\n", duration)
		}

		db.Close()
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println()
	fmt.Println("Expected behavior:")
	fmt.Println("- First few attempts should timeout (~5s)")
	fmt.Println("- After 5 failures, circuit breaker opens")
	fmt.Println("- Subsequent attempts should fail QUICKLY (circuit is OPEN)")
	fmt.Println()

	// Phase 3: Recovery test
	fmt.Println("==========================================")
	fmt.Println("Phase 3: Recovery Test")
	fmt.Println("==========================================")
	fmt.Println("Start MySQL container...")
	fmt.Println("Run in another terminal: docker-compose start mysql")
	fmt.Println()
	fmt.Println("Press Enter when MySQL is started...")
	fmt.Scanln()

	fmt.Println("Waiting 35 seconds for circuit breaker timeout...")
	for i := 35; i > 0; i-- {
		fmt.Printf("\rCountdown: %2d seconds remaining...", i)
		time.Sleep(1 * time.Second)
	}
	fmt.Println("\r✓ Wait complete                          ")
	fmt.Println()

	// Try reconnecting
	fmt.Println("Attempting reconnection...")
	for i := 1; i <= 5; i++ {
		fmt.Printf("Attempt %d: ", i)

		db, err := sql.Open("mysql", proxyDSN)
		if err != nil {
			fmt.Printf("✗ Failed to open: %v\n", err)
			continue
		}

		start := time.Now()
		err = db.Ping()
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("✗ Connection failed in %v - %v\n", duration, err)
		} else {
			fmt.Printf("✓ Connected successfully in %v\n", duration)

			// Test query to verify full functionality
			var result int
			err = db.QueryRow("SELECT 1").Scan(&result)
			if err != nil {
				fmt.Printf("  ✗ Query failed: %v\n", err)
			} else {
				fmt.Println("  ✓ Query executed successfully")
			}

			db.Close()
			break
		}

		db.Close()
		time.Sleep(2 * time.Second)
	}

	fmt.Println()
	fmt.Println("Expected behavior:")
	fmt.Println("- Circuit breaker should transition to HALF-OPEN")
	fmt.Println("- First connection attempt should succeed")
	fmt.Println("- Circuit breaker closes after successful connection")
	fmt.Println()
	fmt.Println("=== Circuit Breaker Test Complete ===")
	fmt.Println()
	fmt.Println("Check proxy logs for circuit breaker state transitions:")
	fmt.Println("- CLOSED → OPEN (after failures)")
	fmt.Println("- OPEN → HALF-OPEN (after timeout)")
	fmt.Println("- HALF-OPEN → CLOSED (after success)")
}
