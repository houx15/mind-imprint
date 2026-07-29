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
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// readingOutcomesByMaterial assembles the record half of the takeaway from
// the student's CONFIRMED (status="completed") reading cards for this
// material. Thin *http.Request-scoped forwarder over readingOutcomesByMaterialCtx
// (S2 spine projection, projectcoach.go, has no request to hang off of).
func (a *API) readingOutcomesByMaterial(r *http.Request, projectID, materialID uuid.UUID) agent.TakeawayRecord {
	return a.readingOutcomesByMaterialCtx(r.Context(), projectID, materialID)
}

// readingOutcomesByMaterialCtx holds the actual assembly logic (moved out of
// readingOutcomesByMaterial so the ctx-only spine projection can reuse it).
// Runs its own ListCardInstancesByProject scan; callers that need this for
// SEVERAL materials in the same request (e.g. buildSpineProjection's 文献库
// loop) should instead fetch cards ONCE and call readingOutcomesFromCards
// per-material to avoid an N+1 scan.
func (a *API) readingOutcomesByMaterialCtx(ctx context.Context, projectID, materialID uuid.UUID) agent.TakeawayRecord {
	cis, err := a.d.Queries.ListCardInstancesByProject(ctx, pgtype.UUID{Bytes: projectID, Valid: true})
	if err != nil {
		return agent.TakeawayRecord{}
	}
	return readingOutcomesFromCards(cis, materialID)
}

// readingOutcomesFromCards is the PURE assembly core: given an already-fetched
// slice of card instances (no DB access), assembles the record half of the
// takeaway for one material. Attribution to a material lives in each card's
// anchors JSON, not in a card_instances column (card_instances has no
// material_id — see notesByMaterial's comment). Best-effort: a card whose
// anchors or framework_fill don't parse is simply skipped, never fabricated.
func readingOutcomesFromCards(cis []sqlc.CardInstance, materialID uuid.UUID) agent.TakeawayRecord {
	var rec agent.TakeawayRecord
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
			if usage.InputTokens > 0 || usage.OutputTokens > 0 {
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

// finalizeReadingReq is the student's FINAL synthesis only — record fields
// (findings/credibility/keyQuotes) are never trusted from the client; they're
// re-assembled server-side from her confirmed reading cards (see
// postFinalizeReading).
type finalizeReadingReq struct {
	NewLeads       []string `json:"new_leads"`
	ProposalImpact string   `json:"proposal_impact"`
}

// postFinalizeReading is the "student confirms" half of the takeaway's
// split-hybrid (getTakeawayDraft is the "AI drafts" half). It RE-ASSEMBLES the
// record half server-side from readingOutcomesByMaterial — the record's
// source of truth is her CONFIRMED reading outcomes, never whatever the
// client happened to send — folds in her authored/edited synthesis
// (new_leads/proposal_impact), and persists the full 5-field takeaway.
// FinalizeReadingTakeaway is an UPDATE: re-finalizing on a re-read SUPERSEDES,
// it never inserts a second row. No LLM call, no metering, no HasEntitlement
// gate — the student already did the thinking; this just records it. Also
// non-blocking on a thin record (HasRecordContent is NOT checked here) — she
// may finalize proposal_impact alone before any reading card is confirmed.
func (a *API) postFinalizeReading(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	rid, err := uuid.Parse(r.PathValue("rid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	var body finalizeReadingReq
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ref, err := a.d.Queries.GetReferenceForProject(r.Context(), sqlc.GetReferenceForProjectParams{ID: rid, ProjectID: projectID})
	if err != nil || !ref.MaterialID.Valid {
		httpx.WriteError(w, r, httpx.ErrNotFound("尚未进入阅读室"))
		return
	}
	materialID := uuid.UUID(ref.MaterialID.Bytes)
	record := a.readingOutcomesByMaterial(r, projectID, materialID)
	takeaway := agent.ReadingTakeaway{
		Findings: record.Findings, Credibility: record.Credibility, KeyQuotes: record.KeyQuotes,
		NewLeads: nonNilStrings(body.NewLeads), ProposalImpact: strings.TrimSpace(body.ProposalImpact),
	}
	raw, merr := json.Marshal(takeaway)
	if merr != nil {
		httpx.WriteError(w, r, merr)
		return
	}
	row, err := a.d.Queries.FinalizeReadingTakeaway(r.Context(), sqlc.FinalizeReadingTakeawayParams{
		ID: rid, ProjectID: projectID, Takeaway: raw,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Best-effort activity log — same helper postPlanGenerate uses; a log
	// failure must never fail the finalize itself.
	if err := a.appendAutoLog(r.Context(), a.d.Queries, projectID, "归纳了《"+truncateRunes(ref.Title, 30)+"》"); err != nil {
		slog.Warn("finalize reading: append auto-log failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	// Best-effort lead materialization (S3 D-S3-2) — a materialize failure
	// must never fail the finalize itself, same posture as the log above.
	a.materializeLeads(r.Context(), projectID, rid, body.NewLeads)
	// Notes re-project from the reference's material — same fix patchReference
	// applies; passing nil here would wipe the reference's notes[] in the
	// response (a prior reviewer already caught this class of bug).
	var notes []readingNoteDTO
	if row.MaterialID.Valid {
		notes = a.notesByMaterial(r, projectID)[uuid.UUID(row.MaterialID.Bytes).String()]
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"reference": toReferenceDTO(row, notes),
	})
}

// materializeLeads turns a finalized takeaway's new_leads into first-class
// exploration branches (S3 D-S3-2, exploration.go's lead lifecycle) —
// origin "takeaway", status "open", attributed to sourceRefID. Idempotent AND
// non-resurrecting: for each non-blank text, a CountExplorationLeadForSource
// hit (by project+source+text) means a lead with that text already exists for
// this source — either materialized by a prior finalize (skip, no dup) or
// materialized-then-pruned by the student (skip, no resurrection) — so the
// same Count check gives both guarantees for free. Best-effort, mirroring
// postFinalizeReading's appendAutoLog posture: any error here is logged and
// swallowed, never surfaced to the caller — the finalize itself already
// succeeded and must not be undone by a leads-side hiccup.
func (a *API) materializeLeads(ctx context.Context, projectID, sourceRefID uuid.UUID, texts []string) {
	existing, err := a.d.Queries.ListExplorationLeads(ctx, projectID)
	if err != nil {
		slog.Warn("finalize reading: materialize leads: list existing failed", "err", err)
		return
	}
	pos := int32(len(existing))
	srcRef := pgtype.UUID{Bytes: sourceRefID, Valid: true}
	for _, text := range texts {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		n, err := a.d.Queries.CountExplorationLeadForSource(ctx, sqlc.CountExplorationLeadForSourceParams{
			ProjectID: projectID, SourceReferenceID: srcRef, Text: text,
		})
		if err != nil {
			slog.Warn("finalize reading: materialize leads: count failed", "err", err, "text", text)
			continue
		}
		if n > 0 {
			continue
		}
		if _, err := a.d.Queries.CreateExplorationLead(ctx, sqlc.CreateExplorationLeadParams{
			ProjectID: projectID, Text: text, Status: "open", Origin: "takeaway",
			SourceReferenceID: srcRef, Position: pos,
		}); err != nil {
			slog.Warn("finalize reading: materialize leads: create failed", "err", err, "text", text)
			continue
		}
		pos++
	}
}
