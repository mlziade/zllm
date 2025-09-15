package main

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

func GetDB() *gorm.DB {
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

		// Auto-migrate the Job model
		if err := db.AutoMigrate(&Job{}); err != nil {
			log.Fatalf("Failed to migrate jobs table: %v", err)
		}
	})
	return db
}

func DeleteAllJobs() error {
	db := GetDB()
	result := db.Exec("DELETE FROM jobs")
	return result.Error
}
