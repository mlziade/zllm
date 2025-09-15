package main

import (
	"log"

	"github.com/joho/godotenv"

	"zllm/internal/config"
	"zllm/internal/database"
	"zllm/internal/jobs"
	"zllm/internal/models"
	"zllm/internal/server"
)

func main() {
	// Load the .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: Error loading .env file")
	}

	// Load configuration
	cfg := config.LoadConfig()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Initialize database
	database.Initialize(&models.Job{})

	// Start the job worker
	jobs.StartJobWorker()

	// Start the HTTP server
	srv := server.New(cfg)
	log.Fatal(srv.Start(":" + cfg.Port))
}