package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"go-crawler-notification/internal/crawler"
	"go-crawler-notification/internal/state"
	"go-crawler-notification/internal/version"
)

type Event struct {
	Status           string
	Mode             string
	Content          string
	URL              string
	LinkURL          string // 슬랙 알림 title_link용 URL (비어있으면 URL 사용)
	NormalizedURL    string
	Keyword          string
	CheckedAt        time.Time
	Match            bool
	LatestVersion    string
	VersionChanged   bool
	VersionPrevious  string
	VersionError     string
	DetectionOptions json.RawMessage
	Err              error
}

type Options struct {
	EnableKeywordAlert bool
	LinkURL            string // 슬랙 알림 title_link용 URL (선택)
	DetectionOptions   json.RawMessage
}

// LogFn은 시스템 로그를 외부에 기록하기 위한 콜백 함수 타입입니다.
// tag: "FIRST_SEEN_VERSION" | "NEW_VERSION" | "VERSION_LOWER" 등
// level: "info" | "warn" | "error"
type LogFn = func(tag, level, message string)

type Runner struct {
	mu       sync.Mutex
	running  bool
	cancel   context.CancelFunc
	interval time.Duration
	store    *state.Store
	logFn    LogFn // 시스템 로그 콜백 (nil이면 log.Printf만 사용)
}

func NewRunner(interval time.Duration, store *state.Store) *Runner {
	return &Runner{interval: interval, store: store}
}

// SetLogFn은 시스템 로그 콜백을 등록합니다. Start() 호출 전에 설정해야 합니다.
func (r *Runner) SetLogFn(fn LogFn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logFn = fn
}

func (r *Runner) syslog(tag, level, message string) {
	log.Printf("[%s] %s", tag, message)
	if r.logFn != nil {
		r.logFn(tag, level, message)
	}
}

func (r *Runner) Start(rawURL, keyword string, options Options, onEvent func(Event)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return fmt.Errorf("monitor already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.running = true
	r.cancel = cancel

	go r.loop(ctx, rawURL, keyword, options, onEvent)

	return nil
}

func (r *Runner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return
	}

	r.cancel()
	r.running = false
	r.cancel = nil
}

func (r *Runner) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.running
}

func (r *Runner) loop(ctx context.Context, rawURL, keyword string, options Options, onEvent func(Event)) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
	lastMatched := false

	runCheck := func() {
		checkedAt := time.Now()
		result, err := crawler.FetchContentWithOptions(rawURL, options.DetectionOptions)
		if err != nil {
			onEvent(Event{Status: "error", URL: rawURL, Keyword: keyword, CheckedAt: checkedAt, Err: err})
			return
		}

		linkURL := options.LinkURL
		if linkURL == "" {
			linkURL = result.LinkURL
		}
		event := Event{Status: "checked", Mode: string(result.Mode), Content: result.Content, URL: rawURL, LinkURL: linkURL, NormalizedURL: result.NormalizedURL, Keyword: keyword, CheckedAt: checkedAt, DetectionOptions: options.DetectionOptions}

		if options.EnableKeywordAlert {
			matched := strings.Contains(strings.ToLower(result.Content), normalizedKeyword)
			event.Match = matched
			if matched && !lastMatched {
				log.Printf("[MATCH][%s] %s 에서 키워드 %q 감지", result.Mode, result.NormalizedURL, keyword)
			}

			if !matched && lastMatched {
				log.Printf("[CLEAR][%s] %s 에서 키워드 %q 가 더 이상 감지되지 않음", result.Mode, result.NormalizedURL, keyword)
			}

			lastMatched = matched
			if matched {
				event.Status = "matched"
			}
		}

		{
			versions, versionErr := version.ExtractVersionsWithOptions(result.Content, options.DetectionOptions)
			if versionErr != nil {
				event.VersionError = versionErr.Error()
				r.syslog("VERSION_PARSE_FAILED", "warn",
					fmt.Sprintf("url=%s mode=%s reason=%v", result.NormalizedURL, result.Mode, versionErr))
			} else {
				latestVersion := versions[0]
				event.LatestVersion = latestVersion
				previous, ok := r.store.Get(result.NormalizedURL)
				if !ok || previous.LastSeenVersion == "" {
					// 최초 감지 — 최신 버전을 기준점으로 저장하고 첫 감지 알림을 발송
					r.syslog("FIRST_SEEN_VERSION", "info",
						fmt.Sprintf("url=%s version=%s", result.NormalizedURL, latestVersion))
					if err := r.store.Set(result.NormalizedURL, state.Entry{LastSeenVersion: latestVersion, LastCheckedAt: checkedAt}); err != nil {
						onEvent(Event{Status: "error", URL: rawURL, NormalizedURL: result.NormalizedURL, Keyword: keyword, CheckedAt: checkedAt, Err: err})
						return
					}
					firstSeenEvent := event
					firstSeenEvent.Status = "version_changed"
					firstSeenEvent.LatestVersion = latestVersion
					firstSeenEvent.VersionChanged = true
					onEvent(firstSeenEvent)
					return
				} else if cmp := version.CompareWithOptions(latestVersion, previous.LastSeenVersion, options.DetectionOptions); cmp > 0 {
					// 저장된 버전보다 높은 모든 버전을 오래된 순서부터 알림
					newerVersions := versionsNewerThan(versions, previous.LastSeenVersion, options.DetectionOptions)
					fromVersion := previous.LastSeenVersion
					for _, nextVersion := range newerVersions {
						versionEvent := event
						versionEvent.Status = "version_changed"
						versionEvent.LatestVersion = nextVersion
						versionEvent.VersionPrevious = fromVersion
						versionEvent.VersionChanged = true
						r.syslog("NEW_VERSION", "info",
							fmt.Sprintf("url=%s %s → %s", result.NormalizedURL, fromVersion, nextVersion))
						onEvent(versionEvent)
						fromVersion = nextVersion
					}
					if err := r.store.Set(result.NormalizedURL, state.Entry{LastSeenVersion: latestVersion, LastCheckedAt: checkedAt}); err != nil {
						onEvent(Event{Status: "error", URL: rawURL, NormalizedURL: result.NormalizedURL, Keyword: keyword, CheckedAt: checkedAt, Err: err})
						return
					}
					return
				} else if cmp == 0 {
					// 동일 버전 — LastCheckedAt만 갱신, 알림 없음
					if err := r.store.Set(result.NormalizedURL, state.Entry{LastSeenVersion: latestVersion, LastCheckedAt: checkedAt}); err != nil {
						onEvent(Event{Status: "error", URL: rawURL, NormalizedURL: result.NormalizedURL, Keyword: keyword, CheckedAt: checkedAt, Err: err})
						return
					}
				} else {
					// 낮은 버전 감지 — 저장하지 않고 기록
					r.syslog("VERSION_LOWER", "warn",
						fmt.Sprintf("url=%s 감지=%q 저장=%q — 무시", result.NormalizedURL, latestVersion, previous.LastSeenVersion))
				}
			}
		}

		onEvent(event)
	}

	runCheck()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runCheck()
		}
	}
}

func versionsNewerThan(versions []string, previous string, detectionOptions json.RawMessage) []string {
	var newer []string
	for _, v := range versions {
		cmp := version.CompareWithOptions(v, previous, detectionOptions)
		if cmp == 1 {
			newer = append(newer, v)
			continue
		}
		if cmp == 0 {
			break
		}
	}
	for i, j := 0, len(newer)-1; i < j; i, j = i+1, j-1 {
		newer[i], newer[j] = newer[j], newer[i]
	}
	return newer
}
