package ui

import (
	"strings"
	"testing"
)

func TestFooterHintsPerResource(t *testing.T) {
	t.Parallel()
	artifacts := renderFooter(200, "artifacts", true)
	for _, want := range []string{"remediated", "export json"} {
		if !strings.Contains(artifacts, want) {
			t.Fatalf("artifacts footer missing %q: %s", want, artifacts)
		}
	}
	// The export sweep is libraries-only; other pages should not advertise it.
	if got := renderFooter(200, "repos", true); strings.Contains(got, "export json") {
		t.Fatalf("repos footer should not offer export: %s", got)
	}
}
