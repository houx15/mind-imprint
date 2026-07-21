package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"strings"

	"github.com/google/uuid"

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
	} else if len([]rune(strings.TrimSpace(studentMessage))) >= MinClassifyRunes {
		// N3b Seam A in Chat: the structural link moment did not fire, so ask
		// the classifier whether a SEMANTIC one did. Metered even on `none`
		// or a parse failure; a classifier error is silence, never a failed
		// turn — the coach still replies below.
		eligible := EligibleMomentsScoped(cards)
		m, usage, cerr := ClassifyMoment(ctx, deps.Provider, deps.Resolved, studentMessage, eligible)
		if usage.InputTokens > 0 || usage.OutputTokens > 0 {
			if rerr := deps.Store.RecordChatLLMCall(ctx, deps.UserID, "classify", deps.Resolved, int32(usage.InputTokens), int32(usage.OutputTokens)); rerr != nil {
				slog.Warn("chat: record classifier usage failed", "thread_id", deps.ThreadID.String(), "err", rerr.Error())
			}
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

	// (2) Coach reply, metered even on reject.
	out, usage, err := ProposeChatReply(ctx, deps.Provider, deps.Resolved, history, summary, flag)
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

func threadMaterialSummary(mats []ScopedMaterial) string {
	var parts []string
	for _, m := range mats {
		if m.Kind == "article" {
			parts = append(parts, m.SourceURL)
		}
	}
	return strings.Join(parts, "、")
}
