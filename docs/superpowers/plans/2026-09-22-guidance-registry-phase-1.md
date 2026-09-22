# 教学内容注册表（一期）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把今天分散在五处的「这一篇该用哪份教学内容」收敛成一条有测试的匹配规则，教学正文一个字都不改。

**Architecture:** 新增 `apps/api/internal/guidance`。核心是一条**泛型匹配规则**（`Scope.Matches` + `Pick[T]`）：一行登记声明它在什么情况下适用，最具体的那一行胜出。文本槽（`Registry.Resolve`）是它上面的一层薄壳。五处调用点改为调用它，各自的正文与数据原样传入。

**Tech Stack:** Go 1.26（`~/sdk/go1.26.0/bin/go`），module `mindimprint/api`，标准库 + 泛型，无新依赖。

**Spec:** `docs/superpowers/specs/2026-09-22-teaching-guidance-registry-design.md`

## Global Constraints

- **一期不改任何一个字的教学正文。** 搬家就是搬家。措辞要改另开一次提交，单独跑 `LIVE_LLM`（AGENTS.md「提示词怎么写」第 7 条）。
- **每一个任务结束前跑一次 parity**：`CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./cmd/promptinspect`。wire request 的 sha256 必须和基线一致。红了就是把正文搬坏了。
- **取不到内容必须返回 error，绝不返回空串。** 空串在线上的样子是「印记话变少了」，没有人会去查。
- `internal/api` 的测试要 `CGO_ENABLED=0` 且 `-timeout 1800s`。
- 发给模型的字里只有指令；理由、日期、谁提的意见写在 Go 注释里（AGENTS.md 第 2 条）。
- 本期**不动数据库**。`Key.Stage` 存在但恒为 `""`；`classes.stage` 的迁移属于二期。
- 本期**不改 `writingGenreOf` 的判法**（关键词表照旧）。改成模型判属于二期。

## File Structure

| 文件 | 职责 |
|---|---|
| `apps/api/internal/guidance/guidance.go` | `Key`、`Scope`、`Row[T]`、`StageBand`。只有类型和匹配规则，不认识任何一段教学内容 |
| `apps/api/internal/guidance/pick.go` | `Pick[T]` —— 最具体的一行胜出 |
| `apps/api/internal/guidance/text.go` | `Slot`、`Registry`、`Resolve` —— 文本槽那一层壳 |
| `apps/api/internal/guidance/registry.go` | 默认注册表：哪个槽在什么情况下用哪段正文。引 `internal/prompts` 的常量 |
| `apps/api/internal/guidance/guidance_test.go` | 匹配规则的边界 |
| `apps/api/internal/guidance/text_test.go` | 文本槽：最具体的赢、取不到要报错 |
| `apps/api/internal/guidance/coverage_test.go` | `TestEveryCombinationResolves` |
| `apps/api/internal/prompts/api_reading_genre.go` | **新增**：`reading_genre.go` 里那三段带读说明的正文搬到这里 |
| `apps/api/internal/prompts/catalog.go` | 加三行，把上面三个常量登记进 `Catalog()` |
| `apps/api/internal/api/guidance_axes_test.go` | **新增**：钉住 vocab 的轴与闭表一致 |
| `apps/api/internal/vocab/vocab.go` | 只加一段注释，说明它为什么是过滤器、不走 `Pick` |
| `apps/api/internal/api/writing_plan_internal_test.go` | 加一条：拼完的成品里不许残留 `@@` |
| `apps/api/internal/api/writing_plan_prompt.go` | `writingPlanSystemFor` 改为走 `Resolve` |
| `apps/api/internal/api/reading_genre.go` | `buildGenreCoachSection` 改为走 `Resolve`；正文移出 |
| `apps/api/internal/api/reading_routines.go` | `pickRoutineForGenre` 改为走 `Pick[readingRoutine]` |
| `apps/api/internal/api/writing_symptoms.go` | `writingSymptomTable` 改为走 `Pick[[]writingSymptom]` |

---

### Task 1: 匹配规则

**Files:**
- Create: `apps/api/internal/guidance/guidance.go`
- Create: `apps/api/internal/guidance/pick.go`
- Test: `apps/api/internal/guidance/guidance_test.go`

**Interfaces:**
- Consumes: 无（这是第一个任务）
- Produces: `guidance.Key{Surface,Lang,Genre,Stage string}`；`guidance.Scope{Surface,Lang string; Genres,Stages []string}`；`guidance.Row[T]{Scope Scope; Value T}`；`func Pick[T any](k Key, rows []Row[T]) (T, bool)`；`func StageBand(stage string) string`；常量 `SurfaceRead/SurfaceWrite/SurfaceComment`

- [ ] **Step 1: 写下会红的测试**

`apps/api/internal/guidance/guidance_test.go`：

