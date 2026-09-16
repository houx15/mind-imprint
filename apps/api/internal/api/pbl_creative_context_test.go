package api

import (
	"strings"
	"testing"
)

func TestCreativeContextExcludesUnsubmittedResponses(t *testing.T) {
	got := creativeContext([]byte(`{"feeling":"chosen","pendingMotif":"private motif","responseDrafts":{"version":{"feedback":"private feedback"}},"trial":{"observation":"submitted"}}`))
	for _, forbidden := range []string{"private", "pendingMotif", "responseDrafts"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("draft reached model context: %s", got)
		}
	}
	if !strings.Contains(got, "chosen") || !strings.Contains(got, "submitted") {
		t.Fatal(got)
	}
	for _, malformed := range []string{"null", "[]", "broken"} {
		if creativeContext([]byte(malformed)) != "" {
			t.Fatal("invalid context accepted", malformed)
		}
	}
}
