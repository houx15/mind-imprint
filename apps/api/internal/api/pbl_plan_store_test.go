package api_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// newPblAtom returns a project atom for the seeded student.
func newPblAtom(t *testing.T, q *sqlc.Queries) uuid.UUID {
	t.Helper()
	at, err := q.CreateAtom(t.Context(), sqlc.CreateAtomParams{Kind: "project", UserID: SeedUserID})
	if err != nil {
		t.Fatalf("CreateAtom: %v", err)
	}
	if _, err := q.CreatePblProject(t.Context(), sqlc.CreatePblProjectParams{
		AtomID: at.ID, Idea: "剩饭去哪了", Kind: "investigation",
	}); err != nil {
		t.Fatalf("CreatePblProject: %v", err)
	}
	return at.ID
}

// Sessions nest, and the depth cap is a CHECK rather than a query.
func TestPblSession_NestsAndCapsAtThree(t *testing.T) {
	_, _, q, _ := liteHandler(t)
	atomID := newPblAtom(t, q)

	mk := func(parent *uuid.UUID, depth int16) (sqlc.PblSession, error) {
		var p pgtype.UUID
		if parent != nil {
			p = pgtype.UUID{Bytes: *parent, Valid: true}
		}
		return q.CreatePblSession(t.Context(), sqlc.CreatePblSessionParams{
			AtomID: atomID, Kind: "free", ParentID: p, Depth: depth,
			AnchorKind: "hook", AnchorRef: "", Question: "为什么剩的都是米饭？",
		})
	}

	lvl0, err := mk(nil, 0)
	if err != nil {
		t.Fatalf("depth 0: %v", err)
	}
	lvl1, err := mk(&lvl0.ID, 1)
	if err != nil {
		t.Fatalf("depth 1: %v", err)
	}
	if _, err := mk(&lvl1.ID, 2); err != nil {
		t.Fatalf("depth 2: %v", err)
	}
	// The fourth level is where thinking stops being nesting and starts being
	// lost. The CHECK is what makes the cap real rather than advisory.
	if _, err := mk(&lvl1.ID, 3); err == nil {
		t.Fatal("expected the depth CHECK to reject a 4th level")
	}
}

// A message belongs to the main thread or to one session, never to both, and
// the two readings must not see each other's rows.
func TestPblSession_ThreadAndMainThreadAreSeparate(t *testing.T) {
	_, _, q, _ := liteHandler(t)
	atomID := newPblAtom(t, q)

	s, err := q.CreatePblSession(t.Context(), sqlc.CreatePblSessionParams{
		AtomID: atomID, Kind: "free", Depth: 0, AnchorKind: "free",
	})
	if err != nil {
		t.Fatalf("CreatePblSession: %v", err)
	}

	appended := func(sessionID pgtype.UUID, body string) {
		t.Helper()
		next, err := q.NextAtomMessageSeq(t.Context(), atomID)
		if err != nil {
			t.Fatalf("NextAtomMessageSeq: %v", err)
		}
		if _, err := q.AppendPblSessionMessage(t.Context(), sqlc.AppendPblSessionMessageParams{
			AtomID: atomID, Seq: next, Role: "student", Content: body, SessionID: sessionID,
		}); err != nil {
			t.Fatalf("append %q: %v", body, err)
		}
	}

	// Interleaved on purpose: the main thread and the session take turns, so a
	// filtered read has gaps in its seq. Gaps are fine; disorder is not.
	appended(pgtype.UUID{}, "主线一")
	appended(pgtype.UUID{Bytes: s.ID, Valid: true}, "支线一")
	appended(pgtype.UUID{}, "主线二")
	appended(pgtype.UUID{Bytes: s.ID, Valid: true}, "支线二")

	main, err := q.ListPblMainThread(t.Context(), atomID)
	if err != nil {
		t.Fatalf("ListPblMainThread: %v", err)
	}
	if len(main) != 2 || main[0].Content != "主线一" || main[1].Content != "主线二" {
		t.Fatalf("main thread = %v, want the two 主线 rows in order", pblMsgContents(main))
	}

	side, err := q.ListPblSessionMessages(t.Context(), sqlc.ListPblSessionMessagesParams{
		AtomID: atomID, SessionID: pgtype.UUID{Bytes: s.ID, Valid: true},
	})
	if err != nil {
		t.Fatalf("ListPblSessionMessages: %v", err)
	}
	if len(side) != 2 || side[0].Content != "支线一" || side[1].Content != "支线二" {
		t.Fatalf("session thread = %v, want the two 支线 rows in order", pblMsgContents(side))
	}
}

