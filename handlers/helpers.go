package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"go-crawler-notification/components/layouts"
	"go-crawler-notification/internal/monitor"
	"go-crawler-notification/internal/notify"
	"go-crawler-notification/internal/state"
	"go-crawler-notification/middleware"
	"go-crawler-notification/models"
	"go-crawler-notification/store"
)

// ─── 전역 상태 ────────────────────────────────────────────────────────────────

var (
	AgentStore   *store.AgentStore
	WebhookStore *store.WebhookStore
	MonitorStore *state.Store
	EventLog     *store.EventLog

	runnersMu sync.RWMutex
	runners   = map[string]*monitor.Runner{}

	subsMu      sync.Mutex
	subscribers = map[string][]chan models.MonitorEvent{}
)

// InitStores는 서버 시작 시 저장소를 초기화합니다.
func InitStores() {
	var err error

	AgentStore, err = store.LoadAgents(store.AgentsFilePath)
	if err != nil {
		log.Printf("agents store load failed: %v", err)
		AgentStore = store.NewAgentStore(store.AgentsFilePath)
	}

	WebhookStore, err = store.LoadWebhooks(store.WebhooksFilePath)
	if err != nil {
		log.Printf("webhooks store load failed: %v", err)
		WebhookStore = store.NewWebhookStore(store.WebhooksFilePath)
	}

	MonitorStore, err = state.Load(state.FilePath)
	if err != nil {
		log.Printf("monitor state load failed: %v", err)
		MonitorStore = state.New(state.FilePath)
	}

	EventLog = store.NewEventLog(500)
}

// ─── Runner 관리 ──────────────────────────────────────────────────────────────

// startRunner는 에이전트에 대한 모니터 Runner를 시작합니다.
func startRunner(agent models.Agent) error {
	runnersMu.Lock()
	defer runnersMu.Unlock()

	// 기존 Runner 정지
	if r, ok := runners[agent.ID]; ok {
		r.Stop()
		delete(runners, agent.ID)
	}

	interval := time.Duration(agent.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}

	r := monitor.NewRunner(interval, MonitorStore)
	options := monitor.Options{
		EnableKeywordAlert: agent.EnableKeyword,
		EnableVersionAlert: agent.EnableVersion,
		AlertOnFirstSeen:   agent.AlertOnFirstSeen,
	}

	if err := r.Start(agent.URL, agent.Keyword, options, func(event monitor.Event) {
		handleEvent(agent, event)
	}); err != nil {
		return err
	}

	runners[agent.ID] = r
	return nil
}

// stopRunner는 에이전트의 모니터 Runner를 중지합니다.
func stopRunner(agentID string) {
	runnersMu.Lock()
	defer runnersMu.Unlock()
	if r, ok := runners[agentID]; ok {
		r.Stop()
		delete(runners, agentID)
	}
}

// isRunning은 해당 에이전트의 Runner가 실행 중인지 확인합니다.
func isRunning(agentID string) bool {
	runnersMu.RLock()
	defer runnersMu.RUnlock()
	r, ok := runners[agentID]
	return ok && r.IsRunning()
}

// handleEvent는 모니터 이벤트를 처리합니다: 로그 기록 + SSE 브로드캐스트 + 웹훅 발송.
func handleEvent(agent models.Agent, event monitor.Event) {
	// 모델로 변환
	me := toMonitorEvent(agent, event)
	EventLog.Add(me)

	// SSE 브로드캐스트
	broadcast(agent.ID, me)

	// 웹훅 알림 (matched 또는 version_changed 또는 error 시만 발송)
	shouldNotify := event.Err != nil ||
		(event.VersionChanged && event.LatestVersion != "") ||
		event.Match

	if shouldNotify && len(agent.WebhookIDs) > 0 {
		webhooks := WebhookStore.All()
		webhookMap := make(map[string]models.Webhook, len(webhooks))
		for _, w := range webhooks {
			webhookMap[w.ID] = w
		}
		for _, wid := range agent.WebhookIDs {
			wh, ok := webhookMap[wid]
			if !ok {
				continue
			}
			cfg := notify.WebhookConfig{
				Kind:   notify.WebhookKind(wh.Kind),
				URL:    wh.URL,
				Config: wh.Config,
			}
			n, err := notify.New(cfg)
			if err != nil {
				log.Printf("[NOTIFY_CFG_ERROR] agent=%s webhook=%s err=%v", agent.Name, wh.Name, err)
				continue
			}
			notify.SendAsync(n, event, agent.Name)
		}
	}
}

// toMonitorEvent는 monitor.Event를 models.MonitorEvent로 변환합니다.
func toMonitorEvent(agent models.Agent, ev monitor.Event) models.MonitorEvent {
	status := ev.Status
	if ev.Err != nil {
		status = "error"
	} else if ev.VersionChanged && ev.LatestVersion != "" {
		status = "version_changed"
	} else if ev.Match {
		status = "matched"
	}

	msg := buildMessage(ev)

	me := models.MonitorEvent{
		AgentID:         agent.ID,
		AgentName:       agent.Name,
		Status:          status,
		Mode:            ev.Mode,
		Message:         msg,
		URL:             ev.URL,
		NormalizedURL:   ev.NormalizedURL,
		Keyword:         ev.Keyword,
		LatestVersion:   ev.LatestVersion,
		VersionPrevious: ev.VersionPrevious,
		VersionError:    ev.VersionError,
		OccurredAt:      ev.CheckedAt,
	}
	if ev.Err != nil {
		me.Err = ev.Err.Error()
	}
	return me
}

func buildMessage(ev monitor.Event) string {
	switch {
	case ev.Err != nil:
		return fmt.Sprintf("오류: %v", ev.Err)
	case ev.VersionChanged && ev.LatestVersion != "":
		if ev.VersionPrevious != "" {
			return fmt.Sprintf("새 버전 감지: %s → %s", ev.VersionPrevious, ev.LatestVersion)
		}
		return fmt.Sprintf("새 버전 감지 (첫 감지): %s", ev.LatestVersion)
	case ev.Match:
		return fmt.Sprintf("키워드 감지됨: %q", ev.Keyword)
	default:
		return fmt.Sprintf("점검 완료 [%s]", ev.Mode)
	}
}

// ─── SSE 구독 관리 ────────────────────────────────────────────────────────────

func subscribe(agentID string) chan models.MonitorEvent {
	ch := make(chan models.MonitorEvent, 32)
	subsMu.Lock()
	subscribers[agentID] = append(subscribers[agentID], ch)
	subsMu.Unlock()
	return ch
}

func unsubscribe(agentID string, ch chan models.MonitorEvent) {
	subsMu.Lock()
	defer subsMu.Unlock()
	subs := subscribers[agentID]
	for i, s := range subs {
		if s == ch {
			subscribers[agentID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	close(ch)
}

func broadcast(agentID string, ev models.MonitorEvent) {
	subsMu.Lock()
	defer subsMu.Unlock()
	for _, ch := range subscribers[agentID] {
		select {
		case ch <- ev:
		default: // 느린 소비자 드롭
		}
	}
}

// ─── 공통 핸들러 헬퍼 ─────────────────────────────────────────────────────────

func makeProps(w http.ResponseWriter, r *http.Request, title string) layouts.PageProps {
	return layouts.PageProps{
		Title:       title,
		Flash:       middleware.GetFlashes(w, r),
		CurrentPath: r.URL.Path,
	}
}

func redirect(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, path, http.StatusFound)
}

// writeJSON은 JSON 응답을 씁니다.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