```go
package guidance

import "testing"

func TestStageBandFoldsGradeToBand(t *testing.T) {
	for grade, want := range map[string]string{
		"junior1": "junior", "junior2": "junior", "junior3": "junior",
		"senior1": "senior", "senior2": "senior", "senior3": "senior",
		"": "", "primary4": "",
	} {
		if got := StageBand(grade); got != want {
			t.Errorf("StageBand(%q) = %q, 想要 %q", grade, got, want)
		}
	}
}

// 🚨 这一条是整个注册表的核心：越具体的那一行越该赢。
// 年级逐字命中 > 只命中学段 > 不限学段。
func TestPickPrefersTheMostSpecificRow(t *testing.T) {
	rows := []Row[string]{
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh"}, Value: "不限学段"},
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh", Stages: []string{"junior"}}, Value: "整个初中"},
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh", Stages: []string{"junior2"}}, Value: "初二"},
	}
	k := Key{Surface: SurfaceWrite, Lang: "zh", Stage: "junior2"}
	if got, ok := Pick(k, rows); !ok || got != "初二" {
		t.Fatalf("初二那一行该赢，拿到 %q ok=%v", got, ok)
	}
	k.Stage = "junior3"
	if got, ok := Pick(k, rows); !ok || got != "整个初中" {
		t.Fatalf("初三没有自己那一行，该退到整个初中，拿到 %q ok=%v", got, ok)
	}
	k.Stage = "senior1"
	if got, ok := Pick(k, rows); !ok || got != "不限学段" {
		t.Fatalf("高一两行都不服务，该退到不限学段，拿到 %q ok=%v", got, ok)
	}
}

func TestPickGenreBeatsNoGenre(t *testing.T) {
	rows := []Row[string]{
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh"}, Value: "任何文体"},
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh", Genres: []string{"narrative"}}, Value: "记叙文"},
	}
	got, ok := Pick(Key{Surface: SurfaceWrite, Lang: "zh", Genre: "narrative"}, rows)
	if !ok || got != "记叙文" {
		t.Fatalf("记叙文那一行该赢，拿到 %q ok=%v", got, ok)
	}
	got, ok = Pick(Key{Surface: SurfaceWrite, Lang: "zh", Genre: "argument"}, rows)
	if !ok || got != "任何文体" {
		t.Fatalf("议论文没有专门的行，该退到任何文体，拿到 %q ok=%v", got, ok)
	}
}

// 面和语言对不上就是不匹配 —— 阅读的内容绝不能漏到写作那一侧去。
func TestPickNeverCrossesSurfaceOrLang(t *testing.T) {
	rows := []Row[string]{
		{Scope: Scope{Surface: SurfaceRead, Lang: "zh"}, Value: "阅读的"},
	}
	if _, ok := Pick(Key{Surface: SurfaceWrite, Lang: "zh"}, rows); ok {
		t.Error("写作面取到了阅读面的内容")
	}
	if _, ok := Pick(Key{Surface: SurfaceRead, Lang: "en"}, rows); ok {
		t.Error("英文取到了中文的内容")
	}
}

// 一样具体时先登记的赢，而且这件事要是稳定的：注册表的顺序是人排的，
// 排在前面就是更该用的那一条。
func TestPickIsStableOnTies(t *testing.T) {
	rows := []Row[string]{
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh"}, Value: "先登记的"},
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh"}, Value: "后登记的"},
	}
	if got, _ := Pick(Key{Surface: SurfaceWrite, Lang: "zh"}, rows); got != "先登记的" {
		t.Errorf("平局该是先登记的赢，拿到 %q", got)
	}
}
```

- [ ] **Step 2: 跑一次，确认它红**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./internal/guidance/
```

预期：`undefined: StageBand`、`undefined: Pick` 等编译错误。

- [ ] **Step 3: 写实现**

`apps/api/internal/guidance/guidance.go`：

```go
// Package guidance 回答一件事：这一次该用哪份教学内容。
//
// # 为什么要有它
//
// 2026-09-22 之前，这个问题由五处各自回答（读法表、带读说明、写作立题的
// switch、vocab 三轴、毛病表三个切片），各有各的兜底、各有各的缺口。
// 这里放的是**唯一那条匹配规则**，五处共用。
//
// 规则本身不认识任何一段教学内容 —— 内容由调用方登记进来，所以阅读的
// readingRoutine 和写作的毛病表可以用同一条规则，不必都变成字符串。
package guidance

// Key 是「这一次是什么情况」。
type Key struct {
	Surface string // SurfaceRead / SurfaceWrite / SurfaceComment
	Lang    string // "zh" | "en"
	Genre   string // argument | narrative | report | explain
	Stage   string // "" 不限 | junior1..3 | senior1..3
}

const (
	SurfaceRead    = "read"
	SurfaceWrite   = "write"
	SurfaceComment = "comment"
)

// Scope 是一行登记：它在什么情况下适用。
// Genres / Stages 为空表示不限；Surface / Lang 为空同理。
type Scope struct {
	Surface string
	Lang    string
	Genres  []string
	Stages  []string // 可以写年级（junior2），也可以写学段（junior）
}

// Row 是注册表里的一行。T 是这一行携带的东西 —— 一段正文、一套读法、
// 一张毛病表，都行。
type Row[T any] struct {
	Scope Scope
	Value T
}

// StageBand 把年级折成学段。
//
// 「整个初中通用」的内容因此只写一份，登记成 Stages: []string{"junior"}，
// 初一初二初三都取得到。
func StageBand(stage string) string {
	switch stage {
	case "junior1", "junior2", "junior3":
		return "junior"
	case "senior1", "senior2", "senior3":
		return "senior"
	}
	return ""
}

// Matches —— 这一行服务这一次吗。
func (s Scope) Matches(k Key) bool {
	if s.Surface != "" && s.Surface != k.Surface {
		return false
	}
	if s.Lang != "" && s.Lang != k.Lang {
		return false
	}
	if len(s.Genres) > 0 && !contains(s.Genres, k.Genre) {
		return false
	}
	if len(s.Stages) > 0 &&
		!contains(s.Stages, k.Stage) && !contains(s.Stages, StageBand(k.Stage)) {
		return false
	}
	return true
}

// specificity —— 这一行有多具体。大的赢。
//
// 权重拉开到 2 的幂，是为了让「轴的条数」永远压过「同一条轴上更细」：
// 定了文体的一行，一定比只在学段上更细的一行更该用。
func (s Scope) specificity(k Key) int {
	n := 0
	if s.Surface != "" {
		n += 16
	}
	if s.Lang != "" {
		n += 8
	}
	if len(s.Genres) > 0 {
		n += 4
	}
	switch {
	case len(s.Stages) == 0:
		// 不限学段，不加分。
	case contains(s.Stages, k.Stage):
		n += 2 // 年级逐字命中
	default:
		n += 1 // 只命中学段
	}
	return n
}

