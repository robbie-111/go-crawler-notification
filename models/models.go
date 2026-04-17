package models

import "time"

// Agent는 하나의 모니터링 단위입니다.
type Agent struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	URL              string    `json:"url"`
	Keyword          string    `json:"keyword"`
	EnableKeyword    bool      `json:"enable_keyword"`
	EnableVersion    bool      `json:"enable_version"`
	AlertOnFirstSeen bool      `json:"alert_on_first_seen"`
	IntervalSeconds  int       `json:"interval_seconds"`
	WebhookIDs       []string  `json:"webhook_ids"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Webhook은 알림 수신 채널 설정입니다.
type Webhook struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Kind      string            `json:"kind"` // "slack" | "telegram" | "custom"
	URL       string            `json:"url"`
	Config    map[string]string `json:"config"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// MonitorEvent는 모니터링 이벤트 로그 항목입니다.
type MonitorEvent struct {
	AgentID         string    `json:"agent_id"`
	AgentName       string    `json:"agent_name"`
	Status          string    `json:"status"` // "checked" | "matched" | "version_changed" | "error"
	Mode            string    `json:"mode"`
	Message         string    `json:"message"`
	URL             string    `json:"url"`
	NormalizedURL   string    `json:"normalized_url"`
	Keyword         string    `json:"keyword"`
	LatestVersion   string    `json:"latest_version"`
	VersionPrevious string    `json:"version_previous"`
	VersionError    string    `json:"version_error"`
	Err             string    `json:"error,omitempty"`
	OccurredAt      time.Time `json:"occurred_at"`
}

// FlashMessage는 페이지 간 플래시 메시지입니다.
type FlashMessage struct {
	Type    string // "notice" | "alert"
	Message string
}
