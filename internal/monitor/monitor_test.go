package monitor

import (
	"strings"
	"testing"
)

func TestVersionsNewerThanReturnsOldestFirst(t *testing.T) {
	versions := []string{"6.21.1.0", "6.21.0.0", "6.20.1.0", "6.20.0.0"}
	got := versionsNewerThan(versions, "6.20.1.0")
	want := []string{"6.21.0.0", "6.21.1.0"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("versionsNewerThan() = %#v, want %#v", got, want)
	}
}

func TestVersionsNewerThanWithoutExactPrevious(t *testing.T) {
	versions := []string{"5.3.0", "5.2.1", "5.2.0"}
	got := versionsNewerThan(versions, "5.1.2")
	want := []string{"5.2.0", "5.2.1", "5.3.0"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("versionsNewerThan() = %#v, want %#v", got, want)
	}
}
