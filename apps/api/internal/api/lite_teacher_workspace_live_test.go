package api_test

// lite_teacher_workspace_live_test.go — 教师工作台的一轮，打真模型。
// Skipped unless LIVE_LLM=1 and a provider key is in the environment.
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 CGO_ENABLED=0 go test ./internal/api -run TestLiveWorkspace -v -count=1 -timeout 1800s
//
// The stub tests in lite_teacher_workspace_test.go prove the loop reads tool
// calls the tests themselves wrote. They cannot answer the question that
// decides whether this feature is usable: the system prompt says 「不要在回复里
// 写学生人数和学生姓名」, and the endpoint FAILS THE WHOLE TURN when a roster
// name appears in a reply that no tool grounded. A real model that ignores that
// line does not degrade the feature — it takes it away.
//
// So this test drives the real HTTP route (auth, roster, tool loop, grounding
// check) with the real router on ClassDialogue, and records every model call so
// a failure can be read rather than guessed at.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/config"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/library"
	"mindimprint/api/internal/liteparent"
	"mindimprint/api/internal/liteweek"
	"mindimprint/api/internal/liteworkspace"
	"mindimprint/api/internal/store/sqlc"
)

// liveWorkspaceRoster is the fixture class. Invented names, three of them, so
// the head-count check has a number to look for that is not 1.
var liveWorkspaceRoster = []struct{ email, name string }{
	{"ws-live-a@demo.local", "陈书宁"},
	{"ws-live-b@demo.local", "罗屿"},
	{"ws-live-c@demo.local", "顾行远"},
}

// liveWorkspaceScenarios are two openings of the same shape: everything but
// the article is decided, and the due date is relative so the model has to
// resolve it against today's Beijing date itself.
//
// They differ in one thing only — how the topic reaches the article. The
// catalogue's single climate article is titled 美国气候队 and carries no 气候变化
// in any headline, so the first scenario's query reaches it only through the
// discipline table, where 气候变化 is an alias of climate-ocean. The second is a
// plain headline match. Running both separates a model that cannot follow the
// tool rules from a search that cannot serve the topic; with only one of them,
// every failure reads like the model's.
//
// The first scenario used to return nothing at all, because the search matched
// a title substring only — the cliff between 「气候」 and 「气候变化」 that a
// teacher cannot tell apart. SearchLibrary now searches what an article is
// about, so both scenarios are expected to find the article.
//
// followUps are the teacher's next answers, sent one at a time while the card
// still has no material. The model asks before it commits an article, so
// without them the search_library → set_material ordering is never exercised.
var liveWorkspaceScenarios = []struct {
	name, opening string
	followUps     []string
}{
	{
		"topic-via-discipline",
		"这周读一篇气候变化的报道，写一篇议论文，周五交",
		[]string{"从阅读库里挑一篇跟气候有关的报道", "就用这篇，定下来"},
	},
	{
		"topic-in-headline",
		"这周读一篇亚运会的报道，写一篇议论文，周五交",
		[]string{"就用你找到的那篇，定下来", "确认，就这篇"},
	},
}

// wsLiveCall is one model call inside a turn.
type wsLiveCall struct {
	Text  string
	Tools []gateway.ToolCall
	Err   error
	// ToolResults are the tool-role messages this call was sent: the answers to
	// the previous call's tool calls. A rejected revise_section or a failed
	// set_material is only readable here.
	ToolResults []string
	Usage       gateway.ChatUsage
	Took        time.Duration
	// Prompt is the last user message the call was sent: for the class
	// summary, the facts the model was given.
	Prompt string
}

// wsLiveRecorder wraps the real provider and keeps one entry per model call.
//
// Complete delegates through gateway.Collect rather than calling the inner
// Completer directly: Collect is what decides streaming vs non-streaming, and
// the non-streaming path is the one that does not drop the last content chunk.
// A wrapper that only forwarded Stream would quietly put every live run back on
// the path that truncates tool-call JSON.
type wsLiveRecorder struct {
	inner gateway.Provider
	mu    sync.Mutex
	calls []wsLiveCall
}

func (r *wsLiveRecorder) Stream(ctx context.Context, res gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	return r.inner.Stream(ctx, res, req)
}

func (r *wsLiveRecorder) Complete(ctx context.Context, res gateway.Resolved, req gateway.ChatRequest) (gateway.ChatResult, error) {
	var results []string
	for i := len(req.Messages) - 1; i >= 0 && req.Messages[i].Role == gateway.RoleTool; i-- {
		results = append([]string{req.Messages[i].Content}, results...)
	}
	var prompt string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == gateway.RoleUser {
			prompt = req.Messages[i].Content
			break
		}
	}
	start := time.Now()
	out, err := gateway.Collect(ctx, r.inner, res, req)
	took := time.Since(start)
	r.mu.Lock()
	r.calls = append(r.calls, wsLiveCall{
		Text: out.Text, Tools: out.ToolCalls, Err: err,
		ToolResults: results, Usage: out.Usage, Took: took, Prompt: prompt,
	})
	r.mu.Unlock()
	return out, err
}

func (r *wsLiveRecorder) mark() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func (r *wsLiveRecorder) since(n int) []wsLiveCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]wsLiveCall{}, r.calls[n:]...)
}

