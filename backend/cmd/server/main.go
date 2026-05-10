// Command server is the composition root for the AI Mock Interview backend.
// Wires config -> logger -> pgx pool -> Clerk verifier -> Gemini client ->
// service -> router -> http.Server with graceful shutdown.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bhavisachdeva/ai-mock-interview-v2/backend/internal/auth"
	"github.com/bhavisachdeva/ai-mock-interview-v2/backend/internal/config"
	httpapi "github.com/bhavisachdeva/ai-mock-interview-v2/backend/internal/http"
	"github.com/bhavisachdeva/ai-mock-interview-v2/backend/internal/llm"
	"github.com/bhavisachdeva/ai-mock-interview-v2/backend/internal/platform/logger"
	"github.com/bhavisachdeva/ai-mock-interview-v2/backend/internal/service"
	"github.com/bhavisachdeva/ai-mock-interview-v2/backend/internal/store"
)

func main() {
	if err := run(); err != nil {
		// We use a bare slog default here because logger setup may itself
		// be the failure mode, and we still want startup errors visible.
		slog.Error("server start failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logger.New(cfg.IsProduction(), cfg.LogLevel)
	slog.SetDefault(log)

	rootCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := store.NewPool(rootCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	llmClient, err := llm.NewClient(rootCtx, cfg.GeminiAPIKey, cfg.GeminiModel)
	if err != nil {
		return err
	}

	interviewRepo := store.NewInterviewRepo(pool)
	answerRepo := store.NewAnswerRepo(pool)

	storeFacade := combinedStore{
		InterviewRepo: interviewRepo,
		AnswerRepo:    answerRepo,
	}

	svc := service.NewInterviewService(storeFacade, llmClient)
	verifier := auth.NewClerkVerifier(cfg.ClerkSecretKey)

	router := httpapi.NewRouter(httpapi.RouterDeps{
		Logger:         log,
		Pool:           pool,
		Verifier:       verifier,
		InterviewSvc:   svc,
		AllowedOrigins: cfg.AllowedOrigins(),
	})

	server := &http.Server{
		Addr:              net.JoinHostPort("", cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      90 * time.Second, // accommodates synchronous LLM calls
		IdleTimeout:       120 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("server listening",
			slog.String("addr", server.Addr),
			slog.String("env", cfg.AppEnv),
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case <-rootCtx.Done():
		log.Info("shutdown signal received")
	case err := <-serverErr:
		if err != nil {
			return err
		}
		return nil
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", slog.String("error", err.Error()))
		return err
	}
	log.Info("server shut down cleanly")
	return nil
}

// combinedStore implements service.Store by composing the two repos. Service
// only sees a single dependency; the split between interview and answer
// repos stays a store-package concern.
type combinedStore struct {
	*store.InterviewRepo
	*store.AnswerRepo
}
