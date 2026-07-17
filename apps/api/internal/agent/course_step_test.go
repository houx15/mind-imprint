package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/skills"
)

func courseTestSkill() skills.Skill {
	return skills.Skill{
		ID: "t", Kind: "course", Cards: []string{"craap"},
		Contracts: map[string]skills.Contract{
			"demonstrate": {Title: "演示", Goal: "g0", SoftCondition: "s0",
				Steps: []int{0, 1},
				Floor: []skills.FloorItem{{Kind: "steps_viewed", Steps: []int{0, 1}}}},
			"guided": {Title: "引导", Goal: "g1", SoftCondition: "s1", Requires: []string{"demonstrate"},
				Cards:          []string{"craap"},
				AnchorMaterial: &skills.AnchorMaterial{Title: "待核实的说法", Text: "卫星图显示……"},
				Floor:          []skills.FloorItem{{Kind: "card_dispositioned", CardID: "craap"}}},
			"reflect": {Title: "回看", Goal: "g2", SoftCondition: "s2", Requires: []string{"guided"},
				Floor: []skills.FloorItem{{Kind: "student_turns_at_least", N: 1}}},
		},
	}
}

func TestNextPhaseWalksTheBindingOrder(t *testing.T) {
	sk := courseTestSkill()
	for _, c := range []struct{ cur, want string }{{"demonstrate", "guided"}, {"guided", "reflect"}} {
		got, ok := NextPhase(sk, c.cur)
		if !ok || got != c.want {
			t.Fatalf("NextPhase(%s) = %q,%v want %q,true", c.cur, got, ok, c.want)
		}
	}
	if _, ok := NextPhase(sk, "reflect"); ok {
		t.Fatal("the last phase has no successor")
	}
	if _, ok := NextPhase(sk, "nope"); ok {
		t.Fatal("an unknown phase has no successor")
	}
}

func TestCheckFloor(t *testing.T) {
	viewed := FloorState{ViewedSteps: []int{0, 1}}
	if unmet := CheckFloor([]skills.FloorItem{{Kind: "steps_viewed", Steps: []int{0, 1}}}, viewed); len(unmet) != 0 {
		t.Fatalf("all steps viewed → floor met, got unmet %+v", unmet)
	}
	if unmet := CheckFloor([]skills.FloorItem{{Kind: "steps_viewed", Steps: []int{0, 1, 2}}}, viewed); len(unmet) != 1 {
		t.Fatalf("a missing step → floor unmet, got %+v", unmet)
	}

	// DEC-12.2's no-dead-end guarantee: a SKIPPED card satisfies the floor. A
	// card is an offer; a floor only a completed card can satisfy would turn
	// the offer into a wall.
	item := []skills.FloorItem{{Kind: "card_dispositioned", CardID: "craap"}}
	if unmet := CheckFloor(item, FloorState{DispositionedCards: []string{"craap"}}); len(unmet) != 0 {
		t.Fatalf("a dispositioned card meets the floor, got unmet %+v", unmet)
	}
	if unmet := CheckFloor(item, FloorState{}); len(unmet) != 1 {
		t.Fatalf("an untouched card leaves the floor unmet, got %+v", unmet)
	}

	if unmet := CheckFloor([]skills.FloorItem{{Kind: "student_turns_at_least", N: 2}}, FloorState{StudentTurns: 2}); len(unmet) != 0 {
		t.Fatalf("turns met → floor met, got %+v", unmet)
	}
	if unmet := CheckFloor([]skills.FloorItem{{Kind: "student_turns_at_least", N: 2}}, FloorState{StudentTurns: 1}); len(unmet) != 1 {
		t.Fatalf("turns short → floor unmet, got %+v", unmet)
	}

	// Fail closed: Load rejects an unknown kind, so this is defense in depth.
	if unmet := CheckFloor([]skills.FloorItem{{Kind: "vibes_ok"}}, FloorState{}); len(unmet) != 1 {
		t.Fatalf("an unknown floor kind must count as UNMET, got %+v", unmet)
	}
}

