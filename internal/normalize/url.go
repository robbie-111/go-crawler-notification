package normalize

import (
	"net/url"
	"strings"
)

func URL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}

	if parsed.Host == "github.com" {
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) >= 5 && parts[2] == "blob" {
			parsed.Scheme = "https"
			parsed.Host = "raw.githubusercontent.com"
			parsed.Path = "/" + strings.Join([]string{parts[0], parts[1], parts[3]}, "/") + "/" + strings.Join(parts[4:], "/")
			parsed.RawQuery = ""
			parsed.Fragment = ""
			return parsed.String()
		}
	}

	return parsed.String()
}
