package api_test

import (
	"encoding/json"
	"mindimprint/api/internal/gateway"
	"strings"
	"testing"
)

func TestHomepageCreativeDraftAndCodeHistoryReachCoach(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"Please inspect your current draft."}`), evidenceScript(`{"supported":true,"issues":[]}`))
	h, c, _, pool := liteHandlerWithProvider(t, provider)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	doc := `{"stage":"hero","feeling":"greenhouse-space-intent","pendingMotif":"UNSELECTED_DRAFT_ONLY","motifs":["greenhouse"],"hero":{"mode":"code","scene":"glass dome","prompt":"make a glass dome"}}`
	if _, err := pool.Exec(t.Context(), `INSERT INTO pbl_creative_direction(atom_id,document,revision) VALUES($1,$2,1)`, id, doc); err != nil {
		t.Fatal(err)
	}
	var version string
	if err := pool.QueryRow(t.Context(), `INSERT INTO pbl_code_version(atom_id,brief_revision,brief,html,feedback) VALUES($1,1,$2,'<html><body>PRIVATE_CODE_MARKER</body></html>','enlarge-glass-dome') RETURNING id`, id, doc).Scan(&version); err != nil {
		t.Fatal(err)
	}
	r := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+id+"/turn", `{"text":"What is my current design?"}`)
	if r.Code != 200 {
		t.Fatal(r.Body)
	}
	raw, _ := json.Marshal(provider.Requests)
	for _, want := range []string{"greenhouse-space-intent", version, "enlarge-glass-dome"} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("coach missing %s", want)
		}
	}
	if strings.Contains(string(raw), "PRIVATE_CODE_MARKER") {
		t.Fatal("full executable code leaked into coaching context")
	}
	if strings.Contains(string(raw), "UNSELECTED_DRAFT_ONLY") {
		t.Fatal("unfinished motif leaked into a model request")
	}
}

func TestHomepageInventedBiographyIsRepairedBeforePersistence(t *testing.T) {
	provider := gateway.NewSequenceStubProvider(
		evidenceScript(`{"reply":"You wrote: I won a robot competition."}`),
		evidenceScript(`{"supported":false,"issues":[{"quote":"I won a robot competition","reason":"No student source contains this claim"}]}`),
		evidenceScript(`{"reply":"What have you made that shows your interest in trying things?"}`),
		evidenceScript(`{"supported":true,"issues":[]}`),
	)
	h, c, _, pool := liteHandlerWithProvider(t, provider)
	id := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
	if _, err := pool.Exec(t.Context(), `INSERT INTO pbl_creative_direction(atom_id,document,revision) VALUES($1,'{"stage":"hero","feeling":"space garden","motifs":["garden"]}',1)`, id); err != nil {
		t.Fatal(err)
	}
	r := siteReq(t, h, c, "POST", "/api/v1/pbl/projects/"+id+"/turn", `{"text":"I like trying things."}`)
	if r.Code != 200 {
		t.Fatal(r.Body)
	}
	thread := siteReq(t, h, c, "GET", "/api/v1/pbl/projects/"+id+"/thread", "")
	if strings.Contains(thread.Body.String(), "robot competition") || !strings.Contains(thread.Body.String(), "What have you made") {
		t.Fatal("unverified biography escaped repair", thread.Body)
	}
	if provider.Calls != 4 {
		t.Fatalf("expected generate,check,repair,check: %d", provider.Calls)
	}
}
