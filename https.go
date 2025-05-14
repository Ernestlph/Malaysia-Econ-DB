package main

import (
	"context"
	"crypto/tls"
	"database/sql" // Import database/sql for sql.ErrNoRows
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	// Assuming your sqlc generated code is in this package
	"github.com/Ernestlph/Malaysia-Econ-DB/internal/database"
	// No longer need config directly here as it's in the state
	// "github.com/Ernestlph/Malaysia-Econ-DB/internal/config"
)

// apiServer holds dependencies for the HTTP handlers, like database access.
type apiServer struct {
	state *AppState // Holds db queries and config
}

// Structure for generic time-series API response expected by the frontend
type TimeSeriesDataPoint struct {
	Date  string  `json:"date"`  // Format YYYY-MM-DD
	Value float64 `json:"value"` // Generic value (price, rate, amount)
}

// runHttpsServer sets up and runs the HTTPS server.
// It now accepts the application state (*state) containing db access and config.
func runHttpsServer(ctx context.Context, wg *sync.WaitGroup, shutdownChan chan struct{}, appState *AppState) {
	defer wg.Done() // Signal WaitGroup when this goroutine exits

	// Create the apiServer instance holding the application state
	server := &apiServer{
		state: appState,
	}

	// Create a new ServeMux to route requests
	mux := http.NewServeMux()

	// --- Register API Handlers ---
	mux.HandleFunc("/api/stock/prices", server.handleGetStockPrices)
	mux.HandleFunc("/api/fx/rates", server.handleGetFxRates)
	// Add more API handlers here as needed (e.g., for loans)
	// mux.HandleFunc("/api/loans/sector", server.handleGetLoanData)

	// --- Register Static File Server (must be general and often last) ---
	// Serve files like index.html, chart.js from the "./frontend" directory
	// Requests to "/" will serve "./frontend/index.html" if it exists
	// Requests to "/chart.js" will serve "./frontend/chart.js" if it exists
	fileServer := http.FileServer(http.Dir("./frontend"))
	mux.Handle("/", fileServer)

	// --- Configure TLS ---
	tlsCfg := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, // Required for HTTP/2
		},
	}

	// --- Create the HTTP Server Instance ---
	srv := &http.Server{
		Addr:         appState.cfg.ServerAddr, // Get server address from config within state
		Handler:      mux,                     // Use the mux with all registered handlers
		TLSConfig:    tlsCfg,
		ReadTimeout:  10 * time.Second, // Reasonable timeouts
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// --- Start Server Goroutine ---
	go func() {
		log.Printf("Starting HTTPS server on %s (serving API and frontend from ./frontend)", srv.Addr)
		// Use CertFile and KeyFile from config within state
		err := srv.ListenAndServeTLS(appState.cfg.CertFile, appState.cfg.KeyFile)
		// ListenAndServeTLS always returns a non-nil error. After Shutdown or Close,
		// the returned error is http.ErrServerClosed. We should not treat that as fatal.
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("FATAL: HTTPS server ListenAndServeTLS error: %v", err) // Use Fatalf to exit if server fails to start
		}
		log.Println("HTTPS server stopped listening.")
	}()

	// --- Graceful Shutdown Logic ---
	// Wait for shutdown signal from the channel (closed by CLI or OS signal handler)
	<-shutdownChan
	log.Println("Shutdown signal received, shutting down HTTPS server...")

	// Create a context with a timeout for the shutdown process
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second) // Allow 15 seconds
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTPS server graceful shutdown error: %v", err)
	} else {
		log.Println("HTTPS server gracefully stopped.")
	}
}

// --- API Handler Implementations ---

type StockPriceDetailResponseItem struct {
	Date        string  `json:"date"`
	Value       float64 `json:"value"`
	CompanyName string  `json:"company_name"` // NEW
	StockCode   string  `json:"stock_code"`   // NEW (optional, good for frontend)
}

