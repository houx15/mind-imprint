package claritytest

import (
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

// Updated after the 2026-09-22 prompt language review. Before/after requests
// and contract comparisons are in docs/reviews/2026-09-22-prompt-language.
// These hashes protect
// message roles, bytes/order, tool schemas and request options together.
// They do not claim that model output or teaching quality is deterministic.
//
//go:embed testdata/request_hashes.json
var requestHashesJSON []byte

func verifyRequestSnapshot(t *testing.T, class string, req gateway.ChatRequest) bool {
	t.Helper()
	var expected map[string]string
	if err := json.Unmarshal(requestHashesJSON, &expected); err != nil {
		t.Fatal(err)
	}
	key := strings.ReplaceAll(t.Name(), "/", "-")
	want, ok := expected[key]
	if !ok {
		return false
	}
	body, err := json.MarshalIndent(struct {
		Class   string
		Request gateway.ChatRequest
	}{class, req}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("%x", sha256.Sum256(body))
	if got != want {
		t.Errorf("production request changed: %s\nwant %s\ngot  %s\nCapture with PROMPT_CAPTURE_DIR and review the diff before deliberately updating the baseline.", key, want, got)
	}
	return true
}
