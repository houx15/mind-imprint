package api

import (
	"encoding/json"
	"testing"
	"time"

	"mindimprint/api/internal/awakening"
	"mindimprint/api/internal/store/sqlc"
)

// awakening_internal_test.go —— 两条在接口走查里真的坏过的不变量。
//
// 两条都是**读代码看不出对错**的那种：一个缺失的 json 标签在 review 里看不见，
// 两处各算各的「下一个节点」在单独读任何一处时都是对的。

/* ── 下一个节点只能有一个答案 ───────────────────────────────────────────── */

func turn(node int, seq int32, text string) sqlc.AwakeningTurn {
	return sqlc.AwakeningTurn{Seq: seq, NodeIndex: int32(node), StudentText: text}
}

const thin = "不知道"
const thick = "我每次看到进度条往前走就想再做一局，昨天为了这个多打了两小时"

func TestNextNodeIndexAdvancesOnARealAnswer(t *testing.T) {
	got := nextNodeIndex([]sqlc.AwakeningTurn{turn(0, 0, thick)})
	if got != 1 {
		t.Errorf("答得实在就该往下走：nextNodeIndex = %d, want 1", got)
	}
}

func TestNextNodeIndexStaysOnceForAThinAnswer(t *testing.T) {
	got := nextNodeIndex([]sqlc.AwakeningTurn{turn(0, 0, thin)})
	if got != 0 {
		t.Errorf("答得太薄该在同一个节点换个问法：nextNodeIndex = %d, want 0", got)
	}
}

// 🚨 这一条守着那个 400。
//
// 同一个节点连着两轮都薄时，第一版里 handler 的响应说「还在这个节点」，
// 而下一个请求重算出来是「已经问完了」，于是回 400 terminal_done。
// 现在两边共用这一个函数，所以只要它自己说得通，两边就一致。
func TestNextNodeIndexNeverAsksTheSameNodeThreeTimes(t *testing.T) {
	turns := []sqlc.AwakeningTurn{turn(3, 0, thin), turn(3, 1, thin)}
	got := nextNodeIndex(turns)
	if got != 4 {
		t.Errorf("一个节点最多问两轮：nextNodeIndex = %d, want 4", got)
	}
}

// 八个节点全部答得很薄，也必须走得完 —— 一个走不完的终端等于她白花了十五分钟。
func TestAThinStudentStillReachesTheEnd(t *testing.T) {
	var turns []sqlc.AwakeningTurn
	var seq int32
	for i := 0; i < 100; i++ {
		node := nextNodeIndex(turns)
		if node >= awakening.NodeCount {
			// 每个节点两轮，八个节点 = 十六轮。
			if want := awakening.NodeCount * maxTurnsPerNode; len(turns) != want {
				t.Errorf("走完用了 %d 轮, want %d", len(turns), want)
			}
			return
		}
		turns = append(turns, turn(node, seq, thin))
		seq++
	}
	t.Fatalf("一百轮之后还没走完，八问卡住了")
}

func TestNextNodeIndexStartsAtZero(t *testing.T) {
	if got := nextNodeIndex(nil); got != 0 {
		t.Errorf("一轮都还没有时该从第一个节点开始，得到 %d", got)
	}
}

/* ── 报告发出去的字段名 ─────────────────────────────────────────────────── */

