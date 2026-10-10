// Command catalog runs the product catalog service.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pavangolla/retail-learn/services/catalog/internal/config"
	"github.com/pavangolla/retail-learn/services/catalog/internal/server"
)

func main() {
	cfg := config.Load()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.New(server.TCPPinger{Addr: cfg.MongoAddr}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Shut down cleanly on Ctrl+C or SIGTERM (what Kubernetes sends).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("catalog listening on %s (mongo: %s)", srv.Addr, cfg.MongoAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
