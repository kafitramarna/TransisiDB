package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

const (
	baseURL = "http://localhost:8080"
	apiKey  = "sk_dev_changeme"
)

func main() {
	fmt.Println("=== TransisiDB Database Viewer ===\n")

	// 1. Health Check
	fmt.Println("1. Health Check:")
	getEndpoint("/health", false)
	fmt.Println()

	// 2. Get Config
	fmt.Println("2. Current Configuration:")
	getEndpoint("/api/v1/config", true)
	fmt.Println()

	// 3. List Tables
	fmt.Println("3. Configured Tables:")
	getEndpoint("/api/v1/tables", true)
	fmt.Println()

	// 4. Get specific table config
	fmt.Println("4. Orders Table Configuration:")
	getEndpoint("/api/v1/tables/orders", true)
	fmt.Println()

	fmt.Println("5. Invoices Table Configuration:")
	getEndpoint("/api/v1/tables/invoices", true)
}

func getEndpoint(endpoint string, needsAuth bool) {
	url := baseURL + endpoint

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return
	}

	if needsAuth {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error making request: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response: %v", err)
		return
	}

	// Pretty print JSON
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		fmt.Println(string(body))
		return
	}

	prettyJSON, err := json.MarshalIndent(jsonData, "", "  ")
	if err != nil {
		log.Printf("Error formatting JSON: %v", err)
		return
	}

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Println(string(prettyJSON))
}
