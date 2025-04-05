package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Ernestlph/Malaysia-Econ-DB/internal/config" // Import config
)

func runHttpsServer(ctx context.Context, wg *sync.WaitGroup, shutdownChan chan struct{}, cfg config.Config) { // Add cfg parameter
	defer wg.Done() // Signal WaitGroup when this goroutine exits

	mux := http.NewServeMux()
	mux.HandleFunc("/", helloHandler)

	// Configure TLS
	tlsCfg := &tls.Config{ // Rename variable to tlsCfg
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}

	// Create the server instance
	srv := &http.Server{
		Addr:         cfg.ServerAddr, // Use config value
		Handler:      mux,
		TLSConfig:    tlsCfg, // Use renamed tlsCfg variable
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start the server in a new goroutine to allow for shutdown signaling
	go func() {
		log.Printf("Starting HTTPS server on %s", cfg.ServerAddr) // Use config value
		// ListenAndServeTLS always returns a non-nil error. After Shutdown or Close,
		// the returned error is http.ErrServerClosed.
		if err := srv.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile); err != nil && err != http.ErrServerClosed { // Use config values
			log.Fatalf("HTTPS server ListenAndServeTLS error: %v", err)
		}
		log.Println("HTTPS server stopped listening.")
	}()

	// Wait for shutdown signal (from CLI or OS)
	<-shutdownChan
	log.Println("Shutdown signal received, shutting down HTTPS server...")

	// Create a deadline context for shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second) // Allow 15 seconds
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTPS server graceful shutdown error: %v", err)
	} else {
		log.Println("HTTPS server gracefully stopped.")
	}
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request from %s for %s", r.RemoteAddr, r.URL.Path)
	fmt.Fprintf(w, "Hello from HTTPS server!\n")
}