func pblMsgContents(rows []sqlc.AtomMessage) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Content)
	}
	return out
}

// 🚨 The invariant this whole slice exists to hold: staging a structural change
// must leave the live plan byte-identical. If this test ever goes green while
// the live steps moved, 隐形重规划 is back.
func TestPblPendingChange_NeverTouchesTheLivePlan(t *testing.T) {
	_, _, q, _ := liteHandler(t)
	atomID := newPblAtom(t, q)

	v1, err := q.CreatePblPlanVersion(t.Context(), sqlc.CreatePblPlanVersionParams{
		AtomID: atomID, Version: 1, Summary: "第一版", Reason: "她批准的", DecidedBy: "student",
	})
	if err != nil {
		t.Fatalf("CreatePblPlanVersion: %v", err)
	}
	if _, err := q.ApprovePblPlanVersion(t.Context(), v1.ID); err != nil {
		t.Fatalf("ApprovePblPlanVersion: %v", err)
	}
	if _, err := q.CreatePblPlanStep(t.Context(), sqlc.CreatePblPlanStepParams{
		VersionID: v1.ID, Ordinal: 1, Title: "去食堂看三天",
		Decide: "你自己判断，剩得最多的是哪一类", Status: "tentative",
	}); err != nil {
		t.Fatalf("CreatePblPlanStep: %v", err)
	}

	before, err := q.ListPblPlanSteps(t.Context(), v1.ID)
	if err != nil {
		t.Fatalf("ListPblPlanSteps: %v", err)
	}

	// 印记 proposes replacing the whole approach. It goes nowhere near the plan.
	ch, err := q.StagePblPendingChange(t.Context(), sqlc.StagePblPendingChangeParams{
		AtomID: atomID, Kind: "modify",
		Diff:     []byte(`{"remove":["去食堂看三天"],"add":["先问食堂阿姨"]}`),
		Evidence: "访谈说法和我们原来的判断不一样",
	})
	if err != nil {
		t.Fatalf("StagePblPendingChange: %v", err)
	}

	after, err := q.ListPblPlanSteps(t.Context(), v1.ID)
	if err != nil {
		t.Fatalf("ListPblPlanSteps after staging: %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("live plan has %d steps after staging, had %d — a staged change reached the plan", len(after), len(before))
	}
	if after[0].Title != before[0].Title || after[0].Status != before[0].Status {
		t.Fatalf("live step changed on staging: %+v vs %+v", after[0], before[0])
	}

	live, err := q.GetPblLivePlan(t.Context(), atomID)
	if err != nil {
		t.Fatalf("GetPblLivePlan: %v", err)
	}
	if live.Version != 1 {
		t.Fatalf("live version = %d, want 1 — staging must not mint a version", live.Version)
	}

	// 「保留原计划」 is a real outcome, recorded with her reason.
	kept, err := q.ResolvePblPendingChange(t.Context(), sqlc.ResolvePblPendingChangeParams{
		ID: ch.ID, Resolution: strPtr("kept"), Reason: "访谈只有一个人，先补证据",
	})
	if err != nil {
		t.Fatalf("ResolvePblPendingChange: %v", err)
	}
	if kept.Reason == "" || !kept.ResolvedAt.Valid {
		t.Fatalf("kept resolution did not record her reason: %+v", kept)
	}
	open, err := q.ListPblOpenChanges(t.Context(), atomID)
	if err != nil {
		t.Fatalf("ListPblOpenChanges: %v", err)
	}
	if len(open) != 0 {
		t.Fatalf("%d change(s) still open after resolving", len(open))
	}
}

// An unapproved version is not the live plan — nothing runs before she approves.
func TestPblPlan_UnapprovedVersionIsNotLive(t *testing.T) {
	_, _, q, _ := liteHandler(t)
	atomID := newPblAtom(t, q)

	if _, err := q.CreatePblPlanVersion(t.Context(), sqlc.CreatePblPlanVersionParams{
		AtomID: atomID, Version: 1, Summary: "印记拟的", Reason: "初稿", DecidedBy: "ai",
	}); err != nil {
		t.Fatalf("CreatePblPlanVersion: %v", err)
	}
	if _, err := q.GetPblLivePlan(t.Context(), atomID); err == nil {
		t.Fatal("an unapproved v1 was returned as the live plan")
	}
}

func strPtr(s string) *string { return &s }
