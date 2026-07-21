package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
)

// DetectURL returns the first http(s) URL in s. Keyless, no fetch — the minted
// material is source="pasted" with empty blocks; CRAAP is self-contained (its
// step 1 link_check captures the URL).
func DetectURL(s string) (string, bool) {
	for _, tok := range strings.Fields(s) {
		tok = strings.Trim(tok, "，。！？;,.!?()（）「」\"'")
		if strings.HasPrefix(tok, "http://") || strings.HasPrefix(tok, "https://") {
			if u, err := url.Parse(tok); err == nil && u.Host != "" {
				return tok, true
			}
		}
	}
	return "", false
}

// ScopedMaterial / ScopedCard are the minimal projection of a scoped graph
// (a chat thread's, a course session's) that the classifier reads. Surface-
// neutral by design: the same shapes serve Chat and Course.
type ScopedMaterial struct {
	ID        uuid.UUID
	Kind      string
	SourceURL string
}
type ScopedCard struct {
	ID     uuid.UUID
	CardID string
	Status string

	// FieldValues/Anchors carry the submitted card body so a COMPLETED card
	// can be refed into the coach's context on the student's next message
	// (N3b Seam B, chat variant). Additive and optional: zero values are
	// valid and every existing fixture keeps compiling.
	FieldValues []byte
	Anchors     []byte
}

// ChatCardCandidate implements the keystone card-moment predicate: offer CRAAP
// (I3) on the most recent article material in the thread, UNLESS the thread
// already has a CRAAP card_instance in ANY status (proposed/active/completed/
// skipped) — an offer, once made or declined, is not re-raised in the thread.
// One source-evaluation offer per thread is a deliberate keystone simplification
// (card_instances carry no material link without the deferred thread graph edges).
func ChatCardCandidate(materials []ScopedMaterial, cards []ScopedCard) (uuid.UUID, string, bool) {
	for _, c := range cards {
		if c.CardID == craapCardID {
			return uuid.Nil, "", false
		}
	}
	var target uuid.UUID
	found := false
	for _, m := range materials {
		if m.Kind == "article" {
			target = m.ID // last article wins (materials come in created order)
			found = true
		}
	}
	if !found {
		return uuid.Nil, "", false
	}
	return target, craapCardID, true
}

// hasInFlightScopedCard reports whether any thread-scoped card is still
// `proposed` (offered, not yet opened) or `active` (open, being filled) —
// the Chat mirror of loop.go's in-flight guard (whole-branch review
// IMPORTANT 2). ChatCardCandidate already suppresses a re-offer of CRAAP
// once any CRAAP instance exists, but nothing stopped the classifier branch
// below from minting a SECOND, DIFFERENT card on top of an unanswered one:
// a student could rack up several unanswered offers across consecutive
// messages (CRAAP, then steelman, then certainty-spectrum, ...), which is
// exactly the nagging posture AGENTS.md's 铁律 2 forbids and undercuts
// 一次只问一个.
func hasInFlightScopedCard(cards []ScopedCard) bool {
	for _, c := range cards {
		if c.Status == "proposed" || c.Status == "active" {
			return true
		}
	}
	return false
}

// ChatStore is RunChatStep's isolated persistence seam (project-free). The
// sqlc adapter is chatstore.go; chat_step_test.go uses an in-memory fake.
type ChatStore interface {
	LoadThreadHistory(ctx context.Context, threadID uuid.UUID, limit int) ([]ChatTurn, error)
	CreateThreadMessage(ctx context.Context, threadID uuid.UUID, role, content, modality string) (uuid.UUID, error)
	ListThreadMaterials(ctx context.Context, threadID uuid.UUID) ([]ScopedMaterial, error)
	CreateThreadMaterial(ctx context.Context, threadID uuid.UUID, kind, source, title, sourceURL string) (uuid.UUID, error)
	ListThreadCards(ctx context.Context, threadID uuid.UUID) ([]ScopedCard, error)
	CreateThreadCardInstance(ctx context.Context, threadID uuid.UUID, cardID string) (uuid.UUID, error)
	InsertThreadEvent(ctx context.Context, threadID uuid.UUID, typ string, payload []byte) error
	RecordChatLLMCall(ctx context.Context, userID uuid.UUID, purpose string, resolved gateway.Resolved, prompt, completion int32) error
}

