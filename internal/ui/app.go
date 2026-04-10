package ui

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"go-crawler-demo/internal/crawler"
	"go-crawler-demo/internal/monitor"
	"go-crawler-demo/internal/state"
	"go-crawler-demo/internal/version"
)

const defaultInterval = 60 * time.Second

func NewWindow(app fyne.App) fyne.Window {
	store, err := state.Load(state.FilePath)
	if err != nil {
		log.Printf("state load failed: %v", err)
		store = state.New(state.FilePath)
	}

	runner := monitor.NewRunner(defaultInterval, store)
	window := app.NewWindow("Go Crawler Demo")
	window.Resize(fyne.NewSize(720, 340))

	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("https://example.com/changelog")

	keywordEntry := widget.NewEntry()
	keywordEntry.SetPlaceHolder("검색할 키워드 입력")

	statusLabel := widget.NewLabel("대기 중")
	statusLabel.Wrapping = fyne.TextWrapWord
	versionLabel := widget.NewLabel("최신 버전: -")
	versionLabel.Wrapping = fyne.TextWrapWord

	keywordCheck := widget.NewCheck("키워드 감지 사용", nil)
	keywordCheck.SetChecked(true)

	versionCheck := widget.NewCheck("새 버전 감지 사용", nil)
	versionCheck.SetChecked(true)

	firstSeenCheck := widget.NewCheck("첫 실행 시에도 알림", nil)

	testButton := widget.NewButton("URL 테스트", func() {
		rawURL := strings.TrimSpace(urlEntry.Text)
		keyword := strings.TrimSpace(keywordEntry.Text)

		if err := validateInput(rawURL, keyword, keywordCheck.Checked, versionCheck.Checked); err != nil {
			statusLabel.SetText(fmt.Sprintf("입력 오류: %v", err))
			return
		}

		statusLabel.SetText("테스트 중...")
		go func() {
			result, err := crawler.FetchContent(rawURL)
			fyne.Do(func() {
				if err != nil {
					statusLabel.SetText(fmt.Sprintf("테스트 실패: %v", err))
					return
				}

				log.Printf("[TEST_CONTENT_DUMP_START][%s] %s length=%d", result.Mode, rawURL, len(result.Content))
				log.Print(result.Content)
				log.Printf("[TEST_CONTENT_DUMP_END][%s] %s", result.Mode, rawURL)

				matched := false
				if keywordCheck.Checked {
					matched = strings.Contains(strings.ToLower(result.Content), strings.ToLower(keyword))
				}

				latestVersion := "-"
				if versionCheck.Checked {
					if parsed, parseErr := version.ExtractLatest(result.Content); parseErr != nil {
						latestVersion = "파싱 실패"
						log.Printf("[VERSION_PARSE_FAILED][%s] url=%s normalized=%s reason=%v", result.Mode, rawURL, result.NormalizedURL, parseErr)
					} else {
						latestVersion = parsed
					}
				}
				versionLabel.SetText(fmt.Sprintf("최신 버전: %s", latestVersion))

				if matched {
					statusLabel.SetText(fmt.Sprintf("테스트 완료 - 키워드 감지됨 [%s]", result.Mode))
					return
				}

				statusLabel.SetText(fmt.Sprintf("테스트 완료 - 키워드 없음 [%s]", result.Mode))
			})
		}()
	})

	button := widget.NewButton("모니터링 시작", nil)
	button.OnTapped = func() {
		if runner.IsRunning() {
			runner.Stop()
			button.SetText("모니터링 시작")
			statusLabel.SetText("모니터링 중지됨")
			return
		}

		rawURL := strings.TrimSpace(urlEntry.Text)
		keyword := strings.TrimSpace(keywordEntry.Text)

		if err := validateInput(rawURL, keyword, keywordCheck.Checked, versionCheck.Checked); err != nil {
			statusLabel.SetText(fmt.Sprintf("입력 오류: %v", err))
			return
		}

		options := monitor.Options{
			EnableKeywordAlert: keywordCheck.Checked,
			EnableVersionAlert: versionCheck.Checked,
			AlertOnFirstSeen:   firstSeenCheck.Checked,
		}

		err := runner.Start(rawURL, keyword, options, func(event monitor.Event) {
			fyne.Do(func() {
				modeSuffix := ""
				if event.Mode != "" {
					modeSuffix = fmt.Sprintf(" [%s]", event.Mode)
				}

				if event.LatestVersion != "" {
					versionLabel.SetText(fmt.Sprintf("최신 버전: %s", event.LatestVersion))
				} else if event.VersionError != "" {
					versionLabel.SetText("최신 버전: 파싱 실패")
				}

				switch {
				case event.Err != nil:
					statusLabel.SetText(fmt.Sprintf("오류: %v", event.Err))
				case event.VersionChanged && event.LatestVersion != "":
					statusLabel.SetText(fmt.Sprintf("새 버전 감지됨%s: %s", modeSuffix, event.LatestVersion))
				case event.Match:
					statusLabel.SetText(fmt.Sprintf("키워드 감지됨%s (%s)", modeSuffix, event.CheckedAt.Format("15:04:05")))
				default:
					statusLabel.SetText(fmt.Sprintf("점검 완료 - 키워드 없음%s (%s)", modeSuffix, event.CheckedAt.Format("15:04:05")))
				}
			})
		})
		if err != nil {
			statusLabel.SetText(fmt.Sprintf("시작 실패: %v", err))
			return
		}

		button.SetText("중지")
		statusLabel.SetText(fmt.Sprintf("모니터링 시작됨 (주기: %s)", defaultInterval))
	}

	content := container.NewVBox(
		widget.NewLabel("릴리즈 노트 URL과 키워드를 입력한 뒤 모니터링을 시작하세요."),
		widget.NewLabel("URL"),
		urlEntry,
		widget.NewLabel("키워드"),
		keywordEntry,
		keywordCheck,
		versionCheck,
		firstSeenCheck,
		container.NewGridWithColumns(2, testButton, button),
		versionLabel,
		statusLabel,
	)

	window.SetContent(container.NewPadded(content))
	window.SetCloseIntercept(func() {
		runner.Stop()
		window.Close()
	})

	return window
}

func validateInput(rawURL, keyword string, keywordEnabled, versionEnabled bool) error {
	if rawURL == "" {
		return fmt.Errorf("url을 입력하세요")
	}

	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return fmt.Errorf("유효한 url이 아닙니다")
	}

	if !keywordEnabled && !versionEnabled {
		return fmt.Errorf("키워드 감지 또는 새 버전 감지 중 하나는 활성화하세요")
	}

	if keywordEnabled && keyword == "" {
		return fmt.Errorf("키워드를 입력하세요")
	}

	return nil
}
