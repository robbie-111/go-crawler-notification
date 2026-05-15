package crawler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"go-crawler-notification/internal/normalize"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"golang.org/x/net/html"
)

const requestTimeout = 15 * time.Second

type ContentMode string

const (
	ContentModeRaw     ContentMode = "raw"
	ContentModeBody    ContentMode = "body"
	ContentModeHTML    ContentMode = "html"
	ContentModeHTMLRaw ContentMode = "html-raw"
	ContentModeUnity   ContentMode = "unity-package"
)

type FetchResult struct {
	Content       string
	Mode          ContentMode
	NormalizedURL string
	LinkURL       string
}

type Options struct {
	ContentSelector string `json:"content_selector"`
}

type unityPackageMeta struct {
	DistTags struct {
		Latest string `json:"latest"`
	} `json:"dist-tags"`
}

func FetchContent(rawURL string) (FetchResult, error) {
	return FetchContentWithOptions(rawURL, nil)
}

func FetchContentWithOptions(rawURL string, rawOptions json.RawMessage) (FetchResult, error) {
	options := parseOptions(rawOptions)
	normalizedURL := normalize.URL(rawURL)

	if _, err := url.ParseRequestURI(normalizedURL); err != nil {
		return FetchResult{}, fmt.Errorf("invalid url: %w", err)
	}

	if isUnityPackageRegistryURL(normalizedURL) {
		return fetchUnityPackageChangelog(normalizedURL)
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

	content, mode, err := fetchHTMLContent(normalizedURL, options)
	if err != nil {
		return FetchResult{}, err
	}

	return FetchResult{Content: content, Mode: mode, NormalizedURL: normalizedURL}, nil
}

func parseOptions(raw json.RawMessage) Options {
	var options Options
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "{}" {
		return options
	}
	_ = json.Unmarshal(raw, &options)
	return options
}

func isUnityPackageRegistryURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return parsed.Host == "packages.unity.com" && strings.Count(strings.Trim(parsed.Path, "/"), "/") == 0 && strings.HasPrefix(strings.Trim(parsed.Path, "/"), "com.unity.")
}

func fetchUnityPackageChangelog(registryURL string) (FetchResult, error) {
	latest, err := fetchUnityPackageLatest(registryURL)
	if err != nil {
		return FetchResult{}, err
	}

	parsed, err := url.Parse(registryURL)
	if err != nil {
		return FetchResult{}, err
	}
	packageName := strings.Trim(parsed.Path, "/")
	changelogURL := unityChangelogURL(packageName, latest)
	content, err := fetchUnityChangelogMarkdown(changelogURL)
	if err != nil {
		return FetchResult{}, err
	}

	return FetchResult{
		Content:       content,
		Mode:          ContentModeUnity,
		NormalizedURL: registryURL,
		LinkURL:       changelogURL,
	}, nil
}

func fetchUnityPackageLatest(registryURL string) (string, error) {
	client := &http.Client{Timeout: requestTimeout}
	response, err := client.Get(registryURL)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("unity registry returned %s", response.Status)
	}

	var meta unityPackageMeta
	if err := json.NewDecoder(response.Body).Decode(&meta); err != nil {
		return "", err
	}
	latest := strings.TrimSpace(meta.DistTags.Latest)
	if latest == "" {
		return "", fmt.Errorf("unity registry latest version is empty")
	}
	return latest, nil
}

func unityChangelogURL(packageName, latestVersion string) string {
	return fmt.Sprintf("https://docs.unity3d.com/Packages/%s@%s/changelog/CHANGELOG.html", packageName, unityDocsVersion(latestVersion))
}

func unityDocsVersion(version string) string {
	base := version
	if idx := strings.IndexAny(base, "-+"); idx != -1 {
		base = base[:idx]
	}
	parts := strings.Split(base, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return base
}

func fetchUnityChangelogMarkdown(changelogURL string) (string, error) {
	client := &http.Client{Timeout: requestTimeout}
	response, err := client.Get(changelogURL)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("unity changelog returned %s", response.Status)
	}

	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return "", err
	}

	var lines []string
	document.Find("h1, h2, h3, p, li").Each(func(_ int, selection *goquery.Selection) {
		text := strings.TrimSpace(selection.Text())
		if text == "" {
			return
		}
		switch goquery.NodeName(selection) {
		case "h1":
			lines = append(lines, "# "+text, "")
		case "h2":
			lines = append(lines, "## "+text, "")
		case "h3":
			lines = append(lines, "### "+text, "")
		case "li":
			lines = append(lines, "- "+text)
		default:
			lines = append(lines, text, "")
		}
	})

	content := strings.TrimSpace(strings.Join(lines, "\n"))
	if content == "" {
		return "", fmt.Errorf("empty unity changelog content")
	}
	return content, nil
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