// liveWorkspaceModel builds the real provider and the real class router, the
// same way cmd/api does.
func liveWorkspaceModel(t *testing.T) (gateway.Provider, func(string) gateway.KeyResolver, gateway.Resolved) {
	t.Helper()
	if testing.Short() {
		t.Skip("live model check needs a container and a network")
	}
	if os.Getenv("LIVE_LLM") != "1" {
		t.Skip("set LIVE_LLM=1 to run live prompt checks")
	}
	// config.Load requires DATABASE_URL. Nothing here reads it — the fixture
	// runs on its own container — so a placeholder fills an empty environment.
	if os.Getenv("DATABASE_URL") == "" {
		t.Setenv("DATABASE_URL", "postgres://unused-by-live-workspace-test")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if cfg.DashScopeKey == "" && cfg.DeepSeekKey == "" && cfg.AnthropicKey == "" && cfg.ZAIKey == "" {
		t.Skip("no provider key in the environment")
	}
	rs, err := gateway.NewResolvers(cfg)
	if err != nil {
		t.Fatalf("resolvers: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	resolved, err := rs.For(gateway.ClassDialogue)(ctx)
	if err != nil {
		t.Fatalf("class %s has no model here: %v", gateway.ClassDialogue, err)
	}
	prov := gateway.NewMuxProvider(map[string]gateway.Provider{
		gateway.KindOpenAICompatible: gateway.NewCatalogProvider(&http.Client{}),
		gateway.KindAnthropic:        gateway.NewAnthropicProvider(&http.Client{}),
	})
	return prov, rs.For, resolved
}

// liveWorkspaceFixture is liteTeacherFixtureWithProvider with the real class
// router in place of the fake lane resolvers, and a roster whose members have
// names a grounding check can see.
func liveWorkspaceFixture(t *testing.T, prov gateway.Provider, route func(string) gateway.KeyResolver) (http.Handler, *http.Cookie, string) {
	t.Helper()
	h, _, teacher, classID := liveWorkspaceFixturePool(t, prov, route)
	return h, teacher, classID
}

// liveWorkspaceFixturePool is liveWorkspaceFixture that also hands back the
// pool, for a scenario that seeds activity for the roster.
func liveWorkspaceFixturePool(t *testing.T, prov gateway.Provider, route func(string) gateway.KeyResolver) (http.Handler, *pgxpool.Pool, *http.Cookie, string) {
	t.Helper()
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool, Provider: prov,
		Route: route, SpecByID: cards.ByID,
	}).Handler()
	if _, err := pool.Exec(context.Background(), `UPDATE schools SET edition = 'lite'`); err != nil {
		t.Fatal(err)
	}
	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "ws-live-teacher@demo.local"))
	classID := createClassViaAPI(t, h, teacher, "高一（2）班 · 阅读写作")
	for _, s := range liveWorkspaceRoster {
		id := createStudent(t, pool, SeedSchoolID, s.email)
		enrollStudent(t, pool, id, classID)
		renameLiteStudent(t, pool, id, s.name)
	}
	return h, pool, teacher, classID
}

// wsLiveHeadCount reports head-count phrasings the reply states.
//
// Only the phrasings that need a person unit, so that 「两个选项」 and a tier
// number do not read as a head count. A bare 名/位 counts: in this reply they
// count people or nothing.
//
// 🚨 Whitespace is stripped first. The first version of this check compared
// contiguous strings and therefore could not see 「发给全班 3 人」, which a real
// run produced — the count was on screen and the check said clean. A detector
// that misses the thing it exists to find turns a passing run into evidence of
// nothing.
func wsLiveHeadCount(reply string, n int) []string {
	reply = wsLiveStripSpaces(reply)
	forms := []string{strconv.Itoa(n)}
	cn := []string{"零", "一", "两", "三", "四", "五", "六", "七", "八", "九"}
	if n < len(cn) {
		forms = append(forms, cn[n])
	}
	var bad []string
	for _, f := range forms {
		for _, p := range []string{f + "名", f + "位", f + "人", f + "个学生", f + "个同学", f + "个孩子"} {
			if strings.Contains(reply, p) {
				bad = append(bad, p)
			}
		}
	}
	return bad
}