// handleGetStockPrices handles requests for stock price data, now including company name
func (s *apiServer) handleGetStockPrices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	queryParams := r.URL.Query()
	stockCode := queryParams.Get("code")
	startDateStr := queryParams.Get("start_date")
	endDateStr := queryParams.Get("end_date")

	if stockCode == "" || startDateStr == "" || endDateStr == "" {
		http.Error(w, "Missing required query parameters: code, start_date, end_date", http.StatusBadRequest)
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid start_date format (use YYYY-MM-DD): %v", err), http.StatusBadRequest)
		return
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid end_date format (use YYYY-MM-DD): %v", err), http.StatusBadRequest)
		return
	}

	// --- Database Query ---
	// Use the query that fetches company details as well
	dbParams := database.GetStockPricesWithDetailsByCodeAndDateRangeParams{ // Correct Params struct
		StockCode: stockCode,
		StartDate: startDate,
		EndDate:   endDate,
	}

	log.Printf("API: Querying stock prices with details for %s from %s to %s", stockCode, startDateStr, endDateStr)
	// Call the correct sqlc generated function
	dbResults, err := s.state.db.GetStockPricesWithDetailsByCodeAndDateRange(r.Context(), dbParams)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("API: No stock price data found for %s between %s and %s", stockCode, startDateStr, endDateStr)
			// Send empty array using the detailed response item type for consistency, though frontend might just expect TimeSeriesDataPoint
			sendJsonResponse(w, []StockPriceDetailResponseItem{})
			return
		}
		log.Printf("API Error: Database error fetching stock prices for %s: %v", stockCode, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// --- Format Response ---
	// Use StockPriceDetailResponseItem for the response
	response := make([]StockPriceDetailResponseItem, 0, len(dbResults))
	for _, dbRow := range dbResults { // dbRow is of type GetStockPricesWithDetailsByCodeAndDateRangeRow
		// dbRow.ClosingPrice is string as per your generated code
		price, convErr := strconv.ParseFloat(dbRow.ClosingPrice, 64)
		if convErr != nil {
			log.Printf("API Error: Failed to convert closing price '%s' to float for %s on %s: %v",
				dbRow.ClosingPrice, dbRow.StockCode, dbRow.PriceDate.Format("2006-01-02"), convErr)
			continue // Skip this data point if conversion fails
		}

		response = append(response, StockPriceDetailResponseItem{
			Date:        dbRow.PriceDate.Format("2006-01-02"),
			Value:       price, // Use the converted float64
			CompanyName: dbRow.CompanyName,
			StockCode:   dbRow.StockCode,
		})
	}

	log.Printf("API: Found %d stock price records (with details) for %s", len(response), stockCode)
	sendJsonResponse(w, response)
}

// handleGetFxRates handles requests for foreign exchange rate data
func (s *apiServer) handleGetFxRates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	queryParams := r.URL.Query()
	currencyCode := queryParams.Get("code") // e.g., "USD"
	startDateStr := queryParams.Get("start_date")
	endDateStr := queryParams.Get("end_date")

	if currencyCode == "" || startDateStr == "" || endDateStr == "" {
		http.Error(w, "Missing required query parameters: code, start_date, end_date", http.StatusBadRequest)
		return
	}

	// Basic validation for currency code format (adjust as needed)
	if len(currencyCode) != 3 {
		http.Error(w, "Invalid currency code format (must be 3 letters)", http.StatusBadRequest)
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid start_date format (use YYYY-MM-DD): %v", err), http.StatusBadRequest)
		return
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid end_date format (use YYYY-MM-DD): %v", err), http.StatusBadRequest)
		return
	}

	// --- Database Query ---
	// Ensure you have this query defined for your foreign_exchange table
	dbParams := database.GetForeignExchangeByCurrencyAndDateRangeParams{
		CurrencyCode: currencyCode,
		StartDate:    startDate,
		EndDate:      endDate,
	}

	log.Printf("API: Querying FX rates for %s from %s to %s", currencyCode, startDateStr, endDateStr)
	dbResults, err := s.state.db.GetForeignExchangeByCurrencyAndDateRange(r.Context(), dbParams)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("API: No FX rate data found for %s between %s and %s", currencyCode, startDateStr, endDateStr)
			sendJsonResponse(w, []TimeSeriesDataPoint{}) // Send empty array
			return
		}
		log.Printf("API Error: Database error fetching FX rates for %s: %v", currencyCode, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// --- Format Response ---
	response := make([]TimeSeriesDataPoint, 0, len(dbResults))
	for _, dbRow := range dbResults {
		// *** CRUCIAL: Decide which rate to use (Middle, Buying, Selling?) ***
		// Using MiddleRate as an example here. Adjust if needed.

		value, err := strconv.ParseFloat(dbRow.MiddleRate, 64)
		if err != nil {
			log.Printf("Error parsing MiddleRate: %v", err)
			// Handle the error, e.g., skip this row or return an error
			continue
		}
		// Also assumes MiddleRate is float64 (or convertible) from sqlc
		response = append(response, TimeSeriesDataPoint{
			Date:  dbRow.Date.Format("2006-01-02"), // Use the 'date' column from foreign_exchange
			Value: value,                           // Use the desired rate column
		})
	}

	log.Printf("API: Found %d FX rate records for %s", len(response), currencyCode)
	sendJsonResponse(w, response)
}

// --- Helper function to send JSON response ---
func sendJsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	// Optional: CORS Header
	// w.Header().Set("Access-Control-Allow-Origin", "*")

	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		log.Printf("API Error: Failed to encode JSON response: %v", err)
		// Attempt to send an internal error, though headers might be sent
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