func fetchHTMLContent(rawURL string, options Options) (string, ContentMode, error) {
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

	if rawHTML != "" && strings.TrimSpace(options.ContentSelector) != "" {
		content, err := renderSelectedHTMLMarkdown(rawHTML, options.ContentSelector)
		if err != nil {
			return "", "", err
		}
		if content != "" {
			return content, ContentModeHTML, nil
		}
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

func renderSelectedHTMLMarkdown(rawHTML, selector string) (string, error) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return "", err
	}
	selection := document.Find(selector).First()
	if selection.Length() == 0 {
		return "", fmt.Errorf("content selector %q did not match", selector)
	}
	return renderMarkdown(selection), nil
}

func renderMarkdown(selection *goquery.Selection) string {
	var lines []string
	for _, node := range selection.Nodes {
		renderNode(node, 0, &lines)
	}
	return cleanMarkdownLines(lines)
}

func renderNode(node *html.Node, depth int, lines *[]string) {
	if node.Type == html.TextNode {
		text := normalizeWhitespace(node.Data)
		if text != "" {
			appendLine(lines, text)
		}
		return
	}
	if node.Type != html.ElementNode {
		return
	}

	switch strings.ToLower(node.Data) {
	case "script", "style", "noscript":
		return
	case "h1":
		appendBlock(lines, "# "+textContent(node, false))
	case "h2":
		appendBlock(lines, "## "+textContent(node, false))
	case "h3":
		appendBlock(lines, "### "+textContent(node, false))
	case "h4":
		appendBlock(lines, "#### "+textContent(node, false))
	case "h5":
		appendBlock(lines, "##### "+textContent(node, false))
	case "h6":
		appendBlock(lines, "###### "+textContent(node, false))
	case "p":
		appendBlock(lines, textContent(node, false))
	case "br":
		appendLine(lines, "")
	case "ul", "ol":
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode && strings.EqualFold(child.Data, "li") {
				renderListItem(child, depth, lines)
			}
		}
		appendLine(lines, "")
	case "li":
		renderListItem(node, depth, lines)
	default:
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			renderNode(child, depth, lines)
		}
	}
}

func renderListItem(node *html.Node, depth int, lines *[]string) {
	text := textContent(node, true)
	if text != "" {
		appendLine(lines, strings.Repeat("    ", depth)+"- "+text)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && (strings.EqualFold(child.Data, "ul") || strings.EqualFold(child.Data, "ol")) {
			renderNode(child, depth+1, lines)
		}
	}
}

func textContent(node *html.Node, skipNestedLists bool) string {
	var buffer bytes.Buffer
	writeTextContent(&buffer, node, skipNestedLists)
	return normalizeWhitespace(buffer.String())
}

func writeTextContent(buffer *bytes.Buffer, node *html.Node, skipNestedLists bool) {
	if node.Type == html.TextNode {
		buffer.WriteString(node.Data)
		buffer.WriteByte(' ')
		return
	}
	if node.Type == html.ElementNode {
		name := strings.ToLower(node.Data)
		if name == "script" || name == "style" || name == "noscript" || (skipNestedLists && (name == "ul" || name == "ol")) {
			return
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		writeTextContent(buffer, child, skipNestedLists)
	}
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func appendBlock(lines *[]string, line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	appendLine(lines, line)
	appendLine(lines, "")
}

func appendLine(lines *[]string, line string) {
	*lines = append(*lines, strings.TrimRight(line, " \t"))
}

func cleanMarkdownLines(lines []string) string {
	var cleaned []string
	blankCount := 0
	for _, line := range lines {
		trimmedRight := strings.TrimRight(line, " \t")
		if strings.TrimSpace(trimmedRight) == "" {
			blankCount++
			if blankCount <= 1 {
				cleaned = append(cleaned, "")
			}
			continue
		}
		blankCount = 0
		cleaned = append(cleaned, trimmedRight)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}
