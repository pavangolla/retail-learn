// Command catalog runs the product catalog service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pavangolla/retail-learn/services/catalog/internal/catalog"
	"github.com/pavangolla/retail-learn/services/catalog/internal/config"
	"github.com/pavangolla/retail-learn/services/catalog/internal/mongostore"
	"github.com/pavangolla/retail-learn/services/catalog/internal/server"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := config.Load()

	// Fail fast: if MongoDB is unreachable at startup, exit with a clear
	// message and let the orchestrator (Docker, Kubernetes) restart us.
	startCtx, cancelStart := context.WithTimeout(context.Background(), 15*time.Second)
	store, err := mongostore.Connect(startCtx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		cancelStart()
		return fmt.Errorf("startup: %w (is `docker compose up -d` running?)", err)
	}
	defer store.Close(context.Background())
	if err := store.EnsureIndexes(startCtx); err != nil {
		cancelStart()
		return fmt.Errorf("startup: %w", err)
	}
	cancelStart()

	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: server.New(server.Deps{
			Mongo:   store,
			Catalog: catalog.NewService(store),
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Shut down cleanly on Ctrl+C or SIGTERM (what Kubernetes sends).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("catalog listening on %s (db %q)", srv.Addr, cfg.MongoDB)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
	}

	log.Println("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
