package api

import (
	"encoding/json"
	"testing"

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
