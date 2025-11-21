package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// Connect directly to MySQL
	dsn := "root:secret@tcp(127.0.0.1:3307)/ecommerce_db?parseTime=true"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Println("=== TransisiDB Data Viewer ===")
	fmt.Println("Connected to MySQL on 127.0.0.1:3307\n")

	// Query Orders table
	fmt.Println("📦 ORDERS TABLE:")
	fmt.Println(strings.Repeat("=", 120))
	fmt.Printf("%-6s %-12s %-15s %-20s %-12s %-20s %-10s\n",
		"ID", "Customer", "Total (IDR)", "Total (IDN)", "Ship (IDR)", "Ship (IDN)", "Status")
	fmt.Println(strings.Repeat("-", 120))

	rows, err := db.Query(`
		SELECT id, customer_id, total_amount, total_amount_idn, 
		       shipping_fee, shipping_fee_idn, status 
		FROM orders 
		ORDER BY id
	`)
	if err != nil {
		log.Printf("Query failed: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var id, customerID int
			var totalIDR, shippingIDR int64
			var totalIDN, shippingIDN sql.NullFloat64
			var status string

			err := rows.Scan(&id, &customerID, &totalIDR, &totalIDN, &shippingIDR, &shippingIDN, &status)
			if err != nil {
				log.Printf("Scan error: %v", err)
				continue
			}

			totalIDNStr := "NULL"
			if totalIDN.Valid {
				totalIDNStr = fmt.Sprintf("%.4f", totalIDN.Float64)
			}

			shippingIDNStr := "NULL"
			if shippingIDN.Valid {
				shippingIDNStr = fmt.Sprintf("%.4f", shippingIDN.Float64)
			}

			fmt.Printf("%-6d %-12d %-15d %-20s %-12d %-20s %-10s\n",
				id, customerID, totalIDR, totalIDNStr, shippingIDR, shippingIDNStr, status)
		}
	}

	fmt.Println()

	// Query Invoices table
	fmt.Println("🧾 INVOICES TABLE:")
	fmt.Println(strings.Repeat("=", 100))
	fmt.Printf("%-6s %-10s %-18s %-20s %-18s %-20s\n",
		"ID", "Order ID", "Grand Total (IDR)", "Grand Total (IDN)", "Tax (IDR)", "Tax (IDN)")
	fmt.Println(strings.Repeat("-", 100))

	rows2, err := db.Query(`
		SELECT id, order_id, grand_total, grand_total_idn, tax_amount, tax_amount_idn
		FROM invoices
		ORDER BY id
	`)
	if err != nil {
		log.Printf("Query failed: %v", err)
	} else {
		defer rows2.Close()
		for rows2.Next() {
			var id, orderID int
			var grandTotalIDR, taxIDR int64
			var grandTotalIDN, taxIDN sql.NullFloat64

			err := rows2.Scan(&id, &orderID, &grandTotalIDR, &grandTotalIDN, &taxIDR, &taxIDN)
			if err != nil {
				log.Printf("Scan error: %v", err)
				continue
			}

			grandTotalIDNStr := "NULL"
			if grandTotalIDN.Valid {
				grandTotalIDNStr = fmt.Sprintf("%.4f", grandTotalIDN.Float64)
			}

			taxIDNStr := "NULL"
			if taxIDN.Valid {
				taxIDNStr = fmt.Sprintf("%.4f", taxIDN.Float64)
			}

			fmt.Printf("%-6d %-10d %-18d %-20s %-18d %-20s\n",
				id, orderID, grandTotalIDR, grandTotalIDNStr, taxIDR, taxIDNStr)
		}
	}

	fmt.Println()

	// Show summary of dual-write status
	fmt.Println("📊 DUAL-WRITE STATUS SUMMARY:")
	fmt.Println(strings.Repeat("=", 80))

	var ordersTotal, ordersWithIDN int
	db.QueryRow("SELECT COUNT(*), SUM(CASE WHEN total_amount_idn IS NOT NULL THEN 1 ELSE 0 END) FROM orders").
		Scan(&ordersTotal, &ordersWithIDN)

	var invoicesTotal, invoicesWithIDN int
	db.QueryRow("SELECT COUNT(*), SUM(CASE WHEN grand_total_idn IS NOT NULL THEN 1 ELSE 0 END) FROM invoices").
		Scan(&invoicesTotal, &invoicesWithIDN)

	fmt.Printf("Orders:   %d total, %d with IDN values (%.1f%% converted)\n",
		ordersTotal, ordersWithIDN, float64(ordersWithIDN)/float64(ordersTotal)*100)
	fmt.Printf("Invoices: %d total, %d with IDN values (%.1f%% converted)\n",
		invoicesTotal, invoicesWithIDN, float64(invoicesWithIDN)/float64(invoicesTotal)*100)
}