func contains(xs []string, s string) bool {
	if s == "" {
		return false
	}
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
```

`apps/api/internal/guidance/pick.go`：

```go
package guidance

// Pick 返回服务这一次的、最具体的那一行。
//
// 一样具体时**先登记的赢**：注册表的顺序是人排的，排在前面就是更该用的
// 那一条。所以这里用 > 而不是 >=。
func Pick[T any](k Key, rows []Row[T]) (T, bool) {
	var best T
	bestScore := -1
	for _, r := range rows {
		if !r.Scope.Matches(k) {
			continue
		}
		if s := r.Scope.specificity(k); s > bestScore {
			best, bestScore = r.Value, s
		}
	}
	return best, bestScore >= 0
}
```

- [ ] **Step 4: 跑测试，确认它绿**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./internal/guidance/
```

预期：`ok mindimprint/api/internal/guidance`

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/guidance/guidance.go apps/api/internal/guidance/pick.go apps/api/internal/guidance/guidance_test.go
git commit -m "feat(guidance): 一条匹配规则 —— 最具体的那一行胜出"
```

---

### Task 2: 文本槽与写作立题

**Files:**
- Create: `apps/api/internal/guidance/text.go`
- Create: `apps/api/internal/guidance/registry.go`
- Modify: `apps/api/internal/api/writing_plan_prompt.go`（`writingPlanSystemFor`，文件第 46–73 行）
- Test: `apps/api/internal/guidance/text_test.go`

**Interfaces:**
- Consumes: `guidance.Key`、`guidance.Scope`、`guidance.Row[T]`、`guidance.Pick[T]`（Task 1）
- Produces: `type Slot string`；常量 `SlotKinds/SlotMaterial/SlotSkeleton/SlotCoach`；`type Registry struct{...}`；`func NewRegistry() *Registry`；`func (r *Registry) Add(slot Slot, scope Scope, text string)`；`func (r *Registry) Resolve(k Key, slots ...Slot) (map[Slot]string, error)`；`func Default() *Registry`

- [ ] **Step 1: 写下会红的测试**

`apps/api/internal/guidance/text_test.go`：

```go
package guidance

import (
	"strings"
	"testing"
)

func TestResolveReturnsTheMostSpecificText(t *testing.T) {
	r := NewRegistry()
	r.Add(SlotSkeleton, Scope{Surface: SurfaceWrite, Lang: "zh"}, "中文骨架")
	r.Add(SlotSkeleton, Scope{Surface: SurfaceWrite, Lang: "en"}, "英文骨架")

	got, err := r.Resolve(Key{Surface: SurfaceWrite, Lang: "en"}, SlotSkeleton)
	if err != nil {
		t.Fatalf("取不到：%v", err)
	}
	if got[SlotSkeleton] != "英文骨架" {
		t.Errorf("拿到 %q，想要英文骨架", got[SlotSkeleton])
	}
}

// 🚨 这一条是这个包存在的理由之一。2026-09-22「按语言挑」那次的教训：
// 把中文那几条挡住之后英文那边空了 —— 少给一整块，而线上看起来只是
// 印记话变少了，没有人会去查。取不到必须炸，不许给空串。
func TestResolveErrorsRatherThanReturningEmpty(t *testing.T) {
	r := NewRegistry()
	r.Add(SlotSkeleton, Scope{Surface: SurfaceWrite, Lang: "zh"}, "中文骨架")

	_, err := r.Resolve(Key{Surface: SurfaceWrite, Lang: "en"}, SlotSkeleton)
	if err == nil {
		t.Fatal("英文没有登记，Resolve 必须报错")
	}
	// 报错要说清是哪个槽、哪一次 —— 半夜看日志的人得能直接定位。
	if !strings.Contains(err.Error(), string(SlotSkeleton)) {
		t.Errorf("报错里没有槽名：%v", err)
	}
	if !strings.Contains(err.Error(), "en") {
		t.Errorf("报错里没有 Key 的内容：%v", err)
	}
}

func TestResolveTakesSeveralSlotsAtOnce(t *testing.T) {
	r := NewRegistry()
	r.Add(SlotKinds, Scope{Surface: SurfaceWrite}, "节点类型")
	r.Add(SlotMaterial, Scope{Surface: SurfaceWrite}, "材料")

	got, err := r.Resolve(Key{Surface: SurfaceWrite, Lang: "zh"}, SlotKinds, SlotMaterial)
	if err != nil {
		t.Fatalf("取不到：%v", err)
	}
	if len(got) != 2 {
		t.Errorf("要两个槽，拿到 %d 个", len(got))
	}
}

// 默认注册表里，写作立题那四种组合都要取得齐。
func TestDefaultRegistryCoversWritingPlan(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		for _, genre := range []string{"argument", "narrative"} {
			k := Key{Surface: SurfaceWrite, Lang: lang, Genre: genre}
			got, err := Default().Resolve(k, SlotKinds, SlotMaterial, SlotSkeleton)
			if err != nil {
				t.Errorf("%s/%s 取不齐：%v", lang, genre, err)
				continue
			}
			for slot, text := range got {
				if strings.TrimSpace(text) == "" {
					t.Errorf("%s/%s 的 %s 是空的", lang, genre, slot)
				}
			}
		}
	}
}
```

- [ ] **Step 2: 跑一次，确认它红**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./internal/guidance/
```

预期：`undefined: NewRegistry`、`undefined: SlotSkeleton` 等。

- [ ] **Step 3: 写实现**

`apps/api/internal/guidance/text.go`：