func TestCourseCardCandidateSurfacesOnce(t *testing.T) {
	sk := courseTestSkill()
	id, ok := CourseCardCandidate(sk, "guided", nil)
	if !ok || id != "craap" {
		t.Fatalf("guided must offer craap, got %q,%v", id, ok)
	}
	if _, ok := CourseCardCandidate(sk, "demonstrate", nil); ok {
		t.Fatal("a phase with no declared card offers nothing")
	}
	for _, status := range []string{"proposed", "active", "completed", "skipped"} {
		if _, ok := CourseCardCandidate(sk, "guided", []ScopedCard{{CardID: "craap", Status: status}}); ok {
			t.Fatalf("an existing craap card in status %s must not be re-offered", status)
		}
	}
}

// fakeCourseStore is an in-memory CourseStore for RunCourseStep's tests.
type fakeCourseStore struct {
	session     CourseSession
	viewedSteps []int
	cards       []ScopedCard
	materials   []ScopedMaterial
	history     []ChatTurn
	turns       int

	llmCalls         int
	assistantMsgs    int
	studentMsgs      int
	materialsCreated int
	cardsCreated     int
	phaseSets        int
	events           []courseEventRecord
}

// courseEventRecord is what fakeCourseStore.InsertUserEvent captured — kept as
// a (type, payload) pair, not just the type string, so tests can assert on
// event payload shape (card_id / to / unprompted), not merely presence.
type courseEventRecord struct {
	typ     string
	payload []byte
}

func newFakeCourseStore(phase string) *fakeCourseStore {
	return &fakeCourseStore{
		session:     CourseSession{ID: uuid.New(), CourseID: uuid.New(), SkillID: "t", Phase: phase},
		viewedSteps: []int{0, 1},
	}
}

func (f *fakeCourseStore) hasEvent(typ string) bool {
	for _, e := range f.events {
		if e.typ == typ {
			return true
		}
	}
	return false
}

// eventPayload returns the payload of the first captured event of typ, or nil
// if none was recorded.
func (f *fakeCourseStore) eventPayload(typ string) []byte {
	for _, e := range f.events {
		if e.typ == typ {
			return e.payload
		}
	}
	return nil
}

func (f *fakeCourseStore) GetSession(ctx context.Context, sessionID uuid.UUID) (CourseSession, error) {
	return f.session, nil
}

func (f *fakeCourseStore) SetSessionPhase(ctx context.Context, sessionID uuid.UUID, phase string) error {
	f.session.Phase = phase
	f.phaseSets++
	return nil
}

func (f *fakeCourseStore) LoadPhaseHistory(ctx context.Context, sessionID uuid.UUID, phase string, limit int) ([]ChatTurn, error) {
	return f.history, nil
}

func (f *fakeCourseStore) CountStudentTurns(ctx context.Context, sessionID uuid.UUID, phase string) (int, error) {
	return f.turns, nil
}

func (f *fakeCourseStore) CreateSessionMessage(ctx context.Context, sessionID uuid.UUID, phase, role, content string) (uuid.UUID, error) {
	f.history = append(f.history, ChatTurn{Role: role, Content: content})
	if role == "student" {
		f.studentMsgs++
		f.turns++
	} else if role == "assistant" {
		f.assistantMsgs++
	}
	return uuid.New(), nil
}

func (f *fakeCourseStore) ListSessionCards(ctx context.Context, sessionID uuid.UUID) ([]ScopedCard, error) {
	return f.cards, nil
}

func (f *fakeCourseStore) CreateSessionCardInstance(ctx context.Context, sessionID uuid.UUID, cardID string) (uuid.UUID, error) {
	id := uuid.New()
	f.cards = append(f.cards, ScopedCard{ID: id, CardID: cardID, Status: "proposed"})
	f.cardsCreated++
	return id, nil
}

