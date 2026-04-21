package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aminsalami/funpokedex/internal/config"
	"github.com/aminsalami/funpokedex/internal/handler"
	"github.com/aminsalami/funpokedex/internal/middleware"
	"github.com/aminsalami/funpokedex/internal/pkg"
	"github.com/aminsalami/funpokedex/internal/pokeapi"
	"github.com/aminsalami/funpokedex/internal/service"
	"github.com/aminsalami/funpokedex/internal/translator"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	httpClient := pkg.NewHTTPClient(cfg.HTTPTimeout)
	cache := pkg.NewTTLCache(cfg.CacheTTL)

	pokeClient := pokeapi.NewClient(httpClient, cfg.PokeAPIBaseURL)

	svc := service.NewPokemonService(pokeClient, cache)
	svc.Register("yoda", translator.NewYodaTranslator(httpClient, cfg.TranslationsBaseURL))
	svc.Register("shakespeare", translator.NewShakespeareTranslator(httpClient, cfg.TranslationsBaseURL))
	h := handler.NewHandler(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		// TODO: real healthcheck!
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /pokemon/{name}", h.GetPokemon)
	mux.HandleFunc("GET /pokemon/translated/{name}", h.GetTranslatedPokemon)

	app := middleware.Chain(
		mux,
		middleware.Recovery,
		middleware.RequestID,
		middleware.Logger,
		middleware.RateLimit(cfg.RateLimitRPS),
		middleware.Timeout(cfg.HTTPTimeout),
	)

	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      app,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: cfg.HTTPTimeout + 1*time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("starting server", "addr", addr)
		errCh <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		slog.Info("shutting down", "signal", sig.String())
	case err := <-errCh:
		slog.Error("server error", "error", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
