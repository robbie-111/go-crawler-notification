package monitor

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"go-crawler-demo/internal/crawler"
	"go-crawler-demo/internal/state"
	"go-crawler-demo/internal/version"
)

type Event struct {
	Status          string
	Mode            string
	Content         string
	URL             string
	NormalizedURL   string
	Keyword         string
	CheckedAt       time.Time
	Match           bool
	LatestVersion   string
	VersionChanged  bool
	VersionPrevious string
	VersionError    string
	Err             error
}

type Options struct {
	EnableKeywordAlert bool
	EnableVersionAlert bool
	AlertOnFirstSeen   bool
}

type Runner struct {
	mu       sync.Mutex
	running  bool
	cancel   context.CancelFunc
	interval time.Duration
	store    *state.Store
}

func NewRunner(interval time.Duration, store *state.Store) *Runner {
	return &Runner{interval: interval, store: store}
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
		result, err := crawler.FetchContent(rawURL)
		if err != nil {
			onEvent(Event{Status: "error", URL: rawURL, Keyword: keyword, CheckedAt: checkedAt, Err: err})
			return
		}

		event := Event{Status: "checked", Mode: string(result.Mode), Content: result.Content, URL: rawURL, NormalizedURL: result.NormalizedURL, Keyword: keyword, CheckedAt: checkedAt}

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

		if options.EnableVersionAlert {
			latestVersion, versionErr := version.ExtractLatest(result.Content)
			if versionErr != nil {
				event.VersionError = versionErr.Error()
				log.Printf("[VERSION_PARSE_FAILED][%s] url=%s normalized=%s reason=%v", result.Mode, rawURL, result.NormalizedURL, versionErr)
			} else {
				event.LatestVersion = latestVersion
				previous, ok := r.store.Get(result.NormalizedURL)
				if !ok || previous.LastSeenVersion == "" {
					if options.AlertOnFirstSeen {
						log.Printf("[FIRST_SEEN_VERSION] %s version=%s", result.NormalizedURL, latestVersion)
						event.VersionChanged = true
					}
					if err := r.store.Set(result.NormalizedURL, state.Entry{LastSeenVersion: latestVersion, LastCheckedAt: checkedAt}); err != nil {
						onEvent(Event{Status: "error", URL: rawURL, NormalizedURL: result.NormalizedURL, Keyword: keyword, CheckedAt: checkedAt, Err: err})
						return
					}
				} else if previous.LastSeenVersion != latestVersion {
					event.VersionChanged = true
					event.VersionPrevious = previous.LastSeenVersion
					log.Printf("[NEW_VERSION] %s 새 버전 감지: %s (이전: %s)", result.NormalizedURL, latestVersion, previous.LastSeenVersion)
					if err := r.store.Set(result.NormalizedURL, state.Entry{LastSeenVersion: latestVersion, LastCheckedAt: checkedAt}); err != nil {
						onEvent(Event{Status: "error", URL: rawURL, NormalizedURL: result.NormalizedURL, Keyword: keyword, CheckedAt: checkedAt, Err: err})
						return
					}
				} else {
					if err := r.store.Set(result.NormalizedURL, state.Entry{LastSeenVersion: latestVersion, LastCheckedAt: checkedAt}); err != nil {
						onEvent(Event{Status: "error", URL: rawURL, NormalizedURL: result.NormalizedURL, Keyword: keyword, CheckedAt: checkedAt, Err: err})
						return
					}
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
