package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/Ernestlph/Malaysia-Econ-DB/internal/config" // Import config package
	_ "github.com/lib/pq"
)

func main() {
	log.Println("Application starting...")

	// --- Load Configuration ---
	cfg, err := config.Read()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Check if certificate files exist using config values
	if _, err := os.Stat(cfg.CertFile); os.IsNotExist(err) {
		log.Printf("Warning: Certificate file not found at %s. HTTPS server might fail.", cfg.CertFile)
		// Consider if this should be fatal depending on requirements
		// log.Fatalf("Certificate file not found: %s. Generate one first.", cfg.CertFile)
	}
	if _, err := os.Stat(cfg.KeyFile); os.IsNotExist(err) {
		log.Printf("Warning: Key file not found at %s. HTTPS server might fail.", cfg.KeyFile)
		// Consider if this should be fatal depending on requirements
		// log.Fatalf("Key file not found: %s. Generate one first.", cfg.KeyFile)
	}

	// Context for overall cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Make sure all paths cancel the context

	// WaitGroup to wait for goroutines to finish
	var wg sync.WaitGroup

	// Channel to signal server shutdown
	// Use a buffered channel of size 1 to prevent the sender from blocking
	// if the receiver isn't ready immediately (e.g., during OS signal handling).
	shutdownChan := make(chan struct{}, 1)

	// --- Goroutine Setup ---
	wg.Add(2) // Expecting two goroutines (server + CLI)

	// Start HTTPS server in a goroutine, passing config
	go runHttpsServer(ctx, &wg, shutdownChan, cfg)

	// Start CLI in a goroutine, passing config and cancel func
	go runCli(cancel, &wg, shutdownChan, cfg)

	// --- Graceful Shutdown Handling (OS Signals) ---
	// Wait for interrupt signal (Ctrl+C) or SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Block until a signal is received OR shutdownChan is closed by the CLI
	select {
	case sig := <-sigChan:
		log.Printf("Received OS signal: %v. Initiating shutdown...", sig)
		// Signal shutdown if not already signaled by CLI
		select {
		case shutdownChan <- struct{}{}:
			// Successfully sent signal
		default:
			// Channel already closed or full, means shutdown already initiated
		}
		cancel() // Cancel the main context
	case <-shutdownChan:
		// Shutdown was initiated by the CLI, just proceed
		log.Println("Shutdown initiated by CLI.")
		cancel() // Ensure context is cancelled
	}

	// --- Wait for Goroutines ---
	log.Println("Waiting for goroutines to finish...")
	wg.Wait() // Wait for runHttpsServer and runCli to complete

	log.Println("Application finished.")
}
