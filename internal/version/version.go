package version

import (
	"fmt"
	"regexp"
	"strings"
)

var versionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^\s{0,3}#{1,6}\s*\[?v?(\d+\.\d+\.\d+(?:[-+][A-Za-z0-9.\-]+)?)\]?`),
	regexp.MustCompile(`(?m)^\s*\[?v?(\d+\.\d+\.\d+(?:[-+][A-Za-z0-9.\-]+)?)\]?\s*(?:-|$)`),
	regexp.MustCompile(`(?m)\bv?(\d+\.\d+\.\d+(?:[-+][A-Za-z0-9.\-]+)?)\b`),
}

func ExtractLatest(content string) (string, error) {
	normalized := strings.TrimSpace(content)
	if normalized == "" {
		return "", fmt.Errorf("empty content")
	}

	for _, pattern := range versionPatterns {
		match := pattern.FindStringSubmatch(normalized)
		if len(match) > 1 {
			return match[1], nil
		}
	}

	return "", fmt.Errorf("no version pattern matched")
}
