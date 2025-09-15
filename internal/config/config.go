package config

import (
	"os"
	"strconv"
)

// Config holds all application configuration
type Config struct {
	OllamaURL              string
	JWTSecret              string
	APIKey                 string
	AdminAPIKey            string
	DatabasePath           string
	JobWorkerIntervalSecs  int
	JobResultExpiryMinutes int
	Port                   string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	cfg := &Config{
		OllamaURL:              getEnv("OLLAMA_URL", "http://localhost:11434"),
		JWTSecret:              getEnv("JWT_SECRET", ""),
		APIKey:                 getEnv("API_KEY", ""),
		AdminAPIKey:            getEnv("ADMIN_API_KEY", ""),
		DatabasePath:           getEnv("DATABASE_PATH", "data"),
		JobWorkerIntervalSecs:  getEnvAsInt("JOB_WORKER_INTERVAL_SECONDS", 5),
		JobResultExpiryMinutes: getEnvAsInt("JOB_RESULT_EXPIRY_MINUTES", 60),
		Port:                   getEnv("PORT", "3000"),
	}

	return cfg
}

// Validate checks if required configuration values are set
func (c *Config) Validate() error {
	// Add validation logic if needed
	return nil
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt gets an environment variable as int with a default value
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}