func (f *fakeCourseStore) CreateSessionMaterial(ctx context.Context, sessionID uuid.UUID, title, text string) (uuid.UUID, error) {
	id := uuid.New()
	f.materials = append(f.materials, ScopedMaterial{ID: id, Kind: "article"})
	f.materialsCreated++
	return id, nil
}

func (f *fakeCourseStore) ListSessionMaterials(ctx context.Context, sessionID uuid.UUID) ([]ScopedMaterial, error) {
	return f.materials, nil
}

func (f *fakeCourseStore) ViewedSteps(ctx context.Context, userID, courseID uuid.UUID) ([]int32, error) {
	out := make([]int32, 0, len(f.viewedSteps))
	for _, v := range f.viewedSteps {
		out = append(out, int32(v))
	}
	return out, nil
}

func (f *fakeCourseStore) InsertUserEvent(ctx context.Context, userID uuid.UUID, surface, typ string, payload []byte) error {
	f.events = append(f.events, courseEventRecord{typ: typ, payload: append([]byte(nil), payload...)})
	return nil
}

func (f *fakeCourseStore) RecordCourseLLMCall(ctx context.Context, userID uuid.UUID, resolved gateway.Resolved, prompt, completion int32) error {
	f.llmCalls++
	return nil
}

func TestRunCourseStepAskGetsAReply(t *testing.T) {
	st := newFakeCourseStore("demonstrate")
	deps := CourseDeps{
		Store: st, Provider: scriptedProvider(`{"type":"reply","body":"你觉得这句话里，哪一部分是证据？"}`),
		Resolved: gateway.Resolved{}, Skill: courseTestSkill(), UserID: uuid.New(), SessionID: st.session.ID,
	}
	res, err := RunCourseStep(context.Background(), deps, "ask", "这条是真的吗？")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Reply == "" {
		t.Fatal("an ask must produce a reply")
	}
	if res.Advanced != "" {
		t.Fatalf("an ask must never advance, got %q", res.Advanced)
	}
	if st.studentMsgs != 1 || st.assistantMsgs != 1 {
		t.Fatalf("both turns must be persisted, got student=%d assistant=%d", st.studentMsgs, st.assistantMsgs)
	}
	if st.llmCalls != 1 {
		t.Fatalf("llmCalls = %d, want exactly 1", st.llmCalls)
	}
	// Spec §7: a course_message event rides alongside the persisted student
	// message, unprompted=true because `ask` is student-initiated.
	if !st.hasEvent("course_message") {
		t.Fatal("an ask must emit a course_message event for the student turn")
	}
	var cm struct {
		Unprompted bool `json:"unprompted"`
	}
	if err := json.Unmarshal(st.eventPayload("course_message"), &cm); err != nil {
		t.Fatalf("course_message payload must unmarshal: %v", err)
	}
	if !cm.Unprompted {
		t.Fatal("an ask's course_message must be unprompted=true")
	}
}

