package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"go-crawler-notification/internal/monitor"
	"go-crawler-notification/internal/version"
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

// LogFn은 시스템 로그를 외부에 기록하기 위한 콜백 함수 타입입니다.
type LogFn func(tag, level, message string)

// Notifier는 이벤트 알림을 발송하는 인터페이스입니다.
type Notifier interface {
	Send(event monitor.Event, agentName string) error
	SetLogFn(fn LogFn)
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

// SendAsync는 goroutine으로 알림을 비동기 발송하고 실패 시 로그를 기록합니다.
func SendAsync(n Notifier, event monitor.Event, agentName string, logFn LogFn) {
	go func() {
		if err := n.Send(event, agentName); err != nil {
			msg := fmt.Sprintf("agent=%s kind=%T err=%v", agentName, n, err)
			log.Printf("[NOTIFY_ERROR] %s", msg)
			if logFn != nil {
				logFn("NOTIFY_ERROR", "error", msg)
			}
		}
	}()
}

// formatMessage는 이벤트를 사람이 읽기 좋은 한 줄 요약 메시지로 변환합니다.
// Slack section text 본문에는 extractChangelog()를 사용하세요.
func formatMessage(event monitor.Event, agentName string) string {
	switch {
	case event.Err != nil:
		return fmt.Sprintf("Agent: %s\n오류: %v", agentName, event.Err)
	case event.VersionChanged && event.LatestVersion != "":
		prev := event.VersionPrevious
		if prev == "" {
			prev = "(첫 감지)"
		}
		return fmt.Sprintf("버전: %s → %s", prev, event.LatestVersion)
	case event.Match:
		return fmt.Sprintf("Agent: %s\n키워드: %q", agentName, event.Keyword)
	default:
		return fmt.Sprintf("Agent: %s", agentName)
	}
}

// extractChangelog는 Content에서 최신 버전 섹션 텍스트를 추출하고
// Slack mrkdwn 형식으로 변환해 반환합니다.
func extractChangelog(event monitor.Event) string {
	if !event.VersionChanged || event.LatestVersion == "" || event.Content == "" {
		return ""
	}
	raw := version.ExtractSection(event.Content, event.LatestVersion)
	if raw == "" {
		return ""
	}
	return mdToMrkdwn(raw)
}

// mdToMrkdwn은 GitHub Flavored Markdown 텍스트를 Slack mrkdwn 형식으로 변환합니다.
//
// 변환 규칙:
//  1. HTML 태그 제거 (<a ...>...</a>, <br/> 등)
//  2. GitBook 템플릿 블록 제거 ({% hint %}, {% code %} 등)
//  3. 절대 URL 링크: [텍스트](https://...) → <https://...|텍스트>
//  4. 상대 경로 링크: [텍스트](/path) → 텍스트 (URL 제거)
//  5. **굵게** → *굵게*
//  6. *기울임* (인라인) → _기울임_
//  7. ## 헤딩 → *헤딩*
//  8. 연속 빈 줄 압축
func mdToMrkdwn(s string) string {
	// 1. HTML 태그 전체 제거 (앵커, span, br 등)
	s = reHTMLTag.ReplaceAllString(s, "")

	// 2. GitBook {% hint %} 블록 — 내부 텍스트만 남김
	s = reGitbookBlock.ReplaceAllString(s, "$1")

	// 3. GitBook {% code %} 블록 — 내부 텍스트만 남김
	s = reGitbookCode.ReplaceAllString(s, "$1")

	// 4. 절대 URL 마크다운 링크 → mrkdwn 링크
	//    [텍스트](https://...) → <https://...|텍스트>
	s = reAbsLink.ReplaceAllStringFunc(s, func(m string) string {
		sub := reAbsLink.FindStringSubmatch(m)
		if len(sub) < 3 {
			return m
		}
		text, url := sub[1], sub[2]
		return "<" + url + "|" + text + ">"
	})

	// 5. 상대 경로 링크 → 텍스트만 남김
	//    [텍스트](/path...) → 텍스트
	s = reRelLink.ReplaceAllString(s, "$1")

	// 6. **굵게** → *굵게*  (먼저 처리해야 단일 * 와 충돌 없음)
	s = reBold.ReplaceAllString(s, "*$1*")

	// 7. ## 헤딩 → *헤딩*  (굵게 이후 처리)
	s = reHeading.ReplaceAllString(s, "*$1*")

	// 9. 줄 끝 백슬래시 줄바꿈 (\) → 실제 줄바꿈으로 변환
	s = reBackslashNewline.ReplaceAllString(s, "\n")

	// 10. 연속 빈 줄 압축 (3개 이상 → 1개)
	s = reMultiBlank.ReplaceAllString(s, "\n\n")

	return strings.TrimSpace(s)
}

// ── 변환에 사용하는 정규식 ──────────────────────────────────

var (
	// HTML 태그 전체 (내용 포함)
	reHTMLTag = regexp.MustCompile(`<[^>]+>`)

	// GitBook hint 블록: {% hint style="..." %} 내용 {% endhint %}
	reGitbookBlock = regexp.MustCompile(`(?s)\{%\s*hint[^%]*%\}(.*?)\{%\s*endhint\s*%\}`)

	// GitBook code 블록: {% code title="..." %} 내용 {% endcode %}
	reGitbookCode = regexp.MustCompile(`(?s)\{%\s*code[^%]*%\}(.*?)\{%\s*endcode\s*%\}`)

	// 절대 URL 마크다운 링크: [text](https://...)
	reAbsLink = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^)]+)\)`)

	// 상대 경로 마크다운 링크: [text](/path) 또는 [text](./path)
	reRelLink = regexp.MustCompile(`\[([^\]]+)\]\((?:/|\.)[^)]*\)`)

	// **굵게** — 줄 중간 또는 줄 전체
	reBold = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)

	// _기울임_ (마크다운 _text_) — 슬랙에서도 동일하게 지원
	// *italic* 형태의 마크다운 기울임은 reBold 이후 남은 홀수 * 를 처리
	// 단, 리스트 "* item" 패턴(줄 시작 * + 공백)은 제외
	// 구현: 줄 단위로 처리 (mdToMrkdwnLine에서 처리)
	reItalicInline = regexp.MustCompile(`(?:(?:^|(?:[^*]))\*)([^*\s][^*]*[^*\s]|[^*\s])\*(?:[^*]|$)`)

	// ## 헤딩 (줄 시작 1~6개 #)
	reHeading = regexp.MustCompile(`(?m)^\s*#{1,6}\s+(.+)`)

	// 줄 끝 백슬래시 줄바꿈 (Markdown hard line break)
	reBackslashNewline = regexp.MustCompile(`\\\n`)

	// 연속 빈 줄 (3개 이상)
	reMultiBlank = regexp.MustCompile(`\n{3,}`)
)

