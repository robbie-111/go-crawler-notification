package crawler

import "testing"

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