// wsLiveStripSpaces removes every space, including the full-width one and the
// markdown line breaks a reply is full of, so a count written 「3 人」 compares
// the same as 「3人」.
func wsLiveStripSpaces(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// wsLiveInventedTitles returns the 《…》 titles in a reply that no catalogue
// article carries. A title the model copied out of a search result is fine; one
// it wrote itself is the failure this looks for.
func wsLiveInventedTitles(reply string) []string {
	known := map[string]bool{}
	for _, a := range library.All() {
		known[a.Title] = true
		known[a.ZhTitle] = true
	}
	var bad []string
	rest := reply
	for {
		i := strings.Index(rest, "《")
		if i < 0 {
			return bad
		}
		rest = rest[i+len("《"):]
		j := strings.Index(rest, "》")
		if j < 0 {
			return bad
		}
		if title := rest[:j]; !known[title] {
			bad = append(bad, title)
		}
		rest = rest[j+len("》"):]
	}
}

// wsLiveLogCalls prints each model call of one turn: the tools it reached for
// with their arguments, and the prose it wrote. This is the diagnosable part —
// a failed assertion below is only readable next to what the model actually
// sent.
func wsLiveLogCalls(t *testing.T, calls []wsLiveCall) {
	t.Helper()
	t.Logf("model calls this turn: %d (budget %d)", len(calls), liteworkspace.ToolLoopMax)
	for i, c := range calls {
		reasoning := 0
		if c.Usage.ReasoningTokens != nil {
			reasoning = *c.Usage.ReasoningTokens
		}
		t.Logf("  call %d usage in=%d out=%d reasoning=%d took=%.1fs",
			i+1, c.Usage.InputTokens, c.Usage.OutputTokens, reasoning, c.Took.Seconds())
		for _, r := range c.ToolResults {
			t.Logf("  call %d was sent tool result: %s", i+1, r)
		}
		for _, tc := range c.Tools {
			args, _ := json.Marshal(tc.Args)
			t.Logf("  call %d tool %s %s", i+1, tc.Name, args)
		}
		if strings.TrimSpace(c.Text) != "" {
			t.Logf("  call %d text: %s", i+1, c.Text)
		}
		if c.Err != nil {
			t.Logf("  call %d error: %v", i+1, c.Err)
		}
	}
}

// TestWsLiveHeadCount pins the detector the live run's decisive assertion uses.
// It is not gated: a live check is only worth what its detector can see, and
// this one already went blind once on a real reply that said 「发给全班 3 人」.
func TestWsLiveHeadCount(t *testing.T) {
	for _, tc := range []struct {
		reply string
		want  bool
	}{
		{"发给全班 3 人", true},
		{"发给全班3人", true},
		{"这份作业发给 3 名学生", true},
		{"三位同学还没交", true},
		{"这些学生都还没开始", false},
		{"发给哪些学生？", false},
		// The count is 3 here, so a tier or an option count that happens to be
		// 3 must not read as a head count.
		{"难度 3 档", false},
		{"给你 3 个选项", false},
	} {
		got := len(wsLiveHeadCount(tc.reply, 3)) > 0
		if got != tc.want {
			t.Errorf("wsLiveHeadCount(%q) = %v, want %v", tc.reply, got, tc.want)
		}
	}
}

// TestLiveWorkspaceAssignmentTurn drives the assignment surface against a real
// model and checks the five things the endpoint's own rules depend on.
func TestLiveWorkspaceAssignmentTurn(t *testing.T) {
	prov, route, resolved := liveWorkspaceModel(t)
	t.Logf("class %s → provider=%s model=%s tier=%s",
		gateway.ClassDialogue, resolved.Provider, resolved.Model, resolved.Tier)
	for _, sc := range liveWorkspaceScenarios {
		t.Run(sc.name, func(t *testing.T) {
			liveWorkspaceRun(t, prov, route, sc.opening, sc.followUps)
		})
	}
}

func liveWorkspaceRun(t *testing.T, prov gateway.Provider, route func(string) gateway.KeyResolver, opening string, followUps []string) {
	rec := &wsLiveRecorder{inner: prov}
	h, teacher, classID := liveWorkspaceFixture(t, rec, route)

	// The card starts where the real one starts: AssignmentForm opens on
	// emptySettings("reading"). A harness that sent a blank artifact would let
	// the server default the kind on its own and would not be testing the card
	// the teacher actually has in front of her.
	artifact := map[string]any{"kind": "reading"}
	var turns []liteworkspace.Turn
	var tools []gateway.ToolCall
	var choices []liteworkspace.Choice
	var replies []string
	var pending []liteworkspace.Choice

	// say sends one teacher turn: typed text, or a clicked option when said is
	// empty. Both are her turn as far as the endpoint is concerned. A clicked
	// option carries its slug back exactly as the client does — that round trip
	// IS the feature, so a harness that dropped it would test the old path.
	say := func(said string, clicked liteworkspace.Choice) {
		t.Helper()
		before := rec.mark()
		body, _ := json.Marshal(map[string]any{
			"surface": "assignment", "classId": classID,
			"artifact": artifact, "turns": turns, "text": said,
			"choiceId": clicked.ID, "choiceSlug": clicked.Slug,
		})
		start := time.Now()
		res := postWorkspaceTurn(t, h, teacher, string(body))
		calls := rec.since(before)
		if said == "" {
			said = clicked.ID
			t.Logf("--- teacher clicked: %s (slug %q) (%.1fs)", clicked.ID, clicked.Slug, time.Since(start).Seconds())
		} else {
			t.Logf("--- teacher said: %s (%.1fs)", said, time.Since(start).Seconds())
		}
		wsLiveLogCalls(t, calls)
		t.Logf("  HTTP %d body: %s", res.Code, res.Body.String())
		for _, c := range calls {
			tools = append(tools, c.Tools...)
		}
		if res.Code != http.StatusOK {
			t.Fatalf("turn failed: HTTP %d — %s", res.Code, res.Body.String())
		}
		out := decodeWorkspaceTurn(t, res)
		for k, v := range out.Patch {
			artifact[k] = v
		}
		turns = append(turns,
			liteworkspace.Turn{Role: "teacher", Text: said},
			liteworkspace.Turn{Role: "ai", Text: out.Reply},
		)
		replies = append(replies, out.Reply)
		choices = append(choices, out.Choices...)
		pending = out.Choices
		// 4. The turn terminated inside the budget. Reaching the cap fails the
		// request, so a 200 already proves it; the number is what we came for.
		if len(calls) > liteworkspace.ToolLoopMax {
			t.Errorf("turn used %d model calls, budget is %d", len(calls), liteworkspace.ToolLoopMax)
		}
		if n := len(out.Choices); n != 0 && (n < 2 || n > liteworkspace.MaxChoices) {
			t.Errorf("turn offered %d choices, want 2 to %d", n, liteworkspace.MaxChoices)
		}
	}

	// The scripted teacher answers the way the product lets her: she clicks an
	// offered button, and types the next scripted line only when there is no
	// button. Typing past a pending ask_choice is what a harness does, not what
	// she does, and it leaves the model answering a 「这篇」 with no antecedent.
	say(opening, liteworkspace.Choice{})
	next := 0
	for len(replies) < liveWorkspaceMaxTurns {
		if artifact["readingSource"] != nil && artifact["dueInput"] != nil {
			break
		}
		if len(pending) > 0 {
			say("", pending[0])
			continue
		}
		if next >= len(followUps) {
			break
		}
		say(followUps[next], liteworkspace.Choice{})
		next++
	}

	named := map[string]bool{}
	for _, tc := range tools {
		named[tc.Name] = true
	}
	t.Logf("tools this run: %v; card now: %v", toolOrder(tools), artifact)
	t.Logf("invented slugs this run: %d of %d library set_material calls",
		wsLiveInventedSlugs(tools), wsLiveLibraryMaterials(tools))

	// 1. Tools rather than invention: a library material must be preceded by a
	// search, and its slug must be a real one. A material she TAPPED needs no
	// search at all — that is the point of the option payload — so the ordering
	// only binds when the model set it with a tool of its own.
	if named["set_material"] {
		if first(tools, "search_library") > first(tools, "set_material") || !named["search_library"] {
			t.Errorf("set_material without a preceding search_library: %v", toolOrder(tools))
		}
	} else if artifact["readingSource"] == nil {
		t.Errorf("no set_material after %d turns — the model never chose a material; tools: %v",
			len(replies), toolOrder(tools))
	}
	if slug, _ := artifact["slug"].(string); slug != "" {
		if _, ok := library.BySlug(slug); !ok {
			t.Errorf("slug %q is not in the catalogue", slug)
		}
	}

	// 6. The card can actually hold what the conversation put on it.
	//
	// 🚨 This is the one no earlier assertion could see, because it is not
	// about the prose or the tools but about the CARD. A browser pass caught
	// the model setting kind=writing, searching the library and announcing
	// 「材料：已选「美国气候队」这篇报道」 — while the card renders the material
	// row only for a reading homework, so the article was nowhere, the article
	// button did nothing, and publishing would have sent a writing task with no
	// article to a class that had been told one was chosen.
	//
	// Both openings say 读一篇…的报道. A homework that ends up without a
	// material, or with one on a card that cannot show it, has dropped half of
	// what she asked for.
	kind, _ := artifact["kind"].(string)
	source, _ := artifact["readingSource"].(string)
	if source != "" && kind != "reading" {
		t.Errorf("the card carries a material (%s) on a %q homework, where nothing renders it: %v",
			source, kind, artifact)
	}
	if source == "" {
		t.Errorf("she asked for a reading and the card ends with no material at all: %v", artifact)
	}
	if kind != "reading" {
		t.Errorf("she asked for a reading and the card ends as kind %q: %v", kind, artifact)
	}

	// 2. An absolute Beijing wall-clock due time, not 周五.
	due, _ := artifact["dueInput"].(string)
	if due == "" {
		t.Errorf("no dueInput after the teacher said 周五")
	} else {
		if _, err := liteworkspace.BeijingWallToUTC(due); err != nil {
			t.Errorf("dueInput %q does not parse: %v", due, err)
		}
		if strings.Contains(due, "周") {
			t.Errorf("dueInput %q is a relative phrase", due)
		}
	}

	// 3. No student name and no head count. The endpoint already fails a turn
	// whose reply carries an UNGROUNDED name, so this checks the stricter rule
	// the prompt states: no roster name at all, grounded or not.
	for i, reply := range replies {
		flat := wsLiveStripSpaces(reply)
		for _, s := range liveWorkspaceRoster {
			if strings.Contains(flat, s.name) {
				t.Errorf("reply %d names a student: %s\n%s", i+1, s.name, reply)
			}
		}
		if bad := wsLiveHeadCount(reply, len(liveWorkspaceRoster)); len(bad) > 0 {
			t.Errorf("reply %d states a head count %v\n%s", i+1, bad, reply)
		}
		if bad := wsLiveInventedTitles(reply); len(bad) > 0 {
			t.Errorf("reply %d cites a title that is not in the catalogue: %v\n%s", i+1, bad, reply)
		}
	}

	// 5. Every choice offered is a short label rather than a sentence. The
	// count is checked per turn, above, where an out-of-range count belongs to
	// one turn rather than to the run.
	for _, c := range choices {
		t.Logf("choice %s = %q (%d runes)", c.ID, c.Label, len([]rune(c.Label)))
		if n := len([]rune(c.Label)); n > liveWorkspaceMaxLabelRunes {
			t.Errorf("choice label runs to %d runes: %q", n, c.Label)
		}
		if strings.ContainsAny(c.Label, "。！？") {
			t.Errorf("choice label is a sentence: %q", c.Label)
		}
	}
}

// liveWorkspaceMaxLabelRunes is where a button label stops being a phrase.
// 「个性化阅读，各找一篇」 is 10; a label that carries a whole article title runs
// past 15. Nothing in product code enforces this — the prompt asks for 名词或
// 短动宾 and this is the number that reads it back.
//
// 🚨 So this is a canary, not a guard. The whole file is skipped unless
// LIVE_LLM=1, which means a green CI says nothing about label length: the only
// thing that ever checks it is a human running this test against a real model.
const liveWorkspaceMaxLabelRunes = 15

// liveWorkspaceMaxTurns bounds the scripted teacher. A card that still has no
// material and no due time after six of her turns is a stall worth failing on;
// a real teacher would have filled the cells herself long before.
const liveWorkspaceMaxTurns = 6

// first returns the index of the first call to name, or len(tools) when it was
// never called, so an absent tool sorts after every present one.
func first(tools []gateway.ToolCall, name string) int {
	for i, tc := range tools {
		if tc.Name == name {
			return i
		}
	}
	return len(tools)
}

// wsLiveLibraryMaterials counts the set_material calls that named a library
// article, and wsLiveInventedSlugs counts how many of those named one that does
// not exist. Their ratio is the number the second pass moved and the third pass
// is watching: each invented slug costs two model calls to recover from.
func wsLiveLibraryMaterials(tools []gateway.ToolCall) int {
	n := 0
	for _, tc := range tools {
		if tc.Name == "set_material" {
			if source, _ := tc.Args["source"].(string); source == "library" {
				n++
			}
		}
	}
	return n
}

func wsLiveInventedSlugs(tools []gateway.ToolCall) int {
	n := 0
	for _, tc := range tools {
		if tc.Name != "set_material" {
			continue
		}
		slug, _ := tc.Args["slug"].(string)
		if slug == "" {
			continue
		}
		if _, found := library.BySlug(slug); !found {
			n++
		}
	}
	return n
}

func toolOrder(tools []gateway.ToolCall) []string {
	out := make([]string, 0, len(tools))
	for _, tc := range tools {
		out = append(out, tc.Name)
	}
	return out
}

// ---------------------------------------------------------------------------
// Round two (Task 17): the home, parent report and assignment surfaces and the
// class summary, each run wsLiveRuns times against the real model.
//
//	LIVE_LLM=1 CGO_ENABLED=0 go test ./internal/api -run 'TestLive(WorkspaceRound2|ClassSummary)' -v -count=1 -timeout 1800s
//
// Every assertion logs one line starting with RESULT, so a run can be read as
// a table: grep RESULT on the -v output. A failed hard assertion also fails
// the run. A soft one logs WARN and does not: it records something worth
// reading (the model needed the retry, it used a different filter) that the
// product itself allows.
// ---------------------------------------------------------------------------

// wsLiveRuns is how many times each scenario runs. One run says little about a
// model that answers differently each time.
const wsLiveRuns = 3

func wsLiveVerdict(t *testing.T, scenario string, run int, name string, ok, soft bool, detail string) {
	t.Helper()
	v := "PASS"
	switch {
	case !ok && soft:
		v = "WARN"
	case !ok:
		v = "FAIL"
	}
	t.Logf("RESULT | %s | run %d | %s | %s | %s", scenario, run, name, v, detail)
	if !ok && !soft {
		t.Errorf("%s run %d: %s: %s", scenario, run, name, detail)
	}
}

// wsLiveWireWordPattern matches the English wire values of a card cell that
// must never reach the teacher (§12.1): the kind and source enums.
var wsLiveWireWordPattern = regexp.MustCompile(`(?i)\b(reading|writing|project|library|personalized|classWeekly|parentReports|assignmentNew)\b`)

// wsLiveSlugShape is a hyphenated lower-case word that starts with a letter.
// liteworkspace's own slugPattern also matches 2026-09-18, which every reply
// with a due date contains; starting with a letter keeps dates out.
var wsLiveSlugShape = regexp.MustCompile(`[a-z][a-z0-9]*(?:-[a-z0-9]+)+`)

// wsLiveWireWords returns every wire value and slug-shaped word in text, and
// every real catalogue slug it contains in any shape.
func wsLiveWireWords(text string) []string {
	var bad []string
	bad = append(bad, wsLiveWireWordPattern.FindAllString(text, -1)...)
	bad = append(bad, wsLiveSlugShape.FindAllString(text, -1)...)
	for _, a := range library.All() {
		if a.Slug != "" && strings.Contains(text, a.Slug) && !slices.Contains(bad, a.Slug) {
			bad = append(bad, a.Slug)
		}
	}
	return bad
}

// wsLiveTurn is the workspace response with every field a scenario reads,
// navigate included (workspaceTurnJSON predates it).
type wsLiveTurn struct {
	Reply    string                 `json:"reply"`
	Choices  []liteworkspace.Choice `json:"choices"`
	Patch    map[string]any         `json:"patch"`
	Cards    []wsLiveCard           `json:"cards"`
	Navigate *struct {
		View         string  `json:"view"`
		ClassID      string  `json:"classId"`
		UserID       *string `json:"userId"`
		AssignmentID *string `json:"assignmentId"`
		Label        string  `json:"label"`
	} `json:"navigate"`
}

type wsLiveCard struct {
	Kind string          `json:"kind"`
	Rows json.RawMessage `json:"rows"`
}

// wsLivePost sends one workspace turn and logs everything the model did in it.
func wsLivePost(t *testing.T, h http.Handler, teacher *http.Cookie, rec *wsLiveRecorder, req map[string]any) (int, string, wsLiveTurn, []wsLiveCall) {
	t.Helper()
	before := rec.mark()
	body, _ := json.Marshal(req)
	start := time.Now()
	res := postWorkspaceTurn(t, h, teacher, string(body))
	took := time.Since(start)
	calls := rec.since(before)
	t.Logf("--- %v turn: text=%q choiceId=%q choiceSlug=%q → HTTP %d in %.1fs",
		req["surface"], req["text"], req["choiceId"], req["choiceSlug"], res.Code, took.Seconds())
	wsLiveLogCalls(t, calls)
	t.Logf("  response body: %s", res.Body.String())
	var out wsLiveTurn
	if res.Code == http.StatusOK {
		if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode turn: %v — %s", err, res.Body.String())
		}
	}
	return res.Code, res.Body.String(), out, calls
}

