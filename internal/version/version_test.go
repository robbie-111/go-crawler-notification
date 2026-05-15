package version

import (
	"encoding/json"
	"strings"
	"testing"
)

var dateHeadingOptions = json.RawMessage(`{
  "version_parser": {
    "type": "date_heading",
    "heading_regex": "^\\s*(?:#{1,6}\\s*)?(\\d{4})\\.\\s*(\\d{1,2})\\.\\s*(\\d{1,2})\\.?\\s*$",
    "section_start_regex": "^\\s*(?:[-*]\\s*)?\\\\?\\[",
    "compare": "date"
  }
}`)

func TestExtractLatestAppLovinAndroidFacebook(t *testing.T) {
	content := `# Changelog

## 6.21.0.0
* Certified with Facebook SDK 6.21.0.
* Updated ad display failed error code.

## 6.20.0.0
* Certified with Facebook SDK 6.20.0.`

	got, err := ExtractLatest(content)
	if err != nil {
		t.Fatalf("ExtractLatest returned error: %v", err)
	}
	if got != "6.21.0.0" {
		t.Fatalf("ExtractLatest() = %q, want %q", got, "6.21.0.0")
	}
}

func TestExtractLatestAppLovinIOSFacebook(t *testing.T) {
	content := `# Changelog

## 6.21.1.0
* Certified with Facebook SDK 6.21.1.

## 6.21.0.0
* Certified with Facebook SDK 6.21.0.`

	got, err := ExtractLatest(content)
	if err != nil {
		t.Fatalf("ExtractLatest returned error: %v", err)
	}
	if got != "6.21.1.0" {
		t.Fatalf("ExtractLatest() = %q, want %q", got, "6.21.1.0")
	}
}

func TestExtractSectionAppLovinFacebook(t *testing.T) {
	content := `# Changelog

## 6.21.0.0
* Certified with Facebook SDK 6.21.0.
* Updated ad display failed error code.

## 6.20.0.0
* Certified with Facebook SDK 6.20.0.`

	got := ExtractSection(content, "6.21.0.0")
	if got == "" {
		t.Fatal("ExtractSection() returned empty string")
	}
	if want := "Updated ad display failed error code."; !strings.Contains(got, want) {
		t.Fatalf("ExtractSection() = %q, want to contain %q", got, want)
	}
	if notWant := "6.20.0.0"; strings.Contains(got, notWant) {
		t.Fatalf("ExtractSection() = %q, should not contain next version %q", got, notWant)
	}
}

func TestExtractSectionUnityPackageChangelog(t *testing.T) {
	content := `# Changelog

## [5.3.0] - 2026-05-04

### Added

- Added support for disabled domain reload in the Editor.

## [5.2.1] - 2026-03-27

### Fixed

- Fixed an older issue.`

	got := ExtractSection(content, "5.3.0")
	if got == "" {
		t.Fatal("ExtractSection() returned empty string")
	}
	if want := "disabled domain reload"; !strings.Contains(got, want) {
		t.Fatalf("ExtractSection() = %q, want to contain %q", got, want)
	}
	if notWant := "5.2.1"; strings.Contains(got, notWant) {
		t.Fatalf("ExtractSection() = %q, should not contain next version %q", got, notWant)
	}
}