type ChatDeps struct {
	Store    ChatStore
	Provider gateway.Provider
	Resolved gateway.Resolved
	UserID   uuid.UUID
	ThreadID uuid.UUID
}

// CardOffer is one card surfaced to a student as an offer — the card
// instance, the card it renders, and the material it hangs on.
type CardOffer struct {
	CardInstanceID, MaterialID uuid.UUID
	CardID                     string
}
type ChatStepResult struct {
	Reply string
	Offer *CardOffer
}

// RunChatStep is the Chat-policy turn (agent-spec §5.4): coach alone, planner
// off. It (1) mints a thread material for a newly pasted URL, (2) asks the
// coach for one reply, metering the call even on enforcement reject, (3) on an
// accepted reply persists it as an assistant chat_message, and (4) surfaces a
// fresh CRAAP offer (I3) when the thread has an un-carded article. The handler
// has already persisted the student message + the prompt_sent event.
func RunChatStep(ctx context.Context, deps ChatDeps, studentMessage string) (ChatStepResult, error) {
	// (1) URL → thread material (dedup by source_url).
	if u, ok := DetectURL(studentMessage); ok {
		mats, err := deps.Store.ListThreadMaterials(ctx, deps.ThreadID)
		if err != nil {
			return ChatStepResult{}, err
		}
		exists := false
		for _, m := range mats {
			if m.SourceURL == u {
				exists = true
				break
			}
		}
		if !exists {
			host := u
			if pu, err := url.Parse(u); err == nil && pu.Host != "" {
				host = pu.Host
			}
			if _, err := deps.Store.CreateThreadMaterial(ctx, deps.ThreadID, "article", "pasted", host, u); err != nil {
				return ChatStepResult{}, err
			}
		}
	}

	// Classifier flag + coach context.
	mats, err := deps.Store.ListThreadMaterials(ctx, deps.ThreadID)
	if err != nil {
		return ChatStepResult{}, err
	}
	cards, err := deps.Store.ListThreadCards(ctx, deps.ThreadID)
	if err != nil {
		return ChatStepResult{}, err
	}
	materialID, cardID, moment := ChatCardCandidate(mats, cards)
	flag := ""
	if moment {
		flag = "学生贴进了一个来源链接，并把它当成论据——这是做「信源辨识（CRAAP）」的时机。"
	} else if !hasInFlightScopedCard(cards) && len([]rune(strings.TrimSpace(studentMessage))) >= MinClassifyRunes {
		// N3b Seam A in Chat: the structural link moment did not fire, so ask
		// the classifier whether a SEMANTIC one did. Metered even on `none`
		// or a parse failure; a classifier error is silence, never a failed
		// turn — the coach still replies below.
		//
		// Gated on hasInFlightScopedCard (whole-branch review IMPORTANT 2):
		// while any thread card is proposed/active, skip the classifier
		// entirely rather than pile a second unanswered offer on top of the
		// first.
		eligible := EligibleMomentsScoped(cards)
		m, usage, cerr := ClassifyMoment(ctx, deps.Provider, deps.Resolved, studentMessage, eligible)
		if usage.InputTokens > 0 || usage.OutputTokens > 0 {
			if rerr := deps.Store.RecordChatLLMCall(ctx, deps.UserID, "classify", deps.Resolved, int32(usage.InputTokens), int32(usage.OutputTokens)); rerr != nil {
				slog.Warn("chat: record classifier usage failed", "thread_id", deps.ThreadID.String(), "err", rerr.Error())
			}
		} else if cerr == nil {
			// The call succeeded and produced an answer, yet reported no
			// usage — the provider stopped emitting it. Mirrors loop.go's
			// coach/classify warnings: the classify call goes unmetered; do
			// not let that happen quietly.
			slog.Warn("chat: classify call returned no usage — turn is unmetered",
				"thread_id", deps.ThreadID.String(), "provider", deps.Resolved.Provider, "model", deps.Resolved.Model)
		}
		if cerr != nil {
			slog.Warn("chat: moment classifier failed; staying silent", "thread_id", deps.ThreadID.String(), "err", cerr.Error())
		} else if m != MomentNone {
			e := momentCard[m]
			flag = e.Flag
			cardID = e.CardID
			materialID = uuid.Nil // a semantic offer is not about one source
			moment = true
		}
	}
	history, err := deps.Store.LoadThreadHistory(ctx, deps.ThreadID, 12)
	if err != nil {
		slog.Warn("chat: load history failed; proceeding without it", "thread_id", deps.ThreadID.String(), "err", err.Error())
		history = nil
	}
	summary := threadMaterialSummary(mats)
	cardSummary := completedCardSummary(cards)

	// (2) Coach reply, metered even on reject.
	out, usage, err := ProposeChatReply(ctx, deps.Provider, deps.Resolved, history, summary, cardSummary, flag)
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		if rerr := deps.Store.RecordChatLLMCall(ctx, deps.UserID, "coach", deps.Resolved, int32(usage.InputTokens), int32(usage.OutputTokens)); rerr != nil {
			slog.Warn("chat: record llm usage failed", "thread_id", deps.ThreadID.String(), "err", rerr.Error())
		}
	}
	if err != nil {
		// Enforcement (or the model call) rejected the reply — stay silent.
		slog.Warn("chat: reply not emitted", "thread_id", deps.ThreadID.String(), "err", err.Error())
		return ChatStepResult{}, nil
	}

	// (3) Persist the accepted reply.
	if _, err := deps.Store.CreateThreadMessage(ctx, deps.ThreadID, "assistant", out.Body, "text"); err != nil {
		return ChatStepResult{}, err
	}
	result := ChatStepResult{Reply: out.Body}

	// (4) Fresh card offer.
	if moment {
		ciID, err := deps.Store.CreateThreadCardInstance(ctx, deps.ThreadID, cardID)
		if err != nil {
			return ChatStepResult{}, err
		}
		result.Offer = &CardOffer{CardInstanceID: ciID, MaterialID: materialID, CardID: cardID}
		payload, _ := json.Marshal(map[string]string{"card_id": cardID})
		if err := deps.Store.InsertThreadEvent(ctx, deps.ThreadID, "card_surfaced", payload); err != nil {
			slog.Warn("chat: append card_surfaced event failed", "thread_id", deps.ThreadID.String(), "err", err.Error())
		}
	}
	return result, nil
}

