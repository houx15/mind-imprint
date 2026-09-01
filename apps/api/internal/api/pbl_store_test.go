package api_test

import (
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// A PBL 项目 is an atom, the same way a reading and a writing are. These two
// tests exist to catch the things that go silently wrong if 0108 drifts: an
// atom kind that no longer accepts 'project', and CHECK constraints that let a
// typo through into a column the whole frontend switches on.
func TestPblProject_RoundTrip(t *testing.T) {
	_, _, q, _ := liteHandler(t)

	at, err := q.CreateAtom(t.Context(), sqlc.CreateAtomParams{Kind: "project", UserID: SeedUserID})
	if err != nil {
		t.Fatalf("CreateAtom(kind=project): %v — 0108 must widen atom_kind_check", err)
	}
	p, err := q.CreatePblProject(t.Context(), sqlc.CreatePblProjectParams{
		AtomID: at.ID, Idea: "我想弄明白我们学校的剩饭到底去哪了", Kind: "investigation",
	})
	if err != nil {
		t.Fatalf("CreatePblProject: %v", err)
	}
	if p.Status != "talking" {
		t.Fatalf("status = %q, want \"talking\" — a new project has no plan yet", p.Status)
	}
	if p.Name != "" {
		t.Fatalf("name = %q, want empty — she names it in the modal, after it exists", p.Name)
	}

	got, err := q.GetPblProject(t.Context(), at.ID)
	if err != nil {
		t.Fatalf("GetPblProject: %v", err)
	}
	if got.UserID != SeedUserID {
		t.Fatalf("user_id = %v, want %v — the patch handler's ownership check reads this", got.UserID, SeedUserID)
	}
	if got.Idea != "我想弄明白我们学校的剩饭到底去哪了" {
		t.Fatalf("idea = %q, want it stored verbatim", got.Idea)
	}

	n, err := q.CountPblProjectsByUser(t.Context(), SeedUserID)
	if err != nil {
		t.Fatalf("CountPblProjectsByUser: %v", err)
	}
	if n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}
}

func TestPblProject_RejectsUnknownKindAndStatus(t *testing.T) {
	_, _, q, _ := liteHandler(t)

	at, err := q.CreateAtom(t.Context(), sqlc.CreateAtomParams{Kind: "project", UserID: SeedUserID})
	if err != nil {
		t.Fatalf("CreateAtom: %v", err)
	}
	if _, err := q.CreatePblProject(t.Context(), sqlc.CreatePblProjectParams{
		AtomID: at.ID, Idea: "x", Kind: "podcast",
	}); err == nil {
		t.Fatal("expected the kind CHECK to reject \"podcast\"")
	}

	// A real project, then an unknown status against it.
	ok, err := q.CreatePblProject(t.Context(), sqlc.CreatePblProjectParams{
		AtomID: at.ID, Idea: "x", Kind: "design",
	})
	if err != nil {
		t.Fatalf("CreatePblProject: %v", err)
	}
	if _, err := q.SetPblProjectStatus(t.Context(), sqlc.SetPblProjectStatusParams{
		AtomID: ok.AtomID, Status: "done",
	}); err == nil {
		t.Fatal("expected the status CHECK to reject \"done\"")
	}
}

// 只列自己的。别人的项目不该出现在她的看板上。
func TestListPblProjects_OnlyMine(t *testing.T) {
	_, _, q, pool := liteHandler(t)

	at, err := q.CreateAtom(t.Context(), sqlc.CreateAtomParams{Kind: "project", UserID: SeedUserID})
	if err != nil {
		t.Fatalf("CreateAtom: %v", err)
	}
	if _, err := q.CreatePblProject(t.Context(), sqlc.CreatePblProjectParams{
		AtomID: at.ID, Idea: "我的", Kind: "website",
	}); err != nil {
		t.Fatalf("CreatePblProject: %v", err)
	}

	otherID := createStudent(t, pool, SeedSchoolID, "other-builder@demo.local")
	oat, err := q.CreateAtom(t.Context(), sqlc.CreateAtomParams{Kind: "project", UserID: otherID})
	if err != nil {
		t.Fatalf("CreateAtom(other): %v", err)
	}
	if _, err := q.CreatePblProject(t.Context(), sqlc.CreatePblProjectParams{
		AtomID: oat.ID, Idea: "别人的", Kind: "website",
	}); err != nil {
		t.Fatalf("CreatePblProject(other): %v", err)
	}

	rows, err := q.ListPblProjectsByUser(t.Context(), SeedUserID)
	if err != nil {
		t.Fatalf("ListPblProjectsByUser: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len = %d, want 1 — another student's project leaked into her board", len(rows))
	}
	if rows[0].Idea != "我的" {
		t.Fatalf("idea = %q, want \"我的\"", rows[0].Idea)
	}
}
