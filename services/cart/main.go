// Command cart runs the shopping cart service.
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

	"github.com/pavangolla/retail-learn/services/cart/internal/cart"
	"github.com/pavangolla/retail-learn/services/cart/internal/catalogclient"
	"github.com/pavangolla/retail-learn/services/cart/internal/config"
	"github.com/pavangolla/retail-learn/services/cart/internal/redisstore"
	"github.com/pavangolla/retail-learn/services/cart/internal/server"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := config.Load()

	// Fail fast, like the catalog: no Redis, no cart service.
	startCtx, cancelStart := context.WithTimeout(context.Background(), 10*time.Second)
	store, err := redisstore.Connect(startCtx, cfg.RedisURL)
	cancelStart()
	if err != nil {
		return fmt.Errorf("startup: %w (is `docker compose up -d` running?)", err)
	}
	defer store.Close()

	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: server.New(server.Deps{
			Redis: store,
			Cart:  cart.NewService(store, catalogclient.New(cfg.CatalogURL)),
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("cart listening on %s (catalog at %s)", srv.Addr, cfg.CatalogURL)
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