func TestRunCourseStepUnmetFloorNeverCallsTheModel(t *testing.T) {
	// demonstrate's floor wants steps 0 and 1 viewed; the store reports none.
	st := newFakeCourseStore("demonstrate")
	st.viewedSteps = nil
	deps := CourseDeps{
		Store: st, Provider: scriptedProvider(`{"type":"advance","to":"guided"}`),
		Resolved: gateway.Resolved{}, Skill: courseTestSkill(), UserID: uuid.New(), SessionID: st.session.ID,
	}
	res, err := RunCourseStep(context.Background(), deps, "request_advance", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Advanced != "" {
		t.Fatal("an unmet floor must NOT advance, whatever the model would have said")
	}
	if st.llmCalls != 0 {
		t.Fatalf("an unmet floor must short-circuit BEFORE the model: llmCalls = %d, want 0", st.llmCalls)
	}
	if res.Reply == "" {
		t.Fatal("a refused advance must tell the student what is still owed")
	}
	if st.phaseSets != 0 {
		t.Fatal("the phase must not move")
	}
}

func TestRunCourseStepMetFloorAdvances(t *testing.T) {
	st := newFakeCourseStore("demonstrate")
	st.viewedSteps = []int{0, 1}
	deps := CourseDeps{
		Store: st, Provider: scriptedProvider(`{"type":"advance","to":"guided"}`),
		Resolved: gateway.Resolved{}, Skill: courseTestSkill(), UserID: uuid.New(), SessionID: st.session.ID,
	}
	res, err := RunCourseStep(context.Background(), deps, "request_advance", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Advanced != "guided" {
		t.Fatalf("Advanced = %q, want guided", res.Advanced)
	}
	if st.phaseSets != 1 || st.session.Phase != "guided" {
		t.Fatalf("the phase must move once, got sets=%d phase=%s", st.phaseSets, st.session.Phase)
	}
	if !st.hasEvent("phase_advanced") {
		t.Fatal("advancing must emit phase_advanced")
	}
	var pa struct {
		To string `json:"to"`
	}
	if err := json.Unmarshal(st.eventPayload("phase_advanced"), &pa); err != nil {
		t.Fatalf("phase_advanced payload must unmarshal: %v", err)
	}
	if pa.To != "guided" {
		t.Fatalf("phase_advanced payload.to = %q, want guided", pa.To)
	}
}

// TestRunCourseStepSkippedCardSatisfiesFloorAndAdvances proves DEC-12.2's
// no-dead-end guarantee through the real RunCourseStep path, not just the
// pure CheckFloor helper: a session card in status `skipped` meets the
// `guided` phase's `card_dispositioned craap` floor and the coach's `advance`
// actually moves the phase. This is the only path where card_dispositioned
// matters at all — guided + request_advance.
func TestRunCourseStepSkippedCardSatisfiesFloorAndAdvances(t *testing.T) {
	st := newFakeCourseStore("guided")
	st.cards = []ScopedCard{{ID: uuid.New(), CardID: "craap", Status: "skipped"}}
	deps := CourseDeps{
		Store: st, Provider: scriptedProvider(`{"type":"advance","to":"reflect"}`),
		Resolved: gateway.Resolved{}, Skill: courseTestSkill(), UserID: uuid.New(), SessionID: st.session.ID,
	}
	res, err := RunCourseStep(context.Background(), deps, "request_advance", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Advanced != "reflect" {
		t.Fatalf("a SKIPPED card must meet the floor and let the coach advance, got Advanced=%q", res.Advanced)
	}
	if st.phaseSets != 1 {
		t.Fatalf("the phase must move once, got sets=%d", st.phaseSets)
	}
	if st.llmCalls != 1 {
		t.Fatalf("the floor was met, so the coach IS called: llmCalls = %d, want 1", st.llmCalls)
	}
}

// TestRunCourseStepCompletedCardSatisfiesFloorAndAdvances is the `skipped`
// test's twin: a COMPLETED card must satisfy the same floor identically. Both
// dispositions are equally "met" — completion is not a stricter requirement.
func TestRunCourseStepCompletedCardSatisfiesFloorAndAdvances(t *testing.T) {
	st := newFakeCourseStore("guided")
	st.cards = []ScopedCard{{ID: uuid.New(), CardID: "craap", Status: "completed"}}
	deps := CourseDeps{
		Store: st, Provider: scriptedProvider(`{"type":"advance","to":"reflect"}`),
		Resolved: gateway.Resolved{}, Skill: courseTestSkill(), UserID: uuid.New(), SessionID: st.session.ID,
	}
	res, err := RunCourseStep(context.Background(), deps, "request_advance", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Advanced != "reflect" {
		t.Fatalf("a COMPLETED card must meet the floor and let the coach advance, got Advanced=%q", res.Advanced)
	}
	if st.phaseSets != 1 {
		t.Fatalf("the phase must move once, got sets=%d", st.phaseSets)
	}
	if st.llmCalls != 1 {
		t.Fatalf("the floor was met, so the coach IS called: llmCalls = %d, want 1", st.llmCalls)
	}
}

// TestRunCourseStepUntouchedCardLeavesFloorUnmetNoLLMCall is the no-dead-end
// guarantee's negative case: a card the student has neither completed nor
// skipped (still `proposed`) must NOT satisfy the floor, and the model must
// never be called — zero LLM calls, zero tokens, whatever the (unreachable)
// scripted model output would have said.
func TestRunCourseStepUntouchedCardLeavesFloorUnmetNoLLMCall(t *testing.T) {
	st := newFakeCourseStore("guided")
	st.cards = []ScopedCard{{ID: uuid.New(), CardID: "craap", Status: "proposed"}}
	deps := CourseDeps{
		Store: st, Provider: scriptedProvider(`{"type":"advance","to":"reflect"}`),
		Resolved: gateway.Resolved{}, Skill: courseTestSkill(), UserID: uuid.New(), SessionID: st.session.ID,
	}
	res, err := RunCourseStep(context.Background(), deps, "request_advance", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Advanced != "" {
		t.Fatalf("an untouched (proposed) card must NOT meet the floor, got Advanced=%q", res.Advanced)
	}
	if st.llmCalls != 0 {
		t.Fatalf("an unmet floor must short-circuit BEFORE the model: llmCalls = %d, want 0", st.llmCalls)
	}
	if st.phaseSets != 0 {
		t.Fatal("the phase must not move")
	}
	if res.Reply == "" {
		t.Fatal("a refused advance must tell the student what is still owed")
	}
}

// TestRunCourseStepRefusalCarriesCardOfferMintedOnce is Important-2's test: a
// request_advance refusal on the card_dispositioned floor must arrive WITH
// the card it is asking for (the offer), not just a sentence about a card
// that does not exist yet — and repeated refusals must not re-mint it.
func TestRunCourseStepRefusalCarriesCardOfferMintedOnce(t *testing.T) {
	st := newFakeCourseStore("guided")
	// No session card yet — the student pressed next-arrow before ever
	// chatting, so runCourseAsk's mint path was never reached.
	deps := CourseDeps{
		Store: st, Provider: scriptedProvider(`{"type":"advance","to":"reflect"}`),
		Resolved: gateway.Resolved{}, Skill: courseTestSkill(), UserID: uuid.New(), SessionID: st.session.ID,
	}

	res, err := RunCourseStep(context.Background(), deps, "request_advance", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Advanced != "" {
		t.Fatal("the floor is unmet (no card at all yet) — must not advance")
	}
	if res.Offer == nil || res.Offer.CardID != "craap" {
		t.Fatalf("the refusal must carry the card offer it is asking about, got %+v", res.Offer)
	}
	if st.materialsCreated != 1 || st.cardsCreated != 1 {
		t.Fatalf("exactly one anchor material + one card instance minted, got m=%d c=%d", st.materialsCreated, st.cardsCreated)
	}
	if st.llmCalls != 0 {
		t.Fatalf("minting a card is store-only — it must not spend a model call: llmCalls = %d, want 0", st.llmCalls)
	}

	// A second refusal (the student still hasn't touched the card) must not
	// mint a second material/card instance — surface-once still holds.
	res2, err := RunCourseStep(context.Background(), deps, "request_advance", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res2.Advanced != "" {
		t.Fatal("still unmet — must still not advance")
	}
	if res2.Offer != nil {
		t.Fatalf("the card already exists (status proposed) — must not be re-offered, got %+v", res2.Offer)
	}
	if st.materialsCreated != 1 || st.cardsCreated != 1 {
		t.Fatalf("no second mint across repeated refusals, got m=%d c=%d", st.materialsCreated, st.cardsCreated)
	}
}

func TestRunCourseStepRejectsAdvanceToANonSuccessor(t *testing.T) {
	// The coach may not skip a phase — forward-only, one at a time, enforced
	// by the runtime rather than requested by the prompt.
	st := newFakeCourseStore("demonstrate")
	st.viewedSteps = []int{0, 1}
	deps := CourseDeps{
		Store: st, Provider: scriptedProvider(`{"type":"advance","to":"reflect"}`),
		Resolved: gateway.Resolved{}, Skill: courseTestSkill(), UserID: uuid.New(), SessionID: st.session.ID,
	}
	res, err := RunCourseStep(context.Background(), deps, "request_advance", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Advanced != "" || st.phaseSets != 0 {
		t.Fatalf("a jump past guided must be refused, got Advanced=%q sets=%d", res.Advanced, st.phaseSets)
	}
}

func TestRunCourseStepSurfacesThePhaseCardOnce(t *testing.T) {
	st := newFakeCourseStore("guided")
	deps := CourseDeps{
		Store: st, Provider: scriptedProvider(`{"type":"reply","body":"我们一起来查查这条说法。"}`),
		Resolved: gateway.Resolved{}, Skill: courseTestSkill(), UserID: uuid.New(), SessionID: st.session.ID,
	}
	res, err := RunCourseStep(context.Background(), deps, "ask", "接下来做什么？")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Offer == nil || res.Offer.CardID != "craap" {
		t.Fatalf("guided must offer craap, got %+v", res.Offer)
	}
	if st.materialsCreated != 1 || st.cardsCreated != 1 {
		t.Fatalf("one anchor material + one card, got m=%d c=%d", st.materialsCreated, st.cardsCreated)
	}
	if !st.hasEvent("card_surfaced") {
		t.Fatal("minting a card offer must emit card_surfaced")
	}
	var cs struct {
		CardID string `json:"card_id"`
	}
	if err := json.Unmarshal(st.eventPayload("card_surfaced"), &cs); err != nil {
		t.Fatalf("card_surfaced payload must unmarshal: %v", err)
	}
	if cs.CardID != "craap" {
		t.Fatalf("card_surfaced payload.card_id = %q, want craap", cs.CardID)
	}

	// Second turn: the card exists now, so it must not be offered again.
	res2, err := RunCourseStep(context.Background(), deps, "ask", "还有别的吗？")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res2.Offer != nil {
		t.Fatal("the card must be surfaced once per session")
	}
	if st.materialsCreated != 1 || st.cardsCreated != 1 {
		t.Fatalf("no second mint, got m=%d c=%d", st.materialsCreated, st.cardsCreated)
	}
}

func TestRunCourseStepRejectedOutputIsSilentButMetered(t *testing.T) {
	// The brief's literal string ("你可以这样写：...") does not match the
	// banned-phrasing corpus (internal/agent/enforcement/banned_phrasing.go
	// only bans "你应该这样写" / "应该这样写：") — Task 5's coach test hit the
	// same issue and switched to the corpus's actual phrase; do the same here.
	st := newFakeCourseStore("demonstrate")
	deps := CourseDeps{
		Store: st, Provider: scriptedProvider(`{"type":"reply","body":"你应该这样写：中国的绿化成就无可否认。"}`),
		Resolved: gateway.Resolved{}, Skill: courseTestSkill(), UserID: uuid.New(), SessionID: st.session.ID,
	}
	res, err := RunCourseStep(context.Background(), deps, "ask", "帮我写一句")
	if err != nil {
		t.Fatalf("a rejected output is silence, NOT an error: %v", err)
	}
	if res.Reply != "" {
		t.Fatal("a rejected output must produce no reply")
	}
	if st.assistantMsgs != 0 {
		t.Fatal("a rejected output must persist no assistant message")
	}
	if st.llmCalls != 1 {
		t.Fatalf("the tokens were spent — llmCalls = %d, want 1", st.llmCalls)
	}
}
