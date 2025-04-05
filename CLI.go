package main

import (
	"context" // Needed for transactions
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	// Needed for fxclient.FxRate
	"github.com/Ernestlph/Malaysia-Econ-DB/internal/config"
	"github.com/Ernestlph/Malaysia-Econ-DB/internal/database"
	"github.com/Ernestlph/Malaysia-Econ-DB/internal/fxclient"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type state struct {
	db     *database.Queries // sqlc queries
	dbConn *sql.DB           // Raw DB connection for dynamic queries
	cfg    *config.Config    // Use imported config type
}

func parseArgs() (cmdName string, cmdArgs []string, err error) {
	if len(os.Args) == 1 {
		fmt.Println("Error: not enough arguments were provided")
		os.Exit(1)
		return
	}
	if (len(os.Args) == 2) && ((os.Args[1] == "login") || (os.Args[1] == "register")) {
		fmt.Println("Error: a username is required")
		os.Exit(1)
		return
	}
	if len(os.Args) < 2 && os.Args[1] != "reset" {
		fmt.Println("Error: unknown command")
		os.Exit(1)
		return
	}
	cmdName = os.Args[1]
	cmdArgs = os.Args[2:]
	return cmdName, cmdArgs, nil

} // Removed extra brace here

func runCli(cancelFunc context.CancelFunc, wg *sync.WaitGroup, shutdownChan chan struct{}, cfg config.Config) { // Re-added runCli function definition
	defer wg.Done() // Signal WaitGroup when this goroutine exits
	defer func() {
		// Ensure shutdown is triggered if CLI exits for any reason
		log.Println("CLI exiting, signaling shutdown...")
		close(shutdownChan) // Signal server to shut down
		cancelFunc()        // Cancel the main context if needed
	}()

	log.Println("Starting CLI. Type 'exit' or 'quit' to stop.")
	// Parse command line args first
	cmdName, cmdArgs, err := parseArgs()
	if err != nil {
		log.Fatal(err)
	}

	// Config is now passed in, no need to read it here.

	// Open database connection using passed-in config
	db, err := sql.Open("postgres", cfg.DBURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close() // Ensure database connection is closed

	// Create queries object
	dbQueries := database.New(db)

	// Create a state object, using the passed-in config and DB connection
	programState := &state{
		cfg:    &cfg, // Use passed-in cfg
		db:     dbQueries,
		dbConn: db, // Store the raw DB connection
	}

	// Create commands struct and initializes empty map
	cmds := commands{
		registeredCommands: make(map[string]func(*state, command) error),
	}

	// Register commands
	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerResetDatabase)
	cmds.register("users", handlerGetUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("feeds", handlerFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollowFeed))
	cmds.register("fx:fetch", handlerFxFetch) // Register new fx:fetch command

	// Runs command if command not found return error with code 1
	err = cmds.run(programState, command{Name: cmdName, Args: cmdArgs})
	if err != nil {
		if err.Error() == "unknown command" {
			fmt.Printf("Error: %s\n", err) // Print error to stderr
			os.Exit(1)
		}
		log.Fatal(err) // Log other fatal errors
	}
}

// --- FX Command Handlers ---

// handlerFxFetch fetches latest FX rates from the API and stores them in the database.
// Usage: go run . fx:fetch <base_currency> <target_currency1> [target_currency2...]
// Example: go run . fx:fetch MYR SGD EUR JPY
func handlerFxFetch(s *state, cmd command) error {
	if len(cmd.Args) < 2 {
		return fmt.Errorf("usage: %s <base_currency> <target_currency1> [target_currency2...]", cmd.Name)
	}
	baseCurrency := strings.ToUpper(cmd.Args[0])
	targetCurrencies := make([]string, len(cmd.Args)-1)
	for i, target := range cmd.Args[1:] {
		targetCurrencies[i] = strings.ToUpper(target)
	}

	log.Printf("Fetching FX rates for %v against base %s", targetCurrencies, baseCurrency)

	// Ensure API Base URL is configured
	if s.cfg.FXAPIBaseURL == "" {
		return fmt.Errorf("FX_API_BASE_URL is not configured in environment variables or .env file")
	}
	// Ensure API Key is configured
	if s.cfg.FXAPIKey == "" {
		return fmt.Errorf("FX_API_KEY is not configured in environment variables or .env file")
	}

	// Create FX client
	client := fxclient.New(*s.cfg, s.cfg.FXAPIBaseURL)

	// Fetch rates from API (using the placeholder implementation for now)
	rates, err := client.FetchLatestRates(baseCurrency, targetCurrencies)
	if err != nil {
		return fmt.Errorf("failed to fetch FX rates: %w", err)
	}

	log.Printf("Fetched %d rates. Storing to database...", len(rates))

	// Store rates in the database (dynamic SQL due to schema)
	// Use the raw *sql.DB connection stored in the state
	dbConn := s.dbConn // Use the connection from the state

	tx, err := dbConn.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("failed to begin database transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if anything fails

	for target, rateData := range rates {
		tableName := fmt.Sprintf("fx_%s", target) // Construct table name dynamically
		query := fmt.Sprintf("INSERT INTO %s (id, exchange_rate, created_at) VALUES ($1, $2, $3)", tableName)

		newID := uuid.New()
		_, err := tx.ExecContext(context.Background(), query, newID, rateData.Rate, rateData.Timestamp)
		if err != nil {
			// Log the specific error and which currency failed
			log.Printf("Error inserting rate for %s into %s: %v", target, tableName, err)
			// Optionally, decide whether to continue with other currencies or fail the whole batch
			// For now, we'll let the rollback handle failure of any single insert.
			return fmt.Errorf("failed to insert rate for %s: %w", target, err)
		}
		log.Printf("Stored rate for %s (ID: %s)", target, newID)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit database transaction: %w", err)
	}

	log.Println("Successfully fetched and stored FX rates.")
	return nil
}

// --- Stub Functions ---

func handlerLogin(s *state, cmd command) error {
	log.Printf("Stub: Login command executed with args: %v", cmd.Args)
	// TODO: Implement login logic
	return nil
}

func handlerRegister(s *state, cmd command) error {
	log.Printf("Stub: Register command executed with args: %v", cmd.Args)
	// TODO: Implement register logic
	return nil
}

func handlerResetDatabase(s *state, cmd command) error {
	log.Println("Stub: Reset database command executed.")
	// TODO: Implement database reset logic
	return nil
}

func handlerGetUsers(s *state, cmd command) error {
	log.Println("Stub: Get users command executed.")
	// TODO: Implement get users logic
	return nil
}

func handlerAgg(s *state, cmd command) error {
	log.Println("Stub: Agg command executed.")
	// TODO: Implement agg logic
	return nil
}

func handlerFeeds(s *state, cmd command) error {
	log.Println("Stub: Feeds command executed.")
	// TODO: Implement feeds logic
	return nil
}

func handlerFollowFeed(s *state, cmd command) error {
	log.Printf("Stub: Follow feed command executed with args: %v", cmd.Args)
	// TODO: Implement follow feed logic
	return nil
}

// Define a type for the handler function signature used by middleware
type authedHandler func(*state, command) error // Assuming this is the signature middleware expects

func middlewareLoggedIn(next authedHandler) func(*state, command) error { // Ensure return type matches registration
	return func(s *state, cmd command) error {
		log.Println("Stub: middlewareLoggedIn executed.")
		// TODO: Implement actual authentication check
		// For now, just call the next handler
		return next(s, cmd)
	}
}
