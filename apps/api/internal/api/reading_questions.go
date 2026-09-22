package api

// reading_questions.go — S8 · GET /readings/{id}/questions, the room's last
// gift on the way out: a few questions that grew from THIS article, worth
// going and writing about.
//
// 产品的原话：「some interesting questions would grow from this reading — why
// xxxx, what is xxx, how people view xxx」. The failure mode to design against
// is genericity — 「你怎么看待环保？」 is a question you could staple onto ANY
// article and it would still "fit". A question like that teaches nothing,
// because answering it never required reading this piece at all.
//
// The guarantee against that is a TYPE, not a prompt manner: every question
// the model returns must carry an anchorQuote, and validateReadingQuestions
// keeps a question ONLY when that quote is a literal substring of the article
// body. A generic question cannot cite a sentence in this particular piece,
// so it structurally cannot survive — the prompt also asks nicely, but the
// prompt is not what is doing the enforcing. Fewer than two survivors and
// NONE are shown: a thin, generic suggestion is worse than no suggestion.
//
// Generated once, on first open, and stored — the same idempotent shape
// writing_setup.go's postWritingOpening pioneered in this codebase (see that
// file's comment for the full argument against a partial unique index here):
// a cheap pre-check outside any transaction, then a transaction-scoped
// pg_advisory_xact_lock keyed on the atom, THEN a re-check of the same read
// UNDER that lock, and only then — still nothing found — the one provider
// call. The lock sits before the call, never after, so a race between two
// concurrent first-opens costs exactly one charge: the loser blocks on the
// lock, wakes up after the winner's commit, and re-reads the winner's rows
// instead of ever reaching the provider.
//
// Task 8 fix round 1: "has it been generated" is NOT "does reading_question
// have rows". A thin article can legitimately validate down to zero
// survivors, and that outcome looks IDENTICAL, in reading_question, to
// "never generated" — so gating on row-count made the room re-call the
// flagship model on every single reopen of a thin, already-finished reading,
// forever, rendering nothing each time. The fix is `reading.questions_at`
// (migration 0104): NULL means "never attempted", non-NULL means "attempted
// once, whatever it produced" — set inside the same transaction as the
// inserts, unconditionally, even when zero rows are inserted. Both existence
// checks (the cheap pre-check and the re-check under the lock) gate on this
// column now, never on row count.

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// readingQuestionsArticleRuneBudget bounds how much article feeds this call.
// Same value and the same reasoning as readingPlanArticleRuneBudget
// (reading_plan.go): generous enough that the model can actually find a
// sentence worth quoting, bounded because lite has no compaction layer. Kept
// as its own constant rather than reused — the two prompts are free to
// diverge in budget later without one accidentally moving the other.
const readingQuestionsArticleRuneBudget = 9000

// readingQuestionDraft is one question as the model proposes it — before
// validateReadingQuestions decides whether it survives. No id, no
// anchorBlock: those only exist once a draft has cleared validation and is
// about to be persisted (anchorBlock in particular is derived server-side by
// finding which paragraph the quote lives in, never trusted from the model).
type readingQuestionDraft struct {
	Text        string `json:"text"`
	AnchorQuote string `json:"anchorQuote"`
}

type readingQuestionsReply struct {
	Questions []readingQuestionDraft `json:"questions"`
}

// parseReadingQuestionsReply decodes the model's JSON object, tolerating the
// same code-fence wrapping parseReadingPlan (reading_plan.go) tolerates.
func parseReadingQuestionsReply(text string) ([]readingQuestionDraft, bool) {
	c := strings.TrimSpace(text)
	if strings.HasPrefix(c, "```json") {
		c = strings.TrimLeft(strings.TrimPrefix(c, "```json"), " \t\r\n")
	} else if strings.HasPrefix(c, "```") {
		c = strings.TrimLeft(c[3:], " \t\r\n")
	}
	if strings.HasSuffix(c, "```") {
		c = strings.TrimRight(c[:len(c)-3], " \t\r\n")
	}
	if i := strings.IndexByte(c, '{'); i > 0 {
		c = c[i:]
	}
	if j := strings.LastIndexByte(c, '}'); j >= 0 && j < len(c)-1 {
		c = c[:j+1]
	}
	var got readingQuestionsReply
	if err := json.Unmarshal([]byte(strings.TrimSpace(c)), &got); err != nil {
		return nil, false
	}
	return got.Questions, true
}

// validateReadingQuestions keeps a question only if the sentence it claims to
// have grown from is literally in the article. This is what makes a generic
// question structurally impossible rather than merely discouraged: 「你怎么看
// 待环保？」 cannot cite a line in this piece, so it cannot survive. Under two
// survivors, none are shown at all — a thin, generic suggestion is worse than
// no suggestion.
func validateReadingQuestions(qs []readingQuestionDraft, body string) []readingQuestionDraft {
	out := make([]readingQuestionDraft, 0, len(qs))
	for _, q := range qs {
		text := strings.TrimSpace(q.Text)
		quote := strings.TrimSpace(q.AnchorQuote)
		if text == "" || quote == "" || !strings.Contains(body, quote) {
			continue
		}
		out = append(out, readingQuestionDraft{Text: text, AnchorQuote: quote})
		if len(out) == 5 {
			break
		}
	}
	if len(out) < 2 {
		return nil
	}
	return out
}