```go
package guidance

import "fmt"

// Slot 是提示词模板上的一个洞。
type Slot string

const (
	SlotKinds    Slot = "kinds"    // 节点类型闭表
	SlotMaterial Slot = "material" // 材料怎么选
	SlotSkeleton Slot = "skeleton" // 常见文章结构
	SlotCoach    Slot = "coach"    // 这一篇怎么带
)

// Registry 是「哪个槽在什么情况下用哪段正文」。
type Registry struct {
	slots map[Slot][]Row[string]
}

func NewRegistry() *Registry {
	return &Registry{slots: map[Slot][]Row[string]{}}
}

// Add 登记一行。同一个槽登记多行时，先登记的在平局时胜出（见 Pick）。
func (r *Registry) Add(slot Slot, scope Scope, text string) {
	r.slots[slot] = append(r.slots[slot], Row[string]{Scope: scope, Value: text})
}

// Resolve 取这一次要用的每个槽。
//
// 🚨 取不到返回 error，不返回空串。见 TestResolveErrorsRatherThanReturningEmpty
// 上面那段注释：空串在线上的样子是「印记话变少了」。
func (r *Registry) Resolve(k Key, slots ...Slot) (map[Slot]string, error) {
	out := make(map[Slot]string, len(slots))
	for _, slot := range slots {
		text, ok := Pick(k, r.slots[slot])
		if !ok || text == "" {
			return nil, fmt.Errorf(
				"guidance: 槽 %q 在 surface=%q lang=%q genre=%q stage=%q 上没有内容",
				slot, k.Surface, k.Lang, k.Genre, k.Stage)
		}
		out[slot] = text
	}
	return out, nil
}
```

`apps/api/internal/guidance/registry.go`：

```go
package guidance

import (
	"sync"

	"mindimprint/api/internal/prompts"
)

// Default 是生产在用的那一份注册表。
//
// 🚨 这里只登记「哪段正文在什么情况下用」，**不改任何一个字的正文**。
// 正文仍然住在 internal/prompts，这里引的是它的常量。
var Default = sync.OnceValue(func() *Registry {
	r := NewRegistry()

	// ── 写作立题 ────────────────────────────────────────────────────────
	// 这四行是 writingPlanSystemFor 原来那个 switch 的逐条搬家。
	// 中文两种文体各一份节点类型表，英文两种各一份。
	w := func(lang, genre string) Scope {
		s := Scope{Surface: SurfaceWrite, Lang: lang}
		if genre != "" {
			s.Genres = []string{genre}
		}
		return s
	}
	r.Add(SlotKinds, w("zh", ""), prompts.WritingPlanArgumentKinds)
	r.Add(SlotKinds, w("zh", "narrative"), prompts.WritingPlanNarrativeKinds)
	r.Add(SlotKinds, w("en", ""), prompts.WritingPlanEnglishArgumentKinds)
	r.Add(SlotKinds, w("en", "narrative"), prompts.WritingPlanEnglishNarrativeKinds)

	r.Add(SlotMaterial, w("zh", ""), prompts.WritingPlanMaterialZH)
	r.Add(SlotMaterial, w("en", ""), prompts.WritingPlanMaterialEN)

	r.Add(SlotSkeleton, w("zh", ""), prompts.WritingPlanSkeletonZH)
	r.Add(SlotSkeleton, w("en", ""), prompts.WritingPlanSkeletonEN)

	return r
})
```

> **实现者注意：** 这四个常量今天**已经**在 `internal/prompts/api_writing_plan.go`
> 和 `api_writing_plan_lang.go` 里导出了，`internal/api` 侧只是
> `const writingPlanArgumentKinds = prompts.WritingPlanArgumentKinds` 这样的别名。
> 所以这一步不需要搬任何正文，直接引就行。那几个别名先留着 —— 下面那段兜底
> 还要用它们。

然后改 `apps/api/internal/api/writing_plan_prompt.go` 的 `writingPlanSystemFor`：

```go
// writingPlanSystemFor 按文体和语言拼出立题那一轮的 system prompt。
//
// 2026-09-22：选哪一段由 internal/guidance 决定，这里只负责把取回来的几段
// 填进模板。正文一个字没动。
func writingPlanSystemFor(genre string, lang string) string {
	k := guidance.Key{Surface: guidance.SurfaceWrite, Lang: lang, Genre: genre}
	parts, err := guidance.Default().Resolve(k,
		guidance.SlotKinds, guidance.SlotMaterial, guidance.SlotSkeleton)
	if err != nil {
		// 🚨 退到中文议论文那一套，而不是发一份带着 @@KINDS@@ 的提示词出去。
		// 登记漏了是我们的 bug，但她那一轮仍然要有一个能用的老师。
		slog.Error("writing plan guidance missing, falling back", "err", err,
			"lang", lang, "genre", genre)
		parts = map[guidance.Slot]string{
			guidance.SlotKinds:    writingPlanArgumentKinds,
			guidance.SlotMaterial: writingPlanMaterialZH,
			guidance.SlotSkeleton: writingPlanSkeletonZH,
		}
	}

	s := strings.Replace(writingPlanSystem, "@@KINDS@@", parts[guidance.SlotKinds], 1)
	s = strings.Replace(s, "@@MATERIAL@@", parts[guidance.SlotMaterial], 1)
	s = strings.Replace(s, "@@SKELETON@@", parts[guidance.SlotSkeleton], 1)
	s = strings.Replace(s, "%d", strconv.Itoa(writingPlanMaxNewNodes), 1)
	if lang == langEnglish {
		s += "\n这篇是英文写作。讨论图中已有内容时，请使用与该节点对应的英文术语并解释其作用，例如 topic sentence 或 commentary。节点 text 必须使用英文，对话 reply 用中文。计划检查中的中文标签只是计数名称，不覆盖这些教学术语。\n"
	}
	return s
}
```

在该文件的 import 块里加 `"log/slog"` 和 `"mindimprint/api/internal/guidance"`。

- [ ] **Step 4: 跑测试与 parity**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./internal/guidance/
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./cmd/promptinspect
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/ -run 'TestWritingPlanSystemFor|TestWritingPlan'
```

预期：三条全绿。**parity 红了就是正文被改动了** —— 回去逐字比对，不要改基线。

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/guidance/ apps/api/internal/api/writing_plan_prompt.go apps/api/internal/prompts/
git commit -m "refactor(guidance): 写作立题的四种组合改走注册表，正文一字未动"
```

---

### Task 3: 阅读带读说明搬进 prompts

**Files:**
- Create: `apps/api/internal/prompts/api_reading_genre.go`
- Modify: `apps/api/internal/api/reading_genre.go`（`genreCoachGuide` 那个 map，文件末段）
- Modify: `apps/api/internal/guidance/registry.go`（登记 `SlotCoach` 三行）
- Test: `apps/api/internal/api/reading_genre_test.go`（已存在，加一条）

