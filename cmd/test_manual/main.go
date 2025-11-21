package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// Connect through proxy with text protocol (for query transformation)
	proxyDSN := "root:secret@tcp(127.0.0.1:3308)/ecommerce_db?parseTime=true&interpolateParams=true"
	db, err := sql.Open("mysql", proxyDSN)
	if err != nil {
		log.Fatalf("Failed to connect to proxy: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping proxy: %v", err)
	}

	fmt.Println("=== Test 3: MySQL CLI Manual Testing ===")
	fmt.Println("Connected to proxy on 127.0.0.1:3308\n")

	// Test 1: Dual-Write INSERT
	fmt.Println("==========================================")
	fmt.Println("Test 1: Dual-Write INSERT")
	fmt.Println("==========================================")

	_, err = db.Exec(`INSERT INTO orders (id, customer_id, total_amount, shipping_fee) 
		VALUES (9999, 8888, 50000000, 15000)`)
	if err != nil {
		log.Printf("INSERT failed: %v", err)
	} else {
		fmt.Println("✓ INSERT executed")
	}

	var id, customerId int
	var totalIDR, shippingIDR int64
	var totalIDN, shippingIDN sql.NullFloat64

	err = db.QueryRow(`SELECT id, customer_id, total_amount, total_amount_idn, shipping_fee, shipping_fee_idn 
		FROM orders WHERE id = 9999`).Scan(&id, &customerId, &totalIDR, &totalIDN, &shippingIDR, &shippingIDN)

	if err != nil {
		log.Printf("SELECT failed: %v", err)
	} else {
		fmt.Printf("Result:\n")
		fmt.Printf("  ID: %d, Customer: %d\n", id, customerId)
		fmt.Printf("  Total Amount: %d IDR -> %s IDN\n", totalIDR, formatNullFloat(totalIDN))
		fmt.Printf("  Shipping Fee: %d IDR -> %s IDN\n", shippingIDR, formatNullFloat(shippingIDN))

		if totalIDN.Valid && totalIDN.Float64 == 50000.0000 {
			fmt.Println("✓ total_amount_idn correct (50000.0000)")
		} else {
			fmt.Printf("✗ total_amount_idn incorrect, expected 50000.0000, got %s\n", formatNullFloat(totalIDN))
		}

		if shippingIDN.Valid && shippingIDN.Float64 == 15.0000 {
			fmt.Println("✓ shipping_fee_idn correct (15.0000)")
		} else {
			fmt.Printf("✗ shipping_fee_idn incorrect, expected 15.0000, got %s\n", formatNullFloat(shippingIDN))
		}
	}
	fmt.Println()

	// Test 2: Dual-Write UPDATE
	fmt.Println("==========================================")
	fmt.Println("Test 2: Dual-Write UPDATE")
	fmt.Println("==========================================")

	_, err = db.Exec(`UPDATE orders SET total_amount = 75000000 WHERE id = 9999`)
	if err != nil {
		log.Printf("UPDATE failed: %v", err)
	} else {
		fmt.Println("✓ UPDATE executed")
	}

	err = db.QueryRow(`SELECT id, total_amount, total_amount_idn FROM orders WHERE id = 9999`).
		Scan(&id, &totalIDR, &totalIDN)

	if err != nil {
		log.Printf("SELECT failed: %v", err)
	} else {
		fmt.Printf("Result:\n")
		fmt.Printf("  ID: %d\n", id)
		fmt.Printf("  Total Amount: %d IDR -> %s IDN\n", totalIDR, formatNullFloat(totalIDN))

		if totalIDN.Valid && totalIDN.Float64 == 75000.0000 {
			fmt.Println("✓ total_amount_idn updated correctly (75000.0000)")
		} else {
			fmt.Printf("✗ total_amount_idn incorrect, expected 75000.0000, got %s\n", formatNullFloat(totalIDN))
		}
	}
	fmt.Println()

	// Test 3: Transaction COMMIT
	fmt.Println("==========================================")
	fmt.Println("Test 3: Transaction COMMIT")
	fmt.Println("==========================================")

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("BEGIN failed: %v", err)
	}

	_, err = tx.Exec(`INSERT INTO orders (id, customer_id, total_amount, shipping_fee) 
		VALUES (9998, 8887, 25000000, 10000)`)
	if err != nil {
		log.Printf("INSERT in TX failed: %v", err)
		tx.Rollback()
	} else {
		fmt.Println("✓ INSERT in transaction executed")

		// Check within transaction
		var count int
		tx.QueryRow(`SELECT COUNT(*) FROM orders WHERE id = 9998`).Scan(&count)
		fmt.Printf("  Row visible within transaction: %d\n", count)

		if err := tx.Commit(); err != nil {
			log.Printf("COMMIT failed: %v", err)
		} else {
			fmt.Println("✓ Transaction committed")
		}
	}

	// Verify after commit
	err = db.QueryRow(`SELECT id, total_amount, total_amount_idn FROM orders WHERE id = 9998`).
		Scan(&id, &totalIDR, &totalIDN)

	if err != nil {
		log.Printf("SELECT after commit failed: %v", err)
	} else {
		fmt.Printf("Result after commit:\n")
		fmt.Printf("  ID: %d, Total: %d IDR -> %s IDN\n", id, totalIDR, formatNullFloat(totalIDN))

		if totalIDN.Valid && totalIDN.Float64 == 25000.0000 {
			fmt.Println("✓ Data persisted with correct IDN value (25000.0000)")
		}
	}
	fmt.Println()

	// Test 4: Transaction ROLLBACK
	fmt.Println("==========================================")
	fmt.Println("Test 4: Transaction ROLLBACK")
	fmt.Println("==========================================")

	tx2, err := db.Begin()
	if err != nil {
		log.Fatalf("BEGIN failed: %v", err)
	}

	_, err = tx2.Exec(`INSERT INTO orders (id, customer_id, total_amount, shipping_fee) 
		VALUES (9997, 8886, 30000000, 12000)`)
	if err != nil {
		log.Printf("INSERT failed: %v", err)
		tx2.Rollback()
	} else {
		fmt.Println("✓ INSERT in transaction executed")

		var countInTx int
		tx2.QueryRow(`SELECT COUNT(*) FROM orders WHERE id = 9997`).Scan(&countInTx)
		fmt.Printf("  Row visible within transaction: %d\n", countInTx)

		if err := tx2.Rollback(); err != nil {
			log.Printf("ROLLBACK failed: %v", err)
		} else {
			fmt.Println("✓ Transaction rolled back")
		}
	}

	// Verify rollback worked
	var countAfterRollback int
	db.QueryRow(`SELECT COUNT(*) FROM orders WHERE id = 9997`).Scan(&countAfterRollback)
	fmt.Printf("Row count after rollback: %d\n", countAfterRollback)

	if countAfterRollback == 0 {
		fmt.Println("✓ Rollback successful, row not persisted")
	} else {
		fmt.Println("✗ Rollback failed, row still exists")
	}
	fmt.Println()

	// Test 5: Banker's Rounding
	fmt.Println("==========================================")
	fmt.Println("Test 5: Banker's Rounding")
	fmt.Println("==========================================")

	_, err = db.Exec(`INSERT INTO orders (id, customer_id, total_amount, shipping_fee) 
		VALUES (9001, 8801, 15500, 10000)`)
	if err != nil {
		log.Printf("INSERT 9001 failed: %v", err)
	}

	_, err = db.Exec(`INSERT INTO orders (id, customer_id, total_amount, shipping_fee) 
		VALUES (9002, 8802, 16500, 10000)`)
	if err != nil {
		log.Printf("INSERT 9002 failed: %v", err)
	}

	rows, err := db.Query(`SELECT id, total_amount, total_amount_idn, shipping_fee, shipping_fee_idn 
		FROM orders WHERE id IN (9001, 9002) ORDER BY id`)

	if err != nil {
		log.Printf("SELECT failed: %v", err)
	} else {
		defer rows.Close()
		fmt.Println("Results:")
		for rows.Next() {
			rows.Scan(&id, &totalIDR, &totalIDN, &shippingIDR, &shippingIDN)
			fmt.Printf("  ID %d: %d IDR -> %s IDN\n", id, totalIDR, formatNullFloat(totalIDN))
		}

		// Verify specific values
		db.QueryRow(`SELECT total_amount_idn FROM orders WHERE id = 9001`).Scan(&totalIDN)
		if totalIDN.Valid && totalIDN.Float64 == 15.5000 {
			fmt.Println("✓ 15500 rounded correctly to 15.5000")
		} else {
			fmt.Printf("✗ 15500 rounding incorrect, got %s\n", formatNullFloat(totalIDN))
		}

		db.QueryRow(`SELECT total_amount_idn FROM orders WHERE id = 9002`).Scan(&totalIDN)
		if totalIDN.Valid && totalIDN.Float64 == 16.5000 {
			fmt.Println("✓ 16500 rounded correctly to 16.5000")
		} else {
			fmt.Printf("✗ 16500 rounding incorrect, got %s\n", formatNullFloat(totalIDN))
		}
	}
	fmt.Println()

	// Cleanup
	fmt.Println("==========================================")
	fmt.Println("Cleanup")
	fmt.Println("==========================================")

	result, err := db.Exec(`DELETE FROM orders WHERE id >= 9000`)
	if err != nil {
		log.Printf("DELETE failed: %v", err)
	} else {
		rowsAffected, _ := result.RowsAffected()
		fmt.Printf("✓ Deleted %d test rows\n", rowsAffected)
	}

	fmt.Println("\n=== All Manual Tests Complete ===")
}

func formatNullFloat(nf sql.NullFloat64) string {
	if nf.Valid {
		return fmt.Sprintf("%.4f", nf.Float64)
	}
	return "NULL"
}
