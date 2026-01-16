// Package main - Entry point for Terraform Cost Estimation server
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"terraform-cost/api"
)

const version = "1.0.0"

func main() {
	addr := flag.String("addr", ":8080", "Server address")
	uiPath := flag.String("ui", "./ui", "Path to UI files")
	flag.Parse()

	// Create API server
	apiServer := api.NewServer(version)

	// Create main mux
	mux := http.NewServeMux()

	// API routes
	mux.Handle("/api/", http.StripPrefix("/api", apiServer))

	// UI static files
	mux.Handle("/", http.FileServer(http.Dir(*uiPath)))

	fmt.Printf("🚀 Terraform Cost Estimation Server v%s\n", version)
	fmt.Printf("   API: http://localhost%s/api\n", *addr)
	fmt.Printf("   UI:  http://localhost%s\n", *addr)
	fmt.Println()

	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