**Interfaces:**
- Consumes: `guidance.Default()`、`guidance.SlotCoach`、`guidance.SurfaceRead`（Task 2）
- Produces: `prompts.ReadingCoachGenreReport`、`prompts.ReadingCoachGenreExplain`、`prompts.ReadingCoachGenreNarrative`

- [ ] **Step 1: 写下会红的测试**

加进 `apps/api/internal/api/reading_genre_test.go`：

```go
// 🚨 议论文那条路一个字节都不变 —— 这是 2026-09-17 定的边界
// （「don't bother the current experience of argument papers」）。
// 认不出来的体裁同理，按议论文办。
func TestGenreCoachSectionStaysEmptyForArgument(t *testing.T) {
	for _, genre := range []string{genreArgument, "", "不认识的体裁"} {
		if got := buildGenreCoachSection(genre); got != "" {
			t.Errorf("体裁 %q 不该有带读说明，拿到 %d 字", genre, len([]rune(got)))
		}
	}
}

// 另外三种体裁各自要拿到自己那一段，而且必须提到自己那块板的格子名。
func TestGenreCoachSectionNamesItsOwnBins(t *testing.T) {
	for genre, bin := range map[string]string{
		genreReport:    "引述",
		genreExplain:   "说明对象",
		genreNarrative: "心理描写",
	} {
		got := buildGenreCoachSection(genre)
		if got == "" {
			t.Errorf("体裁 %q 没有带读说明", genre)
			continue
		}
		if !strings.Contains(got, bin) {
			t.Errorf("体裁 %q 的带读说明里没有格子名 %q", genre, bin)
		}
	}
}
```

- [ ] **Step 2: 跑一次，确认新加的两条红或绿**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/ -run TestGenreCoachSection
```

预期：**这两条现在就该是绿的**（行为还没变）。它们是搬家的安全网 —— 先确认它们绿，搬完再确认还绿。绿不了说明测试本身写错了，先修测试。

- [ ] **Step 3: 把正文搬走**

新建 `apps/api/internal/prompts/api_reading_genre.go`：

```go
package prompts

// Purpose: api/reading_genre.go 的三段带读说明。
// Consumer: internal/api/reading_genre.go 经 internal/guidance 取用。
// Comments are maintenance metadata and are never sent to a model.

// ReadingCoachGenreReport 是新闻报道那一段带读说明。
// 内容来自同事 2026-09-17 阅读模块 PRD 的「四类文章的工作流与交互」表。
const ReadingCoachGenreReport = `（把 reading_genre.go 里 genreCoachGuide[genreReport] 的字符串字面量原样搬到这里）`

// ReadingCoachGenreExplain 是说明文那一段。
const ReadingCoachGenreExplain = `（同上，搬 genreCoachGuide[genreExplain]）`

// ReadingCoachGenreNarrative 是记叙文那一段。
const ReadingCoachGenreNarrative = `（同上，搬 genreCoachGuide[genreNarrative]）`
```

> **实现者注意：** 上面三处括号是**搬运指令**，不是要写的字。打开
> `internal/api/reading_genre.go`，把 `genreCoachGuide` 里三个反引号字符串
> **逐字节**复制过来（含换行），然后删掉原 map。
>
> 搬完之后按下面的字数对一遍 —— 这三个数是 2026-09-22 在
> `ec52b32b` 上量的，不对就是搬漏了或多带了空白：
>
> | 常量 | 字符数 | 字节数 | 行数 |
> |---|---|---|---|
> | `ReadingCoachGenreReport` | 307 | 847 | 8 |
> | `ReadingCoachGenreExplain` | 278 | 750 | 8 |
> | `ReadingCoachGenreNarrative` | 319 | 869 | 9 |

还要把这三个常量登记进 `internal/prompts/catalog.go` 的 `Catalog()`
（`TestCatalogAndAssemblySourcesExist` 会检查 `Source` / `Consumer` 两个文件
真的存在、`Text` 不为空）：

```go
		{ID: "api.readingCoachGenreReport", Source: "internal/prompts/api_reading_genre.go", Consumer: "internal/api/reading_genre.go", Text: ReadingCoachGenreReport},
		{ID: "api.readingCoachGenreExplain", Source: "internal/prompts/api_reading_genre.go", Consumer: "internal/api/reading_genre.go", Text: ReadingCoachGenreExplain},
		{ID: "api.readingCoachGenreNarrative", Source: "internal/prompts/api_reading_genre.go", Consumer: "internal/api/reading_genre.go", Text: ReadingCoachGenreNarrative},
```

在 `registry.go` 的 `Default` 里加：

```go
	// ── 阅读带读说明 ────────────────────────────────────────────────────
	// 🚨 议论文没有这一节，所以议论文不登记 —— Resolve 取不到就是取不到，
	// 由调用方 buildGenreCoachSection 把「没有」翻译成空字符串。
	rd := func(genre string) Scope {
		return Scope{Surface: SurfaceRead, Genres: []string{genre}}
	}
	r.Add(SlotCoach, rd("report"), prompts.ReadingCoachGenreReport)
	r.Add(SlotCoach, rd("explain"), prompts.ReadingCoachGenreExplain)
	r.Add(SlotCoach, rd("narrative"), prompts.ReadingCoachGenreNarrative)
```

改 `reading_genre.go` 里取正文那一句。原来是 `guide, ok := genreCoachGuide[genre]`，改成：

```go
	parts, err := guidance.Default().Resolve(
		guidance.Key{Surface: guidance.SurfaceRead, Genre: genre}, guidance.SlotCoach)
	if err != nil {
		// 议论文和认不出来的体裁没有这一节 —— 那是对的，不是故障。
		return ""
	}
	guide := parts[guidance.SlotCoach]
