package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	fxclient "github.com/Ernestlph/Malaysia-Econ-DB/internal/BNMApiClient"
	"github.com/Ernestlph/Malaysia-Econ-DB/internal/database"
	"github.com/google/uuid"
)

// --- FX Command Handlers ---

// handlerFxFetch fetches latest FX rates for all currencies from the API and stores them in the database.
func handlerFxFetchAll(s *AppState, cmd command) error {

	// Config checks remain the same
	if s.cfg.FXAPIBaseURL == "" {
		return fmt.Errorf("FX_API_BASE_URL is not configured")
	}

	// FX client creation remains the same
	client := fxclient.New(*s.cfg, s.cfg.FXAPIBaseURL) // Assuming New takes base URL

	// Fetch rates from API (using the placeholder implementation for now)
	rates, err := client.FetchLatestRatesAll()
	if err != nil {
		return fmt.Errorf("failed to fetch FX rates: %w", err)
	}
	for _, rate := range rates.Data {
		date, err := time.Parse("2006-01-02", rate.Rate.Date)
		if err != nil {
			return fmt.Errorf("failed to parse date: %w", err)
		}
		err = s.db.UpsertForeignExchange(context.Background(), database.UpsertForeignExchangeParams{
			CurrencyCode: rate.CurrencyCode,
			BuyingRate:   fmt.Sprintf("%.4f", rate.Rate.BuyingRate),
			SellingRate:  fmt.Sprint(rate.Rate.SellingRate),
			MiddleRate:   fmt.Sprintf("%.4f", rate.Rate.MiddleRate),
			CreatedAt:    time.Now(),
			Date:         date,
			ID:           uuid.New(),
		})
		if err != nil {
			log.Printf("Error storing FX rate for %s on %s: %v", rate.CurrencyCode, rate.Rate.Date, err)
			continue
		}
		log.Printf("Stored FX rate for %s with value of %.4f on %s", rate.CurrencyCode, rate.Rate.MiddleRate, rate.Rate.Date)

	}

	log.Printf("FX rates fetched and stored successfully")

	return nil
}

// handlerFxFetchRange fetches FX rates for a specific currency and date range from the API and stores them in the database.
func handlerFxFetchRange(s *AppState, cmd command) error {

	// Config checks remain the same
	if s.cfg.FXAPIBaseURL == "" {
		return fmt.Errorf("FX_API_BASE_URL is not configured")
	}
	if len(cmd.Args) != 3 {
		return fmt.Errorf("usage: %s <currency_code> <start_date YYYY-MM-DD> <end_date YYYY-MM-DD>", cmd.Name)
	}

	targetCurrency := strings.ToUpper(cmd.Args[0])
	startDate := cmd.Args[1]
	endDate := cmd.Args[2]

	// Validate Currency Code (Example)
	if len(targetCurrency) != 3 {
		return fmt.Errorf("invalid currency code format: %s (must be 3 letters)", targetCurrency)
	}

	// Parse the start dates
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return fmt.Errorf("failed to parse start date: %w", err)
	}
	// Parse the end date
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return fmt.Errorf("failed to parse end date: %w", err)
	}

	// Validate date range
	if end.Before(start) {
		return fmt.Errorf("end date must be after start date")
	}

	// Create slice that has all teh days from the start date to the end date
	var dates []string

	// Loop through the dates and add them to the slice
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d.Format("2006-01-02"))
	}

	if len(dates) == 0 {
		return fmt.Errorf("no dates found in the specified range")
	}

	log.Printf("Attempting to fetch FX rates for %s from %s to %s (%d days)", targetCurrency, startDate, endDate, len(dates))

	// Create API client
	client := fxclient.New(*s.cfg, s.cfg.FXAPIBaseURL) // Assuming New takes base URL

	var successfulFetches, failedFetches, successfulStores, failedStores int

	// Fetch rate from API for each date
	for _, dateStr := range dates {
		// Fetch rate for that date
		rateResponse, err := client.FetchTargetCurrencyRates(targetCurrency, dateStr)
		if err != nil {
			log.Printf("Failed to fetch FX rate for %s on %s: %v", targetCurrency, dateStr, err)
			failedFetches++
			continue // Continue to next date
		}
		successfulFetches++

		// Assuming the first entry is the one we want for that date/currency
		rateData := rateResponse.Data

		// Parse date
		parsedDate, err := time.Parse("2006-01-02", rateData.Rate.Date)
		if err != nil {
			log.Printf("Failed to parse date %s: %v", rateData.Rate.Date, err)
			failedStores++
			continue // Try next date
		}

		// Call UPSERT function
		err = s.db.UpsertForeignExchange(context.Background(), database.UpsertForeignExchangeParams{
			CurrencyCode: targetCurrency,
			BuyingRate:   fmt.Sprintf("%.4f", rateData.Rate.BuyingRate),
			SellingRate:  fmt.Sprint(rateData.Rate.SellingRate),
			MiddleRate:   fmt.Sprintf("%.4f", rateData.Rate.MiddleRate),
			CreatedAt:    time.Now(),
			Date:         parsedDate,
			ID:           uuid.New(),
		})
		if err != nil {
			log.Printf("Error storing FX rate for %s on %s: %v", targetCurrency, parsedDate, err)
			failedStores++
			continue
		}
		successfulStores++
		log.Printf("Stored FX rate for %s with value of %.4f on %s", targetCurrency, rateData.Rate.MiddleRate, parsedDate)

	}

	// Log summary
	log.Printf("FX rate fetching complete for range %s to %s.", startDate, endDate)
	log.Printf("API Fetches: %d successful, %d failed.", successfulFetches, failedFetches)
	log.Printf("Database Stores/Updates: %d successful, %d failed.", successfulStores, failedStores)

	return nil

}
