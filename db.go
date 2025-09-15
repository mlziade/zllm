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
		dbPath := os.Getenv("SQLITE_DB_PATH")
		if dbPath == "" {
			dbPath = "jobs.db"
		}

		// Configure GORM with SQLite
		db, err = gorm.Open(sqlite.Open(dbPath+"?_busy_timeout=10000&_journal_mode=WAL"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent), // Set to logger.Info for debugging
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
