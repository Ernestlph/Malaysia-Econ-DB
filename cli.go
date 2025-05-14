package main

import (
	"bufio" // Needed for reading user input line by line
	"context"

	// "database/sql" // No longer needed directly here for setup (might be needed by handlers)
	"fmt"
	"io" // Needed for EOF check
	"log"
	"os"
	"strings"
	"sync"
	// Keep these if handlers or the state struct reference them
	// "github.com/Ernestlph/Malaysia-Econ-DB/internal/config"
	// "github.com/Ernestlph/Malaysia-Econ-DB/internal/database"
	// No longer needed here, only in main.go
	// _ "github.com/lib/pq"
)

// --- Interactive CLI Function ---
// --- MODIFIED: Accept programState *state instead of cfg config.Config ---
func runCli(cancelFunc context.CancelFunc, wg *sync.WaitGroup, shutdownChan chan struct{}, programState *AppState) {
	defer wg.Done() // Signal WaitGroup when this goroutine exits
	defer func() {
		// Ensure shutdown is triggered if CLI exits for any reason
		log.Println("CLI exiting, signaling shutdown...")
		select {
		case <-shutdownChan:
		default:
			close(shutdownChan) // Signal server to shut down
		}
		cancelFunc() // Cancel the main context if needed
	}()

	log.Println("Starting Interactive CLI. Type 'help' for commands, 'exit' or 'quit' to stop.")

	// --- Command Registration ---
	// Create commands struct and initialize empty map
	cmds := commands{
		registeredCommands: make(map[string]func(*AppState, command) error),
	}

	// Register commands (This part remains the same, it uses the programState parameter)
	cmds.register("help", handlerHelp)
	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerResetDatabase)
	cmds.register("users", handlerGetUsers)
	cmds.register("testing", handlerTesting)
	cmds.register("fx:fetch_all", handlerFxFetchAll)
	cmds.register("fx:fetch:range", handlerFxFetchRange)
	cmds.register("stock:fetch:price", handlerStockFetchPrice)
	cmds.register("stock:fetch:price_all", handlerStockFetchPriceAll) // Renamed command key slightly for consistency
	cmds.register("stock:fetch:profile", handlerStockFetchProfile)
	cmds.register("stock:fetch:profile_all", handlerStockFetchPriceAllAndProfiles) // Renamed command key slightly for consistency

	// --- Input Loop ---
	scanner := bufio.NewReader(os.Stdin) // Reader for standard input

	for {
		fmt.Print("Malaysian Econ DB > ") // Display prompt

		input, err := scanner.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				log.Println("CLI received EOF, exiting.")
			} else {
				log.Printf("CLI Error reading input: %v", err)
			}
			return // Exit the loop and the function
		}

		cleanInput := strings.TrimSpace(input)
		if cleanInput == "" {
			continue
		}

		if cleanInput == "exit" || cleanInput == "quit" {
			log.Println("Exit command received.")
			return // Exit the loop and function
		}

		parts := strings.Fields(cleanInput)
		cmdName := parts[0]
		cmdArgs := parts[1:]

		cmdToRun := command{
			Name: cmdName,
			Args: cmdArgs,
		}

		// --- Execute the command using the PASSED-IN programState ---
		err = cmds.run(programState, cmdToRun)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err) // Print execution errors
		}
	}
}

// --- Helper Command Handler (handlerHelp) ---
// (No changes needed here, assuming it doesn't depend on removed setup)
func handlerHelp(s *AppState, cmd command) error {
	fmt.Println("Available commands:")
	fmt.Println("  help                   - Show this help message")
	fmt.Println("  login <user>           - Log in (stub)")
	fmt.Println("  register <user>        - Register a new user (stub)")
	fmt.Println("  reset                  - Reset database (stub)")
	fmt.Println("  users                  - List users (stub)")
	fmt.Println("  fx:fetch_all           - Fetch latest FX rates for all currencies")
	fmt.Println("  fx:fetch:range <CUR> <START> <END> - Fetch FX rates for CUR between dates (YYYY-MM-DD)")
	fmt.Println("  stock:fetch:price <CODE> - Fetch latest price for stock CODE")
	fmt.Println("  stock:fetch:price_all  - Fetch latest price for all stocks in config list") // Corrected command name
	fmt.Println("  testing                - Simple test command")
	fmt.Println("  exit / quit            - Stop the application")
	return nil
}

// --- Stub Functions (handlerLogin, handlerRegister, etc.) ---
// (No changes needed here as they receive the state 's')
func handlerLogin(s *AppState, cmd command) error         { /* ... */ return nil }
func handlerRegister(s *AppState, cmd command) error      { /* ... */ return nil }
func handlerResetDatabase(s *AppState, cmd command) error { /* ... */ return nil }
func handlerGetUsers(s *AppState, cmd command) error      { /* ... */ return nil }
func handlerTesting(s *AppState, cmd command) error       { /* ... */ return nil }