```

`buildGenreCoachSection` 其余部分（标注板那一句、排序板那一段）不动。

- [ ] **Step 4: 跑测试与 parity**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/ -run 'TestGenreCoachSection|TestGenreBoard|TestFitBoard'
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./cmd/promptinspect
```

预期：全绿。步骤 2 那两条测试现在守的是搬完之后行为没变。

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/prompts/api_reading_genre.go apps/api/internal/api/reading_genre.go apps/api/internal/api/reading_genre_test.go apps/api/internal/guidance/registry.go
git commit -m "refactor(guidance): 阅读带读说明搬进 internal/prompts，按 AGENTS.md 该住那儿"
```

---

### Task 4: 读法表改走 Pick

**Files:**
- Modify: `apps/api/internal/api/reading_routines.go`（`serves` 与 `pickRoutineForGenre`，第 166–190 行附近）
- Test: `apps/api/internal/api/reading_routines_internal_test.go`（已存在，加一条）

**Interfaces:**
- Consumes: `guidance.Pick[T]`、`guidance.Row[T]`、`guidance.Scope`、`guidance.Key`（Task 1）
- Produces: 无新导出；`pickRoutineForGenre(chosen readingRoutine, genre string) readingRoutine` 签名不变

- [ ] **Step 1: 写下会红的测试**

加进 `apps/api/internal/api/reading_routines_internal_test.go`：

```go
// 模型挑了一套不服务这个体裁的读法，服务端要换成服务它的那一套，
// 而且必须留在同一种语言里 —— 英文文章换成中文读法，她会看到一份
// 读不懂的清单。
func TestPickRoutineForGenreStaysInTheSameLanguage(t *testing.T) {
	var english readingRoutine
	for _, r := range readingRoutines {
		if r.Lang == "en" && r.serves(genreArgument) {
			english = r
			break
		}
	}
	if english.Key == "" {
		t.Fatal("库里没有服务英文议论文的读法，这条测试的前提不成立")
	}
	got := pickRoutineForGenre(english, genreNarrative)
	if got.Lang != "en" {
		t.Errorf("换成了 %q 语言的读法（key=%s），该留在 en", got.Lang, got.Key)
	}
	if !got.serves(genreNarrative) {
		t.Errorf("换来的读法 %s 不服务记叙文", got.Key)
	}
}

// 模型挑对了就不要动它。
func TestPickRoutineForGenreKeepsAGoodChoice(t *testing.T) {
	for _, r := range readingRoutines {
		if !r.serves(genreReport) {
			continue
		}
		if got := pickRoutineForGenre(r, genreReport); got.Key != r.Key {
			t.Errorf("挑对了还被换掉：%s → %s", r.Key, got.Key)
		}
	}
}

// 体裁为空（老数据、模型漏填）时原样保留 —— 那就是今天的样子。
func TestPickRoutineForGenreKeepsChoiceWhenGenreUnknown(t *testing.T) {
	r := readingRoutines[0]
	if got := pickRoutineForGenre(r, ""); got.Key != r.Key {
		t.Errorf("体裁未知时不该换：%s → %s", r.Key, got.Key)
	}
}
```

- [ ] **Step 2: 跑一次**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/ -run TestPickRoutineForGenre
```

预期：**现在就该绿**（行为还没变）。这三条是改写前的安全网。

- [ ] **Step 3: 改成走 Pick**

把 `pickRoutineForGenre` 改成：

```go
// pickRoutineForGenre 把模型挑的读法和它判的体裁对齐。
//
// 2026-09-22：挑哪一套由 internal/guidance 那条共用规则决定。行为不变 ——
// 挑对了不动，挑错了换成同语言里服务这个体裁的第一套。
func pickRoutineForGenre(chosen readingRoutine, genre string) readingRoutine {
	genre = validateGenre(genre)
	if genre == "" || chosen.serves(genre) {
		return chosen
	}
	rows := make([]guidance.Row[readingRoutine], 0, len(readingRoutines))
	for _, r := range readingRoutines {
		if r.Lang != chosen.Lang {
			continue
		}
		rows = append(rows, guidance.Row[readingRoutine]{
			Scope: guidance.Scope{
				Surface: guidance.SurfaceRead,
				Lang:    r.Lang,
				Genres:  r.Genres,
			},
			Value: r,
		})
	}
	k := guidance.Key{Surface: guidance.SurfaceRead, Lang: chosen.Lang, Genre: genre}
	if fitted, ok := guidance.Pick(k, rows); ok {
		return fitted
	}
	// 同语言里没有服务这个体裁的 —— 保留模型挑的那一套，
	// 给她一份读不对的清单，也好过给她一份读不懂的。
	return chosen
}
```

- [ ] **Step 4: 跑测试**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/ -run 'TestPickRoutineForGenre|TestReadingRoutine'
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./cmd/promptinspect
```

预期：全绿。

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/api/reading_routines.go apps/api/internal/api/reading_routines_internal_test.go
git commit -m "refactor(guidance): 读法表改走同一条匹配规则"
```

---

### Task 5: 毛病表改走 Pick

**Files:**
- Modify: `apps/api/internal/api/writing_symptoms.go`（`writingSymptomTable`，第 214–224 行）
- Test: `apps/api/internal/api/writing_symptoms_test.go`（若不存在则新建）

**Interfaces:**
- Consumes: `guidance.Pick[T]`、`guidance.Row[T]`、`guidance.Scope`、`guidance.Key`（Task 1）
- Produces: 无新导出；`writingSymptomTable(lang, genre string) []writingSymptom` 签名不变

- [ ] **Step 1: 写下会红的测试**

