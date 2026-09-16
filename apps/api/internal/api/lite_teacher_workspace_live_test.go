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
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/config"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/library"
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
// They differ in one thing only — whether search_library can answer the topic.
// The catalogue's single climate article is titled 美国气候队, and SearchLibrary
// matches a substring of the title, so the query a teacher writes for the first
// scenario ("气候变化") returns nothing. Running both separates a model that
// cannot follow the tool rules from a search that cannot serve the topic; with
// only the first, every failure reads like the model's.
//
// followUps are the teacher's next answers, sent one at a time while the card
// still has no material. The model asks before it commits an article, so
// without them the search_library → set_material ordering is never exercised.
var liveWorkspaceScenarios = []struct {
	name, opening string
	followUps     []string
}{
	{
		"catalogue-misses",
		"这周读一篇气候变化的报道，写一篇议论文，周五交",
		[]string{"从阅读库里挑一篇跟气候有关的报道", "就用这篇，定下来"},
	},
	{
		"catalogue-hits",
		"这周读一篇亚运会的报道，写一篇议论文，周五交",
		[]string{"就用你找到的那篇，定下来", "确认，就这篇"},
	},
}

// wsLiveCall is one model call inside a turn.
type wsLiveCall struct {
	Text  string
	Tools []gateway.ToolCall
	Err   error
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
	out, err := gateway.Collect(ctx, r.inner, res, req)
	r.mu.Lock()
	r.calls = append(r.calls, wsLiveCall{Text: out.Text, Tools: out.ToolCalls, Err: err})
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
	return h, teacher, classID
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
