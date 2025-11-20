package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// Wait for proxy to start
	time.Sleep(2 * time.Second)

	// Connect to Proxy
	dsn := "root:secret@tcp(127.0.0.1:3308)/ecommerce_db"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to open connection: %v", err)
	}
	defer db.Close()
	fmt.Println("Connected to proxy successfully")

	// Execute INSERT
	// This table 'orders' is configured in config.yaml for dual-write
	query := "INSERT INTO orders (customer_id, total_amount, shipping_fee) VALUES (1, 100000, 5000)"
	fmt.Printf("Executing query: %s\n", query)

	res, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Failed to execute query: %v", err)
	}

	id, _ := res.LastInsertId()
	affected, _ := res.RowsAffected()
	fmt.Printf("Query executed. LastInsertId: %d, RowsAffected: %d\n", id, affected)
}
