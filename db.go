package main

import (
	"database/sql"
	"log"
	"os"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

var (
	db   *sql.DB
	once sync.Once
)

func GetDB() *sql.DB {
	once.Do(func() {
		var err error
		dbPath := os.Getenv("SQLITE_DB_PATH")
		if dbPath == "" {
			dbPath = "jobs.db"
		}
		// Set busy timeout to 10 seconds for better concurrency
		db, err = sql.Open("sqlite3", dbPath+"?_busy_timeout=10000")
		if err != nil {
			log.Fatalf("Failed to open DB: %v", err)
		}
		// Enable WAL mode for better concurrency
		_, err = db.Exec("PRAGMA journal_mode=WAL;")
		if err != nil {
			log.Fatalf("Failed to set WAL mode: %v", err)
		}
		if err := createJobsTable(); err != nil {
			log.Fatalf("Failed to create jobs table: %v", err)
		}
	})
	return db
}

func createJobsTable() error {
	schema := `
	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		created_at DATETIME NOT NULL,
		fulfilled_at DATETIME,
		status TEXT NOT NULL,
		job_type TEXT NOT NULL,
		prompt TEXT,
		model TEXT NOT NULL,
		result TEXT,
		images_path TEXT
	);`
	_, err := db.Exec(schema)
	return err
}
