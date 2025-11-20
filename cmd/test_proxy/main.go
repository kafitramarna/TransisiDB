package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	fmt.Println("=== TransisiDB Proxy Integration Test ===")
	fmt.Println()

	// Connect through proxy (port 3308)
	proxyDSN := "root:secret@tcp(127.0.0.1:3308)/ecommerce_db?parseTime=true"
	proxyDB, err := sql.Open("mysql", proxyDSN)
	if err != nil {
		log.Fatalf("Failed to connect to proxy: %v", err)
	}
	defer proxyDB.Close()

	// Connect directly to MySQL (port 3307) for verification
	directDSN := "root:secret@tcp(127.0.0.1:3307)/ecommerce_db?parseTime=true"
	directDB, err := sql.Open("mysql", directDSN)
	if err != nil {
		log.Fatalf("Failed to connect directly: %v", err)
	}
	defer directDB.Close()

	// Test 1: Basic Connectivity
	fmt.Println("Test 1: Basic Connectivity")
	if err := testBasicConnectivity(proxyDB); err != nil {
		log.Fatalf("Test 1 failed: %v", err)
	}
	fmt.Println("✓ Test 1 passed")
	fmt.Println()

	// Test 2: Dual-Write INSERT
	fmt.Println("Test 2: Dual-Write INSERT")
	if err := testDualWriteInsert(proxyDB, directDB); err != nil {
		log.Fatalf("Test 2 failed: %v", err)
	}
	fmt.Println("✓ Test 2 passed")
	fmt.Println()

	// Test 3: Dual-Write UPDATE
	fmt.Println("Test 3: Dual-Write UPDATE")
	if err := testDualWriteUpdate(proxyDB, directDB); err != nil {
		log.Fatalf("Test 3 failed: %v", err)
	}
	fmt.Println("✓ Test 3 passed")
	fmt.Println()

	// Test 4: Transaction Handling
	fmt.Println("Test 4: Transaction Handling")
	if err := testTransactions(proxyDB, directDB); err != nil {
		log.Fatalf("Test 4 failed: %v", err)
	}
	fmt.Println("✓ Test 4 passed")
	fmt.Println()

	// Test 5: Banker's Rounding
	fmt.Println("Test 5: Banker's Rounding")
	if err := testBankersRounding(proxyDB, directDB); err != nil {
		log.Fatalf("Test 5 failed: %v", err)
	}
	fmt.Println("✓ Test 5 passed")
	fmt.Println()

	// Test 6: COM_PING
	fmt.Println("Test 6: COM_PING")
	if err := testPing(proxyDB); err != nil {
		log.Fatalf("Test 6 failed: %v", err)
	}
	fmt.Println("✓ Test 6 passed")
	fmt.Println()

	// Test 7: Database switching (COM_INIT_DB)
	fmt.Println("Test 7: Database Switching")
	if err := testDatabaseSwitch(proxyDB); err != nil {
		log.Fatalf("Test 7 failed: %v", err)
	}
	fmt.Println("✓ Test 7 passed")
	fmt.Println()

	fmt.Println("=== All Tests Passed! ===")
}

func testBasicConnectivity(db *sql.DB) error {
	var result int
	err := db.QueryRow("SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("SELECT 1 failed: %w", err)
	}
	if result != 1 {
		return fmt.Errorf("expected 1, got %d", result)
	}
	fmt.Println("  → SELECT 1 = 1")
	return nil
}

func testDualWriteInsert(proxyDB, directDB *sql.DB) error {
	// Clean up test data
	directDB.Exec("DELETE FROM orders WHERE id >= 1000")

	// Insert through proxy
	orderID := 1001
	totalAmount := int64(15000000) // IDR
	expectedIDN := 15000.0000      // IDN (15000000 / 1000)

	_, err := proxyDB.Exec(
		"INSERT INTO orders (id, total_amount) VALUES (?, ?)",
		orderID, totalAmount,
	)
	if err != nil {
		return fmt.Errorf("INSERT failed: %w", err)
	}

	fmt.Printf("  → Inserted: id=%d, total_amount=%d IDR\n", orderID, totalAmount)

	// Verify shadow column was populated
	var actualIDR int64
	var actualIDN float64
	err = directDB.QueryRow(
		"SELECT total_amount, total_amount_idn FROM orders WHERE id = ?",
		orderID,
	).Scan(&actualIDR, &actualIDN)
	if err != nil {
		return fmt.Errorf("SELECT verification failed: %w", err)
	}

	fmt.Printf("  → Verified: total_amount=%d IDR, total_amount_idn=%.4f IDN\n", actualIDR, actualIDN)

	if actualIDR != totalAmount {
		return fmt.Errorf("IDR mismatch: expected %d, got %d", totalAmount, actualIDR)
	}
	if actualIDN != expectedIDN {
		return fmt.Errorf("IDN mismatch: expected %.4f, got %.4f", expectedIDN, actualIDN)
	}

	return nil
}

