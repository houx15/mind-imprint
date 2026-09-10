package api

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// 「这一步卡住了」的判据。见 reading_coach_repeat.go 的头注：判的是**这一步在
// 她身上待了几轮**，不是措辞像不像 —— 走查里那七句重复的是意图不是词，
// 文本相似度只有 0.05–0.67，想抓全它们就得把阈值压到跟正常推进只差一倍多的
// 位置，那是拿十个样本拟合一条线。

var t0 = time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

func stepAt(pos int32, status string, done time.Time) sqlc.ReadingTask {
	row := sqlc.ReadingTask{ID: uuid.New(), Position: pos, Kind: "read", Status: status}
	if !done.IsZero() {
		row.CompletedAt = pgtype.Timestamptz{Time: done, Valid: true}
	}
	return row
}

func saidAt(role string, min int) sqlc.AtomMessage {
	return sqlc.AtomMessage{Role: role, Content: "…", CreatedAt: t0.Add(time.Duration(min) * time.Minute)}
}

func TestCoachStuckAfterThreeTurnsOnOneStep(t *testing.T) {
	// 第一步还开着，印记 已经说了三句 —— 走查里那一串就是这样开始的。
	tasks := []sqlc.ReadingTask{stepAt(0, "pending", time.Time{}), stepAt(1, "pending", time.Time{})}
	msgs := []sqlc.AtomMessage{
		saidAt("ai", 1), saidAt("student", 2),
		saidAt("ai", 3), saidAt("student", 4),
		saidAt("ai", 5),
	}
	if !coachStepStuck(tasks, msgs) {
		t.Fatal("同一步说到第三句，应该判卡住")
	}
}

func TestCoachNotStuckOnTheFirstTwoTurns(t *testing.T) {
	// 第一句领这一步，第二句换个说法再说一遍 —— prompt 明确允许，而且经常管用。
	tasks := []sqlc.ReadingTask{stepAt(0, "pending", time.Time{})}
	msgs := []sqlc.AtomMessage{saidAt("ai", 1), saidAt("student", 2), saidAt("ai", 3)}
	if coachStepStuck(tasks, msgs) {
		t.Fatal("才说两句，不该判卡住")
	}
}

func TestCoachStuckCountsOnlySinceThisStepBecameCurrent(t *testing.T) {
	// 🚨 这是这个判据存在的理由。前面几步聊了很多轮，那些一句都不该算到当前
	// 这一步头上 —— 按整段转写去数，一篇文章读到一半之后每一轮都会被判成卡住，
	// 那条提示就变成了常驻的，而常驻的提示会抢掉这一轮真正该做的事。
	done := t0.Add(10 * time.Minute)
	tasks := []sqlc.ReadingTask{
		stepAt(0, "done", done),
		stepAt(1, "pending", time.Time{}),
	}
	msgs := []sqlc.AtomMessage{
		saidAt("ai", 1), saidAt("ai", 3), saidAt("ai", 5), saidAt("ai", 7), saidAt("ai", 9),
		// 上一步是第 10 分钟落定的，之后只说过一句。
		saidAt("ai", 11),
	}
	if coachStepStuck(tasks, msgs) {
		t.Fatalf("前面几步的对话被算进来了 —— 当前这一步只说过一句")
	}
	// 再说两句就该判了。
	msgs = append(msgs, saidAt("ai", 13), saidAt("ai", 15))
	if !coachStepStuck(tasks, msgs) {
		t.Fatal("当前这一步说到第三句，应该判卡住")
	}
}

func TestCoachStuckIgnoresHerOwnMessages(t *testing.T) {
	// 她连着说五句，是她的自由。卡住说的是**印记**在原地打转。
	tasks := []sqlc.ReadingTask{stepAt(0, "pending", time.Time{})}
	msgs := []sqlc.AtomMessage{
		saidAt("student", 1), saidAt("student", 2), saidAt("student", 3),
		saidAt("student", 4), saidAt("ai", 5),
	}
	if coachStepStuck(tasks, msgs) {
		t.Fatal("只有一条 ai 的话，不该判卡住")
	}
}

func TestCoachNotStuckWhenEverythingIsDone(t *testing.T) {
	// 全部走完之后没有「当前这一步」，也就没有卡住这回事。
	done := t0.Add(5 * time.Minute)
	tasks := []sqlc.ReadingTask{stepAt(0, "done", done), stepAt(1, "skipped", done)}
	msgs := []sqlc.AtomMessage{saidAt("ai", 6), saidAt("ai", 7), saidAt("ai", 8)}
	if coachStepStuck(tasks, msgs) {
		t.Fatal("没有当前步骤，不该判卡住")
	}
	if coachStepStuck(nil, msgs) {
		t.Fatal("一步都没有，不该判卡住")
	}
}