```go
// 中文记叙拿到的是「通用 + 记叙」两张表接起来，不是只有记叙那张。
func TestWritingSymptomTableNarrativeZHIsTheUnion(t *testing.T) {
	got := writingSymptomTable("zh", genreNarrative)
	if len(got) <= len(writingSymptomsZH) {
		t.Fatalf("记叙表 %d 条，该多于通用表的 %d 条", len(got), len(writingSymptomsZH))
	}
	has := func(id string) bool {
		for _, s := range got {
			if s.ID == id {
				return true
			}
		}
		return false
	}
	if !has(writingSymptomsZH[0].ID) {
		t.Error("记叙表里少了通用表的条目")
	}
	if !has(writingSymptomsNarrativeZH[0].ID) {
		t.Error("记叙表里少了记叙专有的条目")
	}
}

func TestWritingSymptomTableZHArgumentIsTheGeneralTable(t *testing.T) {
	if got := writingSymptomTable("zh", genreArgument); len(got) != len(writingSymptomsZH) {
		t.Errorf("中文议论该是通用表 %d 条，拿到 %d 条", len(writingSymptomsZH), len(got))
	}
}

// 🚨 英文今天**不分文体**：两种体裁拿到的是同一张表。
// 这是一个已知缺口（spec 四期补英文记叙那张表），不是这一次要改的行为。
// 钉住它，免得搬家时悄悄变了样。
func TestWritingSymptomTableENIgnoresGenreForNow(t *testing.T) {
	arg := writingSymptomTable("en", genreArgument)
	nar := writingSymptomTable("en", genreNarrative)
	if len(arg) != len(nar) || len(arg) != len(writingSymptomsEN) {
		t.Errorf("英文两种体裁今天该是同一张表：议论 %d、记叙 %d、表 %d",
			len(arg), len(nar), len(writingSymptomsEN))
	}
}
```

- [ ] **Step 2: 跑一次**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/ -run TestWritingSymptomTable
```

预期：**现在就该绿**。这三条是改写前的安全网。

- [ ] **Step 3: 改成走 Pick**

```go
// writingSymptomTable —— 这一次摆给模型看的是哪张毛病表。
//
// 2026-09-22：挑哪一张由 internal/guidance 那条共用规则决定。行为不变。
//
// 🚨 英文今天只有一张表，不分文体。补英文记叙那一张属于四期，
// 不在这一次的范围里（TestWritingSymptomTableENIgnoresGenreForNow 钉着）。
func writingSymptomTable(lang string, genre string) []writingSymptom {
	narrativeZH := make([]writingSymptom, 0, len(writingSymptomsZH)+len(writingSymptomsNarrativeZH))
	narrativeZH = append(narrativeZH, writingSymptomsZH...)
	narrativeZH = append(narrativeZH, writingSymptomsNarrativeZH...)

	rows := []guidance.Row[[]writingSymptom]{
		// 先登记的在平局时胜出，所以更具体的那几行要排在前面。
		{Scope: guidance.Scope{Surface: guidance.SurfaceWrite, Lang: "zh",
			Genres: []string{genreNarrative}}, Value: narrativeZH},
		{Scope: guidance.Scope{Surface: guidance.SurfaceWrite, Lang: "zh"},
			Value: writingSymptomsZH},
		{Scope: guidance.Scope{Surface: guidance.SurfaceWrite, Lang: "en"},
			Value: writingSymptomsEN},
	}
	k := guidance.Key{Surface: guidance.SurfaceWrite, Lang: lang, Genre: genre}
	if got, ok := guidance.Pick(k, rows); ok {
		return got
	}
	// 语言认不出来时按中文通用表办 —— 和 2026-09-22 之前一致。
	return writingSymptomsZH
}
```

- [ ] **Step 4: 跑测试**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/ -run 'TestWritingSymptom|TestLookupWritingSymptom|TestWritingComment'
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./cmd/promptinspect
```

预期：全绿。

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/api/writing_symptoms.go apps/api/internal/api/writing_symptoms_test.go
git commit -m "refactor(guidance): 毛病表改走同一条匹配规则"
```

---

### Task 6: vocab 不搬，但要钉住它不漂

**Files:**
- Create: `apps/api/internal/api/guidance_axes_test.go`
- Modify: `apps/api/internal/vocab/vocab.go`（只加一段注释，不改逻辑）

**Interfaces:**
- Consumes: `guidance.Key`（Task 1）、`vocab.ForLang`、`vocab.Structures`
- Produces: 无新导出

**为什么这一个不搬。** spec §1 数了五处选取，前四处这一期都搬到了同一条规则上。
第五处 `vocab.ForLang` / `vocab.Structures` **是过滤器，不是选择器** —— 它们返回
**一组**方法（一页卡片上摆四条论证结构），而 `Pick` 返回**一条**。硬塞进 `Pick`
会把「全都要」变成「只要最具体的那一条」，那是行为改变，不是搬家。

真正的风险不是它没用同一个函数，是**两边的词表会各自漂**：`guidance` 认
`argument/narrative/report/explain`，`vocab` 的 `genre` 字段是 methods.json 里
写的字。这一条测试钉住它们一致。

- [ ] **Step 1: 写下会红的测试**

`apps/api/internal/api/guidance_axes_test.go`：

```go
package api

import (
	"testing"

	"mindimprint/api/internal/vocab"
)

// 🚨 vocab 是过滤器不是选择器，所以它不走 guidance.Pick（见 plan Task 6）。
// 但两边的轴必须说同一套词 —— 一边改了另一边没改，症状是某个组合悄悄
// 少给一整块内容，而线上看起来只是印记话变少了。
func TestVocabAxesAgreeWithGuidanceClosedSets(t *testing.T) {
	// 写作面今天用得上的两个体裁，vocab 里都要真的有方法。
	for _, genre := range []string{genreArgument, genreNarrative} {
		for _, lang := range []string{"zh", "en"} {
			if got := vocab.ForLang(lang, genre); len(got) == 0 {
				t.Errorf("vocab.ForLang(%q,%q) 一条方法都没有", lang, genre)
			}
			if got := vocab.Structures(genre, lang); len(got) == 0 {
				t.Errorf("vocab.Structures(%q,%q) 一条结构都没有", genre, lang)
			}
		}
	}
}

