package fxclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Ernestlph/Malaysia-Econ-DB/internal/config"
)

// Client is a client for fetching FX data.
type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string // Base URL of the FX API
}

// FxRate represents a single exchange rate.
// Adjust this struct based on the actual API response.
type FxRate struct {
	BaseCurrency   string    `json:"base"`
	TargetCurrency string    `json:"target"` // Assuming API provides this, though DB schema doesn't store it directly
	Rate           float64   `json:"rate"`
	Timestamp      time.Time `json:"timestamp"` // Timestamp from the API provider
}

// New creates a new FX API client.
func New(cfg config.Config, apiBaseURL string) *Client {
	return &Client{
		apiKey: cfg.FXAPIKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: apiBaseURL, // e.g., "https://api.examplefx.com/v1"
	}
}

// FetchLatestRates fetches the latest rates for specified target currencies against a base currency.
// This is a placeholder and needs implementation based on the specific API.
func (c *Client) FetchLatestRates(baseCurrency string, targetCurrencies []string) (map[string]FxRate, error) {
	// TODO: Implement actual API call logic here.
	// This will involve:
	// 1. Constructing the API request URL (e.g., c.baseURL + "/latest?base=" + baseCurrency + "&symbols=" + strings.Join(targetCurrencies, ","))
	// 2. Adding necessary headers (e.g., Authorization with c.apiKey)
	// 3. Making the HTTP GET request using c.httpClient
	// 4. Reading and parsing the JSON response body
	// 5. Handling potential errors (network, API errors, JSON parsing)

	fmt.Printf("Placeholder: Fetching latest rates for %v against %s from %s\n", targetCurrencies, baseCurrency, c.baseURL)

	// Example placeholder response structure (replace with actual API call)
	mockRates := make(map[string]FxRate)
	for _, target := range targetCurrencies {
		mockRates[target] = FxRate{
			BaseCurrency:   baseCurrency,
			TargetCurrency: target,
			Rate:           4.50 + (float64(len(target)) * 0.01), // Mock rate
			Timestamp:      time.Now(),
		}
	}

	if c.apiKey == "" {
		return nil, fmt.Errorf("FX API key is not configured")
	}
	if c.baseURL == "" {
		return nil, fmt.Errorf("FX API base URL is not configured")
	}

	// Replace this mock return with actual API fetching logic
	return mockRates, nil // Return mock data for now
}

// --- Helper function example (you might need more) ---

func (c *Client) makeAPIRequest(endpoint string, target interface{}) error {
	reqURL := fmt.Sprintf("%s/%s", c.baseURL, endpoint)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create API request: %w", err)
	}

	// Add API Key header (adjust based on API requirements)
	req.Header.Set("Authorization", "Bearer "+c.apiKey) // Example, might be different
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute API request to %s: %w", reqURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// TODO: Read error body for more details if available
		return fmt.Errorf("API request failed with status %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("failed to decode API response: %w", err)
	}

	return nil
}
