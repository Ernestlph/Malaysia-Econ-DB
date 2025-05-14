package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds application configuration values.
type Config struct {
	DBURL                     string
	FXAPIKey                  string
	ServerAddr                string
	CertFile                  string
	KeyFile                   string
	FXAPIBaseURL              string // Added field for API base URL
	I3InvestorBaseURL         string
	I3InvestorStockProfileURL string
	StockList                 []string
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
	// Load stock list separately
	stockListStr := getEnv("STOCK_LIST", "")
	var stockList []string
	if stockListStr != "" {
		// Split by comma
		rawList := strings.Split(stockListStr, ",")
		// Trim whitespace from each element and filter out empty strings
		for _, code := range rawList {
			trimmedCode := strings.TrimSpace(code)
			if trimmedCode != "" { // Only add non-empty codes
				stockList = append(stockList, trimmedCode)
			}
		}
	} else {
		log.Println("Warning: STOCK_LIST environment variable not set or empty.")
		stockList = []string{} // Initialize as empty slice
	}

	cfg := Config{
		DBURL:                     getEnv("DB_URL", ""),           // Provide a default or handle error if critical
		ServerAddr:                getEnv("SERVER_ADDR", ":8443"), // Default HTTPS port
		CertFile:                  getEnv("CERT_FILE", "./certs/cert.pem"),
		KeyFile:                   getEnv("KEY_FILE", "./certs/key.pem"),
		FXAPIBaseURL:              getEnv("FX_API_BASE_URL", ""), // Read API base URL
		I3InvestorBaseURL:         getEnv("I3_INVESTOR_BASE_URL", ""),
		I3InvestorStockProfileURL: getEnv("I3_INVESTOR_STOCK_PROFILE_URL", ""),
		StockList:                 stockList,
	}

	// Add validation if needed (e.g., check if critical variables are set)
	if cfg.DBURL == "" {
		log.Println("Warning: DATABASE_URL environment variable not set.")
		// Depending on requirements, you might return an error here:
		// return Config{}, errors.New("DATABASE_URL environment variable is required")
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
