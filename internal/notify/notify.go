package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"go-crawler-notification/internal/monitor"
)

// WebhookKind은 웹훅 종류를 나타냅니다.
type WebhookKind string

const (
	KindSlack    WebhookKind = "slack"
	KindTelegram WebhookKind = "telegram"
	KindCustom   WebhookKind = "custom"
)

// WebhookConfig는 웹훅 설정을 나타냅니다.
type WebhookConfig struct {
	Kind   WebhookKind
	URL    string            // Slack Incoming Webhook URL / Custom URL
	Config map[string]string // Kind별 추가 설정
	// slack:    없음 (URL이 Incoming Webhook URL 자체)
	// telegram: bot_token, chat_id
	// custom:   headers (JSON), body_template
}

// Notifier는 이벤트 알림을 발송하는 인터페이스입니다.
type Notifier interface {
	Send(event monitor.Event, agentName string) error
}

// New는 WebhookConfig에 따라 적절한 Notifier를 반환합니다.
func New(cfg WebhookConfig) (Notifier, error) {
	switch cfg.Kind {
	case KindSlack:
		if cfg.URL == "" {
			return nil, fmt.Errorf("slack: webhook URL이 필요합니다")
		}
		return &slackNotifier{webhookURL: cfg.URL}, nil
	case KindTelegram:
		token := cfg.Config["bot_token"]
		chatID := cfg.Config["chat_id"]
		if token == "" || chatID == "" {
			return nil, fmt.Errorf("telegram: bot_token과 chat_id가 필요합니다")
		}
		return &telegramNotifier{botToken: token, chatID: chatID}, nil
	case KindCustom:
		if cfg.URL == "" {
			return nil, fmt.Errorf("custom: URL이 필요합니다")
		}
		return &customNotifier{url: cfg.URL, headersJSON: cfg.Config["headers"], bodyTemplate: cfg.Config["body_template"]}, nil
	default:
		return nil, fmt.Errorf("알 수 없는 웹훅 종류: %s", cfg.Kind)
	}
}

// SendAsync는 goroutine으로 알림을 비동기 발송하고 실패 시 로그만 기록합니다.
func SendAsync(n Notifier, event monitor.Event, agentName string) {
	go func() {
		if err := n.Send(event, agentName); err != nil {
			log.Printf("[NOTIFY_ERROR] agent=%s kind=%T err=%v", agentName, n, err)
		}
	}()
}

// formatMessage는 이벤트를 사람이 읽기 좋은 메시지로 변환합니다.
func formatMessage(event monitor.Event, agentName string) string {
	switch {
	case event.Err != nil:
		return fmt.Sprintf("Agent: %s\n오류: %v", agentName, event.Err)
	case event.VersionChanged && event.LatestVersion != "":
		prev := event.VersionPrevious
		if prev == "" {
			prev = "(첫 감지)"
		}
		msg := fmt.Sprintf("Agent: %s\n버전: %s → %s", agentName, prev, event.LatestVersion)
		if event.Content != "" {
			msg += "\n\n" + event.Content
		}
		return msg
	case event.Match:
		return fmt.Sprintf("Agent: %s\n키워드: %q", agentName, event.Keyword)
	default:
		return fmt.Sprintf("Agent: %s", agentName)
	}
}

// ─── Slack ────────────────────────────────────────────────

type slackPayload struct {
	Text        string            `json:"text"`
	Attachments []slackAttachment `json:"attachments"`
}

type slackAttachment struct {
	Color     string       `json:"color"`
	Title     string       `json:"title,omitempty"`
	TitleLink string       `json:"title_link,omitempty"`
	Text      string       `json:"text"`
	Fields    []slackField `json:"fields,omitempty"`
	Footer    string       `json:"footer,omitempty"`
	Ts        int64        `json:"ts,omitempty"`
}

type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

type slackNotifier struct {
	webhookURL string
}

func slackColor(event monitor.Event) string {
	switch {
	case event.Err != nil:
		return "danger"
	case event.VersionChanged:
		return "warning"
	case event.Match:
		return "warning"
	default:
		return "good"
	}
}

