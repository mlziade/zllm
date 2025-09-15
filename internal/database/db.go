package database

import (
	"log"
	"os"
	"sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	db   *gorm.DB
	once sync.Once
)

// Initialize initializes the database connection with the given models
func Initialize(models ...interface{}) *gorm.DB {
	once.Do(func() {
		var err error

		// Get database path from environment
		dbPath := os.Getenv("DATABASE_PATH")
		if dbPath == "" {
			dbPath = "data" // default
		}

		// Ensure database directory exists
		if err := os.MkdirAll(dbPath, 0755); err != nil {
			log.Fatalf("Failed to create database directory: %v", err)
		}

		// Construct full database file path
		dbFile := dbPath + "/jobs.db"

		// Configure GORM with SQLite
		db, err = gorm.Open(sqlite.Open(dbFile+"?_busy_timeout=10000&_journal_mode=WAL"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			log.Fatalf("Failed to open DB: %v", err)
		}

		// Auto-migrate the provided models
		if len(models) > 0 {
			if err := db.AutoMigrate(models...); err != nil {
				log.Fatalf("Failed to migrate database: %v", err)
			}
		}
	})
	return db
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	if db == nil {
		log.Fatal("Database not initialized. Call Initialize() first.")
	}
	return db
}

// DeleteAllJobs removes all jobs from the database
func DeleteAllJobs() error {
	db := GetDB()
	result := db.Exec("DELETE FROM jobs")
	return result.Error
}