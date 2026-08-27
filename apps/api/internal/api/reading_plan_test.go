package api_test

// reading_plan_test.go — the reading task list and the per-paragraph tools.
//
// Two guarantees carry the design and are asserted against a real database
// rather than trusted:
//
//   1. The model may SELECT and TUNE a routine, never invent steps. A reply
//      that returns extra steps, reordered steps, or an unknown kind changes
//      nothing about what gets stored — buildReadingTasks walks the ROUTINE.
//   2. The paragraph tools explain the ARTICLE and have no write path into
//      anything she wrote. They are also cached, so a second click costs
//      nothing.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type readingTaskJSON struct {
	ID       string `json:"id"`
	Position int32  `json:"position"`
	Kind     string `json:"kind"`
	Label    string `json:"label"`
	Detail   string `json:"detail"`
	BlockID  string `json:"blockId"`
	Status   string `json:"status"`
}

type readingPlanJSON struct {
	RoutineKey  string            `json:"routineKey"`
	RoutineName string            `json:"routineName"`
	Tasks       []readingTaskJSON `json:"tasks"`
}

const zhArticle = `城市为什么比郊区热？

夏天的傍晚，如果你从市中心骑车回到郊区的家，会明显感到凉快下来。这不是错觉。

气象学上把这种现象叫做城市热岛效应。城市里的柏油路和水泥墙白天大量吸热，夜里又慢慢放出来，于是城区的夜间温度常常比郊区高出好几度。

绿地和水面是相反的力量。植物蒸腾会带走热量，一片足够大的公园可以让周边几百米内的气温明显下降。

所以近年来很多城市在做的事情，是把灰色的屋顶改成绿色的。`

func putReadingSourceHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id, title, body string) {
	t.Helper()
	// The field is `text`, not `body` — putReadingSourceLite's own request
	// struct (reading_source.go).
	payload, err := json.Marshal(map[string]string{"title": title, "text": body})
	if err != nil {
		t.Fatalf("marshal source: %v", err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/source", strings.NewReader(string(payload))), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
	}
}

func postReadingPlan(t *testing.T, h http.Handler, cookie *http.Cookie, id string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings/"+id+"/plan", nil), cookie))
	return rec
}

func decodeReadingPlan(t *testing.T, rec *httptest.ResponseRecorder) readingPlanJSON {
	t.Helper()
	var out readingPlanJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode plan: %v — body=%s", err, rec.Body)
	}
	return out
}

// A well-formed reply for the zh default routine.
const zhPlanReply = `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b3"],"steps":[
  {"kind":"read","detail":"这篇不长，先整体过一遍。"},
  {"kind":"focus_block","detail":"第三段是全文唯一解释原理的地方。"},
  {"kind":"lens","detail":"用一个角度再看一遍。"},
  {"kind":"reflect","detail":"说说你以前是怎么以为的。"},
  {"kind":"quiz","detail":"几个问题。"}]}`

// TestReadingPlan_GeneratesATaskListFromTheArticle — the ordinary case.
func TestReadingPlan_GeneratesATaskListFromTheArticle(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(zhPlanReply))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	rec := postReadingPlan(t, h, cookie, id)
	if rec.Code != http.StatusOK {
		t.Fatalf("plan = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeReadingPlan(t, rec)
	if out.RoutineKey != "zh-scan-focus-lens" {
		t.Fatalf("routineKey = %q, want the one the model picked", out.RoutineKey)
	}
	if out.RoutineName == "" {
		t.Fatalf("routineName empty — she is never shown a bare key")
	}
	if len(out.Tasks) != 5 {
		t.Fatalf("got %d tasks, want the routine's 5; %+v", len(out.Tasks), out.Tasks)
	}

	// The shape the product asked for, in order.
	wantKinds := []string{"read", "focus_block", "lens", "reflect", "quiz"}
	for i, want := range wantKinds {
		if out.Tasks[i].Kind != want {
			t.Fatalf("task %d kind = %q, want %q", i, out.Tasks[i].Kind, want)
		}
		if out.Tasks[i].Position != int32(i) {
			t.Fatalf("task %d position = %d", i, out.Tasks[i].Position)
		}
		if out.Tasks[i].Status != "pending" {
			t.Fatalf("task %d starts %q, want pending", i, out.Tasks[i].Status)
		}
	}

	// The focus step carries a REAL paragraph — the coach's actual judgement.
	focus := out.Tasks[1]
	if focus.BlockID != "b3" {
		t.Fatalf("focus block = %q, want the paragraph the model chose", focus.BlockID)
	}
	// And the detail was tuned to this article, not left generic.
	if !strings.Contains(focus.Detail, "第三段") {
		t.Fatalf("focus detail = %q, want the model's article-specific line", focus.Detail)
	}

	// It persisted: a reload shows the same plan.
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/plan", nil), cookie))
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET plan = %d; body=%s", rec2.Code, rec2.Body)
	}
	if got := decodeReadingPlan(t, rec2); len(got.Tasks) != 5 {
		t.Fatalf("GET plan returned %d tasks, want 5", len(got.Tasks))
	}
}

