// Package main - Entry point for Terraform Cost Estimation server
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"terraform-cost/api"
	"terraform-cost/db"
)

const version = "1.0.0"

func main() {
	addr := flag.String("addr", ":8080", "Server address")
	uiPath := flag.String("ui", "./ui", "Path to UI files")
	flag.Parse()

	// Connect to database
	store, err := getDBStore()
	if err != nil {
		log.Printf("Warning: Database not available: %v", err)
		log.Printf("API will work in read-only mode without pricing data")
	} else {
		defer store.Close()
		log.Printf("✓ Connected to pricing database")
		
		// Check active snapshots
		ctx := context.Background()
		if snap, err := store.GetActiveSnapshot(ctx, db.AWS, "us-east-1", "default"); err == nil && snap != nil {
			count, _ := store.CountRates(ctx, snap.ID)
			log.Printf("✓ Active AWS us-east-1 snapshot: %d rates", count)
		}
	}

	// Create API server with database
	apiServer := api.NewServerWithStore(version, store)

	// Create main mux
	mux := http.NewServeMux()

	// API routes
	mux.Handle("/api/", http.StripPrefix("/api", apiServer))

	// Health check at root
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","version":"%s"}`, version)
	})

	// UI static files
	if _, err := os.Stat(*uiPath); err == nil {
		mux.Handle("/", http.FileServer(http.Dir(*uiPath)))
	}

	// Create server with graceful shutdown
	server := &http.Server{
		Addr:         *addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		fmt.Printf("🚀 Terraform Cost Estimation Server v%s\n", version)
		fmt.Printf("   API:    http://localhost%s/api\n", *addr)
		fmt.Printf("   Health: http://localhost%s/health\n", *addr)
		fmt.Println()
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	log.Println("Server stopped")
}

// getDBStore creates database connection from environment
func getDBStore() (db.PricingStore, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		return db.NewPostgresStoreFromURL(dbURL)
	}

	// Try individual environment variables
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := 5432
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "terraform_cost"
	}
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "terraform_cost_dev"
	}
	database := os.Getenv("DB_NAME")
	if database == "" {
		database = "terraform_cost"
	}
	sslmode := os.Getenv("DB_SSLMODE")
	if sslmode == "" {
		sslmode = "disable"
	}

	return db.NewPostgresStore(db.Config{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Database: database,
		SSLMode:  sslmode,
	})
}