func buildSlackFields(event monitor.Event, agentName string) []slackField {
	fields := []slackField{
		{Title: "Agent", Value: agentName, Short: true},
	}
	switch {
	case event.Err != nil:
		fields = append(fields,
			slackField{Title: "오류", Value: event.Err.Error(), Short: false},
		)
	case event.VersionChanged:
		prev := event.VersionPrevious
		if prev == "" {
			prev = "(첫 감지)"
		}
		fields = append(fields,
			slackField{Title: "버전 변경", Value: prev + " → " + event.LatestVersion, Short: true},
		)
	case event.Match:
		fields = append(fields,
			slackField{Title: "키워드", Value: event.Keyword, Short: true},
		)
	}
	return fields
}

func (s *slackNotifier) Send(event monitor.Event, agentName string) error {
	var title string
	switch {
	case event.VersionChanged && event.LatestVersion != "":
		title = agentName + " " + event.LatestVersion
	case event.Match:
		title = agentName + " 키워드 감지"
	case event.Err != nil:
		title = agentName + " 오류"
	}

	payload := slackPayload{
		Text: agentName + " 알림",
		Attachments: []slackAttachment{
			{
				Color:     slackColor(event),
				Title:     title,
				TitleLink: event.URL,
				Text:      formatMessage(event, agentName),
				Fields:    buildSlackFields(event, agentName),
				Footer:    "Crawler Monitor",
				Ts:        event.CheckedAt.Unix(),
			},
		},
	}

	return postJSON(s.webhookURL, payload, nil)
}

// ─── Telegram ─────────────────────────────────────────────

type telegramNotifier struct {
	botToken string
	chatID   string
}

func (t *telegramNotifier) Send(event monitor.Event, agentName string) error {
	msg := formatMessage(event, agentName)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)
	payload := map[string]string{
		"chat_id": t.chatID,
		"text":    msg,
	}
	return postJSON(url, payload, nil)
}

// ─── Custom ───────────────────────────────────────────────

type customNotifier struct {
	url          string
	headersJSON  string
	bodyTemplate string
}

func (c *customNotifier) Send(event monitor.Event, agentName string) error {
	// 바디 템플릿 렌더링 ({{message}} 치환)
	body := c.bodyTemplate
	if body == "" {
		msg := formatMessage(event, agentName)
		payload := map[string]string{"message": msg, "agent": agentName, "url": event.URL}
		b, _ := json.Marshal(payload)
		body = string(b)
	} else {
		msg := formatMessage(event, agentName)
		body = strings.ReplaceAll(body, "{{message}}", msg)
		body = strings.ReplaceAll(body, "{{agent}}", agentName)
		body = strings.ReplaceAll(body, "{{url}}", event.URL)
		body = strings.ReplaceAll(body, "{{version}}", event.LatestVersion)
		body = strings.ReplaceAll(body, "{{keyword}}", event.Keyword)
	}

	// 헤더 파싱
	headers := map[string]string{"Content-Type": "application/json"}
	if c.headersJSON != "" {
		var extra map[string]string
		if err := json.Unmarshal([]byte(c.headersJSON), &extra); err == nil {
			for k, v := range extra {
				headers[k] = v
			}
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodPost, c.url, bytes.NewBufferString(body))
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("custom webhook HTTP %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// ─── 공통 헬퍼 ─────────────────────────────────────────────

func postJSON(url string, payload interface{}, headers map[string]string) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Request 로그 (pretty)
	var prettyReq bytes.Buffer
	if err := json.Indent(&prettyReq, b, "", "  "); err == nil {
		log.Printf("[HTTP_REQUEST]\nPOST %s\nContent-Type: application/json\n\n%s\n", url, prettyReq.String())
	} else {
		log.Printf("[HTTP_REQUEST]\nPOST %s\nContent-Type: application/json\n\n%s\n", url, string(b))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[HTTP_RESPONSE]\nerror: %v\n", err)
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Response 로그 (pretty — JSON이면 indent, 아니면 그대로)
	var prettyResp bytes.Buffer
	if json.Indent(&prettyResp, body, "", "  ") == nil {
		log.Printf("[HTTP_RESPONSE]\nStatus: %d %s\nBody:\n%s\n", resp.StatusCode, http.StatusText(resp.StatusCode), prettyResp.String())
	} else {
		log.Printf("[HTTP_RESPONSE]\nStatus: %d %s\nBody: %s\n", resp.StatusCode, http.StatusText(resp.StatusCode), string(body))
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
