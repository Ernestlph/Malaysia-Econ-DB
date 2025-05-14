package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http" // Required for HTTP requests
	"strconv"  // Required for converting string price to float
	"strings"
	"time"

	"github.com/Ernestlph/Malaysia-Econ-DB/internal/database" // Your sqlc generated package

	"github.com/PuerkitoBio/goquery" // Import goquery
)

// handlerStockFetchPrice scrapes the last price for a given stock code from i3investor
// Usage: stock:fetch:price <stock_code>
// Example: stock:fetch:price 1155
func handlerStockFetchPrice(s *AppState, cmd command) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: %s <stock_code>", cmd.Name)
	}
	stockCode := cmd.Args[0]
	profileURL := s.cfg.I3InvestorBaseURL + stockCode

	log.Printf("Fetching stock price for %s from %s", stockCode, profileURL)

	// --- Step 1: Fetch HTML Content ---
	// Create a client (good practice to reuse clients, but okay here for CLI command)
	client := &http.Client{
		Timeout: 15 * time.Second, // Set a timeout
	}
	req, err := http.NewRequest("GET", profileURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for %s: %w", profileURL, err)
	}
	// Set a User-Agent header, as some sites block requests without one
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch URL %s: %w", profileURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 status code %d from %s", resp.StatusCode, profileURL)
	}

	// --- Step 2: Parse HTML using goquery ---
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to parse HTML from %s: %w", profileURL, err)
	}

	// --- Step 3: Find the Target Element and Extract Price ---
	var priceStr string
	var found bool

	// Find the specific div structure
	// Iterate over potential divs, look for the one preceded by "Last Price" text.
	// This selector targets divs that are likely containers for stock stats.
	doc.Find("div.col-md-3.col-6").EachWithBreak(func(i int, s *goquery.Selection) bool {
		// Check the first <p> tag within the div for the label "Last Price"
		labelText := s.Find("p").First().Text()
		if strings.Contains(labelText, "Last Price") {
			// If label matches, find the price in the <p><strong> structure within the *same* div
			priceSelection := s.Find("p > strong") // Look for strong tag within any p tag in this div
			if priceSelection.Length() > 0 {
				priceStr = priceSelection.First().Text() // Get text from the first strong tag found
				found = true
				return false // Stop iterating once found
			}
		}
		return true // Continue iterating
	})

	if !found || priceStr == "" {
		return fmt.Errorf("could not find 'Last Price' element or value on page %s", profileURL)
	}

	log.Printf("Found raw price string: '%s'", priceStr)

	// --- Step 4: Clean and Convert Price ---
	priceStr = strings.TrimSpace(priceStr)
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return fmt.Errorf("failed to parse price string '%s' to float: %w", priceStr, err)
	}

	log.Printf("Parsed price: %.4f", price)

	// --- Step 5: Prepare Data for Database ---
	// Use today's date (UTC). You might adjust this if the site indicates a specific date.
	priceDate := time.Now().UTC()

	// --- Step 6: Insert/Update Database ---
	log.Printf("Upserting price %.4f for %s on %s into database...", price, stockCode, priceDate.Format("2006-01-02"))

	err = s.db.UpsertStockPrice(context.Background(), database.UpsertStockPriceParams{
		StockCode:    stockCode,
		PriceDate:    priceDate, // sqlc should handle time.Time -> DATE conversion
		ClosingPrice: fmt.Sprintf("%.4f", price),
		SourceUrl:    sql.NullString{String: profileURL, Valid: true}, // Use sql.NullString for optional columns
	})

	if err != nil {
		return fmt.Errorf("failed to upsert stock price for %s: %w", stockCode, err)
	}

	log.Printf("Successfully stored stock price for %s.", stockCode)
	fmt.Printf("Fetched and stored price for %s: %.4f\n", stockCode, price) // User feedback

	return nil
}

func handlerStockFetchPriceAll(s *AppState, cmd command) error {
	if len(cmd.Args) != 0 {
		return fmt.Errorf("usage: %s", cmd.Name)
	}

	// Fetch all stock codes from the database
	stockCodes := s.cfg.StockList

	// Iterate over each stock code and fetch its price
	for _, stockCode := range stockCodes {
		cmd := command{
			Name: "stock:fetch:price",
			Args: []string{stockCode},
		}
		if err := handlerStockFetchPrice(s, cmd); err != nil {
			log.Printf("Failed to fetch price for %s: %v", stockCode, err)
		}
	}

	return nil
}

