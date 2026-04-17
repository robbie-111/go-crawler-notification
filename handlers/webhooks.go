package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	webhookComponents "go-crawler-notification/components/webhooks"
	"go-crawler-notification/internal/monitor"
	"go-crawler-notification/internal/notify"
	"go-crawler-notification/middleware"
	"go-crawler-notification/models"
)

func WebhooksIndex(w http.ResponseWriter, r *http.Request) {
	props := makeProps(w, r, "Webhooks")
	webhooks := WebhookStore.All()
	webhookComponents.Index(props, webhooks).Render(r.Context(), w)
}

func WebhooksNew(w http.ResponseWriter, r *http.Request) {
	props := makeProps(w, r, "New Webhook")
	webhookComponents.NewWebhook(props).Render(r.Context(), w)
}

func WebhooksCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form parse error", http.StatusBadRequest)
		return
	}
	wh, err := webhookFromForm(r)
	if err != nil {
		middleware.SetFlash(w, r, "alert", fmt.Sprintf("입력 오류: %v", err))
		redirect(w, r, "/webhooks/new")
		return
	}
	wh.ID = uuid.New().String()
	wh.CreatedAt = time.Now()
	wh.UpdatedAt = time.Now()

	if err := WebhookStore.Save(wh); err != nil {
		middleware.SetFlash(w, r, "alert", "저장 실패: "+err.Error())
		redirect(w, r, "/webhooks/new")
		return
	}
	middleware.SetFlash(w, r, "notice", fmt.Sprintf("웹훅 '%s'가 생성되었습니다.", wh.Name))
	redirect(w, r, "/webhooks")
}

func WebhooksEdit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	wh, ok := WebhookStore.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	props := makeProps(w, r, "Edit "+wh.Name)
	webhookComponents.EditWebhook(props, wh).Render(r.Context(), w)
}

func WebhooksUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, ok := WebhookStore.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form parse error", http.StatusBadRequest)
		return
	}
	wh, err := webhookFromForm(r)
	if err != nil {
		middleware.SetFlash(w, r, "alert", fmt.Sprintf("입력 오류: %v", err))
		redirect(w, r, fmt.Sprintf("/webhooks/%s/edit", id))
		return
	}
	wh.ID = id
	wh.CreatedAt = existing.CreatedAt
	wh.UpdatedAt = time.Now()

	if err := WebhookStore.Save(wh); err != nil {
		middleware.SetFlash(w, r, "alert", "저장 실패: "+err.Error())
		redirect(w, r, fmt.Sprintf("/webhooks/%s/edit", id))
		return
	}
	middleware.SetFlash(w, r, "notice", fmt.Sprintf("웹훅 '%s'가 업데이트되었습니다.", wh.Name))
	redirect(w, r, "/webhooks")
}

func WebhooksDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	wh, ok := WebhookStore.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := WebhookStore.Delete(id); err != nil {
		middleware.SetFlash(w, r, "alert", "삭제 실패: "+err.Error())
	} else {
		middleware.SetFlash(w, r, "notice", fmt.Sprintf("웹훅 '%s'가 삭제되었습니다.", wh.Name))
	}
	redirect(w, r, "/webhooks")
}

// WebhooksTest는 테스트 메시지를 웹훅으로 발송합니다.
func WebhooksTest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	wh, ok := WebhookStore.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	cfg := notify.WebhookConfig{
		Kind:   notify.WebhookKind(wh.Kind),
		URL:    wh.URL,
		Config: wh.Config,
	}
	n, err := notify.New(cfg)
	if err != nil {
		middleware.SetFlash(w, r, "alert", fmt.Sprintf("웹훅 설정 오류: %v", err))
		redirect(w, r, "/webhooks")
		return
	}

	testEvent := monitor.Event{
		Status:    "checked",
		URL:       "https://example.com/changelog",
		Keyword:   "test",
		CheckedAt: time.Now(),
	}

	if err := n.Send(testEvent, "[테스트] Crawler Monitor"); err != nil {
		middleware.SetFlash(w, r, "alert", fmt.Sprintf("전송 실패: %v", err))
	} else {
		middleware.SetFlash(w, r, "notice", fmt.Sprintf("'%s' 웹훅으로 테스트 메시지가 발송되었습니다.", wh.Name))
	}
	redirect(w, r, "/webhooks")
}

// ─── 폼 파싱 헬퍼 ─────────────────────────────────────────────────────────────

func webhookFromForm(r *http.Request) (models.Webhook, error) {
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		return models.Webhook{}, fmt.Errorf("이름을 입력하세요")
	}
	kind := r.FormValue("kind")
	if kind == "" {
		return models.Webhook{}, fmt.Errorf("종류를 선택하세요")
	}

	cfg := map[string]string{}
	var webhookURL string

	switch kind {
	case "slack":
		webhookURL = strings.TrimSpace(r.FormValue("url"))
		if webhookURL == "" {
			return models.Webhook{}, fmt.Errorf("Slack Webhook URL을 입력하세요")
		}
	case "telegram":
		token := strings.TrimSpace(r.FormValue("bot_token"))
		chatID := strings.TrimSpace(r.FormValue("chat_id"))
		if token == "" || chatID == "" {
			return models.Webhook{}, fmt.Errorf("Bot Token과 Chat ID를 모두 입력하세요")
		}
		cfg["bot_token"] = token
		cfg["chat_id"] = chatID
	case "custom":
		webhookURL = strings.TrimSpace(r.FormValue("url"))
		if webhookURL == "" {
			return models.Webhook{}, fmt.Errorf("URL을 입력하세요")
		}
		if headers := strings.TrimSpace(r.FormValue("headers")); headers != "" {
			cfg["headers"] = headers
		}
		if body := strings.TrimSpace(r.FormValue("body_template")); body != "" {
			cfg["body_template"] = body
		}
	default:
		return models.Webhook{}, fmt.Errorf("알 수 없는 종류: %s", kind)
	}

	return models.Webhook{
		Name:   name,
		Kind:   kind,
		URL:    webhookURL,
		Config: cfg,
	}, nil
}