// ─── Slack ────────────────────────────────────────────────

// ── Block Kit 구조체 ──

type slackBlockPayload struct {
	Text   string       `json:"text"`
	Blocks []slackBlock `json:"blocks"`
}

type slackBlock struct {
	Type     string      `json:"type"`
	Text     *slackText  `json:"text,omitempty"`
	Fields   []slackText `json:"fields,omitempty"`
	Elements []slackText `json:"elements,omitempty"`
}

type slackText struct {
	Type string `json:"type"` // "mrkdwn" | "plain_text"
	Text string `json:"text"`
}

// ── notifier ──

type slackNotifier struct {
	webhookURL string
	logFn      LogFn
}

func (s *slackNotifier) SetLogFn(fn LogFn) { s.logFn = fn }

func slackEmoji(event monitor.Event) string {
	switch {
	case event.Err != nil:
		return "🔴"
	case event.VersionChanged:
		return "✅"
	case event.Match:
		return "✅"
	default:
		return "⚪"
	}
}

func slackTitle(event monitor.Event, agentName string) string {
	switch {
	case event.VersionChanged && event.LatestVersion != "":
		return agentName + " " + event.LatestVersion
	case event.Match:
		return agentName + " 키워드 감지"
	case event.Err != nil:
		return agentName + " 오류"
	default:
		return agentName
	}
}

func buildBlockFields(event monitor.Event, agentName string) []slackText {
	fields := []slackText{
		{Type: "mrkdwn", Text: "*Agent*\n" + agentName},
	}
	switch {
	case event.Err != nil:
		fields = append(fields,
			slackText{Type: "mrkdwn", Text: "*오류*\n" + event.Err.Error()},
		)
	case event.VersionChanged:
		prev := event.VersionPrevious
		if prev == "" {
			prev = "(첫 감지)"
		}
		fields = append(fields,
			slackText{Type: "mrkdwn", Text: "*버전 변경*\n" + prev + " → " + event.LatestVersion},
		)
	case event.Match:
		fields = append(fields,
			slackText{Type: "mrkdwn", Text: "*키워드*\n" + event.Keyword},
		)
	}
	return fields
}

func (s *slackNotifier) Send(event monitor.Event, agentName string) error {
	return s.sendBlocks(event, agentName)
}