// --- Helper function to extract text after a specific label ---
// Example: extractTextAfterLabel(pSelection, "Country Code:") returns "MY"
func extractTextAfterLabel(pSelection *goquery.Selection, label string) string {
	rawText := pSelection.Text()
	// Remove the label part and any leading/trailing spaces or hidden elements
	// The HTML has <a href="#" class="d-none">MY</a> MY
	// So, .Text() might give "MY MY" or "Country Code: MY MY"
	// We need to be careful to get the visible text.

	// Try to get the text of the last text node directly to avoid hidden 'a' tags.
	// This is a bit more robust if the structure is consistent.
	var visibleText string
	pSelection.Contents().Each(func(i int, s *goquery.Selection) {
		if goquery.NodeName(s) == "#text" { // Check if it's a text node
			trimmedNodeText := strings.TrimSpace(s.Text())
			if trimmedNodeText != "" {
				visibleText = trimmedNodeText // Keep the last non-empty text node
			}
		}
	})

	if visibleText != "" {
		// If label is present in rawText (e.g., "Country Code: MY"), remove it
		if strings.Contains(rawText, label) {
			parts := strings.SplitN(rawText, label, 2)
			if len(parts) > 1 {
				// Re-check visibleText if the label was part of the rawText
				// This logic might need more refinement based on goquery's .Text() behavior with mixed content
				checkVisible := strings.TrimSpace(parts[1])
				if strings.HasSuffix(checkVisible, visibleText) { // if "MY MY" contains "MY"
					return visibleText
				}
				return checkVisible // Fallback if visibleText logic didn't catch it right
			}
		}
		return visibleText // Return the last visible text node found
	}

	// Fallback if direct text node extraction didn't work well
	textAfterLabel := strings.TrimSpace(strings.TrimPrefix(rawText, label))
	// If there are multiple instances of the value (due to hidden 'a' tag), split by space and take the last part
	parts := strings.Fields(textAfterLabel)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func handlerStockFetchProfile(s *AppState, cmd command) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("usage: %s <stock_code>", cmd.Name)
	}

	stockCode := cmd.Args[0]
	// Ensure this URL points to the overview/profile page
	profileURL := s.cfg.I3InvestorStockProfileURL + stockCode

	log.Printf("Fetching stock profile for %s from %s", stockCode, profileURL)

	// --- Step 1: Fetch HTML Content (remains the same) ---
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", profileURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for %s: %w", profileURL, err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch URL %s: %w", profileURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 status code %d from %s", resp.StatusCode, profileURL)
	}

	// --- Step 2: Parse HTML using goquery ---
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to parse HTML from %s: %w", profileURL, err)
	}

	// --- Step 3: Extract Profile Information ---
	var companyName, countryCode, sector, subsector string

	// --- Extract Company Name from the main heading first (more reliable) ---
	// Selector for: <h5 class="mb-0" id="stock-heading" ...> <a ...> <strong>COMPANY NAME</strong> </a> </h5>
	companyName = strings.TrimSpace(doc.Find("h5#stock-heading a strong").First().Text())
	if companyName == "" {
		// Fallback: try the h6 within a potential profile section if the main one fails
		// This targets an h6 that is a sibling of an h5 containing "Profile"
		doc.Find("h5").EachWithBreak(func(i int, h5 *goquery.Selection) bool {
			if strings.TrimSpace(h5.Text()) == "Profile" {
				companyName = strings.TrimSpace(h5.NextFiltered("h6").Find("strong").First().Text())
				return false // Stop searching
			}
			return true
		})
	}
	if companyName == "" {
		log.Printf("Warning: Could not find company name for %s using primary selectors.", stockCode)
	}

	// --- Extract other details from the specific profile info div ---
	// Selector for: <div class="row" id="profile-info">
	profileInfoDiv := doc.Find("div#profile-info").First() // Target by ID is more specific

	if profileInfoDiv.Length() == 0 {
		log.Printf("Warning: Could not find 'div#profile-info' for %s. Profile details might be missing.", stockCode)
	} else {
		profileInfoDiv.Find("p").Each(func(i int, p *goquery.Selection) {
			text := p.Text()
			// Using Contains is okay, but StartsWith might be slightly more robust if labels are always at the beginning
			if strings.Contains(text, "Country Code:") {
				countryCode = extractTextAfterLabel(p, "Country Code:")
			} else if strings.Contains(text, "Sector:") {
				sector = extractTextAfterLabel(p, "Sector:")
			} else if strings.Contains(text, "Subsector:") {
				subsector = extractTextAfterLabel(p, "Subsector:")
			}
		})
	}

	log.Printf("Extracted Profile for %s: Name='%s', Country='%s', Sector='%s', Subsector='%s'",
		stockCode, companyName, countryCode, sector, subsector)

	// Modify the check: Company Name is the most critical piece.
	// If only company name is found, maybe that's acceptable for an initial insert.
	if companyName == "" { // Only fail if company name is absolutely missing
		return fmt.Errorf("failed to extract company name for %s from %s. Check HTML structure and selectors", stockCode, profileURL)
	}
	if countryCode == "" && sector == "" && subsector == "" {
		log.Printf("Warning: Extracted company name '%s' for %s, but other profile details (country, sector, subsector) are missing.", companyName, stockCode)
	}

	// --- Step 4: Store/Update in Database (companies table) ---
	// (This part remains the same as your previous working version, using sql.NullString)
	params := database.UpsertCompanyParams{
		StockCode:        stockCode,
		CompanyName:      companyName, // Should have a value if we passed the check above
		CountryCode:      sql.NullString{String: countryCode, Valid: countryCode != ""},
		Sector:           sql.NullString{String: sector, Valid: sector != ""},
		Subsector:        sql.NullString{String: subsector, Valid: subsector != ""},
		ListingDate:      sql.NullTime{Valid: false}, // Assuming not scraped yet
		ProfileSourceUrl: sql.NullString{String: profileURL, Valid: true},
	}

	err = s.db.UpsertCompany(context.Background(), params)
	if err != nil {
		return fmt.Errorf("failed to upsert company profile for %s: %w", stockCode, err)
	}

	log.Printf("Successfully stored/updated profile for stock %s.", stockCode)
	fmt.Printf("Profile for %s: Name: %s, Country: %s, Sector: %s, Subsector: %s\n",
		stockCode, companyName, countryCode, sector, subsector)

	return nil
}

