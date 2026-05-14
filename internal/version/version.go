package version

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// sectionHeadingRe는 "## Version X.Y.Z" 또는 "## Release X.Y.Z" 형태의 헤딩을 매칭합니다.
// 뒤에 앵커 HTML (<a href="..."></a>) 이 붙어있어도 매칭됩니다.
var sectionHeadingRe = regexp.MustCompile(`(?mi)^#{1,6}\s*(?:Version|Release)\s+\d+\.\d+\.\d+[^\n]*\n`)

// dividerRe는 마크다운 수평선(*** 또는 --- 또는 ___) 을 매칭합니다.
var dividerRe = regexp.MustCompile(`(?m)^\*{3,}$|^-{3,}$|^_{3,}$`)

const maxSectionLen = 2800 // Slack section text 3000자 제한에 안전 마진 적용

// ExtractSection은 content에서 targetVersion 헤딩부터 다음 버전 헤딩 직전까지의
// 텍스트를 추출합니다. 매칭 실패 시 빈 문자열을 반환합니다.
func ExtractSection(content, targetVersion string) string {
	if content == "" || targetVersion == "" {
		return ""
	}

	// "## Version 8.4.5" 또는 "## Version 8.4.5 <a ...></a>" 형태를 찾는 패턴
	startRe := regexp.MustCompile(
		`(?mi)^#{1,6}\s*(?:Version|Release)\s+` +
			regexp.QuoteMeta(targetVersion) +
			`\b[^\n]*\n`,
	)

	startLoc := startRe.FindStringIndex(content)
	if startLoc == nil {
		return ""
	}

	// 헤딩 줄 다음부터 시작
	body := content[startLoc[1]:]

	// 다음 버전 헤딩 위치 찾기
	nextLoc := sectionHeadingRe.FindStringIndex(body)
	if nextLoc != nil {
		body = body[:nextLoc[0]]
	}

	// 수평선(***) 제거
	body = dividerRe.ReplaceAllString(body, "")

	// 앞뒤 공백 정리, 연속 빈 줄 압축
	lines := strings.Split(body, "\n")
	var cleaned []string
	blankCount := 0
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		if trimmed == "" {
			blankCount++
			if blankCount <= 1 {
				cleaned = append(cleaned, "")
			}
		} else {
			blankCount = 0
			cleaned = append(cleaned, trimmed)
		}
	}
	result := strings.TrimSpace(strings.Join(cleaned, "\n"))

	// Slack 3000자 제한 안전 마진 적용
	if len(result) > maxSectionLen {
		// 단어 경계에서 자르기
		result = result[:maxSectionLen]
		if idx := strings.LastIndex(result, "\n"); idx > maxSectionLen-200 {
			result = result[:idx]
		}
		result = strings.TrimSpace(result) + "\n…"
	}

	return result
}

// versionPatterns는 우선순위 순으로 버전 문자열을 추출합니다.
var versionPatterns = []*regexp.Regexp{
	// 패턴 0: "Version X.Y.Z" 또는 "Release X.Y.Z" 키워드 — HTML 파싱 결과에 최적화
	//   Colly가 <body> 텍스트를 추출하면 ## 기호가 사라지고 "Version 8.4.5" 형태만 남음
	regexp.MustCompile(`(?mi)^\s*(?:version|release)\s+v?(\d+\.\d+\.\d+(?:[-+][A-Za-z0-9.\-]+)?)\b`),

	// 패턴 1: 마크다운 헤딩 스타일 — raw/markdown 파일에 최적화
	//   예: ## [1.2.3], # v2.0.0-beta
	regexp.MustCompile(`(?m)^\s{0,3}#{1,6}\s*\[?v?(\d+\.\d+\.\d+(?:[-+][A-Za-z0-9.\-]+)?)\]?`),

	// 패턴 2: 줄 시작 버전 목록 스타일
	//   예: [1.2.3] - 2024-01-01, v1.0.0
	regexp.MustCompile(`(?m)^\s*\[?v?(\d+\.\d+\.\d+(?:[-+][A-Za-z0-9.\-]+)?)\]?\s*(?:-|$)`),

	// 패턴 3: 본문 내 임의 위치 폴백
	//   예: Version 3.1.4, released v2.0.0+build
	regexp.MustCompile(`(?m)\bv?(\d+\.\d+\.\d+(?:[-+][A-Za-z0-9.\-]+)?)\b`),
}

// ExtractLatest는 content에서 가장 먼저 등장하는 semver 형식 버전을 반환합니다.
// 페이지 최상단에 최신 버전이 위치한다는 Changelog 관례를 따릅니다.
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

// parseVersionParts는 "X.Y.Z" 또는 "X.Y.Z-suffix" 형태를 분해합니다.
// 반환: (major, minor, patch, preRelease, error)
func parseVersionParts(v string) (int, int, int, string, error) {
	// pre-release suffix 분리 (- 또는 + 기준)
	core := v
	pre := ""
	if idx := strings.IndexAny(v, "-+"); idx != -1 {
		core = v[:idx]
		pre = v[idx:]
	}

	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return 0, 0, 0, "", fmt.Errorf("invalid version: %q", v)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, "", fmt.Errorf("invalid major in %q: %w", v, err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, "", fmt.Errorf("invalid minor in %q: %w", v, err)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, "", fmt.Errorf("invalid patch in %q: %w", v, err)
	}

	return major, minor, patch, pre, nil
}

// Compare는 두 버전 문자열을 비교합니다.
//
//	반환값:  1  → a > b  (a가 더 높은 버전)
//	         0  → a == b
//	        -1  → a < b  (a가 더 낮은 버전)
//	        -2  → 파싱 실패 (비교 불가)
//
// pre-release suffix (-alpha, -exp.2 등)가 있는 버전은
// 동일한 X.Y.Z에서 suffix 없는 버전보다 낮게 처리합니다.
func Compare(a, b string) int {
	aMaj, aMin, aPat, aPre, err := parseVersionParts(a)
	if err != nil {
		return -2
	}
	bMaj, bMin, bPat, bPre, err := parseVersionParts(b)
	if err != nil {
		return -2
	}

	// MAJOR 비교
	if aMaj != bMaj {
		if aMaj > bMaj {
			return 1
		}
		return -1
	}

	// MINOR 비교
	if aMin != bMin {
		if aMin > bMin {
			return 1
		}
		return -1
	}

	// PATCH 비교
	if aPat != bPat {
		if aPat > bPat {
			return 1
		}
		return -1
	}

	// X.Y.Z 동일 — pre-release suffix 비교
	// suffix 없음 > suffix 있음 (정식 릴리즈 > pre-release)
	switch {
	case aPre == "" && bPre == "":
		return 0
	case aPre == "" && bPre != "":
		return 1 // a가 정식 릴리즈, b가 pre-release → a > b
	case aPre != "" && bPre == "":
		return -1 // a가 pre-release, b가 정식 릴리즈 → a < b
	default:
		// 둘 다 pre-release — 문자열 비교
		if aPre == bPre {
			return 0
		}
		if aPre > bPre {
			return 1
		}
		return -1
	}
}
