package version

import (
	"strings"
	"testing"
)

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
