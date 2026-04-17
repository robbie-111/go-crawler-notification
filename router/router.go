package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"go-crawler-notification/handlers"
)

func New() http.Handler {
	r := chi.NewRouter()

	// 전역 미들웨어
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.CleanPath)

	// 정적 파일
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// 루트: 에이전트 목록으로 리다이렉트
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/agents", http.StatusFound)
	})

	// ─── Agents ────────────────────────────────────────────────
	r.Get("/agents", handlers.AgentsIndex)
	r.Get("/agents/new", handlers.AgentsNew)
	r.Post("/agents", handlers.AgentsCreate)
	r.Get("/agents/{id}", handlers.AgentsShow)
	r.Get("/agents/{id}/edit", handlers.AgentsEdit)
	r.Post("/agents/{id}", handlers.AgentsUpdate) // HTML form은 PUT을 지원 안 하므로 POST 사용
	r.Post("/agents/{id}/start", handlers.AgentsStart)
	r.Post("/agents/{id}/stop", handlers.AgentsStop)
	r.Post("/agents/{id}/delete", handlers.AgentsDelete)
	r.Get("/agents/{id}/events", handlers.AgentsEvents) // SSE 스트림
	r.Get("/agents/{id}/status", handlers.AgentsStatus) // JSON 상태

	// ─── Events ────────────────────────────────────────────────
	r.Get("/events", handlers.EventsIndex)

	// ─── Webhooks ──────────────────────────────────────────────
	r.Get("/webhooks", handlers.WebhooksIndex)
	r.Get("/webhooks/new", handlers.WebhooksNew)
	r.Post("/webhooks", handlers.WebhooksCreate)
	r.Get("/webhooks/{id}/edit", handlers.WebhooksEdit)
	r.Post("/webhooks/{id}", handlers.WebhooksUpdate)
	r.Post("/webhooks/{id}/delete", handlers.WebhooksDelete)
	r.Post("/webhooks/{id}/test", handlers.WebhooksTest)

	return r
}
