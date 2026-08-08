package api_test

// studiostate_handler_test.go — Task 6 · GET /projects/{id}/studio-state reads
// the AI-managed studio_state so the frontend can resume a project at its
// stage. A fresh project's column is the DEFAULT '{...}' (no special-casing
// needed); one orchestrator turn that emits set_status persists an advanced
// stage, which this endpoint must then reflect.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mindimprint/api/internal/agent"
)

func TestGetStudioState_FreshProjectIsDefault(t *testing.T) {
	h, cookie, _ := orchestratorHandler(t, "")
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/studio-state", nil), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("studio-state = %d — %s", rr.Code, rr.Body)
	}

	var st struct {
		Stage    string `json:"stage"`
		OpenTool string `json:"openTool"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &st); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if st.Stage != "topic_discussion" || st.OpenTool != "chat" {
		t.Fatalf("fresh default wrong: %+v — %s", st, rr.Body)
	}
}

func TestGetStudioState_ReturnsPersistedState(t *testing.T) {
	// GET /studio-state echoes the persisted state verbatim (no reconcile). Set a
	// stage directly and assert it round-trips — transitions are deterministic
	// server-side now, not driven by a coach set_status tool.
	h, cookie, pool := orchestratorHandler(t, `{"narrate":"ok","tools":[]}`)
	setStudioStage(t, pool, seedProjectID, agent.StagePlanGeneration)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/studio-state", nil), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("studio-state = %d — %s", rr.Code, rr.Body)
	}

	var st struct {
		Stage string `json:"stage"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &st); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if st.Stage != "plan_generation" {
		t.Fatalf("stage=%v — %s", st.Stage, rr.Body)
	}
}