func wsLiveToolArgs(calls []wsLiveCall, name string) []map[string]any {
	var out []map[string]any
	for _, c := range calls {
		for _, tc := range c.Tools {
			if tc.Name == name {
				out = append(out, tc.Args)
			}
		}
	}
	return out
}

func wsLiveUsage(calls []wsLiveCall) string {
	in, out := 0, 0
	var took time.Duration
	for _, c := range calls {
		in += c.Usage.InputTokens
		out += c.Usage.OutputTokens
		took += c.Took
	}
	return fmt.Sprintf("calls=%d in=%d out=%d model_time=%.1fs", len(calls), in, out, took.Seconds())
}

// TestLiveWorkspaceRound2AssignmentRecommends — scenario 1. She asks for a
// reading homework and leaves the article to the AI.
func TestLiveWorkspaceRound2AssignmentRecommends(t *testing.T) {
	prov, route, resolved := liveWorkspaceModel(t)
	t.Logf("class %s → provider=%s model=%s", gateway.ClassDialogue, resolved.Provider, resolved.Model)
	for run := 1; run <= wsLiveRuns; run++ {
		t.Run(fmt.Sprintf("run-%d", run), func(t *testing.T) { liveAssignmentRecommendRun(t, prov, route, run) })
	}
}

const wsLiveRecommendOpening = "给这个班布置一篇阅读作业，下周三交，文章你帮我挑"