// sendBlocks — Block Kit 방식
func (s *slackNotifier) sendBlocks(event monitor.Event, agentName string) error {
	title := slackTitle(event, agentName)
	emoji := slackEmoji(event)

	blocks := []slackBlock{
		// 1. Header: 이모지 + 에이전트명 + 버전
		{
			Type: "header",
			Text: &slackText{Type: "plain_text", Text: emoji + " " + title},
		},
	}

	// 2. URL 링크 — LinkURL 우선, 없으면 모니터링 URL
	titleLink := event.URL
	if event.LinkURL != "" {
		titleLink = event.LinkURL
	}
	if titleLink != "" {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf("*<%s|변경 내역 보기>*", titleLink)},
		})
	}

	// 3. Agent / 버전 요약 fields
	blocks = append(blocks, slackBlock{
		Type:   "section",
		Fields: buildBlockFields(event, agentName),
	})

	// 4. Changelog 섹션 — 해당 버전 헤딩 간 내용만 추출해서 삽입
	if changelog := extractChangelog(event); changelog != "" {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: changelog},
		})
	}

	// 5. Divider
	blocks = append(blocks, slackBlock{Type: "divider"})

	// 6. Context (footer + timestamp)
	blocks = append(blocks, slackBlock{
		Type: "context",
		Elements: []slackText{
			{
				Type: "mrkdwn",
				Text: fmt.Sprintf("Crawler Monitor | <!date^%d^{date_short_pretty} {time}|%s>",
					event.CheckedAt.Unix(),
					event.CheckedAt.Format("2006-01-02 15:04"),
				),
			},
		},
	})

	payload := slackBlockPayload{
		Text:   agentName + " 알림",
		Blocks: blocks,
	}
	return postJSON(s.webhookURL, payload, nil, s.logFn)
}

// ─── Telegram ─────────────────────────────────────────────

type telegramNotifier struct {
	botToken string
	chatID   string
	logFn    LogFn
}

func (t *telegramNotifier) SetLogFn(fn LogFn) { t.logFn = fn }

func (t *telegramNotifier) Send(event monitor.Event, agentName string) error {
	msg := formatMessage(event, agentName)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)
	payload := map[string]string{
		"chat_id": t.chatID,
		"text":    msg,
	}
	return postJSON(url, payload, nil, t.logFn)
}

// ─── Custom ───────────────────────────────────────────────

type customNotifier struct {
	url          string
	headersJSON  string
	bodyTemplate string
	logFn        LogFn
}

func (c *customNotifier) SetLogFn(fn LogFn) { c.logFn = fn }

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
		if c.logFn != nil {
			c.logFn("HTTP_RESPONSE", "error", fmt.Sprintf("POST %s → error: %v", c.url, err))
		}
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	respMsg := fmt.Sprintf("POST %s → %d %s\n%s", c.url, resp.StatusCode, http.StatusText(resp.StatusCode), string(respBody))
	if c.logFn != nil {
		level := "info"
		if resp.StatusCode >= 300 {
			level = "error"
		}
		c.logFn("HTTP_RESPONSE", level, respMsg)
	}
	log.Printf("[HTTP_RESPONSE]\n%s\n", respMsg)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("custom webhook HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// ─── 공통 헬퍼 ─────────────────────────────────────────────

func postJSON(targetURL string, payload interface{}, headers map[string]string, logFn LogFn) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Request 로그 — 콘솔 + LogStore
	var prettyReq bytes.Buffer
	reqBody := string(b)
	if err := json.Indent(&prettyReq, b, "", "  "); err == nil {
		reqBody = prettyReq.String()
	}
	reqMsg := fmt.Sprintf("POST %s\nContent-Type: application/json\n\n%s", targetURL, reqBody)
	log.Printf("[HTTP_REQUEST]\n%s\n", reqMsg)
	if logFn != nil {
		logFn("HTTP_REQUEST", "info", reqMsg)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		errMsg := fmt.Sprintf("POST %s → error: %v", targetURL, err)
		log.Printf("[HTTP_RESPONSE]\n%s\n", errMsg)
		if logFn != nil {
			logFn("HTTP_RESPONSE", "error", errMsg)
		}
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// Response 로그 — 콘솔 + LogStore
	var prettyResp bytes.Buffer
	respBodyStr := string(respBody)
	if json.Indent(&prettyResp, respBody, "", "  ") == nil {
		respBodyStr = prettyResp.String()
	}
	respMsg := fmt.Sprintf("POST %s → %d %s\n%s", targetURL, resp.StatusCode, http.StatusText(resp.StatusCode), respBodyStr)
	log.Printf("[HTTP_RESPONSE]\n%s\n", respMsg)
	respLevel := "info"
	if resp.StatusCode >= 300 {
		respLevel = "error"
	}
	if logFn != nil {
		logFn("HTTP_RESPONSE", respLevel, respMsg)
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
