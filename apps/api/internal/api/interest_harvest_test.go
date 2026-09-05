package api_test

// interest_harvest_test.go — 扫尾任务「捞哪些 atom」这条 SQL 的守卫。
//
// 采集要花一次旗舰调用，所以这条查询选错了对象，代价是真金白银：把没读完的
// 文章算成完成，就会对一篇读了三段就关掉的东西发调用，还在她树上长出一个她没
// 想过的词；把已经采过的算成没采过，就会对同一篇反复付钱。
//
// 2026-09-04 之后它还多管两件事：**每学生配额**（一个读了二十篇的学生不能把
// 一轮占满，否则其他人全被饿死）和**活跃窗口**（不给早就不来的账号花钱）。
// 这三类错在代码里都长得很正常，只有跑一遍才看得出来 —— 正是 AGENTS.md 说的
// 「读代码看不出对错」的那一类。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

// sweep 跑扫尾那条查询。三个上限都放得很松，好让「什么算完成」那几条测试
// 只测它们自己那一件事；配额与窗口有各自的测试把上限收紧。
func sweep(t *testing.T, q *sqlc.Queries) map[uuid.UUID]string {
	t.Helper()
	return sweepWith(t, q, 30, 50, 500)
}

func sweepWith(t *testing.T, q *sqlc.Queries, windowDays, perUser, total int32) map[uuid.UUID]string {
	t.Helper()
	rows, err := q.ListPendingHarvestAtoms(context.Background(),
		sqlc.ListPendingHarvestAtomsParams{
			WindowDays: windowDays, PerUser: perUser, Total: total,
		})
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	out := map[uuid.UUID]string{}
	for _, r := range rows {
		out[r.ID] = r.Kind
	}
	return out
}

// countFor 数这一轮里属于某个学生的有几个。
func countFor(t *testing.T, q *sqlc.Queries, pool *pgxpool.Pool, user uuid.UUID,
	windowDays, perUser, total int32) int {
	t.Helper()
	got := sweepWith(t, q, windowDays, perUser, total)
	n := 0
	for id := range got {
		var owner uuid.UUID
		if err := pool.QueryRow(t.Context(), `SELECT user_id FROM atom WHERE id = $1`, id).Scan(&owner); err != nil {
			t.Fatalf("owner: %v", err)
		}
		if owner == user {
			n++
		}
	}
	return n
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

	got := sweep(t, q)

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

	got := sweep(t, q)
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

	if _, in := sweep(t, q)[id]; !in {
		t.Fatal("刚完成的阅读不在待采集里")
	}
	if err := q.MarkAtomInterestHarvested(context.Background(), id); err != nil {
		t.Fatalf("stamp: %v", err)
	}
	if _, in := sweep(t, q)[id]; in {
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

// 整轮的总量上限 —— 它就是「一轮最多花多少次调用」。
func TestHarvestSweep_RespectsTheTotalCap(t *testing.T) {
	_, _, q, pool := liteHandler(t)
	for i := 0; i < 5; i++ {
		mkReading(t, pool, "finished")
	}
	if n := len(sweepWith(t, q, 30, 50, 3)); n != 3 {
		t.Fatalf("total 3 返回了 %d 条", n)
	}
}

// 🚨 每学生配额。没有它，一个一口气读了二十篇的学生会把整轮占满，其他所有人
// 都要等下一轮 —— 而「下一轮」对一个刚读完一篇、正打开树的学生来说太晚了。
func TestHarvestSweep_CapsEachStudentPerRound(t *testing.T) {
	_, _, q, pool := liteHandler(t)
	for i := 0; i < 6; i++ {
		mkReading(t, pool, "finished")
	}
	if n := countFor(t, q, pool, SeedUserID, 30, 2, 500); n != 2 {
		t.Fatalf("per_user 2，这个学生却拿到了 %d 条", n)
	}
}

// 配额是**按学生**分的，不是全局的：一个人读得多，不该让另一个人一条都排不上。
func TestHarvestSweep_LeavesRoomForOtherStudents(t *testing.T) {
	_, _, q, pool := liteHandler(t)
	for i := 0; i < 6; i++ {
		mkReading(t, pool, "finished")
	}
	other := mkReadingFor(t, pool, otherUserID(t, pool), "finished")

	got := sweepWith(t, q, 30, 2, 500)
	if _, in := got[other]; !in {
		t.Fatal("另一个学生唯一那篇被挤掉了 —— 配额没有按学生分")
	}
}

// 🚨 活跃窗口。没有它，一次上线会把历史上每一个完成过任何东西的账号都跑一遍，
// 钱花在早就不来的人身上。
func TestHarvestSweep_SkipsStudentsWhoStoppedComingBack(t *testing.T) {
	_, _, q, pool := liteHandler(t)
	old := mkReading(t, pool, "finished")
	if _, err := pool.Exec(t.Context(),
		`UPDATE atom SET last_activity_at = now() - interval '90 days' WHERE id = $1`, old); err != nil {
		t.Fatalf("age the atom: %v", err)
	}
	if _, in := sweep(t, q)[old]; in {
		t.Fatal("九十天没动过的 atom 仍然被捞出来采集")
	}
	// 但窗口放宽之后它该回来 —— 挡它的是窗口，不是别的什么。
	if _, in := sweepWith(t, q, 365, 50, 500)[old]; !in {
		t.Fatal("把窗口放到一年，它还是没回来 —— 挡住它的不是窗口")
	}
}

/* ── helpers ────────────────────────────────────────────────────────────── */

func otherUserID(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(t.Context(),
		`SELECT id FROM users WHERE id <> $1 LIMIT 1`, SeedUserID).Scan(&id); err != nil {
		t.Fatalf("other user: %v", err)
	}
	return id
}

func mkReadingFor(t *testing.T, pool *pgxpool.Pool, user uuid.UUID, status string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO atom (kind, user_id) VALUES ('reading', $1) RETURNING id`, user).Scan(&id); err != nil {
		t.Fatalf("insert atom: %v", err)
	}
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO reading (atom_id, title, lang, status) VALUES ($1, '别人的阅读', 'zh', $2)`,
		id, status); err != nil {
		t.Fatalf("insert reading: %v", err)
	}
	return id
}

// 🚨 队列没起来时，完成一篇阅读仍然必须成功。
//
// 这不是防御性测试：`Deps.River` 在**每一个 internal/api 测试里都是 nil**，而且
// 生产上 river 启动失败时 cmd/api 是「log 之后继续 serve」的（队列可降级，接口
// 不可）。所以「nil client」是一条真的走得到的分支，而它一旦 panic，学生看到的
// 是点「完成这篇」之后整个请求挂掉 —— 她做完的事没被记下来。
func TestFinishReading_WorksWithNoQueueAttached(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	id := mkReading(t, pool, "active")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest(
		"POST", "/api/v1/readings/"+id.String()+"/finish", strings.NewReader("{}")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("队列为 nil 时完成阅读 = %d，want 200；body=%s", rec.Code, rec.Body)
	}

	var status string
	if err := pool.QueryRow(t.Context(),
		`SELECT status FROM reading WHERE atom_id = $1`, id).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if status != "finished" {
		t.Fatalf("status = %q，want finished —— 入队失败不该回滚她的完成动作", status)
	}
}