func liveAssignmentRecommendRun(t *testing.T, prov gateway.Provider, route func(string) gateway.KeyResolver, run int) {
	const sc = "1-assignment-recommend"
	rec := &wsLiveRecorder{inner: prov}
	h, teacher, classID := liveWorkspaceFixture(t, rec, route)
	artifact := map[string]any{"kind": "reading"}
	var turns []liteworkspace.Turn
	var all []wsLiveCall
	var texts []string // every reply and option label she saw
	var offered []liteworkspace.Choice
	var recommended []string // zhTitles recommend_articles put on the canvas
	said, clicked := wsLiveRecommendOpening, liteworkspace.Choice{}
	for turn := 1; turn <= 3; turn++ {
		req := map[string]any{
			"surface": "assignment", "classId": classID, "artifact": artifact,
			"turns": turns, "text": said, "choiceId": clicked.ID, "choiceSlug": clicked.Slug,
		}
		code, body, out, calls := wsLivePost(t, h, teacher, rec, req)
		all = append(all, calls...)
		wsLiveVerdict(t, sc, run, fmt.Sprintf("turn %d HTTP 200", turn), code == http.StatusOK, false, body)
		if code != http.StatusOK {
			break
		}
		for k, v := range out.Patch {
			artifact[k] = v
		}
		texts = append(texts, out.Reply)
		for _, c := range out.Choices {
			texts = append(texts, c.Label)
		}
		offered = append(offered, out.Choices...)
		for _, card := range out.Cards {
			if card.Kind != "articles" {
				continue
			}
			var rows []struct {
				ZhTitle string `json:"zhTitle"`
			}
			_ = json.Unmarshal(card.Rows, &rows)
			for _, r := range rows {
				recommended = append(recommended, r.ZhTitle)
			}
		}
		herTurn := said
		if herTurn == "" {
			herTurn = clicked.ID
		}
		turns = append(turns, liteworkspace.Turn{Role: "teacher", Text: herTurn}, liteworkspace.Turn{Role: "ai", Text: out.Reply})
		recommendedYet := len(wsLiveToolArgs(all, "recommend_articles")) > 0
		if recommendedYet || len(out.Choices) == 0 {
			break
		}
		said, clicked = "", out.Choices[0]
	}

	var wire []string
	for _, s := range texts {
		wire = append(wire, wsLiveWireWords(s)...)
	}
	wsLiveVerdict(t, sc, run, "no wire value or slug in replies and labels", len(wire) == 0, false,
		fmt.Sprintf("found %v in %q", wire, texts))

	wsLiveVerdict(t, sc, run, "recommend_articles used", len(wsLiveToolArgs(all, "recommend_articles")) > 0, false,
		fmt.Sprintf("tools %v", wsLiveAllTools(all)))

	slugged, missing := 0, []string{}
	for _, c := range offered {
		if c.Slug == "" {
			continue
		}
		slugged++
		if c.Article == nil || c.Article.Slug != c.Slug || c.Article.ZhTitle == "" {
			missing = append(missing, c.ID)
		}
	}
	wsLiveVerdict(t, sc, run, "every option with an article carries article", len(missing) == 0, false,
		fmt.Sprintf("%d options with a slug, %d without article %v", slugged, len(missing), missing))

	// An option whose label names a recommended article but carries no slug is
	// a button that cannot set the article it names.
	var unslugged []string
	for _, c := range offered {
		for _, title := range recommended {
			if title != "" && strings.Contains(c.Label, title) && c.Slug == "" {
				unslugged = append(unslugged, c.Label)
			}
		}
	}
	wsLiveVerdict(t, sc, run, "options naming a recommended article carry its slug", len(unslugged) == 0, false,
		fmt.Sprintf("recommended %v; unslugged %v; options %+v", recommended, unslugged, offered))
	wsLiveVerdict(t, sc, run, "article options offered (observation)", slugged > 0, true,
		fmt.Sprintf("%d of %d options carry an article", slugged, len(offered)))
	t.Logf("USAGE | %s | run %d | %s", sc, run, wsLiveUsage(all))
}

