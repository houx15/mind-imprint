package agent

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
)

func TestDetectURL(t *testing.T) {
	if u, ok := DetectURL("看这个 https://nasa.gov/x 它证明了"); !ok || u != "https://nasa.gov/x" {
		t.Fatalf("got %q %v", u, ok)
	}
	if _, ok := DetectURL("我觉得中国让地球更可持续"); ok {
		t.Fatalf("no URL should be detected")
	}
}

func TestChatCardCandidate(t *testing.T) {
	m := uuid.New()
	mats := []ScopedMaterial{{ID: m, Kind: "article", SourceURL: "https://e.com"}}

	// fresh article, no card yet → offer craap on it
	mid, cid, ok := ChatCardCandidate(mats, nil)
	if !ok || cid != "craap" || mid != m {
		t.Fatalf("want craap offer, got %v %s %v", mid, cid, ok)
	}

	// a craap card already exists in the thread (any status) → suppressed
	for _, st := range []string{"proposed", "active", "completed", "skipped"} {
		if _, _, ok := ChatCardCandidate(mats, []ScopedCard{{ID: uuid.New(), CardID: "craap", Status: st}}); ok {
			t.Fatalf("status %s should suppress re-offer", st)
		}
	}
}

// fakeChatStore is an in-memory ChatStore for RunChatStep's tests: no
// materials/cards/history by default, counters for the calls RunChatStep is
// expected to make.
type fakeChatStore struct {
	materials []ScopedMaterial
	cards     []ScopedCard
	history   []ChatTurn

	llmCalls         int
	llmCallPurposes  []string
	assistantMsgs    int
	materialsCreated int
	cardsCreated     int
	events           int

	lastEventThreadID uuid.UUID
	lastEventType     string
	lastEventPayload  []byte
}

func newFakeChatStore() *fakeChatStore { return &fakeChatStore{} }

func (f *fakeChatStore) LoadThreadHistory(ctx context.Context, threadID uuid.UUID, limit int) ([]ChatTurn, error) {
	return f.history, nil
}

func (f *fakeChatStore) CreateThreadMessage(ctx context.Context, threadID uuid.UUID, role, content, modality string) (uuid.UUID, error) {
	if role == "assistant" {
		f.assistantMsgs++
	}
	return uuid.New(), nil
}

func (f *fakeChatStore) ListThreadMaterials(ctx context.Context, threadID uuid.UUID) ([]ScopedMaterial, error) {
	return f.materials, nil
}

func (f *fakeChatStore) CreateThreadMaterial(ctx context.Context, threadID uuid.UUID, kind, source, title, sourceURL string) (uuid.UUID, error) {
	id := uuid.New()
	f.materials = append(f.materials, ScopedMaterial{ID: id, Kind: kind, SourceURL: sourceURL})
	f.materialsCreated++
	return id, nil
}

func (f *fakeChatStore) ListThreadCards(ctx context.Context, threadID uuid.UUID) ([]ScopedCard, error) {
	return f.cards, nil
}

func (f *fakeChatStore) CreateThreadCardInstance(ctx context.Context, threadID uuid.UUID, cardID string) (uuid.UUID, error) {
	id := uuid.New()
	f.cards = append(f.cards, ScopedCard{ID: id, CardID: cardID, Status: "proposed"})
	f.cardsCreated++
	return id, nil
}

func (f *fakeChatStore) InsertThreadEvent(ctx context.Context, threadID uuid.UUID, typ string, payload []byte) error {
	f.events++
	f.lastEventThreadID = threadID
	f.lastEventType = typ
	f.lastEventPayload = payload
	return nil
}

func (f *fakeChatStore) RecordChatLLMCall(ctx context.Context, userID uuid.UUID, purpose string, resolved gateway.Resolved, prompt, completion int32) error {
	f.llmCalls++
	f.llmCallPurposes = append(f.llmCallPurposes, purpose)
	return nil
}