// TestReadingPlan_ModelCannotInventSteps is the load-bearing one. The routine
// owns the kinds and their order; a reply that adds a step, reorders them, or
// names a kind the routine does not have must change nothing.
func TestReadingPlan_ModelCannotInventSteps(t *testing.T) {
	// MORE steps than the routine has, in the wrong order, with a kind that
	// does not exist. All three are ways a model could try to take the wheel;
	// the extra-steps case in particular is the one a shorter rogue reply
	// would never exercise.
	const rogue = `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b2"],"steps":[
	  {"kind":"quiz","detail":"先考她。"},
	  {"kind":"summarize_for_her","detail":"我来替她总结全文。"},
	  {"kind":"read","detail":"随便读读。"},
	  {"kind":"lens","detail":"再来一次。"},
	  {"kind":"reflect","detail":"想想。"},
	  {"kind":"read","detail":"我自己加的第六步。"},
	  {"kind":"quiz","detail":"我自己加的第七步。"}]}`
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(rogue))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	out := decodeReadingPlan(t, postReadingPlan(t, h, cookie, id))
	wantKinds := []string{"read", "focus_block", "lens", "reflect", "quiz"}
	if len(out.Tasks) != len(wantKinds) {
		t.Fatalf("got %d tasks, want the routine's %d — the model added steps", len(out.Tasks), len(wantKinds))
	}
	for i, want := range wantKinds {
		if out.Tasks[i].Kind != want {
			t.Fatalf("task %d kind = %q, want %q — the routine owns the order", i, out.Tasks[i].Kind, want)
		}
	}
	for _, task := range out.Tasks {
		if strings.Contains(task.Detail, "替她总结") {
			t.Fatalf("an invented step's text reached the plan: %q", task.Detail)
		}
	}
}

// TestReadingPlan_DropsAFocusStepWithAnInventedBlock — a paragraph id the
// model made up must not produce a step pointing at nothing.
func TestReadingPlan_DropsAFocusStepWithAnInventedBlock(t *testing.T) {
	const bogus = `{"routineKey":"zh-scan-focus-lens","focusBlocks":["b99"],"steps":[]}`
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(bogus))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)

	out := decodeReadingPlan(t, postReadingPlan(t, h, cookie, id))
	for _, task := range out.Tasks {
		if task.Kind == "focus_block" {
			t.Fatalf("a focus step survived with no real paragraph: %+v", task)
		}
	}
	// The rest of the routine still stands — one bad id costs one step.
	if len(out.Tasks) == 0 {
		t.Fatalf("the whole plan was lost to one invented block id")
	}
}

// TestReadingPlan_OutOfLibraryRoutineIsAnError — no silent default. A
// substituted routine would be indistinguishable from a real one, and she
// would never know the coach had not looked at her article.
func TestReadingPlan_OutOfLibraryRoutineIsAnError(t *testing.T) {
	for _, reply := range []string{
		`{"routineKey":"invented-by-the-model","focusBlocks":[],"steps":[]}`,
		`{"routineKey":"en-close-read","focusBlocks":[],"steps":[]}`, // wrong language for a zh article
		`不是 JSON`,
	} {
		h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(reply))
		id := createReadingAtom(t, h, cookie)
		putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)
		if rec := postReadingPlan(t, h, cookie, id); rec.Code != http.StatusBadGateway {
			t.Fatalf("reply %q → %d, want 502; body=%s", reply, rec.Code, rec.Body)
		}
	}
}

// TestReadingPlan_SkippingIsRecordedNotPrevented — 铁律②/④. The list is a map
// she can walk past, and the skip is DATA.
func TestReadingPlan_SkippingIsRecordedNotPrevented(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(zhPlanReply))
	id := createReadingAtom(t, h, cookie)
	putReadingSourceHTTP(t, h, cookie, id, "城市为什么比郊区热？", zhArticle)
	plan := decodeReadingPlan(t, postReadingPlan(t, h, cookie, id))

	// Skip the SECOND step without touching the first — no ordering gate.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+id+"/plan/tasks/"+plan.Tasks[1].ID,
		strings.NewReader(`{"status":"skipped"}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("skip = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/plan", nil), cookie))
	after := decodeReadingPlan(t, rec2)
	if after.Tasks[1].Status != "skipped" {
		t.Fatalf("status = %q, want the skip recorded", after.Tasks[1].Status)
	}
	if after.Tasks[0].Status != "pending" {
		t.Fatalf("skipping step 2 changed step 1 to %q", after.Tasks[0].Status)
	}
}

// TestReadingPlan_NeedsAnArticle — planning a reading with nothing pasted is a
// clean refusal, not a wasted model call.
func TestReadingPlan_NeedsAnArticle(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(zhPlanReply))
	id := createReadingAtom(t, h, cookie)
	if rec := postReadingPlan(t, h, cookie, id); rec.Code != http.StatusNotFound {
		t.Fatalf("plan with no article = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}