func wsLiveAllTools(calls []wsLiveCall) []string {
	var out []string
	for _, c := range calls {
		out = append(out, toolOrder(c.Tools)...)
	}
	return out
}

// wsLivePastedPassage is original text written for this test: long enough to
// be a real reading material, with digits and a quote so a model that
// "tidies" it is visible.
const wsLivePastedPassage = `北京的很多新建小区开始铺透水砖。雨水落到透水砖上，会穿过砖缝和下面的碎石层，慢慢渗进土里，而不是全部流进下水道。` +
	`有研究人员在一个试点小区测了三年，发现暴雨时路面积水的时间比普通路面短了一半左右。` +
	`不过透水砖也有问题：砖缝容易被泥沙堵住，每年需要用高压水枪清理 2 次，否则几年后就和普通路面差不多了。` +
	`一位负责维护的工人说：“铺的时候大家都很积极，后来没人管，效果就慢慢没了。”` +
	`所以，一座城市能不能真正“吸水”，不只取决于用了什么材料，还取决于有没有人长期维护。`

// TestLiveWorkspaceRound2AssignmentPastedText — scenario 2. She pastes a
// passage and asks for it to be the material.
func TestLiveWorkspaceRound2AssignmentPastedText(t *testing.T) {
	prov, route, resolved := liveWorkspaceModel(t)
	t.Logf("class %s → provider=%s model=%s", gateway.ClassDialogue, resolved.Provider, resolved.Model)
	for run := 1; run <= wsLiveRuns; run++ {
		t.Run(fmt.Sprintf("run-%d", run), func(t *testing.T) {
			const sc = "2-assignment-pasted-text"
			rec := &wsLiveRecorder{inner: prov}
			h, teacher, classID := liveWorkspaceFixture(t, rec, route)
			said := "这次的阅读材料就用我贴的这段，请把它设为材料：\n\n" + wsLivePastedPassage
			code, body, out, calls := wsLivePost(t, h, teacher, rec, map[string]any{
				"surface": "assignment", "classId": classID,
				"artifact": map[string]any{"kind": "reading"}, "text": said,
			})
			wsLiveVerdict(t, sc, run, "HTTP 200", code == http.StatusOK, false, body)
			source, _ := out.Patch["readingSource"].(string)
			wsLiveVerdict(t, sc, run, "readingSource is text", source == "text", false,
				fmt.Sprintf("patch=%v; set_material args %v", out.Patch, wsLiveToolArgs(calls, "set_material")))
			text, _ := out.Patch["text"].(string)
			wsLiveVerdict(t, sc, run, "text is her passage verbatim", text == wsLivePastedPassage, false,
				fmt.Sprintf("got %d runes, want %d; got %q", len([]rune(text)), len([]rune(wsLivePastedPassage)), text))
			wire := wsLiveWireWords(out.Reply)
			for _, c := range out.Choices {
				wire = append(wire, wsLiveWireWords(c.Label)...)
			}
			wsLiveVerdict(t, sc, run, "no wire value or slug in reply and labels", len(wire) == 0, false,
				fmt.Sprintf("found %v in %q", wire, out.Reply))
			t.Logf("USAGE | %s | run %d | %s", sc, run, wsLiveUsage(calls))
		})
	}
}

// TestLiveWorkspaceRound2Home — scenario 3. One of three students is active
// this week, so 「这周谁还没动」 has an answer of two, not the class size.
func TestLiveWorkspaceRound2Home(t *testing.T) {
	prov, route, resolved := liveWorkspaceModel(t)
	t.Logf("class %s → provider=%s model=%s", gateway.ClassDialogue, resolved.Provider, resolved.Model)
	for run := 1; run <= wsLiveRuns; run++ {
		t.Run(fmt.Sprintf("run-%d", run), func(t *testing.T) { liveHomeRun(t, prov, route, run) })
	}
}

func liveHomeRun(t *testing.T, prov gateway.Provider, route func(string) gateway.KeyResolver, run int) {
	const sc = "3-home"
	rec := &wsLiveRecorder{inner: prov}
	h, pool, teacher, classID := liveWorkspaceFixturePool(t, rec, route)
	active := userIDByEmail(t, pool, liveWorkspaceRoster[0].email)
	atom := seedLiteReadingForUser(t, pool, active, "active", 600)
	seedBucket(t, pool, atom, liteweek.Day(time.Now()), 600)

	rosterNames := make([]string, 0, len(liveWorkspaceRoster))
	for _, s := range liveWorkspaceRoster {
		rosterNames = append(rosterNames, s.name)
	}

	const first = "这周谁还没动"
	code, body, out, calls := wsLivePost(t, h, teacher, rec, map[string]any{
		"surface": "home", "classId": classID, "text": first,
	})
	wsLiveVerdict(t, sc, run, "turn 1 HTTP 200 (server §6 passed)", code == http.StatusOK, false, body)

	listed := wsLiveToolArgs(calls, "list_students")
	wsLiveVerdict(t, sc, run, "turn 1 list_students used", len(listed) > 0, false,
		fmt.Sprintf("tools %v", wsLiveAllTools(calls)))
	inactiveFilter := false
	for _, a := range listed {
		if a["filter"] == "inactive_this_week" {
			inactiveFilter = true
		}
	}
	wsLiveVerdict(t, sc, run, "turn 1 filter inactive_this_week (observation)", inactiveFilter, true,
		fmt.Sprintf("list_students args %v", listed))

	// The same evidence the server uses, rebuilt here from what the canvas
	// received: the names and sizes of the student lists, plus the class size.
	var grounded []string
	counts := []int{len(liveWorkspaceRoster)}
	for _, card := range out.Cards {
		if card.Kind != "students" {
			continue
		}
		var rows []struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(card.Rows, &rows)
		counts = append(counts, len(rows))
		for _, r := range rows {
			grounded = append(grounded, r.Name)
		}
	}
	badNames := liteworkspace.UngroundedNames(out.Reply, rosterNames, grounded)
	badCounts := liteworkspace.UngroundedCounts(out.Reply, counts)
	wsLiveVerdict(t, sc, run, "turn 1 reply has no ungrounded name or count",
		len(badNames) == 0 && len(badCounts) == 0, false,
		fmt.Sprintf("names %v counts %v (grounded %v %v); reply %q", badNames, badCounts, grounded, counts, out.Reply))

	// The prompt's own rule is stricter: no name and no head count at all.
	var named []string
	for _, n := range rosterNames {
		if strings.Contains(wsLiveStripSpaces(out.Reply), n) {
			named = append(named, n)
		}
	}
	heads := append(wsLiveHeadCount(out.Reply, 2), wsLiveHeadCount(out.Reply, 3)...)
	stated := liteworkspace.StatedCounts(out.Reply)
	wsLiveVerdict(t, sc, run, "turn 1 reply names nobody and states no count (prompt rule)",
		len(named) == 0 && len(heads) == 0 && len(stated) == 0, true,
		fmt.Sprintf("names %v head counts %v stated %v; reply %q", named, heads, stated, out.Reply))
	wire := wsLiveWireWords(out.Reply)
	wsLiveVerdict(t, sc, run, "turn 1 no wire value in reply", len(wire) == 0, false, fmt.Sprintf("%v in %q", wire, out.Reply))

	turns := []liteworkspace.Turn{{Role: "teacher", Text: first}, {Role: "ai", Text: out.Reply}}
	code2, body2, out2, calls2 := wsLivePost(t, h, teacher, rec, map[string]any{
		"surface": "home", "classId": classID, "text": "带我去看周报", "turns": turns,
	})
	wsLiveVerdict(t, sc, run, "turn 2 HTTP 200", code2 == http.StatusOK, false, body2)
	view := ""
	if out2.Navigate != nil {
		view = out2.Navigate.View
	}
	wsLiveVerdict(t, sc, run, "turn 2 navigate offered", out2.Navigate != nil, false,
		fmt.Sprintf("navigate view %q; open_page args %v; tools %v", view, wsLiveToolArgs(calls2, "open_page"), wsLiveAllTools(calls2)))
	wsLiveVerdict(t, sc, run, "turn 2 navigate is classWeekly", view == "classWeekly", false, fmt.Sprintf("view %q", view))
	wire2 := wsLiveWireWords(out2.Reply)
	wsLiveVerdict(t, sc, run, "turn 2 no wire value in reply", len(wire2) == 0, false, fmt.Sprintf("%v in %q", wire2, out2.Reply))
	t.Logf("USAGE | %s | run %d | turn1 %s | turn2 %s", sc, run, wsLiveUsage(calls), wsLiveUsage(calls2))
}

