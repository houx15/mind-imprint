package agent

import (
	"context"
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
