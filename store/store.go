package store

import (
	"encoding/json"
	"errors"
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

// ─── 시간 유틸 ─────────────────────────────────────────────────────────────────

func Now() time.Time {
	return time.Now()
}
