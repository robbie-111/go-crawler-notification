package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"go-crawler-notification/models"
)

const (
	AgentsFilePath   = "agents.json"
	WebhooksFilePath = "webhooks.json"
)

// ─── AgentStore ──────────────────────────────────────────────────────────────

// AgentStore는 에이전트 설정을 agents.json에 영속 저장합니다.
type AgentStore struct {
	mu     sync.Mutex
	path   string
	agents []models.Agent
}

func NewAgentStore(path string) *AgentStore {
	return &AgentStore{path: path, agents: []models.Agent{}}
}

func LoadAgents(path string) (*AgentStore, error) {
	s := NewAgentStore(path)
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, err
	}
	if len(body) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(body, &s.agents); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *AgentStore) All() []models.Agent {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]models.Agent, len(s.agents))
	copy(cp, s.agents)
	return cp
}

func (s *AgentStore) Get(id string) (models.Agent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.agents {
		if a.ID == id {
			return a, true
		}
	}
	return models.Agent{}, false
}

func (s *AgentStore) Save(a models.Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for i, existing := range s.agents {
		if existing.ID == a.ID {
			s.agents[i] = a
			found = true
			break
		}
	}
	if !found {
		s.agents = append(s.agents, a)
	}
	return s.flush()
}

func (s *AgentStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := s.agents[:0]
	for _, a := range s.agents {
		if a.ID != id {
			filtered = append(filtered, a)
		}
	}
	s.agents = filtered
	return s.flush()
}

func (s *AgentStore) flush() error {
	body, err := json.MarshalIndent(s.agents, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, body, 0644)
}

// ─── WebhookStore ─────────────────────────────────────────────────────────────

// WebhookStore는 웹훅 설정을 webhooks.json에 영속 저장합니다.
type WebhookStore struct {
	mu       sync.Mutex
	path     string
	webhooks []models.Webhook
}

func NewWebhookStore(path string) *WebhookStore {
	return &WebhookStore{path: path, webhooks: []models.Webhook{}}
}

func LoadWebhooks(path string) (*WebhookStore, error) {
	s := NewWebhookStore(path)
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, err
	}
	if len(body) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(body, &s.webhooks); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *WebhookStore) All() []models.Webhook {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]models.Webhook, len(s.webhooks))
	copy(cp, s.webhooks)
	return cp
}

func (s *WebhookStore) Get(id string) (models.Webhook, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, w := range s.webhooks {
		if w.ID == id {
			return w, true
		}
	}
	return models.Webhook{}, false
}

func (s *WebhookStore) Save(w models.Webhook) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for i, existing := range s.webhooks {
		if existing.ID == w.ID {
			s.webhooks[i] = w
			found = true
			break
		}
	}
	if !found {
		s.webhooks = append(s.webhooks, w)
	}
	return s.flush()
}

func (s *WebhookStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := s.webhooks[:0]
	for _, w := range s.webhooks {
		if w.ID != id {
			filtered = append(filtered, w)
		}
	}
	s.webhooks = filtered
	return s.flush()
}

func (s *WebhookStore) flush() error {
	body, err := json.MarshalIndent(s.webhooks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, body, 0644)
}

// ─── EventLog ─────────────────────────────────────────────────────────────────

// EventLog는 메모리 내 이벤트 링버퍼입니다 (최대 500건).
type EventLog struct {
	mu     sync.Mutex
	events []models.MonitorEvent
	max    int
}

func NewEventLog(max int) *EventLog {
	return &EventLog{max: max, events: []models.MonitorEvent{}}
}

func (e *EventLog) Add(ev models.MonitorEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append([]models.MonitorEvent{ev}, e.events...)
	if len(e.events) > e.max {
		e.events = e.events[:e.max]
	}
}

func (e *EventLog) All() []models.MonitorEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	cp := make([]models.MonitorEvent, len(e.events))
	copy(cp, e.events)
	return cp
}

func (e *EventLog) ByAgent(agentID string) []models.MonitorEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	var result []models.MonitorEvent
	for _, ev := range e.events {
		if ev.AgentID == agentID {
			result = append(result, ev)
		}
	}
	return result
}

// ClearAgent는 메모리 EventLog에서 특정 에이전트의 이벤트를 모두 제거합니다.
func (e *EventLog) ClearAgent(agentID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	filtered := e.events[:0]
	for _, ev := range e.events {
		if ev.AgentID != agentID {
			filtered = append(filtered, ev)
		}
	}
	e.events = filtered
}

// ─── LogStore ─────────────────────────────────────────────────────────────────

// LogStore는 에이전트별 시스템 운영 로그를 logs/{agentID}.json에 영속 저장합니다.
// 저장 대상: FIRST_SEEN_VERSION, NEW_VERSION, HTTP_REQUEST, HTTP_RESPONSE, NOTIFY_ERROR 등
// 일상 폴링 결과("점검 완료 [raw]" 등)는 저장하지 않습니다.
type LogStore struct {
	mu  sync.Mutex
	dir string
	max int
}

func NewLogStore(dir string) (*LogStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("LogStore: mkdir %s: %w", dir, err)
	}
	return &LogStore{dir: dir, max: 500}, nil
}

func (s *LogStore) path(agentID string) string {
	return fmt.Sprintf("%s/%s.json", s.dir, agentID)
}

// Add는 SystemLog를 해당 에이전트 로그 파일의 선두에 추가합니다 (최신순 유지, 최대 500건).
func (s *LogStore) Add(sl models.SystemLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logs := s.load(sl.AgentID)
	logs = append([]models.SystemLog{sl}, logs...)
	if len(logs) > s.max {
		logs = logs[:s.max]
	}
	return s.flush(sl.AgentID, logs)
}

// ByAgent는 해당 에이전트의 시스템 로그를 JSON 파일에서 읽어 반환합니다.
func (s *LogStore) ByAgent(agentID string) []models.SystemLog {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(agentID)
}

// ClearAgent는 해당 에이전트의 로그 파일을 삭제합니다.
func (s *LogStore) ClearAgent(agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.path(agentID)
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// load는 잠금 없이 파일을 읽습니다 (호출자가 mu를 보유해야 함).
func (s *LogStore) load(agentID string) []models.SystemLog {
	body, err := os.ReadFile(s.path(agentID))
	if err != nil {
		return []models.SystemLog{}
	}
	var logs []models.SystemLog
	if err := json.Unmarshal(body, &logs); err != nil {
		return []models.SystemLog{}
	}
	return logs
}

// flush는 잠금 없이 파일에 씁니다 (호출자가 mu를 보유해야 함).
func (s *LogStore) flush(agentID string, logs []models.SystemLog) error {
	body, err := json.MarshalIndent(logs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(agentID), body, 0644)
}

// ─── 시간 유틸 ─────────────────────────────────────────────────────────────────

func Now() time.Time {
	return time.Now()
}
