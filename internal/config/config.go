package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds application configuration values.
type Config struct {
	DBURL        string
	FXAPIKey     string
	ServerAddr   string
	CertFile     string
	KeyFile      string
	FXAPIBaseURL string // Added field for API base URL
}

// Read loads configuration from environment variables.
// It loads from a .env file first if it exists.
func Read() (Config, error) {
	// Attempt to load .env file, ignore error if it doesn't exist
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, reading environment variables directly.")
	} else {
		log.Println("Loaded configuration from .env file.")
	}

	cfg := Config{
		DBURL:        getEnv("DATABASE_URL", ""),     // Provide a default or handle error if critical
		FXAPIKey:     getEnv("FX_API_KEY", ""),       // Provide a default or handle error if critical
		ServerAddr:   getEnv("SERVER_ADDR", ":8443"), // Default HTTPS port
		CertFile:     getEnv("CERT_FILE", "./certs/cert.pem"),
		KeyFile:      getEnv("KEY_FILE", "./certs/key.pem"),
		FXAPIBaseURL: getEnv("FX_API_BASE_URL", ""), // Read API base URL
	}

	// Add validation if needed (e.g., check if critical variables are set)
	if cfg.DBURL == "" {
		log.Println("Warning: DATABASE_URL environment variable not set.")
		// Depending on requirements, you might return an error here:
		// return Config{}, errors.New("DATABASE_URL environment variable is required")
	}
	if cfg.FXAPIKey == "" {
		log.Println("Warning: FX_API_KEY environment variable not set.")
	}
	if cfg.FXAPIBaseURL == "" {
		log.Println("Warning: FX_API_BASE_URL environment variable not set.")
	}

	return cfg, nil
}

// getEnv retrieves an environment variable or returns a default value.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