func TestRunChatStepReplyOnly(t *testing.T) {
	fs := newFakeChatStore() // no materials, no url in message
	prov := scriptedProvider("你为什么这么想？")
	res, err := RunChatStep(context.Background(), ChatDeps{Store: fs, Provider: prov, Resolved: gateway.Resolved{Provider: "deepseek", Model: "x"}, UserID: uuid.New(), ThreadID: uuid.New()}, "我觉得我对")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Reply == "" || res.Offer != nil {
		t.Fatalf("want reply-only, got %+v", res)
	}
	if fs.llmCalls != 1 {
		t.Fatalf("coach call must be metered once, got %d", fs.llmCalls)
	}
	if fs.assistantMsgs != 1 {
		t.Fatalf("reply must be persisted")
	}
}

func TestRunChatStepMintsMaterialAndOffers(t *testing.T) {
	fs := newFakeChatStore()
	prov := scriptedProvider("这确实值得核实一下。")
	threadID := uuid.New()
	res, err := RunChatStep(context.Background(), ChatDeps{Store: fs, Provider: prov, Resolved: gateway.Resolved{Provider: "deepseek", Model: "x"}, UserID: uuid.New(), ThreadID: threadID}, "看 https://nasa.gov/x 它证明了我的观点")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fs.materialsCreated != 1 {
		t.Fatalf("a thread material must be minted for the URL")
	}
	if res.Offer == nil || res.Offer.CardID != "craap" {
		t.Fatalf("want a craap offer, got %+v", res.Offer)
	}
	if fs.events != 1 || fs.lastEventType != "card_surfaced" || fs.lastEventThreadID != threadID {
		t.Fatalf("want exactly one thread-scoped card_surfaced event for thread %s, got events=%d type=%q threadID=%s",
			threadID, fs.events, fs.lastEventType, fs.lastEventThreadID)
	}
}

func TestRunChatStepBannedReplyStaysSilentButMeters(t *testing.T) {
	fs := newFakeChatStore()
	prov := scriptedProvider("你应该这样写：中国让地球更可持续。")
	res, err := RunChatStep(context.Background(), ChatDeps{Store: fs, Provider: prov, Resolved: gateway.Resolved{Provider: "deepseek", Model: "x"}, UserID: uuid.New(), ThreadID: uuid.New()}, "帮我写")
	if err != nil {
		t.Fatalf("silence is not an error: %v", err)
	}
	if res.Reply != "" {
		t.Fatalf("rejected reply must not be emitted")
	}
	if fs.llmCalls != 1 {
		t.Fatalf("rejected reply must still be metered")
	}
	if fs.assistantMsgs != 0 {
		t.Fatalf("rejected reply must not be persisted")
	}
}

// failOnceThenProvider fails the first Stream call (the classifier's), then
// delegates to `after` for every subsequent call (the coach's) — countingProvider
// and erroringProvider (loop_test.go) can't express "fails once, then succeeds",
// so this small, non-duplicative double covers the one Chat test that needs it:
// proving a classifier failure does not block the coach reply that follows it
// on the same provider.
type failOnceThenProvider struct {
	failed bool
	err    error
	after  gateway.Provider
}

func (f *failOnceThenProvider) Stream(ctx context.Context, r gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	if !f.failed {
		f.failed = true
		return nil, f.err
	}
	return f.after.Stream(ctx, r, req)
}

