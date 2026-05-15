package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	agentComponents "go-crawler-notification/components/agents"
	"go-crawler-notification/middleware"
	"go-crawler-notification/models"
)

func AgentsIndex(w http.ResponseWriter, r *http.Request) {
	agents := AgentStore.All()
	runningIDs := map[string]bool{}
	for _, a := range agents {
		runningIDs[a.ID] = isRunning(a.ID)
	}
	webhookMap := map[string]models.Webhook{}
	for _, wh := range WebhookStore.All() {
		webhookMap[wh.ID] = wh
	}
	props := makeProps(w, r, "Agents")
	agentComponents.Index(props, agents, runningIDs, webhookMap).Render(r.Context(), w)
}

func AgentsNew(w http.ResponseWriter, r *http.Request) {
	props := makeProps(w, r, "New Agent")
	webhooks := WebhookStore.All()
	agentComponents.New(props, webhooks).Render(r.Context(), w)
}

func AgentsCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form parse error", http.StatusBadRequest)
		return
	}
	agent, err := agentFromForm(r.Form)
	if err != nil {
		middleware.SetFlash(w, r, "alert", fmt.Sprintf("입력 오류: %v", err))
		redirect(w, r, "/agents/new")
		return
	}
	agent.ID = uuid.New().String()
	agent.CreatedAt = time.Now()
	agent.UpdatedAt = time.Now()

	if err := AgentStore.Save(agent); err != nil {
		middleware.SetFlash(w, r, "alert", "저장 실패: "+err.Error())
		redirect(w, r, "/agents/new")
		return
	}
	middleware.SetFlash(w, r, "notice", fmt.Sprintf("에이전트 '%s'가 생성되었습니다.", agent.Name))
	redirect(w, r, "/agents")
}

func AgentsShow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, ok := AgentStore.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	// 시스템 로그 (LogStore에서 읽음)
	var syslogs []models.SystemLog
	if LogStore != nil {
		syslogs = LogStore.ByAgent(id)
	}
	running := isRunning(id)

	// SSE 탭 표시용 메모리 이벤트 로그
	events := EventLog.ByAgent(id)

	// 연결된 웹훅 목록
	allWebhooks := WebhookStore.All()
	var connectedWebhooks []models.Webhook
	whMap := map[string]models.Webhook{}
	for _, wh := range allWebhooks {
		whMap[wh.ID] = wh
	}
	for _, wid := range agent.WebhookIDs {
		if wh, ok := whMap[wid]; ok {
			connectedWebhooks = append(connectedWebhooks, wh)
		}
	}

	props := makeProps(w, r, agent.Name)
	props.LoadSSE = true
	agentComponents.Show(props, agent, events, syslogs, running, connectedWebhooks).Render(r.Context(), w)
}

func AgentsEdit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, ok := AgentStore.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	props := makeProps(w, r, "Edit "+agent.Name)
	webhooks := WebhookStore.All()
	agentComponents.Edit(props, agent, webhooks).Render(r.Context(), w)
}

func AgentsUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, ok := AgentStore.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "form parse error", http.StatusBadRequest)
		return
	}
	agent, err := agentFromForm(r.Form)
	if err != nil {
		middleware.SetFlash(w, r, "alert", fmt.Sprintf("입력 오류: %v", err))
		redirect(w, r, fmt.Sprintf("/agents/%s/edit", id))
		return
	}
	agent.ID = id
	agent.CreatedAt = existing.CreatedAt
	agent.UpdatedAt = time.Now()

	// 실행 중이면 재시작
	wasRunning := isRunning(id)
	if wasRunning {
		stopRunner(id)
	}

	if err := AgentStore.Save(agent); err != nil {
		middleware.SetFlash(w, r, "alert", "저장 실패: "+err.Error())
		redirect(w, r, fmt.Sprintf("/agents/%s/edit", id))
		return
	}

	if wasRunning {
		if err := startRunner(agent); err != nil {
			middleware.SetFlash(w, r, "alert", fmt.Sprintf("모니터 재시작 실패: %v", err))
		}
	}

	middleware.SetFlash(w, r, "notice", fmt.Sprintf("에이전트 '%s'가 업데이트되었습니다.", agent.Name))
	redirect(w, r, fmt.Sprintf("/agents/%s", id))
}

func AgentsDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, ok := AgentStore.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	stopRunner(id)
	if err := AgentStore.Delete(id); err != nil {
		middleware.SetFlash(w, r, "alert", "삭제 실패: "+err.Error())
	} else {
		middleware.SetFlash(w, r, "notice", fmt.Sprintf("에이전트 '%s'가 삭제되었습니다.", agent.Name))
	}
	redirect(w, r, "/agents")
}

func AgentsStart(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, ok := AgentStore.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := startRunner(agent); err != nil {
		middleware.SetFlash(w, r, "alert", fmt.Sprintf("시작 실패: %v", err))
	} else {
		middleware.SetFlash(w, r, "notice", fmt.Sprintf("'%s' 모니터링이 시작되었습니다.", agent.Name))
	}
	redirect(w, r, fmt.Sprintf("/agents/%s", id))
}

func AgentsStop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, ok := AgentStore.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	stopRunner(id)
	middleware.SetFlash(w, r, "notice", fmt.Sprintf("'%s' 모니터링이 중지되었습니다.", agent.Name))
	redirect(w, r, fmt.Sprintf("/agents/%s", id))
}

// AgentsEvents는 SSE 스트림 핸들러입니다.
func AgentsEvents(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, ok := AgentStore.Get(id); !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// 초기 연결 확인 메시지
	fmt.Fprintf(w, "data: {\"type\":\"connected\"}\n\n")
	flusher.Flush()

	ch := subscribe(id)
	defer unsubscribe(id, ch)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			b, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	}
}

// AgentsLogsIndex는 SystemLogsTable HTML partial을 반환합니다 (AJAX 새로고침용).
func AgentsLogsIndex(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var syslogs []models.SystemLog
	if LogStore != nil {
		syslogs = LogStore.ByAgent(id)
	}
	agentComponents.SystemLogsTable(syslogs).Render(r.Context(), w)
}

// AgentsLogsClear는 해당 에이전트의 시스템 로그를 삭제하고 빈 테이블 HTML을 반환합니다.
func AgentsLogsClear(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if LogStore != nil {
		if err := LogStore.ClearAgent(id); err != nil {
			http.Error(w, "로그 삭제 실패: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	agentComponents.SystemLogsTable([]models.SystemLog{}).Render(r.Context(), w)
}

// AgentsStatus는 에이전트 실행 상태를 JSON으로 반환합니다.
func AgentsStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	running := isRunning(id)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"agent_id": id,
		"running":  running,
	})
}

// ─── 폼 파싱 헬퍼 ─────────────────────────────────────────────────────────────

func agentFromForm(form url.Values) (models.Agent, error) {
	name := strings.TrimSpace(form.Get("name"))
	if name == "" {
		return models.Agent{}, fmt.Errorf("이름을 입력하세요")
	}
	rawURL := strings.TrimSpace(form.Get("url"))
	if rawURL == "" {
		return models.Agent{}, fmt.Errorf("URL을 입력하세요")
	}
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return models.Agent{}, fmt.Errorf("유효한 URL이 아닙니다")
	}

	linkURL := strings.TrimSpace(form.Get("link_url"))
	if linkURL != "" {
		if _, err := url.ParseRequestURI(linkURL); err != nil {
			return models.Agent{}, fmt.Errorf("링크 URL이 유효하지 않습니다")
		}
	}

	enableKeyword := form.Get("enable_keyword") == "1"
	keyword := strings.TrimSpace(form.Get("keyword"))
	if enableKeyword && keyword == "" {
		return models.Agent{}, fmt.Errorf("키워드를 입력하세요")
	}

	interval, _ := strconv.Atoi(form.Get("interval_seconds"))
	if interval <= 0 {
		interval = 60
	}

	webhookIDs := form["webhook_ids"]
	detectionOptions := strings.TrimSpace(form.Get("detection_options"))
	if detectionOptions == "" {
		detectionOptions = "{}"
	}
	if !json.Valid([]byte(detectionOptions)) {
		return models.Agent{}, fmt.Errorf("감지 옵션 JSON이 유효하지 않습니다")
	}

	return models.Agent{
		Name:             name,
		URL:              rawURL,
		LinkURL:          linkURL,
		Keyword:          keyword,
		EnableKeyword:    enableKeyword,
		DetectionOptions: json.RawMessage(detectionOptions),
		IntervalSeconds:  interval,
		WebhookIDs:       webhookIDs,
		Enabled:          true,
	}, nil
}
