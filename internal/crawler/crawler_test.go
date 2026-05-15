package crawler

import (
	"strings"
	"testing"
)

func TestUnityDocsVersion(t *testing.T) {
	for _, tc := range []struct {
		version string
		want    string
	}{
		{version: "5.3.0", want: "5.3"},
		{version: "5.2.0-pre.2", want: "5.2"},
		{version: "4.12.1", want: "4.12"},
	} {
		t.Run(tc.version, func(t *testing.T) {
			if got := unityDocsVersion(tc.version); got != tc.want {
				t.Fatalf("unityDocsVersion(%q) = %q, want %q", tc.version, got, tc.want)
			}
		})
	}
}

func TestUnityChangelogURL(t *testing.T) {
	got := unityChangelogURL("com.unity.purchasing", "5.3.0")
	want := "https://docs.unity3d.com/Packages/com.unity.purchasing@5.3/changelog/CHANGELOG.html"
	if got != want {
		t.Fatalf("unityChangelogURL() = %q, want %q", got, want)
	}
}

func TestIsUnityPackageRegistryURL(t *testing.T) {
	if !isUnityPackageRegistryURL("https://packages.unity.com/com.unity.purchasing") {
		t.Fatal("expected Unity registry package URL")
	}
	if isUnityPackageRegistryURL("https://docs.unity3d.com/Packages/com.unity.purchasing@5.3/changelog/CHANGELOG.html") {
		t.Fatal("did not expect Unity docs URL to be registry URL")
	}
}

func TestRenderSelectedHTMLMarkdownPreservesNestedLists(t *testing.T) {
	rawHTML := `<html><body>
<nav>Navigation noise</nav>
<section class="page__content-wrapper">
  <h3>2026. 05. 12.</h3>
  <ul>
    <li>[Android] 1.13.2.0
      <ul>
        <li>무결성 인증 개선
          <ul>
            <li>서명 검증 대상을 라이브러리 Split APK까지 확장했습니다.</li>
          </ul>
        </li>
        <li>네이티브 라이브러리 변조 탐지 기능 추가</li>
      </ul>
    </li>
    <li>[iOS] 1.5.2</li>
    <li>디버그 환경 탐지 개선
      <ul>
        <li>frida attach 탐지를 개선했습니다.</li>
      </ul>
    </li>
  </ul>
</section>
</body></html>`

	got, err := renderSelectedHTMLMarkdown(rawHTML, "section.page__content-wrapper")
	if err != nil {
		t.Fatalf("renderSelectedHTMLMarkdown returned error: %v", err)
	}
	for _, want := range []string{
		"### 2026. 05. 12.",
		"- [Android] 1.13.2.0",
		"    - 무결성 인증 개선",
		"        - 서명 검증 대상을 라이브러리 Split APK까지 확장했습니다.",
		"- [iOS] 1.5.2",
		"    - frida attach 탐지를 개선했습니다.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderSelectedHTMLMarkdown() = %q, want to contain %q", got, want)
		}
	}
	if strings.Contains(got, "Navigation noise") {
		t.Fatalf("renderSelectedHTMLMarkdown() = %q, should not contain text outside selector", got)
	}
}