// findAnchorBlock reports the id of the first block whose text contains
// quote — the paragraph the question is anchored to, for the client to
// scroll/highlight. A quote that (by construction, having just passed
// validateReadingQuestions) is a substring of the whole body but happens to
// straddle a blank-line split so no single block contains it whole returns
// "" — the question itself is still shown; only the highlight is lost.
func findAnchorBlock(blocks []Block, quote string) string {
	for _, b := range blocks {
		if strings.Contains(b.Text, quote) {
			return b.ID
		}
	}
	return ""
}

// readingQuestionDTO is the wire shape: {"id","text","anchorQuote","anchorBlock"}.
type readingQuestionDTO struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	AnchorQuote string `json:"anchorQuote"`
	AnchorBlock string `json:"anchorBlock"`
}

func toReadingQuestionDTOs(rows []sqlc.ReadingQuestion) []readingQuestionDTO {
	out := make([]readingQuestionDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, readingQuestionDTO{
			ID: row.ID.String(), Text: row.Text, AnchorQuote: row.AnchorQuote, AnchorBlock: row.AnchorBlock,
		})
	}
	return out
}

// getReadingQuestions is GET /api/v1/readings/{id}/questions. See the file
// comment for the idempotent/single-charge shape and the anchorQuote
// guarantee.
func (a *API) getReadingQuestions(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	// Cheap pre-check outside any transaction: after the first attempt (of
	// EITHER outcome — survivors or none) this is the only cost, and it
	// short-circuits every reopen without ever touching the lock. Gated on
	// reading.questions_at, never on row count — see the file comment.
	rd, err := a.d.Queries.GetReading(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// F5: the whole guarantee this endpoint makes — ONE flagship generation,
	// spent once, on a reading she has actually finished — depends on never
	// reaching the provider before then. Gated here, before even the cheap
	// questions_at pre-check, so an unfinished reading costs nothing and
	// stamps nothing: a direct call mid-reading (the component only mounts on
	// the finished screen, but nothing stops a direct request) must not burn
	// the single generation early and permanently. Not an error — an
	// unfinished reading simply has no questions yet.
	if rd.Status != "finished" {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"questions": []readingQuestionDTO{}})
		return
	}
	if rd.QuestionsAt.Valid {
		rows, err := a.d.Queries.ListReadingQuestions(r.Context(), at.ID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"questions": toReadingQuestionDTOs(rows)})
		return
	}

	src, err := a.d.Queries.GetReadingSource(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err) // no article yet → 404
		return
	}
	blocks := SplitBlocks(src.Body)
	if len(blocks) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"questions": []readingQuestionDTO{}})
		return
	}

	// Run to completion even if she navigates away mid-call — same posture as
	// every other lite provider call (see detachedModelCtx, reading_lens.go).
	qCtx, cancel := detachedModelCtx(r)
	defer cancel()

	tx, err := a.d.Pool.Begin(qCtx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(qCtx)) }()
	// hashtext of the atom uuid, namespaced so this can never collide with an
	// advisory lock some other endpoint takes on the same atom id (e.g.
	// writing_setup.go's ":opening" suffix on writing atoms — a different
	// kind entirely, but the discipline is the same).
	if _, err := tx.Exec(qCtx, "SELECT pg_advisory_xact_lock(hashtext($1))", at.ID.String()+":questions"); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	qtx := a.d.Queries.WithTx(tx)

	// Re-read UNDER the lock, same column as the cheap pre-check. A racing
	// request that already generated (and committed — of either outcome)
	// while this one waited on the lock wins outright: this one returns its
	// rows (possibly empty) without ever reaching the provider.
	rd, err = qtx.GetReading(qCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if rd.QuestionsAt.Valid {
		rows, err := qtx.ListReadingQuestions(qCtx, at.ID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if err := tx.Commit(qCtx); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"questions": toReadingQuestionDTOs(rows)})
		return
	}

	// §model-routing · compose. Deriving the comprehension questions for a text
	// is structure generation over material already on the page. Only 过程评估
	// carries the never-downgrade promise; this call never did — it inherited
	// the flagship lane because there was no middle one.
	resolved, okResolve := a.route(qCtx, gateway.ClassCompose)
	if !okResolve {
		slog.Warn("reading questions: no provider resolved", "atom_id", at.ID)
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	res, cerr := gateway.Collect(qCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: readingQuestionsSystem},
			{Role: gateway.RoleUser, Content: buildReadingQuestionsPrompt(src.Title, blocks)},
		},
	})
	a.recordLiteLLMCall(qCtx, u.ID, at.ID, "read_questions", resolved, res.Usage)
	if cerr != nil {
		slog.Warn("reading questions: provider call failed", "err", cerr, "atom_id", at.ID)
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	drafts, okParse := parseReadingQuestionsReply(res.Text)
	if !okParse {
		slog.Warn("reading questions: unparseable reply", "atom_id", at.ID)
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	survivors := validateReadingQuestions(drafts, src.Body)

	inserted := make([]sqlc.ReadingQuestion, 0, len(survivors))
	for i, q := range survivors {
		row, err := qtx.InsertReadingQuestion(qCtx, sqlc.InsertReadingQuestionParams{
			AtomID: at.ID, Position: int32(i), Text: q.Text, AnchorQuote: q.AnchorQuote,
			AnchorBlock: findAnchorBlock(blocks, q.AnchorQuote),
		})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		inserted = append(inserted, row)
	}
	// Record the ATTEMPT, unconditionally — even when survivors is empty.
	// This is the whole point of the fix: the outcome (2-5 questions, or
	// none) must never be confused with "never tried" on the next open.
	if _, err := qtx.MarkReadingQuestionsGenerated(qCtx, at.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(qCtx); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"questions": toReadingQuestionDTOs(inserted)})
}