func TestExtractVersions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		want    []string
	}{
		{
			name: "applovin ios facebook",
			content: `# Changelog

## 6.21.1.0
* Certified with Facebook SDK 6.21.1.

## 6.21.0.0
* Certified with Facebook SDK 6.21.0.

## 6.20.1.0
* Certified with Facebook SDK 6.20.1.`,
			want: []string{"6.21.1.0", "6.21.0.0", "6.20.1.0"},
		},
		{
			name: "unity package",
			content: `# Changelog

## [5.3.0] - 2026-05-04

## [5.2.1] - 2026-03-27`,
			want: []string{"5.3.0", "5.2.1"},
		},
		{
			name: "dt exchange",
			content: `# DT Exchange iOS Changelog

## Version 8.4.7

## Version 8.4.6`,
			want: []string{"8.4.7", "8.4.6"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractVersions(tc.content)
			if err != nil {
				t.Fatalf("ExtractVersions returned error: %v", err)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("ExtractVersions() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestCompareFourPartVersions(t *testing.T) {
	for _, tc := range []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "android adapter bump", a: "6.21.0.0", b: "6.20.0.0", want: 1},
		{name: "ios patch bump", a: "6.21.1.0", b: "6.21.0.0", want: 1},
		{name: "fourth part bump", a: "6.21.0.1", b: "6.21.0.0", want: 1},
		{name: "three part equals padded four part", a: "6.21.0", b: "6.21.0.0", want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Compare(tc.a, tc.b); got != tc.want {
				t.Fatalf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestExtractVersionsWithDateHeadingOptions(t *testing.T) {
	content := `Release Notes

2026. 05. 12.

[Android] 1.13.2.0
- Android SDK update.

2026. 04. 08.

[iOS] 1.5.2
- iOS SDK update.`

	got, err := ExtractVersionsWithOptions(content, dateHeadingOptions)
	if err != nil {
		t.Fatalf("ExtractVersionsWithOptions returned error: %v", err)
	}
	want := []string{"2026.05.12", "2026.04.08"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ExtractVersionsWithOptions() = %#v, want %#v", got, want)
	}
}

func TestExtractSectionWithDateHeadingOptions(t *testing.T) {
	content := `Release Notes

2026. 05. 12.

[Android] 1.13.2.0
- Android SDK update.

[Unity] 0.4.28
- Unity SDK update.

2026. 04. 08.

[iOS] 1.5.2
- iOS SDK update.`

	got := ExtractSectionWithOptions(content, "2026.05.12", dateHeadingOptions)
	if got == "" {
		t.Fatal("ExtractSectionWithOptions() returned empty string")
	}
	if want := "[Android] 1.13.2.0"; !strings.Contains(got, want) {
		t.Fatalf("ExtractSectionWithOptions() = %q, want to contain %q", got, want)
	}
	if want := "[Unity] 0.4.28"; !strings.Contains(got, want) {
		t.Fatalf("ExtractSectionWithOptions() = %q, want to contain %q", got, want)
	}
	if notWant := "[iOS] 1.5.2"; strings.Contains(got, notWant) {
		t.Fatalf("ExtractSectionWithOptions() = %q, should not contain next date section %q", got, notWant)
	}
}

func TestDateHeadingOptionsIgnoreSummaryTableDates(t *testing.T) {
	content := `Platform
Version
Release Date
Status

Android
1.13.2.0
2026. 05. 12.
latest

Android
1.12.4.18
2026. 03. 10.
stable

iOS
1.5.2
2026. 05. 12.
latest

2026. 05. 12.

[Android] 1.13.2.0
- 무결성 인증 개선
- 네이티브 라이브러리 변조 탐지 기능 추가

[iOS] 1.5.2
- 디버그 환경 탐지 개선

[Unity] 0.4.28
- (iOS) iOS SDK 1.5.2 업데이트

[Console]
- 무결성 인증 활성화 요청 모달 추가

2026. 04. 22.

[Android] 1.13.1.0
- 비신뢰 환경 탐지 기능 추가`

	versions, err := ExtractVersionsWithOptions(content, dateHeadingOptions)
	if err != nil {
		t.Fatalf("ExtractVersionsWithOptions returned error: %v", err)
	}
	wantVersions := []string{"2026.05.12", "2026.04.22"}
	if strings.Join(versions, ",") != strings.Join(wantVersions, ",") {
		t.Fatalf("ExtractVersionsWithOptions() = %#v, want %#v", versions, wantVersions)
	}

	section := ExtractSectionWithOptions(content, "2026.05.12", dateHeadingOptions)
	if section == "" {
		t.Fatal("ExtractSectionWithOptions() returned empty string")
	}
	for _, want := range []string{"[Android] 1.13.2.0", "[iOS] 1.5.2", "[Unity] 0.4.28", "[Console]"} {
		if !strings.Contains(section, want) {
			t.Fatalf("ExtractSectionWithOptions() = %q, want to contain %q", section, want)
		}
	}
	for _, notWant := range []string{"latest", "1.12.4.18", "[Android] 1.13.1.0"} {
		if strings.Contains(section, notWant) {
			t.Fatalf("ExtractSectionWithOptions() = %q, should not contain %q", section, notWant)
		}
	}
}

func TestCompareWithDateHeadingOptions(t *testing.T) {
	if got := CompareWithOptions("2026.05.12", "2026.04.08", dateHeadingOptions); got != 1 {
		t.Fatalf("CompareWithOptions() = %d, want 1", got)
	}
	if got := CompareWithOptions("2026.04.08", "2026.05.12", dateHeadingOptions); got != -1 {
		t.Fatalf("CompareWithOptions() = %d, want -1", got)
	}
}