// TestLiveWorkspaceRound2ParentReport — scenario 4. A report is drafted by the
// real route (itself a live check of the draft path), then she asks for the
// reading section to be more specific.
func TestLiveWorkspaceRound2ParentReport(t *testing.T) {
	prov, route, resolved := liveWorkspaceModel(t)
	t.Logf("class %s → provider=%s model=%s", gateway.ClassDialogue, resolved.Provider, resolved.Model)
	for run := 1; run <= wsLiveRuns; run++ {
		t.Run(fmt.Sprintf("run-%d", run), func(t *testing.T) { liveParentReportRun(t, prov, route, run) })
	}
}

// liveParentReportClassmate is on the roster and must never appear in her
// report.
const liveParentReportClassmate = "罗屿"

func liveParentReportRun(t *testing.T, prov gateway.Provider, route func(string) gateway.KeyResolver, run int) {
	const sc = "4-parent-report"
	rec := &wsLiveRecorder{inner: prov}
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool, Provider: rec, Route: route, SpecByID: cards.ByID,
	}).Handler()
	mustExec(t, pool, `UPDATE schools SET edition = 'lite'`)
	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "pr-live-teacher@demo.local"))
	classID := createClassViaAPI(t, h, teacher, "高一（2）班 · 阅读写作")
	student := createStudent(t, pool, SeedSchoolID, "pr-live-student@demo.local")
	enrollStudent(t, pool, student, classID)
	renameLiteStudent(t, pool, student, "陈书宁")
	mate := createStudent(t, pool, SeedSchoolID, "pr-live-mate@demo.local")
	enrollStudent(t, pool, mate, classID)
	renameLiteStudent(t, pool, mate, liveParentReportClassmate)
	backdateWeeklyStart(t, pool, classID)

	// Two finished readings in the default range, each with a moment, reading
	// time on three days, and a new keyword: enough for a reading section with
	// something specific to say.
	today := liteweek.Day(time.Now())
	first := seedParentReading(t, pool, student, true)
	seedBucket(t, pool, first, today.AddDate(0, 0, -3), 1500)
	seedBucket(t, pool, first, today.AddDate(0, 0, -4), 900)
	second := seedLiteReadingForUser(t, pool, student, "finished", 0)
	mustExec(t, pool, `UPDATE reading SET title = '透水砖能让城市吸水吗', finished_at = $2 WHERE atom_id = $1`,
		second, today.AddDate(0, 0, -5).Add(9*time.Hour))
	seedReport(t, pool, second, "reading", `{"version":1,"moments":[{"quote":"砖缝被堵住以后就没用了","where":""}]}`)
	seedBucket(t, pool, second, today.AddDate(0, 0, -5), 1200)
	seedWeekKeyword(t, pool, student, "海绵城市", today.AddDate(0, 0, -3).Add(8*time.Hour))

	before := rec.mark()
	start := time.Now()
	var gen parentReportResp
	code, body := parentDo(t, h, teacher, "POST", parentReportsPath(classID, student), "", &gen)
	genCalls := rec.since(before)
	t.Logf("--- generate report → HTTP %d in %.1fs", code, time.Since(start).Seconds())
	wsLiveLogCalls(t, genCalls)
	t.Logf("  response body: %s", body)
	if code != http.StatusCreated {
		wsLiveVerdict(t, sc, run, "report generated (fixture)", false, false, body)
		return
	}
	rep := gen.Report
	draftErr := ""
	if gen.DraftError != nil {
		draftErr = *gen.DraftError
	}
	wsLiveVerdict(t, sc, run, "draft accepted (fixture)", gen.DraftError == nil, true, "draftError "+draftErr)
	wsLiveVerdict(t, sc, run, "report has a reading section (fixture)", slices.Contains(rep.Sections, "reading"), false,
		fmt.Sprintf("sections %v", rep.Sections))

	bodyNow := rep.Body
	if bodyNow == nil {
		bodyNow = map[string]string{}
	}
	code2, body2, out, calls := wsLivePost(t, h, teacher, rec, map[string]any{
		"surface": "parentReport", "reportId": rep.ID, "text": "把阅读那段写得具体一点",
		"artifact": map[string]any{"body": bodyNow},
	})
	wsLiveVerdict(t, sc, run, "turn HTTP 200 (server §6 passed)", code2 == http.StatusOK, false, body2)

	var rejected []string
	for _, c := range calls {
		for _, r := range c.ToolResults {
			if strings.Contains(r, `"ok":false`) {
				rejected = append(rejected, r)
			}
		}
	}
	t.Logf("revise_section attempts %d, rejected %d: %v", len(wsLiveToolArgs(calls, "revise_section")), len(rejected), rejected)

	patchBody, _ := out.Patch["body"].(map[string]any)
	reading, _ := patchBody["reading"].(string)
	wsLiveVerdict(t, sc, run, "patch carries body.reading", strings.TrimSpace(reading) != "", false,
		fmt.Sprintf("patch %v; tools %v", out.Patch, wsLiveAllTools(calls)))

	facts := liteparent.VisibleFacts(rep.Facts, rep.Hidden)
	checkErr := agent.CheckLiteParentSections(map[string]string{"reading": reading}, []string{"reading"}, facts,
		[]string{liveParentReportClassmate})
	wsLiveVerdict(t, sc, run, "body.reading passes CheckLiteParentSections", reading != "" && checkErr == nil, false,
		fmt.Sprintf("err %v; text %q", checkErr, reading))

	keys := make([]string, 0, len(patchBody))
	for k := range patchBody {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	wsLiveVerdict(t, sc, run, "only the reading section revised", len(keys) == 1 && keys[0] == "reading", false,
		fmt.Sprintf("revised %v", keys))
	wsLiveVerdict(t, sc, run, "reading text changed (observation)", reading != "" && reading != bodyNow["reading"], true,
		fmt.Sprintf("before %d runes %q; after %d runes", len([]rune(bodyNow["reading"])), bodyNow["reading"], len([]rune(reading))))
	wire := wsLiveWireWords(out.Reply)
	wsLiveVerdict(t, sc, run, "no section key or wire value in reply", len(wire) == 0, false, fmt.Sprintf("%v in %q", wire, out.Reply))
	t.Logf("USAGE | %s | run %d | generate %s | turn %s", sc, run, wsLiveUsage(genCalls), wsLiveUsage(calls))
}

