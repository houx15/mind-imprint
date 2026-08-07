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
	"strings"
	"testing"
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
	out := `{"narrate":"ok","tools":[{"name":"set_status","args":{"stage":"plan_generation"}}]}`
	h, cookie, _ := orchestratorHandler(t, out)
	base := "/api/v1/projects/" + seedProjectID

	rrCoach := httptest.NewRecorder()
	h.ServeHTTP(rrCoach, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"go"}`)), cookie))
	if rrCoach.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rrCoach.Code, rrCoach.Body)
	}

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