// completedCardSummary serializes the thread's COMPLETED cards for the coach's
// context. Only completed cards contribute: a proposed or active card has
// nothing finished to say, and a skipped one is a decline we do not re-raise.
// Unlike the Studio surface (Task 5's refeedCandidate, a dedicated post-submit
// turn), chat manufactures no turn of its own — this rides the student's NEXT
// message, so it must tolerate an unknown card id (skip, not fail) exactly
// like refeedCandidate does.
func completedCardSummary(scoped []ScopedCard) string {
	var parts []string
	for _, sc := range scoped {
		if sc.Status != "completed" {
			continue
		}
		spec, ok := cards.ByID(sc.CardID)
		if !ok {
			continue
		}
		inst := CardInstance{ID: sc.ID.String(), CardID: sc.CardID, Status: sc.Status}
		_ = json.Unmarshal(sc.Anchors, &inst.Anchors)         // absent/invalid → no anchor steps
		_ = json.Unmarshal(sc.FieldValues, &inst.FieldValues) // absent/invalid → no field steps
		parts = append(parts, renderCompletedCard(SerializeCardForRefeed(spec, inst)))
	}
	return strings.Join(parts, "\n")
}

// renderCompletedCard renders one completed card's refeed payload as text —
// the same card-name + step-title + answer shape BuildCoachContext's Studio
// refeed block uses (coach_prompt.go), reused here as plain text rather than
// a prompt-writer method since chat has no graph/edges block to sit beside.
func renderCompletedCard(p RefeedPayload) string {
	var b strings.Builder
	fmt.Fprintf(&b, "卡片：%s", p.CardName)
	for _, step := range p.Steps {
		fmt.Fprintf(&b, "\n## %s", step.Title)
		for _, a := range step.Answers {
			fmt.Fprintf(&b, "\n- %s：%v", a.Label, a.Value)
		}
	}
	return b.String()
}

func threadMaterialSummary(mats []ScopedMaterial) string {
	var parts []string
	for _, m := range mats {
		if m.Kind == "article" {
			parts = append(parts, m.SourceURL)
		}
	}
	return strings.Join(parts, "、")
}
