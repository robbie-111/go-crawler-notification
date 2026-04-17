package crawler

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"go-crawler-notification/internal/normalize"

	"github.com/gocolly/colly/v2"
)

const requestTimeout = 15 * time.Second

type ContentMode string

const (
	ContentModeRaw     ContentMode = "raw"
	ContentModeBody    ContentMode = "body"
	ContentModeHTML    ContentMode = "html"
	ContentModeHTMLRaw ContentMode = "html-raw"
)

type FetchResult struct {
	Content       string
	Mode          ContentMode
	NormalizedURL string
}

func FetchContent(rawURL string) (FetchResult, error) {
	normalizedURL := normalize.URL(rawURL)

	if _, err := url.ParseRequestURI(normalizedURL); err != nil {
		return FetchResult{}, fmt.Errorf("invalid url: %w", err)
	}

	contentType, err := detectContentType(normalizedURL)
	if err != nil {
		return FetchResult{}, err
	}

	if shouldUseRaw(normalizedURL, contentType) {
		content, err := fetchRawContent(normalizedURL)
		if err != nil {
			return FetchResult{}, err
		}

		return FetchResult{Content: content, Mode: ContentModeRaw, NormalizedURL: normalizedURL}, nil
	}

	content, mode, err := fetchHTMLContent(normalizedURL)
	if err != nil {
		return FetchResult{}, err
	}

	return FetchResult{Content: content, Mode: mode, NormalizedURL: normalizedURL}, nil
}

func detectContentType(rawURL string) (string, error) {
	request, err := http.NewRequest(http.MethodHead, rawURL, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: requestTimeout}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	return strings.ToLower(response.Header.Get("Content-Type")), nil
}

func shouldUseRaw(rawURL, contentType string) bool {
	if strings.Contains(contentType, "text/html") {
		return false
	}

	ext := strings.ToLower(path.Ext(rawURL))
	switch ext {
	case ".md", ".markdown", ".txt":
		return true
	case ".html", ".htm":
		return false
	}

	return strings.Contains(contentType, "text/plain") || strings.Contains(contentType, "text/markdown")
}

func fetchRawContent(rawURL string) (string, error) {
	client := &http.Client{Timeout: requestTimeout}
	response, err := client.Get(rawURL)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	content := strings.TrimSpace(string(body))
	if content == "" {
		return "", fmt.Errorf("empty raw content")
	}

	return content, nil
}

func fetchHTMLContent(rawURL string) (string, ContentMode, error) {
	collector := colly.NewCollector(
		colly.UserAgent("go-crawler-demo/1.0"),
	)
	collector.SetRequestTimeout(requestTimeout)

	var bodyText string
	var htmlText string
	var rawHTML string
	var visitErr error
	var responseReceived bool

	collector.OnHTML("body", func(element *colly.HTMLElement) {
		if bodyText == "" {
			bodyText = strings.TrimSpace(element.Text)
		}
	})

	collector.OnHTML("html", func(element *colly.HTMLElement) {
		if htmlText == "" {
			htmlText = strings.TrimSpace(element.Text)
		}
	})

	collector.OnResponse(func(response *colly.Response) {
		responseReceived = true
		rawHTML = strings.TrimSpace(string(response.Body))
	})

	collector.OnError(func(_ *colly.Response, err error) {
		visitErr = err
	})

	if err := collector.Visit(rawURL); err != nil {
		return "", "", err
	}

	if visitErr != nil {
		return "", "", visitErr
	}

	if bodyText != "" {
		return bodyText, ContentModeBody, nil
	}

	if htmlText != "" {
		return htmlText, ContentModeHTML, nil
	}

	if rawHTML != "" {
		return rawHTML, ContentModeHTMLRaw, nil
	}

	if responseReceived {
		return "", "", fmt.Errorf("empty html response after parsing")
	}

	return "", "", fmt.Errorf("empty page text")
}