// The structural link→CRAAP moment still wins; no classifier call is made.
func TestRunChatStepStructuralMomentWinsOverClassifier(t *testing.T) {
	fs := newFakeChatStore()
	articleID := uuid.New()
	fs.materials = []ScopedMaterial{{ID: articleID, Kind: "article", SourceURL: "https://e.com"}}
	prov := &countingProvider{inner: scriptedProvider("这确实值得核实一下。")}

	res, err := RunChatStep(context.Background(), ChatDeps{Store: fs, Provider: prov, Resolved: testResolved, UserID: uuid.New(), ThreadID: uuid.New()}, longEnoughText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Offer == nil || res.Offer.CardID != "craap" || res.Offer.MaterialID != articleID {
		t.Fatalf("want the structural craap offer on the article, got %+v", res.Offer)
	}
	if prov.calls != 1 {
		t.Fatalf("want exactly one provider call (coach only; classifier must not run), got %d", prov.calls)
	}
	if fs.llmCalls != 1 || len(fs.llmCallPurposes) != 1 || fs.llmCallPurposes[0] != "coach" {
		t.Fatalf("want exactly one metered coach call and no classify call, got calls=%d purposes=%v", fs.llmCalls, fs.llmCallPurposes)
	}
}

// With no link and no CRAAP moment, a named semantic moment mints that card.
func TestRunChatStepSemanticMomentOffersItsCard(t *testing.T) {
	fs := newFakeChatStore()
	prov := &countingProvider{inner: scriptedProvider("one_sided")}

	res, err := RunChatStep(context.Background(), ChatDeps{Store: fs, Provider: prov, Resolved: testResolved, UserID: uuid.New(), ThreadID: uuid.New()}, longEnoughText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Offer == nil || res.Offer.CardID != "concession" {
		t.Fatalf("want a concession offer from the one_sided moment, got %+v", res.Offer)
	}
	if prov.calls != 2 {
		t.Fatalf("want two provider calls (classify + coach), got %d", prov.calls)
	}
	if fs.cardsCreated != 1 {
		t.Fatalf("want exactly one card instance minted, got %d", fs.cardsCreated)
	}
}

// The semantic offer carries a nil material and still emits card_surfaced.
func TestRunChatStepSemanticOfferHasNoMaterial(t *testing.T) {
	fs := newFakeChatStore()
	prov := scriptedProvider("one_sided")
	threadID := uuid.New()

	res, err := RunChatStep(context.Background(), ChatDeps{Store: fs, Provider: prov, Resolved: testResolved, UserID: uuid.New(), ThreadID: threadID}, longEnoughText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Offer == nil || res.Offer.MaterialID != uuid.Nil {
		t.Fatalf("want a semantic offer with a nil material, got %+v", res.Offer)
	}
	if fs.events != 1 || fs.lastEventType != "card_surfaced" || fs.lastEventThreadID != threadID {
		t.Fatalf("want one thread-scoped card_surfaced event, got events=%d type=%q threadID=%s",
			fs.events, fs.lastEventType, fs.lastEventThreadID)
	}
}

// Any-status suppression: a skipped concession is never re-offered in the thread.
func TestRunChatStepSemanticSuppressedByAnyStatus(t *testing.T) {
	fs := newFakeChatStore()
	fs.cards = []ScopedCard{{ID: uuid.New(), CardID: "concession", Status: "skipped"}}
	// The classifier still names one_sided (a real, but now-ineligible, moment) —
	// ClassifyMoment's exact-match-against-eligible contract must collapse this
	// to MomentNone rather than re-offering the already-skipped concession.
	prov := &countingProvider{inner: scriptedProvider("one_sided")}

	res, err := RunChatStep(context.Background(), ChatDeps{Store: fs, Provider: prov, Resolved: testResolved, UserID: uuid.New(), ThreadID: uuid.New()}, longEnoughText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Offer != nil {
		t.Fatalf("want no offer — concession was already dispositioned (skipped), got %+v", res.Offer)
	}
	if fs.cardsCreated != 0 {
		t.Fatalf("want no card instance minted, got %d", fs.cardsCreated)
	}
	// fact_opinion is still eligible, so the classifier still runs.
	if prov.calls != 2 {
		t.Fatalf("want two provider calls (classify still runs for the remaining eligible moments, + coach), got %d", prov.calls)
	}
}

// TestRunChatStepSuppressedByInFlightCard is the regression for whole-branch
// review IMPORTANT 2: ChatCardCandidate only suppresses a re-offer of the
// SAME card id (CRAAP), so nothing stopped the classifier branch from
// minting a second, DIFFERENT card offer on top of one still unanswered —
// a student could rack up CRAAP, then concession, then certainty-spectrum,
// ... across consecutive messages with none of them opened, which is the
// nagging posture 铁律 2 forbids. Here a "craap" card is already `proposed`
// (unrelated to the moment cards), no link is in the message, and the
// classifier would otherwise name "one_sided" — the fix must suppress the
// classifier call entirely.
func TestRunChatStepSuppressedByInFlightCard(t *testing.T) {
	for _, status := range []string{"proposed", "active"} {
		t.Run(status, func(t *testing.T) {
			fs := newFakeChatStore()
			fs.cards = []ScopedCard{{ID: uuid.New(), CardID: "craap", Status: status}}
			prov := &countingProvider{inner: scriptedProvider("one_sided")}

			res, err := RunChatStep(context.Background(), ChatDeps{Store: fs, Provider: prov, Resolved: testResolved, UserID: uuid.New(), ThreadID: uuid.New()}, longEnoughText)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Offer != nil {
				t.Fatalf("want no offer while a card (%s) is in flight, got %+v", status, res.Offer)
			}
			if fs.cardsCreated != 0 {
				t.Fatalf("want no second card instance minted while one (%s) is in flight, got %d", status, fs.cardsCreated)
			}
			if prov.calls != 1 {
				t.Fatalf("want exactly one provider call (coach only; classifier must not run), got %d", prov.calls)
			}
			for _, p := range fs.llmCallPurposes {
				if p == "classify" {
					t.Fatalf("want no metered classify call while a card is in flight, got purposes=%v", fs.llmCallPurposes)
				}
			}
		})
	}
}

// TestRunChatStepClassifyNoUsageWarnsUnmetered is the Chat-surface regression
// for whole-branch review MINOR 4, mirroring
// TestRunAgentStepClassifyNoUsageWarnsUnmetered in loop_test.go: the classify
// metering block here only ever ran `if usage.InputTokens > 0 || ...`, with
// no `else` — a provider that stops reporting usage would silently drop the
// classify call from the cost ledger. The turn must still succeed.
func TestRunChatStepClassifyNoUsageWarnsUnmetered(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	fs := newFakeChatStore()
	prov := scriptedProviderNoUsage("one_sided")

	res, err := RunChatStep(context.Background(), ChatDeps{Store: fs, Provider: prov, Resolved: testResolved, UserID: uuid.New(), ThreadID: uuid.New()}, longEnoughText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Offer == nil || res.Offer.CardID != "concession" {
		t.Fatalf("want a concession offer from the one_sided moment despite zero usage, got %+v", res.Offer)
	}
	for _, p := range fs.llmCallPurposes {
		if p == "classify" {
			t.Fatalf("want no metered classify call when usage is zero (nothing to meter), got purposes=%v", fs.llmCallPurposes)
		}
	}
	if !strings.Contains(buf.String(), "classify call returned no usage") {
		t.Fatalf("want a warning that the classify call went unmetered, got log:\n%s", buf.String())
	}
}

// The classify call is metered through RecordChatLLMCall with purpose "classify".
func TestRunChatStepMetersClassifyCall(t *testing.T) {
	fs := newFakeChatStore()
	prov := scriptedProvider("one_sided")

	if _, err := RunChatStep(context.Background(), ChatDeps{Store: fs, Provider: prov, Resolved: testResolved, UserID: uuid.New(), ThreadID: uuid.New()}, longEnoughText); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	classifyCount, coachCount := 0, 0
	for _, p := range fs.llmCallPurposes {
		switch p {
		case "classify":
			classifyCount++
		case "coach":
			coachCount++
		}
	}
	if classifyCount != 1 {
		t.Fatalf("want exactly one classify-purpose metered call, got %d in %v", classifyCount, fs.llmCallPurposes)
	}
	if coachCount != 1 {
		t.Fatalf("want exactly one coach-purpose metered call, got %d in %v", coachCount, fs.llmCallPurposes)
	}
}

// A classifier failure leaves the coach reply intact — silence, not failure.
func TestRunChatStepClassifierErrorStillReplies(t *testing.T) {
	fs := newFakeChatStore()
	prov := &failOnceThenProvider{err: errors.New("model unavailable"), after: scriptedProvider("我们接着聊聊这个。")}

	res, err := RunChatStep(context.Background(), ChatDeps{Store: fs, Provider: prov, Resolved: testResolved, UserID: uuid.New(), ThreadID: uuid.New()}, longEnoughText)
	if err != nil {
		t.Fatalf("a classifier failure must not fail the turn: %v", err)
	}
	if res.Reply == "" {
		t.Fatalf("want the coach reply to still go through despite the classifier failing, got empty reply")
	}
	if res.Offer != nil {
		t.Fatalf("want no offer when the classifier failed, got %+v", res.Offer)
	}
	if fs.llmCalls != 1 || len(fs.llmCallPurposes) != 1 || fs.llmCallPurposes[0] != "coach" {
		t.Fatalf("want only the successful coach call metered (the classifier errored before Collect returned usage), got calls=%d purposes=%v", fs.llmCalls, fs.llmCallPurposes)
	}
}

// TestRunChatStepFoldsCompletedCardsIntoContext covers N3b Seam B's chat
// variant mainline: chat manufactures no dedicated post-submit turn (unlike
// the Studio's card_refeed trigger, Task 5), so a completed card's summary
// must ride the context of the student's very next message instead. The
// prompt actually sent to the model must carry the card's own name and a
// submitted answer, proving completedCardSummary → SerializeCardForRefeed is
// really wired into RunChatStep, not just present in the package.
func TestRunChatStepFoldsCompletedCardsIntoContext(t *testing.T) {
	fs := newFakeChatStore()
	fs.cards = []ScopedCard{{
		ID: uuid.New(), CardID: "money-trail", Status: "completed",
		FieldValues: moneyFieldValues(t),
	}}
	prov := &capturingProvider{inner: scriptedProvider("你为什么这么想？")}

	res, err := RunChatStep(context.Background(), ChatDeps{Store: fs, Provider: prov, Resolved: testResolved, UserID: uuid.New(), ThreadID: uuid.New()}, "我觉得我对")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Reply == "" {
		t.Fatalf("want a reply")
	}
	if prov.lastPrompt == "" {
		t.Fatal("expected the provider to have been called with a non-empty prompt")
	}
	if !strings.Contains(prov.lastPrompt, "资金链溯源卡") {
		t.Fatalf("prompt did not carry the completed card's name:\n%s", prov.lastPrompt)
	}
	if !strings.Contains(prov.lastPrompt, "证明中国让地球更可持续") {
		t.Fatalf("prompt did not carry the completed card's submitted answer:\n%s", prov.lastPrompt)
	}
}

// TestRunChatStepIgnoresIncompleteCardsInContext covers 铁律 2/4 for the chat
// variant: a proposed or active card has nothing finished to say, and a
// skipped one is a decline we do not re-raise — only "completed" contributes.
func TestRunChatStepIgnoresIncompleteCardsInContext(t *testing.T) {
	fs := newFakeChatStore()
	fs.cards = []ScopedCard{
		{ID: uuid.New(), CardID: "money-trail", Status: "proposed", FieldValues: moneyFieldValues(t)},
		{ID: uuid.New(), CardID: "money-trail", Status: "active", FieldValues: moneyFieldValues(t)},
		{ID: uuid.New(), CardID: "money-trail", Status: "skipped", FieldValues: moneyFieldValues(t)},
	}
	prov := &capturingProvider{inner: scriptedProvider("你为什么这么想？")}

	res, err := RunChatStep(context.Background(), ChatDeps{Store: fs, Provider: prov, Resolved: testResolved, UserID: uuid.New(), ThreadID: uuid.New()}, "我觉得我对")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Reply == "" {
		t.Fatalf("want a reply")
	}
	if strings.Contains(prov.lastPrompt, "资金链溯源卡") {
		t.Fatalf("an incomplete card must not contribute to the context:\n%s", prov.lastPrompt)
	}
	if strings.Contains(prov.lastPrompt, "证明中国让地球更可持续") {
		t.Fatalf("an incomplete card's answers must not leak into the context:\n%s", prov.lastPrompt)
	}
	if strings.Contains(prov.lastPrompt, "此对话中已完成的工具卡") {
		t.Fatalf("no completed-card heading should appear when nothing is completed:\n%s", prov.lastPrompt)
	}
}
