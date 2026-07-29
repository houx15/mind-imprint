package api

// reading_takeaway.go — S2 · assembles the record half of the reading
// takeaway (findings/credibility/key_quotes) from the student's ALREADY
// CONFIRMED reading cards. This is the deterministic counterpart to
// agent.ComposeReadingTakeawaySuggestions (Task 4's isolated compose core,
// which seeds only new_leads/proposal_impact) — record fields are never
// re-guessed by an LLM. Same source notesByMaterial (workspace_library.go)
// projects, but read through framework_fill (the persisted selectionEvalDTO,
// readeval.go) so we get the verdict + judgment, not just quote+finding.

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// readingOutcomesByMaterial assembles the record half of the takeaway from
// the student's CONFIRMED (status="completed") reading cards for this
// material. Attribution to a material lives in each card's anchors JSON, not
// in a card_instances column (card_instances has no material_id — see
// notesByMaterial's comment). Best-effort: a card whose anchors or
// framework_fill don't parse is simply skipped, never fabricated.
func (a *API) readingOutcomesByMaterial(r *http.Request, projectID, materialID uuid.UUID) agent.TakeawayRecord {
	var rec agent.TakeawayRecord
	cis, err := a.d.Queries.ListCardInstancesByProject(r.Context(), pgtype.UUID{Bytes: projectID, Valid: true})
	if err != nil {
		return rec
	}
	for _, ci := range cis {
		if ci.Status != "completed" {
			continue
		}
		// Material attribution lives in the anchors (card_instances has no
		// material_id column) — the same read notesByMaterial uses.
		var anchors []agent.Anchor
		if json.Unmarshal(ci.Anchors, &anchors) != nil || len(anchors) == 0 {
			continue
		}
		if anchors[0].MaterialID != materialID.String() {
			continue
		}
		var ev selectionEvalDTO
		if len(ci.FrameworkFill) > 0 {
			_ = json.Unmarshal(ci.FrameworkFill, &ev)
		}
		if ev.Finding != "" {
			rec.Findings = append(rec.Findings, ev.Finding)
		}
		for _, an := range anchors {
			if an.Author == "student" && an.Quote != "" {
				rec.KeyQuotes = append(rec.KeyQuotes, agent.KeyQuote{Quote: an.Quote, Why: ev.Judgment})
			}
		}
		// First CRAAP/SIFT verdict seen sets credibility.
		if rec.Credibility.Verdict == "" && (ci.CardID == "craap" || ci.CardID == "sift") && ev.VerdictLabel != "" {
			rec.Credibility = agent.Credibility{Verdict: ev.VerdictLabel, Why: ev.VerdictReason}
		}
	}
	return rec
}

// credibilityDTO/keyQuoteDTO/takeawayRecordDTO are agent.TakeawayRecord's
// wire shape — camelCase since the TS side Zod-parses this response directly
// (Task 8), and slices are never emitted as `null` (nonNilStrings/an explicit
// make below), so the student's draft editor never has to null-guard.
type credibilityDTO struct {
	Verdict string `json:"verdict"`
	Why     string `json:"why"`
}

type keyQuoteDTO struct {
	Quote string `json:"quote"`
	Why   string `json:"why"`
}

type takeawayRecordDTO struct {
	Findings    []string       `json:"findings"`
	Credibility credibilityDTO `json:"credibility"`
	KeyQuotes   []keyQuoteDTO  `json:"keyQuotes"`
}

func toTakeawayRecordDTO(rec agent.TakeawayRecord) takeawayRecordDTO {
	quotes := make([]keyQuoteDTO, 0, len(rec.KeyQuotes))
	for _, q := range rec.KeyQuotes {
		quotes = append(quotes, keyQuoteDTO{Quote: q.Quote, Why: q.Why})
	}
	return takeawayRecordDTO{
		Findings:    nonNilStrings(rec.Findings),
		Credibility: credibilityDTO{Verdict: rec.Credibility.Verdict, Why: rec.Credibility.Why},
		KeyQuotes:   quotes,
	}
}

// nonNilStrings returns ss, or an empty (never nil) slice — nil encodes as
// JSON `null`, and the wire contract for this endpoint is `[]`.
func nonNilStrings(ss []string) []string {
	if ss == nil {
		return []string{}
	}
	return ss
}

// getTakeawayDraft is the "AI drafts, student confirms" step of the reading
// takeaway's split-hybrid: assembles the record half deterministically
// (readingOutcomesByMaterial, never re-guessed) and runs ONE isolated
// mid-tier compose (agent.ComposeReadingTakeawaySuggestions) to SEED the two
// synthesis fields the student will edit. Read-only — nothing is persisted or
// finalized here (that's Task 6's PUT/finalize). 克制: if the resolver is
// unavailable or the compose errs, this still returns 200 with the assembled
// record and empty suggestions rather than 500ing; the student can always
// write her own. If the record is empty (nothing confirmed yet — a reachable
// state when she opens this before completing any reading card), the whole
// resolver/compose/meter block is skipped entirely: no network call, no
// llm_call row — metering must only ever reflect a call that actually
// happened.
func (a *API) getTakeawayDraft(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	if entitled, err := HasEntitlement(r.Context(), u); err != nil {
		httpx.WriteError(w, r, err)
		return
	} else if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}
	rid, err := uuid.Parse(r.PathValue("rid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	ref, err := a.d.Queries.GetReferenceForProject(r.Context(), sqlc.GetReferenceForProjectParams{ID: rid, ProjectID: projectID})
	if err != nil || !ref.MaterialID.Valid {
		httpx.WriteError(w, r, httpx.ErrNotFound("尚未进入阅读室"))
		return
	}
	materialID := uuid.UUID(ref.MaterialID.Bytes)
	record := a.readingOutcomesByMaterial(r, projectID, materialID)

	in := agent.ReadingTakeawayInput{Brief: a.readingBriefFor(r.Context(), projectID, materialID), Record: record}
	var leads []string
	var impact string
	// An empty record has nothing to organize —
	// agent.ComposeReadingTakeawaySuggestions would early-return an error
	// before ever calling the gateway, so resolving a provider and metering a
	// call here would be phantom bookkeeping for a network call that never
	// happened. Skip the whole block.
	if agent.HasRecordContent(record) {
		if resolved, rerr := a.d.ChatResolver(r.Context()); rerr == nil {
			l, imp, usage, cerr := agent.ComposeReadingTakeawaySuggestions(r.Context(), a.d.Provider, resolved, in)
			if resolved.Provider != "" {
				store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
				if e := store.RecordLLMCall(r.Context(), agent.LLMCallRow{
					ProjectID: projectID, Surface: "studio", Purpose: "reading_takeaway_draft",
					Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
				}); e != nil {
					slog.Warn("takeaway draft: record llm", "err", e, "request_id", httpx.RequestIDFromContext(r.Context()))
				}
			}
			if cerr == nil {
				leads, impact = l, imp
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"record":                  toTakeawayRecordDTO(record),
		"suggestedNewLeads":       nonNilStrings(leads),
		"suggestedProposalImpact": impact,
	})
}