// Your handlerStockFetchPriceAll can be modified to call this new handler
// for each stock to populate the companies table initially.
func handlerStockFetchPriceAllAndProfiles(s *AppState, cmd command) error { // Renamed for clarity
	if len(cmd.Args) != 0 {
		return fmt.Errorf("usage: %s (no arguments)", cmd.Name)
	}

	stockCodes := s.cfg.StockList
	if len(stockCodes) == 0 {
		log.Println("No stock codes found in configuration to fetch.")
		return nil
	}

	log.Printf("Starting to fetch prices and profiles for %d stocks.", len(stockCodes))

	for _, stockCode := range stockCodes {
		// Fetch Profile
		profileCmd := command{Name: "stock:fetch:profile", Args: []string{stockCode}}
		log.Printf("--- Fetching Profile for %s ---", stockCode)
		if err := handlerStockFetchProfile(s, profileCmd); err != nil {
			log.Printf("Failed to fetch/store profile for %s: %v", stockCode, err)
			// Decide if you want to continue to price fetching if profile fails
		} else {
			log.Printf("Profile for %s processed.", stockCode)
		}

		// Fetch Price (your existing logic)
		priceCmd := command{Name: "stock:fetch:price", Args: []string{stockCode}}
		log.Printf("--- Fetching Price for %s ---", stockCode)
		if err := handlerStockFetchPrice(s, priceCmd); err != nil {
			log.Printf("Failed to fetch/store price for %s: %v", stockCode, err)
		} else {
			log.Printf("Price for %s processed.", stockCode)
		}
		log.Println("--- --- ---")

		// Optional: Add a small delay to be polite to the server
		time.Sleep(500 * time.Millisecond) // 0.5 second delay
	}
	log.Println("Finished fetching all stock prices and profiles.")
	return nil
}
