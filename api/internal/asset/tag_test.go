package asset

import (
	"regexp"
	"testing"
)

func TestGenerateTagFormat(t *testing.T) {
	re := regexp.MustCompile(`^AST-[A-Z0-9]{8}$`)
	seen := map[string]bool{}
	for range 1000 {
		tag := GenerateTag()
		if !re.MatchString(tag) {
			t.Fatalf("bad tag %q", tag)
		}
		seen[tag] = true
	}
	if len(seen) < 990 {
		t.Fatalf("tags not random: %d distinct of 1000", len(seen))
	}
}

func TestNormalizeTagTrimsOnly(t *testing.T) {
	if got := NormalizeTag("  ast-001 \t"); got != "ast-001" {
		t.Fatalf("got %q", got)
	}
}