func testDualWriteUpdate(proxyDB, directDB *sql.DB) error {
	orderID := 1001
	newAmount := int64(25000000) // IDR
	expectedIDN := 25000.0000    // IDN

	_, err := proxyDB.Exec(
		"UPDATE orders SET total_amount = ? WHERE id = ?",
		newAmount, orderID,
	)
	if err != nil {
		return fmt.Errorf("UPDATE failed: %w", err)
	}

	fmt.Printf("  → Updated: id=%d, total_amount=%d IDR\n", orderID, newAmount)

	// Verify
	var actualIDR int64
	var actualIDN float64
	err = directDB.QueryRow(
		"SELECT total_amount, total_amount_idn FROM orders WHERE id = ?",
		orderID,
	).Scan(&actualIDR, &actualIDN)
	if err != nil {
		return fmt.Errorf("SELECT verification failed: %w", err)
	}

	fmt.Printf("  → Verified: total_amount=%d IDR, total_amount_idn=%.4f IDN\n", actualIDR, actualIDN)

	if actualIDR != newAmount {
		return fmt.Errorf("IDR mismatch: expected %d, got %d", newAmount, actualIDR)
	}
	if actualIDN != expectedIDN {
		return fmt.Errorf("IDN mismatch: expected %.4f, got %.4f", expectedIDN, actualIDN)
	}

	return nil
}

func testTransactions(proxyDB, directDB *sql.DB) error {
	orderID := 1002
	directDB.Exec("DELETE FROM orders WHERE id = ?", orderID)

	// Start transaction
	tx, err := proxyDB.Begin()
	if err != nil {
		return fmt.Errorf("BEGIN failed: %w", err)
	}

	// Insert within transaction
	_, err = tx.Exec(
		"INSERT INTO orders (id, total_amount) VALUES (?, ?)",
		orderID, 10000000,
	)
	if err != nil {
		return fmt.Errorf("INSERT in transaction failed: %w", err)
	}

	// Commit
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("COMMIT failed: %w", err)
	}

	fmt.Printf("  → Transaction committed: id=%d\n", orderID)

	// Verify
	var count int
	err = directDB.QueryRow("SELECT COUNT(*) FROM orders WHERE id = ?", orderID).Scan(&count)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	if count != 1 {
		return fmt.Errorf("expected 1 row, got %d", count)
	}

	// Test rollback
	orderID2 := 1003
	tx2, err := proxyDB.Begin()
	if err != nil {
		return fmt.Errorf("BEGIN failed: %w", err)
	}

	_, err = tx2.Exec(
		"INSERT INTO orders (id, total_amount) VALUES (?, ?)",
		orderID2, 20000000,
	)
	if err != nil {
		return fmt.Errorf("INSERT in transaction failed: %w", err)
	}

	// Rollback
	if err := tx2.Rollback(); err != nil {
		return fmt.Errorf("ROLLBACK failed: %w", err)
	}

	fmt.Printf("  → Transaction rolled back: id=%d\n", orderID2)

	// Verify rollback worked
	err = directDB.QueryRow("SELECT COUNT(*) FROM orders WHERE id = ?", orderID2).Scan(&count)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	if count != 0 {
		return fmt.Errorf("expected 0 rows after rollback, got %d", count)
	}

	return nil
}

func testBankersRounding(proxyDB, directDB *sql.DB) error {
	directDB.Exec("DELETE FROM orders WHERE id >= 2000 AND id < 3000")

	testCases := []struct {
		id       int
		idr      int64
		expected float64
	}{
		{2001, 15000, 15.0000}, // Exact division
		{2002, 15500, 15.5000}, // Half-way case (banker's round to even: 15.5)
		{2003, 16500, 16.5000}, // Half-way case (banker's round to even: 16.5)
		{2004, 15250, 15.2500}, // Quarter
		{2005, 15750, 15.7500}, // Three-quarters
	}

	for _, tc := range testCases {
		_, err := proxyDB.Exec(
			"INSERT INTO orders (id, total_amount) VALUES (?, ?)",
			tc.id, tc.idr,
		)
		if err != nil {
			return fmt.Errorf("INSERT failed for id=%d: %w", tc.id, err)
		}

		var actualIDN float64
		err = directDB.QueryRow(
			"SELECT total_amount_idn FROM orders WHERE id = ?",
			tc.id,
		).Scan(&actualIDN)
		if err != nil {
			return fmt.Errorf("SELECT failed for id=%d: %w", tc.id, err)
		}

		fmt.Printf("  → %d IDR = %.4f IDN (expected %.4f)\n", tc.idr, actualIDN, tc.expected)

		if actualIDN != tc.expected {
			return fmt.Errorf("rounding mismatch for id=%d: expected %.4f, got %.4f",
				tc.id, tc.expected, actualIDN)
		}
	}

	return nil
}

func testPing(db *sql.DB) error {
	start := time.Now()
	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	duration := time.Since(start)
	fmt.Printf("  → Ping successful (%.2fms)\n", duration.Seconds()*1000)
	return nil
}

func testDatabaseSwitch(db *sql.DB) error {
	// Switch to mysql database
	_, err := db.Exec("USE mysql")
	if err != nil {
		return fmt.Errorf("USE mysql failed: %w", err)
	}

	// Verify we can query system tables
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM user").Scan(&count)
	if err != nil {
		return fmt.Errorf("SELECT from mysql.user failed: %w", err)
	}

	fmt.Printf("  → Switched to mysql database (user count: %d)\n", count)

	// Switch back to ecommerce_db
	_, err = db.Exec("USE ecommerce_db")
	if err != nil {
		return fmt.Errorf("USE ecommerce_db failed: %w", err)
	}

	fmt.Println("  → Switched back to ecommerce_db")

	return nil
}
