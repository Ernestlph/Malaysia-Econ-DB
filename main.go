package main

import (
	"context"
	"database/sql" // Import database/sql
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time" // Import time for DB connection timeout

	"github.com/Ernestlph/Malaysia-Econ-DB/internal/config"   // Import config package
	"github.com/Ernestlph/Malaysia-Econ-DB/internal/database" // Import database package
	_ "github.com/lib/pq"                                     // Import PostgreSQL driver
)

// --- state struct definition (as shown above, or imported) ---
type AppState struct {
	db     *database.Queries
	dbConn *sql.DB // Keep if raw connection needed, otherwise remove
	cfg    *config.Config
}

// --- End Struct Definition ---

func main() {
	log.Println("Application starting...")

	// --- Load Configuration ---
	cfg, err := config.Read() // Load config first
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Check if certificate files exist (remains the same)
	if _, err := os.Stat(cfg.CertFile); os.IsNotExist(err) {
		log.Printf("Warning: Certificate file not found at %s. HTTPS server might fail.", cfg.CertFile)
	}
	if _, err := os.Stat(cfg.KeyFile); os.IsNotExist(err) {
		log.Printf("Warning: Key file not found at %s. HTTPS server might fail.", cfg.KeyFile)
	}

	// --- Establish Database Connection ---
	log.Println("Connecting to database...")
	// Use a context with timeout for the initial connection attempt
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dbCancel() // Ensure the context is cancelled

	dbConn, err := sql.Open("postgres", cfg.DBURL) // Use cfg.DBURL loaded earlier
	if err != nil {
		// This error is rare (e.g., driver not found), but fatal
		log.Fatalf("FATAL: Failed to prepare database connection: %v", err)
	}
	// Defer closing the connection pool until main function exits
	defer func() {
		log.Println("Closing database connection pool...")
		if err := dbConn.Close(); err != nil {
			log.Printf("Error closing database connection: %v", err)
		}
	}()

	// Verify the connection is actually working
	err = dbConn.PingContext(dbCtx)
	if err != nil {
		log.Fatalf("FATAL: Failed to connect to database: %v", err)
	}
	log.Println("Database connection successful.")

	// --- Create Shared Application State ---
	dbQueries := database.New(dbConn) // Initialize sqlc queries
	programState := &AppState{        // Create the state struct instance
		db:     dbQueries,
		dbConn: dbConn, // Pass raw connection if needed by any handler
		cfg:    &cfg,   // Pass pointer to the loaded config
	}

	// --- Setup for Graceful Shutdown (remains the same) ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure context is cancelled on exit

	var wg sync.WaitGroup
	shutdownChan := make(chan struct{}, 1) // Buffered channel

	// --- Goroutine Setup ---
	wg.Add(2) // Expecting two goroutines (server + CLI)

	// Start HTTPS server, passing the shared programState
	go runHttpsServer(ctx, &wg, shutdownChan, programState) // <<< MODIFIED: Pass programState

	// Start CLI, passing the shared programState and cancel func
	go runCli(cancel, &wg, shutdownChan, programState) // <<< MODIFIED: Pass programState

	// --- Graceful Shutdown Handling (OS Signals - remains the same) ---
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		log.Printf("Received OS signal: %v. Initiating shutdown...", sig)
		// Non-blocking send to shutdownChan
		select {
		case shutdownChan <- struct{}{}:
		default:
		}
		cancel() // Cancel the main context
	case <-shutdownChan:
		log.Println("Shutdown initiated by CLI.")
		cancel() // Ensure context is cancelled
	}

	// --- Wait for Goroutines (remains the same) ---
	log.Println("Waiting for goroutines to finish...")
	wg.Wait()

	log.Println("Application finished.")
}
