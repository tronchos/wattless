package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tronchos/wattless/server/internal/config"
	"github.com/tronchos/wattless/server/internal/hosting"
	apihttp "github.com/tronchos/wattless/server/internal/http"
	"github.com/tronchos/wattless/server/internal/insights"
	"github.com/tronchos/wattless/server/internal/scanner"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid_config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	hostingClient := hosting.NewClient(cfg.GreencheckBaseURL, cfg.RequestTimeout)
	ruleBasedProvider := insights.NewRuleBasedProvider()
	var insightProvider insights.Provider = ruleBasedProvider
	if cfg.AIProvider == "gemini" && cfg.GeminiAPIKey != "" {
		geminiProvider := insights.NewGeminiProvider(cfg.GeminiAPIKey, cfg.GeminiModel, cfg.LLMTimeout)
		insightProvider = insights.NewCompositeProvider(geminiProvider, ruleBasedProvider)
	}
	scanService := scanner.NewService(cfg, hostingClient, insightProvider, logger)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           apihttp.NewRouter(cfg, scanService, logger),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	logger.Info("server_starting", "addr", server.Addr)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server_failed", "error", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	logger.Info("server_shutting_down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown_failed", "error", err)
		os.Exit(1)
	}
}
