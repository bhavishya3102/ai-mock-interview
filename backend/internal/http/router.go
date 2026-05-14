package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/bhavishya3102/ai-mock-interview/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/cors"
)

// RouterDeps bundles router dependencies. Composition root passes them in
// from cmd/server/main.go.
type RouterDeps struct {
	Logger         *slog.Logger
	Pool           *pgxpool.Pool
	Verifier       auth.Verifier
	InterviewSvc   InterviewService
	AllowedOrigins []string
}

// NewRouter wires middleware, CORS, healthz (unauthed), and the v1 routes
// (authed via Clerk middleware).
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	r.Use(requestIDMiddleware)
	r.Use(recoverer(deps.Logger))
	r.Use(requestLogger(deps.Logger))

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   deps.AllowedOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           300,
	})
	r.Use(corsHandler.Handler)

	r.Get("/healthz", healthzHandler(deps.Pool, deps.Logger))

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.Middleware(deps.Verifier, deps.Logger))

		ih := NewInterviewHandler(deps.InterviewSvc, deps.Logger)
		r.Route("/interviews", func(r chi.Router) {
			r.Post("/", ih.Create)
			r.Get("/", ih.List)
			r.Get("/{mockId}", ih.Get)
			r.Post("/{mockId}/answers", ih.SubmitAnswer)
			r.Post("/{mockId}/transcribe", ih.Transcribe)
			r.Post("/{mockId}/follow-up", ih.JudgeFollowUp)
			r.Get("/{mockId}/feedback", ih.ListFeedback)
		})
	})

	return r
}

// healthzHandler returns liveness/readiness — pings the pool with a short
// context and reports db status. The endpoint is unauthenticated by design.
func healthzHandler(pool *pgxpool.Pool, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "ok"
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if pool == nil {
			dbStatus = "down"
		} else if err := pool.Ping(ctx); err != nil {
			dbStatus = "down"
			log.Warn("healthz db ping failed", slog.String("error", err.Error()))
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"db":     dbStatus,
		})
	}
}
