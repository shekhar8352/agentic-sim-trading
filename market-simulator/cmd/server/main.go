package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agentic-sim-trading/market-simulator/internal/api"
	"github.com/agentic-sim-trading/market-simulator/internal/clock"
	"github.com/agentic-sim-trading/market-simulator/internal/db"
	"github.com/agentic-sim-trading/market-simulator/internal/market"
	"github.com/agentic-sim-trading/market-simulator/internal/portfolio"
	"github.com/agentic-sim-trading/market-simulator/internal/redisconn"
)

func main() {
	ctx := context.Background()

	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	if pool != nil {
		defer pool.Close()
	}

	rdb := redisconn.New()

	data := market.NewData(pool)
	pm := portfolio.NewManager(pool)
	reg := clock.NewRegistry(pool, rdb, data)

	h := &api.Handler{
		DB:        pool,
		Redis:     rdb,
		Market:    data,
		Portfolio: pm,
		Clocks:    reg,
	}

	addr := ":8070"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}

	r := api.NewRouter(h)
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("market-simulator listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