// methods.json 里的 genre 字段只许出现闭表里的词（或空＝不限）。
// 现造一个词，就是在同一口气里教模型造词（AGENTS.md 提示词第 5 条）。
func TestVocabGenreFieldStaysInTheClosedSet(t *testing.T) {
	allowed := map[string]bool{
		"": true, genreArgument: true, genreNarrative: true,
		genreReport: true, genreExplain: true,
	}
	for _, m := range vocab.All() {
		if !allowed[m.Genre] {
			t.Errorf("方法 %s 的 genre=%q 不在闭表里", m.ID, m.Genre)
		}
	}
}
```

> **实现者注意：** 若 `internal/vocab` 还没有导出 `All()`，加一个：
> ```go
> // All 返回库里全部方法，供校验与 --print 用。返回副本，调用方改不到 loaded。
> func All() []Method { return append([]Method(nil), loaded...) }
> ```

- [ ] **Step 2: 跑一次**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/ -run TestVocab
```

预期：两条都绿。`TestVocabGenreFieldStaysInTheClosedSet` 红了说明 methods.json
里有人写了闭表外的词 —— 改数据，不要放宽测试。

- [ ] **Step 3: 在 vocab.go 里写清楚它为什么不走那条规则**

在 `ForLang` 上面加：

```go
// 🚨 ForLang / Structures 是**过滤器**，不走 internal/guidance 的 Pick。
//
// Pick 回答的是「哪一条最合适」，这两个回答的是「哪些都合适」——
// 一页上要摆四条论证结构，不是摆最具体的那一条。塞进 Pick 会把
// 「全都要」变成「只要一条」，那是行为改变，不是搬家。
//
// 两边的体裁词表由 TestVocabGenreFieldStaysInTheClosedSet 钉住一致。
```

- [ ] **Step 4: 跑测试**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/ -run TestVocab
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./internal/vocab/
```

预期：全绿。

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/api/guidance_axes_test.go apps/api/internal/vocab/vocab.go
git commit -m "test(guidance): 钉住 vocab 的轴和闭表一致 —— 它是过滤器，不搬"
```

---

### Task 7: 组合覆盖测试

**Files:**
- Create: `apps/api/internal/guidance/coverage_test.go`
- Test: 同上

**Interfaces:**
- Consumes: `guidance.Default()`、`guidance.Key`、所有 `Slot` 常量（Task 2、Task 3）
- Produces: 无（这是这一期的验收）

- [ ] **Step 1: 写测试**

```go
package guidance

import (
	"strings"
	"testing"
)

// 🚨 这条测试是做这整件事的回报。
//
// AGENTS.md「提示词怎么写」第 6 条记着那次事故：parity 基线 28 个样例，
// 写作立题只覆盖了中文议论文一种，于是 @@KINDS@@ 在没被覆盖的分支上
// 原样发到了线上，而整套测试照样绿。
//
// 抽样查不出这类毛病，只有**枚举**能。
func TestEveryCombinationResolves(t *testing.T) {
	langs := []string{"zh", "en"}
	stages := []string{"", "junior1", "junior2", "junior3", "senior1", "senior2", "senior3"}

	// 写作面：四个槽里的三个，两种文体。
	for _, lang := range langs {
		for _, genre := range []string{"argument", "narrative"} {
			for _, stage := range stages {
				k := Key{Surface: SurfaceWrite, Lang: lang, Genre: genre, Stage: stage}
				got, err := Default().Resolve(k, SlotKinds, SlotMaterial, SlotSkeleton)
				if err != nil {
					t.Errorf("%+v 取不齐：%v", k, err)
					continue
				}
				for slot, text := range got {
					if strings.TrimSpace(text) == "" {
						t.Errorf("%+v 的 %s 是空的", k, slot)
					}
					if strings.Contains(text, "@@") {
						t.Errorf("%+v 的 %s 里残留着占位符", k, slot)
					}
				}
			}
		}
	}
}
```

「拼完的成品里有没有残留占位符」那一条**不住在这里** —— `internal/guidance`
不能 import `internal/api`（会成环）。它在 Step 3 里加到 `internal/api` 那一侧。

- [ ] **Step 2: 跑一次，确认枚举那条绿**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./internal/guidance/ -run TestEveryCombinationResolves -v
```

预期：绿。红了说明 Task 2 的登记有漏 —— 补登记，不要改测试。

- [ ] **Step 3: 把「成品里没有占位符」那条放到 internal/api**

删掉上面那个 `t.Skip` 的壳，在 `apps/api/internal/api/writing_plan_internal_test.go` 里加：

```go
// 🚨 拼完的成品里不许残留 @@。模板上多开一个洞而忘了登记，
// 只有这一条查得出来 —— guidance 那边查的是槽的内容，不是成品。
func TestWritingPlanSystemForLeavesNoPlaceholder(t *testing.T) {
	for _, lang := range []string{"zh", langEnglish} {
		for _, genre := range []string{genreArgument, genreNarrative} {
			s := writingPlanSystemFor(genre, lang)
			if strings.Contains(s, "@@") {
				t.Errorf("%s/%s 的提示词里残留着占位符", lang, genre)
			}
			if strings.Contains(s, "%d") {
				t.Errorf("%s/%s 的提示词里残留着 %%d", lang, genre)
			}
			if len([]rune(s)) < 500 {
				t.Errorf("%s/%s 的提示词只有 %d 字，像是少拼了几段",
					lang, genre, len([]rune(s)))
			}
		}
	}
}
```

- [ ] **Step 4: 跑全套**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./internal/guidance/
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./cmd/promptinspect
```

预期：三条全绿。

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/guidance/coverage_test.go apps/api/internal/api/writing_plan_internal_test.go
git commit -m "test(guidance): 枚举所有组合 —— 抽样查不出 @@KINDS@@ 那类事故"
```

---

## 这一期做完之后

- 学生看不到任何变化。这是它对的样子 —— 一期只搬家。
- 线上走查照跑一遍（`npx playwright test -c e2e/online.config.ts`），确认写作与阅读两条链子的行为没变。
- 二期（英文写作 + `classes.stage` 迁移 0186）、三期（句式与例句）、四期（批改）各自另写一份 plan。
