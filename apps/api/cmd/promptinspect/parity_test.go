package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"mindimprint/api/internal/api"
)

func requestHash(e api.PromptExample) string {
	// Metadata/traces are not wire input. Include model class plus all request
	// options and tools, preserving message/tool order.
	b, err := json.Marshal(struct {
		Class   string
		Request any
	}{e.Class, e.Request})
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

// Baseline requests were rendered by main 03c31b99, not reconstructed from
// refactored text. Explicit external input allows a reviewed future migration
// to establish a new baseline without silently blessing the current output.
func TestMainRequestParity(t *testing.T) {
	expected := map[string]string{}
	baseline := os.Getenv("PROMPT_COMPARE_BASELINE")
	if baseline != "" {
		b, err := os.ReadFile(baseline)
		if err != nil {
			t.Fatal(err)
		}
		var old []api.PromptExample
		if err = json.Unmarshal(b, &old); err != nil {
			t.Fatal(err)
		}
		for _, e := range old {
			expected[e.ID] = requestHash(e)
		}
	} else {
		b, err := os.ReadFile("testdata/main_request_hashes.json")
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, &expected); err != nil {
			t.Fatal(err)
		}
	}
	if len(expected) == 0 {
		t.Fatal("empty baseline")
	}
	current := map[string]string{}
	for _, e := range examples() {
		current[e.ID] = requestHash(e)
	}
	for id, want := range expected {
		if current[id] != want {
			t.Errorf("request differs from main baseline: %s", id)
		}
	}
	if baseline != "" && !t.Failed() && os.Getenv("PROMPT_RECORD_BASELINE") == "1" {
		if err := os.MkdirAll("testdata", 0755); err != nil {
			t.Fatal(err)
		}
		b, err := json.MarshalIndent(expected, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile("testdata/main_request_hashes.json", append(b, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
