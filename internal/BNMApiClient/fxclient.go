package fxclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Ernestlph/Malaysia-Econ-DB/internal/config"
)

// --- Structs for FetchLatestRatesAll (Multiple Rates) ---
type RateInfoMulti struct { // Renamed inner Rate struct slightly for clarity
	Date        string  `json:"date"`
	BuyingRate  float64 `json:"buying_rate"`  // Keep as float64 for now
	SellingRate float64 `json:"selling_rate"` // Keep as float64 for now
	MiddleRate  float64 `json:"middle_rate"`
}

type CurrencyRateMulti struct { // Renamed struct within the Data array
	CurrencyCode string        `json:"currency_code"`
	Unit         int           `json:"unit"`
	Rate         RateInfoMulti `json:"rate"`
}

type MultiRateApiResponse struct { // Renamed main struct for clarity
	Data []CurrencyRateMulti    `json:"data"` // Data is an ARRAY here
	Meta map[string]interface{} `json:"meta"`
}

// --- Structs for FetchTargetCurrencyRates (Single Rate) ---
type RateInfoSingle struct {
	Date        string  `json:"date"`
	BuyingRate  float64 `json:"buying_rate"`
	SellingRate float64 `json:"selling_rate"`
	MiddleRate  float64 `json:"middle_rate"`
}

type CurrencyRateSingle struct { // Renamed struct for the single data object
	CurrencyCode string         `json:"currency_code"`
	Unit         int            `json:"unit"`
	Rate         RateInfoSingle `json:"rate"`
}

type SingleRateApiResponse struct { // New struct for this specific endpoint
	Data CurrencyRateSingle     `json:"data"` // Data is an OBJECT here
	Meta map[string]interface{} `json:"meta"`
}

// --- Client Definition (Remains the same) ---
type Client struct {
	BaseURL    string
	APIKey     string
	httpClient *http.Client
}

func New(cfg config.Config, baseURL string) *Client { // No change needed
	return &Client{
		BaseURL: baseURL,
		APIKey:  cfg.FXAPIKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// --- Updated FetchTargetCurrencyRates ---
func (c *Client) FetchTargetCurrencyRates(targetCurrency string, targetDate string) (SingleRateApiResponse, error) { // Changed return type

	var apiResponse SingleRateApiResponse // Use the new struct type

	apiEndpoint := fmt.Sprintf("%s/%s/date/%s?session=1200&quote=rm", c.BaseURL, targetCurrency, targetDate)
	req, err := http.NewRequest("GET", apiEndpoint, nil)
	if err != nil {
		return apiResponse, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.BNM.API.v1+json")
	// Add Auth header if needed: req.Header.Set("apikey", c.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return apiResponse, fmt.Errorf("error making API request: %w", err)
	}
	defer resp.Body.Close()

	// Check for 404 specifically, treat it as "no data for this date" not necessarily a fatal error
	if resp.StatusCode == http.StatusNotFound {
		// Return the empty struct and a specific error or nil depending on how you want to handle it upstream
		return apiResponse, fmt.Errorf("API returned 404 Not Found for %s on %s (likely no data)", targetCurrency, targetDate)
	}

	if resp.StatusCode != http.StatusOK {
		return apiResponse, fmt.Errorf("API request failed with status code: %d %s", resp.StatusCode, resp.Status)
	}

	// Decode into the correct struct
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return apiResponse, fmt.Errorf("error decoding API response: %w", err)
	}

	// fmt.Println("This is in fxclient.go: For Target Rates API Response:", apiResponse) // Keep for debugging if needed

	return apiResponse, nil
}

// --- Updated FetchLatestRatesAll ---
func (c *Client) FetchLatestRatesAll() (MultiRateApiResponse, error) { // Changed return type

	var apiResponse MultiRateApiResponse // Use the struct where Data is an array

	apiEndpoint := fmt.Sprintf("%s?session=1200&quote=rm", c.BaseURL)
	req, err := http.NewRequest("GET", apiEndpoint, nil)
	if err != nil {
		return apiResponse, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.BNM.API.v1+json")
	// Add Auth header if needed: req.Header.Set("apikey", c.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return apiResponse, fmt.Errorf("error making API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return apiResponse, fmt.Errorf("API request failed with status code: %d %s", resp.StatusCode, resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return apiResponse, fmt.Errorf("error decoding API response: %w", err)
	}

	// fmt.Println("This is in fxclient.go: API Response:", apiResponse) // Keep for debugging if needed

	return apiResponse, nil
}