// TestLiveClassSummary — scenario 5. Three students: one earns a praise card
// (a new keyword), one a watch card (no activity), one neither.
func TestLiveClassSummary(t *testing.T) {
	prov, route, _ := liveWorkspaceModel(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	digest, err := route(gateway.ClassDigest)(ctx)
	if err != nil {
		t.Fatalf("class %s has no model here: %v", gateway.ClassDigest, err)
	}
	t.Logf("class %s → provider=%s model=%s", gateway.ClassDigest, digest.Provider, digest.Model)
	for run := 1; run <= wsLiveRuns; run++ {
		t.Run(fmt.Sprintf("run-%d", run), func(t *testing.T) { liveClassSummaryRun(t, prov, route, run) })
	}
}

var wsLiveSummaryCount = regexp.MustCompile(`(班级人数|本周活跃学生数|完成项数)：(\d+)`)

func liveClassSummaryRun(t *testing.T, prov gateway.Provider, route func(string) gateway.KeyResolver, run int) {
	const sc = "5-class-summary"
	rec := &wsLiveRecorder{inner: prov}
	h, pool, teacher, classID := liveWorkspaceFixturePool(t, rec, route)
	now := time.Now()
	praised := userIDByEmail(t, pool, liveWorkspaceRoster[0].email)
	a := seedLiteReadingForUser(t, pool, praised, "active", 900)
	seedBucket(t, pool, a, liteweek.Day(now), 900)
	seedWeekKeyword(t, pool, praised, "海绵城市", now.Add(-time.Minute))
	quiet := userIDByEmail(t, pool, liveWorkspaceRoster[2].email)
	b := seedLiteReadingForUser(t, pool, quiet, "active", 300)
	seedBucket(t, pool, b, liteweek.Day(now), 300)
	// liveWorkspaceRoster[1] has no activity at all: a never_used watch card.

	start := time.Now()
	res := doJSON(t, h, teacher, "POST", summaryPath(classID), "")
	calls := rec.since(0)
	t.Logf("--- class summary → HTTP %d in %.1fs", res.Code, time.Since(start).Seconds())
	wsLiveLogCalls(t, calls)
	t.Logf("  response body: %s", res.Body.String())
	prompt := ""
	if len(calls) > 0 {
		prompt = calls[0].Prompt
	}
	t.Logf("  facts the model was given:\n%s", prompt)

	wsLiveVerdict(t, sc, run, "fixture: praise and watch names in the facts",
		strings.Contains(prompt, liveWorkspaceRoster[0].name) && strings.Contains(prompt, liveWorkspaceRoster[1].name) &&
			!strings.Contains(prompt, liveWorkspaceRoster[2].name), false, "see facts above")

	wsLiveVerdict(t, sc, run, "HTTP 200 (server §6 passed)", res.Code == http.StatusOK, false, res.Body.String())
	var out classSummaryJSON
	if res.Code == http.StatusOK {
		_ = json.Unmarshal(res.Body.Bytes(), &out)
	}
	wsLiveVerdict(t, sc, run, "not served from cache", !out.Cached, false, res.Body.String())
	wsLiveVerdict(t, sc, run, "first attempt passed, no retry (observation)", len(calls) == 1, true,
		fmt.Sprintf("%d model calls; texts %q", len(calls), wsLiveTexts(calls)))

	rosterNames := make([]string, 0, len(liveWorkspaceRoster))
	var grounded []string
	for _, s := range liveWorkspaceRoster {
		rosterNames = append(rosterNames, s.name)
		if strings.Contains(prompt, s.name) {
			grounded = append(grounded, s.name)
		}
	}
	var counts []int
	for _, m := range wsLiveSummaryCount.FindAllStringSubmatch(prompt, -1) {
		n, _ := strconv.Atoi(m[2])
		counts = append(counts, n)
	}
	badNames := liteworkspace.UngroundedNames(out.Summary, rosterNames, grounded)
	badCounts := liteworkspace.UngroundedCounts(out.Summary, counts)
	wsLiveVerdict(t, sc, run, "§6 re-check: names and counts grounded",
		out.Summary != "" && len(badNames) == 0 && len(badCounts) == 0, false,
		fmt.Sprintf("names %v counts %v (grounded %v %v); summary %q", badNames, badCounts, grounded, counts, out.Summary))
	wsLiveVerdict(t, sc, run, "no exclamation mark", !strings.ContainsAny(out.Summary, "!！"), false, out.Summary)
	t.Logf("USAGE | %s | run %d | %s", sc, run, wsLiveUsage(calls))
}

func wsLiveTexts(calls []wsLiveCall) []string {
	out := make([]string, 0, len(calls))
	for _, c := range calls {
		out = append(out, c.Text)
	}
	return out
}
