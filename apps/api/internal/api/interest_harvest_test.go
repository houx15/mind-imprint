package api_test

// interest_harvest_test.go — 「哪些 atom 该被采集」这条 SQL 的守卫。
//
// 采集本身要花一次旗舰调用，所以这条查询选错了对象，代价是真金白银：把没读完的
// 文章算成完成，就会对一篇读了三段就关掉的东西发调用，还在她树上长出一个她没
// 想过的词；把已经采过的算成没采过，就会对同一篇反复付钱。
//
// 这两种错在代码里都长得很正常，只有跑一遍才看得出来 —— 正是 AGENTS.md 说的
// 「读代码看不出对错」的那一类。

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// mkReading 建一个属于种子学生的阅读 atom，status 由调用方给。
func mkReading(t *testing.T, pool *pgxpool.Pool, status string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO atom (kind, user_id) VALUES ('reading', $1) RETURNING id`,
		SeedUserID).Scan(&id); err != nil {
		t.Fatalf("insert atom: %v", err)
	}
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO reading (atom_id, title, lang, status) VALUES ($1, '一篇阅读', 'zh', $2)`,
		id, status); err != nil {
		t.Fatalf("insert reading: %v", err)
	}
	return id
}

func mkWriting(t *testing.T, pool *pgxpool.Pool, status string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO atom (kind, user_id) VALUES ('writing', $1) RETURNING id`,
		SeedUserID).Scan(&id); err != nil {
		t.Fatalf("insert atom: %v", err)
	}
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO writing (atom_id, title, lang, status) VALUES ($1, '一篇写作', 'zh', $2)`,
		id, status); err != nil {
		t.Fatalf("insert writing: %v", err)
	}
	return id
}

func mkProject(t *testing.T, pool *pgxpool.Pool, status string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO atom (kind, user_id) VALUES ('project', $1) RETURNING id`,
		SeedUserID).Scan(&id); err != nil {
		t.Fatalf("insert atom: %v", err)
	}
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO pbl_project (atom_id, idea, kind, status)
		 VALUES ($1, '我想做一个东西', 'research', $2)`, id, status); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	return id
}

func pending(t *testing.T, q *sqlc.Queries, limit int32) map[uuid.UUID]string {
	t.Helper()
	rows, err := q.ListUnharvestedFinishedAtoms(context.Background(),
		sqlc.ListUnharvestedFinishedAtomsParams{UserID: SeedUserID, Limit: limit})
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	out := map[uuid.UUID]string{}
	for _, r := range rows {
		out[r.ID] = r.Kind
	}
	return out
}

// 没完成的东西不该长词：一篇读了三段就关掉的文章说不出她关心什么。
func TestHarvestPending_OnlyPicksUpFinishedWork(t *testing.T) {
	_, _, q, pool := liteHandler(t)

	activeRead := mkReading(t, pool, "active")
	doneRead := mkReading(t, pool, "finished")
	activeWrite := mkWriting(t, pool, "active")
	doneWrite := mkWriting(t, pool, "finished")
	talkingProj := mkProject(t, pool, "talking")
	runningProj := mkProject(t, pool, "running")
	reviewProj := mkProject(t, pool, "review")

	got := pending(t, q, 50)

	for name, id := range map[string]uuid.UUID{
		"未完成的阅读": activeRead, "未完成的写作": activeWrite,
		"还在聊的项目": talkingProj, "还在做的项目": runningProj,
	} {
		if _, in := got[id]; in {
			t.Errorf("%s 被算成了可采集", name)
		}
	}
	for name, id := range map[string]uuid.UUID{
		"完成的阅读": doneRead, "完成的写作": doneWrite, "进入复盘的项目": reviewProj,
	} {
		if _, in := got[id]; !in {
			t.Errorf("%s 没被算成可采集", name)
		}
	}
}

// 复盘之后的两个状态也算完成 —— 她把项目收进成果或归档，都不该让它错过采集。
func TestHarvestPending_CountsProjectsPastReview(t *testing.T) {
	_, _, q, pool := liteHandler(t)
	keeping := mkProject(t, pool, "keeping")
	archived := mkProject(t, pool, "archived")

	got := pending(t, q, 50)
	if _, in := got[keeping]; !in {
		t.Error("keeping 的项目没被算成可采集")
	}
	if _, in := got[archived]; !in {
		t.Error("archived 的项目没被算成可采集")
	}
}

// 盖过章的不再回来。这是「一篇薄阅读被反复重采」那个 bug 的守卫。
func TestHarvestPending_StampedAtomsNeverComeBack(t *testing.T) {
	_, _, q, pool := liteHandler(t)
	id := mkReading(t, pool, "finished")

	if _, in := pending(t, q, 50)[id]; !in {
		t.Fatal("刚完成的阅读不在待采集里")
	}
	if err := q.MarkAtomInterestHarvested(context.Background(), id); err != nil {
		t.Fatalf("stamp: %v", err)
	}
	if _, in := pending(t, q, 50)[id]; in {
		t.Fatal("盖过章的 atom 又回到了待采集里——每次打开树都会再花一次钱")
	}
}

// 盖章是幂等的，而且不会把第一次的时间往后挪。什么时候采的是一个关于过去的事实。
func TestHarvestPending_StampIsIdempotent(t *testing.T) {
	_, _, q, pool := liteHandler(t)
	id := mkReading(t, pool, "finished")
	ctx := context.Background()

	if err := q.MarkAtomInterestHarvested(ctx, id); err != nil {
		t.Fatalf("stamp: %v", err)
	}
	first, err := q.GetAtomInterestHarvestedAt(ctx, id)
	if err != nil {
		t.Fatalf("read stamp: %v", err)
	}
	if err := q.MarkAtomInterestHarvested(ctx, id); err != nil {
		t.Fatalf("re-stamp: %v", err)
	}
	again, err := q.GetAtomInterestHarvestedAt(ctx, id)
	if err != nil {
		t.Fatalf("read stamp again: %v", err)
	}
	if !first.Valid || !again.Valid || !first.Time.Equal(again.Time) {
		t.Fatalf("重复盖章把时间挪了：%v → %v", first.Time, again.Time)
	}
}

func TestHarvestPending_RespectsTheBatchLimit(t *testing.T) {
	_, _, q, pool := liteHandler(t)
	for i := 0; i < 5; i++ {
		mkReading(t, pool, "finished")
	}
	if n := len(pending(t, q, 3)); n != 3 {
		t.Fatalf("limit 3 返回了 %d 条", n)
	}
}

// 别的学生完成的东西，不该出现在她的待采集队列里。
func TestHarvestPending_IsScopedToOneStudent(t *testing.T) {
	_, _, q, pool := liteHandler(t)
	var otherID uuid.UUID
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO atom (kind, user_id)
		 SELECT 'reading', id FROM users WHERE id <> $1 LIMIT 1 RETURNING id`,
		SeedUserID).Scan(&otherID); err != nil {
		t.Fatalf("insert other atom: %v", err)
	}
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO reading (atom_id, title, lang, status)
		 VALUES ($1, '别人的阅读', 'zh', 'finished')`, otherID); err != nil {
		t.Fatalf("insert other reading: %v", err)
	}
	if _, in := pending(t, q, 50)[otherID]; in {
		t.Fatal("别的学生完成的阅读进了她的待采集队列")
	}
}
