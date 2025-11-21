package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	dsn := "root:secret@tcp(127.0.0.1:3307)/ecommerce_db?parseTime=true&allowNativePasswords=true"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to open: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Ping failed: %v", err)
	}
	fmt.Println("Connected to MySQL on 3307")

	// Fix root user plugin
	_, err = db.Exec("ALTER USER 'root'@'%' IDENTIFIED WITH mysql_native_password BY 'secret'")
	if err != nil {
		log.Printf("Failed to alter root@%%: %v", err)
		// Try localhost just in case
		_, err = db.Exec("ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY 'secret'")
		if err != nil {
			log.Fatalf("Failed to alter root@localhost: %v", err)
		}
	}
	fmt.Println("Successfully altered root user to use mysql_native_password")
}
