package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

func TestReadingBriefAndTakeawayRoundTrip(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	projectID := mustUUID(seedProjectID)

	// Seed a reference in the project. tags/search_hints are jsonb NOT NULL
	// (DEFAULT '[]' only applies when the column is omitted from the INSERT,
	// but CreateReference's :one always supplies all columns) — a nil []byte
	// param encodes as SQL NULL, so they must be set explicitly here.
	ref, err := q.CreateReference(ctx, sqlc.CreateReferenceParams{
		ProjectID: projectID, Title: "NASA 报告",
		Tags: []byte("[]"), SearchHints: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("CreateReference: %v", err)
	}

	reason, focus, phase := "验证碳排放是否构成反例", "看它引用了谁", "反例检验"
	updated, err := q.UpdateReadingBrief(ctx, sqlc.UpdateReadingBriefParams{
		ID: ref.ID, ProjectID: projectID,
		ReadingReason: &reason, ReadingFocus: &focus, PhaseTag: &phase,
	})
	if err != nil {
		t.Fatalf("UpdateReadingBrief: %v", err)
	}
	if updated.ReadingReason == nil || *updated.ReadingReason != reason {
		t.Fatalf("reading_reason not persisted: %+v", updated.ReadingReason)
	}
	if updated.TakeawayFinalizedAt.Valid {
		t.Fatalf("finalized_at should be null before finalize")
	}

	obj := map[string]any{"proposal_impact": "把它作为让步段的证据"}
	raw, _ := json.Marshal(obj)
	fin, err := q.FinalizeReadingTakeaway(ctx, sqlc.FinalizeReadingTakeawayParams{
		ID: ref.ID, ProjectID: projectID, Takeaway: raw,
	})
	if err != nil {
		t.Fatalf("FinalizeReadingTakeaway: %v", err)
	}
	if !fin.TakeawayFinalizedAt.Valid {
		t.Fatalf("finalized_at should be set after finalize")
	}
	var got map[string]any
	if err := json.Unmarshal(fin.Takeaway, &got); err != nil || got["proposal_impact"] != obj["proposal_impact"] {
		t.Fatalf("takeaway round-trip mismatch: %v / %v", err, got)
	}

	// Scope guard: wrong project id must not find it.
	if _, err := q.GetReferenceForProject(ctx, sqlc.GetReferenceForProjectParams{
		ID: ref.ID, ProjectID: uuid.New(),
	}); err == nil {
		t.Fatalf("GetReferenceForProject should 404 across projects")
	}
}

// seedReadingOutcome seeds a material + a reference linked to it (same
// SetReferenceMaterial wiring as TestPutReadingBrief_PreservesNotes) plus TWO
// CONFIRMED (status=completed) reading cards attributed to the material via
// their anchors: a craap card carrying a verdict in framework_fill (->
// record.credibility) and a plain (toulmin) card carrying findingText in
// framework_fill (-> record.findings accumulation). Both cards also carry a
// student-authored anchor so record.keyQuotes has something too — the craap
// card's own quote plus quoteText on the toulmin card.
func seedReadingOutcome(t *testing.T, h http.Handler, cookie *http.Cookie, q *sqlc.Queries, findingText, quoteText string) (ref sqlc.Reference, materialID string) {
	t.Helper()
	ctx := context.Background()
	projectID := uuid.MustParse(seedProjectID)

	matID := ingestMaterialForTest(t, h, cookie, seedProjectID, "NASA 报告", craapMaterialText)
	r, err := q.CreateReference(ctx, sqlc.CreateReferenceParams{
		ProjectID: projectID, Title: "NASA 报告",
		Tags: []byte("[]"), SearchHints: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("CreateReference: %v", err)
	}
	if _, err := q.SetReferenceMaterial(ctx, sqlc.SetReferenceMaterialParams{
		ID: r.ID, ProjectID: projectID,
		MaterialID: pgtype.UUID{Bytes: uuid.MustParse(matID), Valid: true},
	}); err != nil {
		t.Fatalf("link reference to material: %v", err)
	}

	// Card 1: craap verdict + a student anchor -> record.credibility.
	craapCI, err := q.CreateProjectCardInstance(ctx, sqlc.CreateProjectCardInstanceParams{
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, CardID: "craap", Status: "active",
	})
	if err != nil {
		t.Fatalf("create craap card instance: %v", err)
	}
	craapAnchors := fmt.Sprintf(`[{"id":"a0","material_id":%q,"quote":"官方数据来自 NASA 卫星","author":"student"}]`, matID)
	if _, err := q.SetCardInstanceAnchors(ctx, sqlc.SetCardInstanceAnchorsParams{
		ID: craapCI.ID, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, Anchors: []byte(craapAnchors),
	}); err != nil {
		t.Fatalf("set craap anchors: %v", err)
	}
	craapFramework := []byte(`{"verdictLabel":"三手 · 需溯源","verdictReason":"引用链未溯源到一手数据"}`)
	if _, err := q.SetCardInstanceFramework(ctx, sqlc.SetCardInstanceFrameworkParams{
		ID: craapCI.ID, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, FrameworkFill: craapFramework,
	}); err != nil {
		t.Fatalf("set craap framework: %v", err)
	}
	if _, err := q.SetCardInstanceStatus(ctx, sqlc.SetCardInstanceStatusParams{
		ID: craapCI.ID, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, Status: "completed",
	}); err != nil {
		t.Fatalf("complete craap card: %v", err)
	}

	// Card 2: plain finding + student quote -> record.findings accumulation.
	toulminCI, err := q.CreateProjectCardInstance(ctx, sqlc.CreateProjectCardInstanceParams{
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, CardID: "toulmin", Status: "active",
	})
	if err != nil {
		t.Fatalf("create toulmin card instance: %v", err)
	}
	toulminAnchors := fmt.Sprintf(`[{"id":"a0","material_id":%q,"quote":%q,"author":"student"}]`, matID, quoteText)
	if _, err := q.SetCardInstanceAnchors(ctx, sqlc.SetCardInstanceAnchorsParams{
		ID: toulminCI.ID, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, Anchors: []byte(toulminAnchors),
	}); err != nil {
		t.Fatalf("set toulmin anchors: %v", err)
	}
	toulminFramework, merr := json.Marshal(map[string]string{"finding": findingText, "judgment": "支撑论点"})
	if merr != nil {
		t.Fatalf("marshal toulmin framework: %v", merr)
	}
	if _, err := q.SetCardInstanceFramework(ctx, sqlc.SetCardInstanceFrameworkParams{
		ID: toulminCI.ID, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, FrameworkFill: toulminFramework,
	}); err != nil {
		t.Fatalf("set toulmin framework: %v", err)
	}
	if _, err := q.SetCardInstanceStatus(ctx, sqlc.SetCardInstanceStatusParams{
		ID: toulminCI.ID, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, Status: "completed",
	}); err != nil {
		t.Fatalf("complete toulmin card: %v", err)
	}

	return r, matID
}

// TestGetTakeawayDraft_AssemblesAndSeeds — Task 5: GET .../takeaway-draft
// assembles the student's confirmed record (readingOutcomesByMaterial) and
// runs ONE mid-tier compose (agent.ComposeReadingTakeawaySuggestions, via the
// CHAT (mid-tier) resolver, not EvalResolver) to seed new_leads/
// proposal_impact — a draft the student will edit, never persisted here.
func TestGetTakeawayDraft_AssemblesAndSeeds(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	draftReply := `{"new_leads":["核实中国近五年的人均碳排放变化趋势"],"proposal_impact":"作为让步段里承认反例的关键证据"}`
	h := New(Deps{
		Queries: q, Pool: pool,
		Provider: readingStubProvider(draftReply), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	findingText := "中国碳排放总量常年全球第一，这是论证要正面处理的反例"
	quoteText := "中国碳排放全球第一"
	ref, _ := seedReadingOutcome(t, h, cookie, q, findingText, quoteText)

	rr := doJSON(t, h, cookie, http.MethodGet,
		"/api/v1/projects/"+seedProjectID+"/references/"+ref.ID.String()+"/takeaway-draft", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), quoteText) {
		t.Fatalf("draft should assemble the confirmed key quote: %s", rr.Body.String())
	}

	var out struct {
		Record struct {
			Findings    []string `json:"findings"`
			Credibility struct {
				Verdict string `json:"verdict"`
				Why     string `json:"why"`
			} `json:"credibility"`
			KeyQuotes []struct {
				Quote string `json:"quote"`
				Why   string `json:"why"`
			} `json:"keyQuotes"`
		} `json:"record"`
		SuggestedNewLeads       []string `json:"suggestedNewLeads"`
		SuggestedProposalImpact string   `json:"suggestedProposalImpact"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode draft response: %v — %s", err, rr.Body.String())
	}
	foundFinding := false
	for _, f := range out.Record.Findings {
		if f == findingText {
			foundFinding = true
		}
	}
	if !foundFinding {
		t.Fatalf("record.findings missing seeded finding: %+v", out.Record.Findings)
	}
	if out.Record.Credibility.Verdict != "三手 · 需溯源" {
		t.Fatalf("record.credibility.verdict = %q, want the seeded craap/sift verdict", out.Record.Credibility.Verdict)
	}
	if len(out.SuggestedNewLeads) == 0 {
		t.Fatalf("suggestedNewLeads should be seeded by the stub provider, got empty")
	}
	if out.SuggestedProposalImpact == "" {
		t.Fatalf("suggestedProposalImpact should be seeded by the stub provider, got empty")
	}

	// Exactly ONE mid-tier compose call was metered for this draft.
	if n := countLLMCallsByPurpose(t, pool, seedProjectID, "reading_takeaway_draft"); n != 1 {
		t.Fatalf("want 1 draft llm_call, got %d", n)
	}
}

// TestFinalizeReading_PersistsAndSupersedesNoSpend — Task 6: POST
// finalize-reading re-assembles the record server-side (readingOutcomesByMaterial,
// never trusted from the client), folds in the student's authored synthesis
// (new_leads/proposal_impact), persists the full takeaway, and stamps
// takeaway_finalized_at. A second finalize call on the same reference
// SUPERSEDES (same row, not a second insert) and neither call spends —
// finalize is the student confirming her own synthesis, not an LLM call.
func TestFinalizeReading_PersistsAndSupersedesNoSpend(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{
		Queries: q, Pool: pool,
		Provider: readingStubProvider(`{"new_leads":[],"proposal_impact":""}`), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	findingText := "中国碳排放总量常年全球第一，这是论证要正面处理的反例"
	quoteText := "中国碳排放全球第一"
	ref, _ := seedReadingOutcome(t, h, cookie, q, findingText, quoteText)
	url := "/api/v1/projects/" + seedProjectID + "/references/" + ref.ID.String() + "/finalize-reading"

	rr := doJSON(t, h, cookie, http.MethodPost, url, `{"new_leads":["人均口径"],"proposal_impact":"让步段反例"}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "让步段反例") {
		t.Fatalf("finalize 1 failed: %d %s", rr.Code, rr.Body.String())
	}
	// Re-finalize supersedes.
	rr2 := doJSON(t, h, cookie, http.MethodPost, url, `{"new_leads":[],"proposal_impact":"改写后的影响"}`)
	if rr2.Code != http.StatusOK || !strings.Contains(rr2.Body.String(), "改写后的影响") {
		t.Fatalf("re-finalize should supersede: %d %s", rr2.Code, rr2.Body.String())
	}
	if strings.Contains(rr2.Body.String(), "让步段反例") {
		t.Fatalf("re-finalize should replace, not append, the prior synthesis: %s", rr2.Body.String())
	}
	// Finalize spends NOTHING.
	if n := countLLMCallsByPurpose(t, pool, seedProjectID, "reading_takeaway_draft") + countLLMCallsByPurpose(t, pool, seedProjectID, "reading_takeaway"); n != 0 {
		t.Fatalf("finalize must not spend, got %d calls", n)
	}
}

// TestGetTakeawayDraft_EmptyRecordNoSpend — the regression pin for the
// phantom-metering bug: a reference linked to a material with ZERO completed
// reading cards (readingOutcomesByMaterial returns an empty record) must
// short-circuit BEFORE the resolver/compose block — no network call, no
// llm_call row — even though the resolver itself would happily succeed.
// Before the fix, resolved.Provider != "" alone (regardless of whether
// compose ever ran) triggered a metered 0-token row for a call that never
// happened.
func TestGetTakeawayDraft_EmptyRecordNoSpend(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{
		Queries: q, Pool: pool,
		Provider: readingStubProvider(`{"new_leads":[],"proposal_impact":""}`), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	ctx := context.Background()
	projectID := uuid.MustParse(seedProjectID)
	matID := ingestMaterialForTest(t, h, cookie, seedProjectID, "NASA 报告", craapMaterialText)
	ref, err := q.CreateReference(ctx, sqlc.CreateReferenceParams{
		ProjectID: projectID, Title: "NASA 报告",
		Tags: []byte("[]"), SearchHints: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("CreateReference: %v", err)
	}
	if _, err := q.SetReferenceMaterial(ctx, sqlc.SetReferenceMaterialParams{
		ID: ref.ID, ProjectID: projectID,
		MaterialID: pgtype.UUID{Bytes: uuid.MustParse(matID), Valid: true},
	}); err != nil {
		t.Fatalf("link reference to material: %v", err)
	}
	// Deliberately NO card instances created for this material — the student
	// has opened takeaway-draft before confirming any reading.

	rr := doJSON(t, h, cookie, http.MethodGet,
		"/api/v1/projects/"+seedProjectID+"/references/"+ref.ID.String()+"/takeaway-draft", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var out struct {
		Record struct {
			Findings []string `json:"findings"`
		} `json:"record"`
		SuggestedNewLeads       []string `json:"suggestedNewLeads"`
		SuggestedProposalImpact string   `json:"suggestedProposalImpact"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode draft response: %v — %s", err, rr.Body.String())
	}
	if len(out.Record.Findings) != 0 {
		t.Fatalf("record.findings should be empty, got %+v", out.Record.Findings)
	}
	if len(out.SuggestedNewLeads) != 0 {
		t.Fatalf("suggestedNewLeads should be empty, got %+v", out.SuggestedNewLeads)
	}
	if out.SuggestedProposalImpact != "" {
		t.Fatalf("suggestedProposalImpact should be empty, got %q", out.SuggestedProposalImpact)
	}

	// The regression pin: an empty record must never touch the resolver/
	// compose/meter block, so NO llm_call row is written for this purpose.
	if n := countLLMCallsByPurpose(t, pool, seedProjectID, "reading_takeaway_draft"); n != 0 {
		t.Fatalf("want 0 draft llm_call for an empty record, got %d", n)
	}
}
