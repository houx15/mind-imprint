package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestSetWritingStage_TraceAndStageAreAtomic — 铁律④'s guarantee, executed
// rather than reasoned about.
//
// Every stage change writes TWO rows: the new `writing.stage`, and an
// `atom_message` (role='system') recording from→to. The second row is the whole
// point. Stages are a map, not a gate — the student is free to skip 大纲
// entirely — and the product's answer to that freedom is not to prevent the
// skip but to RECORD it, so the evidence survives into her report. A stage
// change that lands without its trace is therefore not a small inconsistency:
// it is a skip that silently never happened, which is the single outcome
// 铁律④ exists to prevent.
//
// The implementer reported these two writes share one transaction. That is a
// claim about the FAILURE path, and this phase has already learned twice that
// an unexercised failure path is a comment, not a guarantee. So exercise it:
// a BEFORE INSERT trigger on atom_message makes the TRACE fail, and the
// assertion is that the STAGE did not move either.
//
// This also guards a specific, easy, invisible bug the review was asked to look
// for: writing the stage through the transaction-scoped queries but the trace
// through the pool-level handle (or vice versa). That code reads as atomic and
// is not, and no happy-path test can tell the difference.
func TestSetWritingStage_TraceAndStageAreAtomic(t *testing.T) {
	h, cookie, q, pool := liteHandler(t)
	ctx := context.Background()

	id := createWritingAtomHTTP(t, h, cookie, "我想写气候与城市")
	atomID := mustUUID(id)

	before, err := q.GetWriting(ctx, atomID)
	if err != nil {
		t.Fatalf("GetWriting before: %v", err)
	}
	if before.Stage != "ideate" {
		t.Fatalf("precondition: stage = %q, want \"ideate\"", before.Stage)
	}

	if _, err := pool.Exec(ctx, `
		CREATE FUNCTION mi_test_block_stage_trace() RETURNS trigger AS $$
		BEGIN RAISE EXCEPTION 'mi_test: atom_message blocked'; END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER mi_test_block_stage_trace
			BEFORE INSERT ON atom_message
			FOR EACH ROW EXECUTE FUNCTION mi_test_block_stage_trace();
	`); err != nil {
		t.Fatalf("install blocking trigger: %v", err)
	}
	defer func() {
		// Deadline + logged-not-fatal: see writings_rollback_test.go. A
		// transaction left open by the regression under test still holds its
		// locks, and DROP TRIGGER would then block forever precisely when this
		// test has just caught a real bug.
		cleanupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanupCtx, `
			DROP TRIGGER IF EXISTS mi_test_block_stage_trace ON atom_message;
			DROP FUNCTION IF EXISTS mi_test_block_stage_trace();
		`); err != nil {
			t.Logf("cleanup: could not drop the blocking trigger (harmless, container is discarded): %v", err)
		}
	}()

	// The skip that must not become invisible: ideate → snippets, past 大纲.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("POST", "/api/v1/writings/"+id+"/stage",
			strings.NewReader(`{"stage":"snippets"}`)), cookie))

	if rec.Code == http.StatusOK {
		t.Fatalf("stage change returned 200 even though its trace insert failed; body=%s", rec.Body)
	}

	// THE assertion: the stage must not have moved without its trace.
	after, err := q.GetWriting(ctx, atomID)
	if err != nil {
		t.Fatalf("GetWriting after: %v", err)
	}
	if after.Stage != before.Stage {
		t.Fatalf("stage moved %q → %q while its trace insert failed — the two writes are NOT atomic, so a skip can be recorded in the stage but lost from the transcript",
			before.Stage, after.Stage)
	}

	// And no transaction left open — the leak-shaped half of the same bug.
	var open int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_stat_activity
		WHERE datname = current_database()
		  AND state IN ('idle in transaction', 'idle in transaction (aborted)')
	`).Scan(&open); err != nil {
		t.Fatalf("count open transactions: %v", err)
	}
	if open != 0 {
		t.Fatalf("failed stage change left %d transaction(s) open — the deferred Rollback did not run", open)
	}
}
