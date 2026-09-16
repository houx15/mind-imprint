package api

import (
	"encoding/json"
	"testing"
)

func TestCurrentArtifactSourceExcludesPriorEditTargets(t *testing.T) {
	input := []byte(`{"body":"新正文\n**保留格式**","edits":[{"old":"旧正文","new":"新正文"}],"paperEdits":[{}],"baseArtifactId":"old-id","replacesArtifactId":"older-id","paperLayout":{"title":"当前图形"},"guessed":["过时字段"]}`)
	before := string(input)
	var got map[string]json.RawMessage
	if err := json.Unmarshal([]byte(currentArtifactSource(input, []byte(`["当前假设"]`), []byte(`["尚未试用"]`))), &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"edits", "paperEdits", "baseArtifactId", "replacesArtifactId"} {
		if _, ok := got[key]; ok {
			t.Fatalf("prior target leaked: %s", key)
		}
	}
	var body string
	if err := json.Unmarshal(got["body"], &body); err != nil || body != "新正文\n**保留格式**" {
		t.Fatalf("source altered: %q %v", body, err)
	}
	if string(got["guessed"]) != `["当前假设"]` || string(got["admits"]) != `["尚未试用"]` || string(got["paperLayout"]) != `{"title":"当前图形"}` {
		t.Fatalf("current fields lost: %s", got)
	}
	if string(input) != before {
		t.Fatal("changed stored provenance")
	}
}