// 🚨 2026-09-19 的接口走查抓到的：awakening.Planted 当时没有 json 标签，
// 于是报告里每个词序列化成 `Zh` / `Evidence`，前端读 `zh` / `evidence` 全是
// undefined —— 报告上每张卡都是空的。
//
// 树上的词是对的，日志里没有任何报错，Go 的单元测试也全绿。**只有把结构真的
// 序列化一次再按前端读的那个名字取，才看得见这件事。**
func TestReportSerialisesWithTheNamesTheClientReads(t *testing.T) {
	rep := awakening.Report{
		Version:   awakening.ReportVersion,
		AttemptNo: 2,
		Pursuing: []awakening.Planted{{
			InterestID: "ocean", Zh: "海洋", En: "Ocean", Field: "science",
			Note: "一句话", Evidence: "她的原话", Verdict: awakening.VerdictConfirm,
			Strength: 3,
		}},
		Drivers:     []awakening.Driver{{Label: "进度看得见", Evidence: "原话", Confidence: 0.8}},
		Question:    "为什么？",
		WorkConcept: "一张图解",
		Talent:      []awakening.TalentPile{{Key: "energy", Label: "有能量", Cards: []string{"logic"}}},
		Readings:    []awakening.ReadingPick{{Slug: "s", Title: "T", ZhTitle: "中文", Field: "science", Tier: 2, Why: []string{"ecology"}}},
		OpenFields:  []string{"arts"},
		Summary:     "一句总结",
		Diff:        &awakening.Diff{Stronger: []string{"海洋"}, New: nil, PreviousQuestion: "上次的问题", DaysBetween: 47},
		Answers:     []string{"她的原话"},
	}

	raw, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("反序列化失败：%v", err)
	}

	// 顶层：前端逐字段读这些名字（apps/lite-web/src/api/awakening.ts）。
	for _, k := range []string{
		"version", "attemptNo", "navigator", "pursuing", "drivers", "question",
		"workConcept", "talent", "readings", "openFields", "summary", "diff", "answers",
		"selectionFailed",
	} {
		if _, ok := got[k]; !ok {
			t.Errorf("报告里少了 %q：%s", k, raw)
		}
	}

	// 这一层就是当时坏掉的地方。
	pursuing, _ := got["pursuing"].([]any)
	if len(pursuing) != 1 {
		t.Fatalf("pursuing 长度 = %d", len(pursuing))
	}
	w, _ := pursuing[0].(map[string]any)
	for k, want := range map[string]any{
		"interestId": "ocean",
		"zh":         "海洋",
		"en":         "Ocean",
		"field":      "science",
		"note":       "一句话",
		"evidence":   "她的原话",
		"verdict":    "confirm",
	} {
		if w[k] != want {
			t.Errorf("pursuing[0].%s = %v, want %v", k, w[k], want)
		}
	}
	if w["strength"] != float64(3) {
		t.Errorf("pursuing[0].strength = %v, want 3", w["strength"])
	}

	// 驱动力、推荐、三堆、和上次比 —— 同一条理由，一起守住。
	d, _ := got["drivers"].([]any)[0].(map[string]any)
	for _, k := range []string{"label", "evidence", "confidence"} {
		if _, ok := d[k]; !ok {
			t.Errorf("drivers[0] 少了 %q", k)
		}
	}
	r, _ := got["readings"].([]any)[0].(map[string]any)
	for _, k := range []string{"slug", "title", "zhTitle", "field", "tier", "why"} {
		if _, ok := r[k]; !ok {
			t.Errorf("readings[0] 少了 %q", k)
		}
	}
	p, _ := got["talent"].([]any)[0].(map[string]any)
	for _, k := range []string{"key", "label", "cards"} {
		if _, ok := p[k]; !ok {
			t.Errorf("talent[0] 少了 %q", k)
		}
	}
	df, _ := got["diff"].(map[string]any)
	for _, k := range []string{"stronger", "new", "previousQuestion", "daysBetween"} {
		if _, ok := df[k]; !ok {
			t.Errorf("diff 少了 %q", k)
		}
	}
}

/* ── 重复总结不该白花两次生成 ───────────────────────────────────────────── */

// 一条线索可以总结不止一次（她总结完又想到新的东西，接着答两问再总结）。
// 挡住「连点两下」的就是这一条判据：上一份报告之后有没有新的回答。
//
// 🚨 判**严格之后**。和报告同一时刻的那几轮正是被那份报告总结掉的，把它们
// 算成新材料，等于每次点开都重新生成一份一模一样的报告。
func TestHasTurnAfterOnlyCountsWhatCameLater(t *testing.T) {
	at := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	older := sqlc.AwakeningTurn{CreatedAt: at.Add(-time.Minute)}
	same := sqlc.AwakeningTurn{CreatedAt: at}
	newer := sqlc.AwakeningTurn{CreatedAt: at.Add(time.Minute)}

	cases := []struct {
		name  string
		turns []sqlc.AwakeningTurn
		want  bool
	}{
		{"没有轮次", nil, false},
		{"都在报告之前", []sqlc.AwakeningTurn{older, older}, false},
		{"和报告同一时刻", []sqlc.AwakeningTurn{older, same}, false},
		{"之后又答了一轮", []sqlc.AwakeningTurn{older, same, newer}, true},
	}
	for _, c := range cases {
		if got := hasTurnAfter(c.turns, at); got != c.want {
			t.Errorf("%s：hasTurnAfter = %v, want %v", c.name, got, c.want)
		}
	}
}

/* ── 回到一条答了一半的线索，问的是她停在的那一问 ───────────────────────── */

// 🚨 线索库让「回到一条答了一半的线索」成了常走的路，而这一格原来恒给开场
// 那一问：一个已经答到第 2 问的学生，回来看见的是开场白，等于被往回推了一问。
//
// 判据和 postAwakeningTurn 那一处必须是同一条 —— 「接着问」和「刚答完一轮」
// 看到的应当是同一句话。
func TestResumeAskFollowsWhereSheStopped(t *testing.T) {
	brief := awakening.BuildBrief(nil, 1, 0)

	if got := resumeAsk(nil, brief); got != brief.OpeningAsk() {
		t.Errorf("一轮都没答时应当是开场那一问，得到 %q", got)
	}

	answered := []sqlc.AwakeningTurn{turn(0, 0, "最近老是刷到潮汐发电的视频，看了四十分钟")}
	want := awakening.NodeAt(1).Ask
	if got := resumeAsk(answered, brief); got != want {
		t.Errorf("答过第一问之后应当问第二问，得到 %q", got)
	}
	if resumeAsk(answered, brief) == brief.OpeningAsk() {
		t.Error("回到半途的线索时又把开场那一问摆了出来")
	}

	// 八问答满：这时终端显示的是「完成探询」，没有下一问。
	var full []sqlc.AwakeningTurn
	for i := 0; i < awakening.NodeCount; i++ {
		full = append(full, turn(i, int32(i), "她在这一问上写下的一段回答"))
	}
	if got := resumeAsk(full, brief); got != "" {
		t.Errorf("八问答完之后不该再有下一问，得到 %q", got)
	}
}
