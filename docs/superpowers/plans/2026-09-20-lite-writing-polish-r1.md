# 写作面 R1 实施计划 —— 图的骨架、拖得出来、引导不再被覆盖

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让思维导图有一副强制的骨架（结尾不可能被印成「分论点 3」）、节点能从
子层拖回上层、「卡住了？」不再覆盖她读过的那份引导、被禁止的请求会先说一句
说明再往下走。

**Architecture:** 给 `writing_outline` 加一列 `kind`（闭表）。模型不再返回
`parentId`，改返回 `kind`；服务端从 `kind` 算出深度与父节点，于是错误的摆放在
结构上不可表示。下游四处的关键词猜测（`slots.ts`、`writing_blocks.go`、
`writingGuideAppliesTo`、`rootInsertPosition`）改读 `kind` 并删掉猜测。拖动补上
`"after"`（兄弟）与「落在空白 = 升到最上层」两条路径，挪完按新位置改写 `kind`。

**Tech Stack:** Go 1.26（`~/sdk/go1.26.0/bin`）、pgx/sqlc v1.27.0、goose 迁移、
React + Vite + TypeScript + Tailwind、vitest、Playwright。

**Spec:** `docs/superpowers/specs/2026-09-20-lite-writing-polish-design.md`

## Global Constraints

- **工具链（这台机器上会踩的坑）：** 每个 shell 先
  `export PATH="$HOME/sdk/go1.26.0/bin:$PATH"`。`~/go/bin/sqlc` 是 x86_64
  的，macOS 27 拿走 Rosetta 之后跑不了；本仓库已下好 arm64 版在
  `./.tooling/sqlc`（`.tooling/` 在 .gitignore 里），用它，不要 `go install`
  （pg_query_go 的 strchrnul 编不出来）。
- **Go 测试：** `CGO_ENABLED=0 go test ./... -timeout 1800s`。
  🚨 不要 `go test ./... | grep | tail` —— 退出码会变成 tail 的。
- **lite 不许弄坏 pro：** `apps/api` 的包、`queries/`、`migrations/`、sqlc 产物
  是两个版本共用的。**新建一个已经存在的文件**就能毁掉 pro 的代码；动手前先
  `ls` 一眼。
- **界面文案：** 标签是名词（「论据」不是「你找的那个材料」）；按钮写「做什么／
  做完了」，用书面词；状态用「已／待／处理中」；不写文学腔（无比喻、无抒情副词、
  「长出来／长成」只留给树）。见 AGENTS.md「界面文案怎么写」。
- **只写逻辑测试：** 纯函数、reducer、normalizer、Go handler、权限校验。
  **不要**为「标题渲染了／类名存在／列表有 N 项」写断言。UI 用真浏览器看。
- **Go→TS 边界两个杀手：** 每个新字段必须有 json 标签；nil 切片 marshal 成
  `null` 会让前端 `.join()` 崩 —— 返回 `[]T{}` 不返回 nil，并写一条 marshal
  测试按前端真实读的名字取值、断言没有 null。
- **迁移号：** 下一个是 **0182**（现存最大 0181）。分支开久了会撞车，撞了 goose
  直接起不来 —— 合并前再确认一次。
- 提交信息结尾带 `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`。

## 文件结构

**新建**
- `apps/api/internal/store/migrations/0182_writing_outline_kind.sql` —— 加列 + 回填。
- `apps/api/internal/api/writing_kind.go` —— `kind` 闭表这一个概念的全部：
  常量、显示标题、深度、父的选法、老 `role` 的映射。**单一真相源**，
  下游都从这里取。
- `apps/api/internal/api/writing_kind_internal_test.go`
- `apps/lite-web/src/writings/outlineKind.ts` —— 上面那个文件的 TS 孪生。
- `apps/lite-web/src/writings/outlineKind.test.ts`
- `apps/api/internal/api/writing_refusal.go` —— 被禁请求的识别与校验。
- `apps/api/internal/api/writing_refusal_test.go`

**修改**
- `apps/api/internal/store/queries/writing_atom.sql` —— 三条语句带上 `kind`。
- `apps/api/internal/api/writing_plan.go` —— 提示词改 `kind`；`insertPlanNode`
  从 `kind` 算深度和父；删掉 `rootInsertPosition`。
- `apps/api/internal/api/writing_plan_state.go` —— `gap` 不计入材料。
- `apps/api/internal/api/writing_blocks.go` —— 改读 `kind`，删关键词表。
- `apps/api/internal/api/writing_outline.go` —— DTO 带 `kind`；PUT 收 `kind`。
- `apps/api/internal/api/writing_guide.go` —— `writingGuideAppliesTo` 改读
  `kind`；`{current, previous}`；问题规则改写。
- `apps/lite-web/src/writings/slots.ts` —— 改读 `kind`，删关键词表。
- `apps/lite-web/src/writings/useMindMapDrag.ts` —— 三区落点 + 空白落点。
- `apps/lite-web/src/writings/outlineMove.ts` —— 挪完改写 `kind`。
- `apps/lite-web/src/writings/MindMap.tsx` —— 标题来自表；落点提示。
- `apps/lite-web/src/writings/GuideBox.tsx` —— 「换一组问题」+「看上一组」。
- `apps/lite-web/src/api/writingRoom.ts` —— 类型带 `kind`。

---

### Task 1: `kind` 闭表（Go 侧的单一真相源）

**Files:**
- Create: `apps/api/internal/api/writing_kind.go`
- Test: `apps/api/internal/api/writing_kind_internal_test.go`

**Interfaces:**
- Consumes: 无（这是第一块）。
- Produces:
  - `type writingKind = string` 与常量 `writingKindOpening/Thesis/Point/Evidence/Counter/Rebuttal/Gap/Closing`
  - `func writingKindValid(k string) bool`
  - `func writingKindDepth(k string) int32`
  - `func writingKindLabel(k, source string) string`
  - `func writingKindParentOf(k string, rows []sqlc.WritingOutline) *sqlc.WritingOutline`
  - `func writingKindIsMaterial(k string) bool`
  - `func writingKindFromRole(role string, depth int32) string`
  - `func writingKindAppliesTo(k string) string`

- [ ] **Step 1: 写失败的测试**

```go
package api

import (
	"testing"

	"github.com/google/uuid"
	"mindimprint/api/internal/store/sqlc"
)

func TestWritingKindDepth(t *testing.T) {
	cases := map[string]int32{
		writingKindOpening: 0, writingKindThesis: 0, writingKindClosing: 0,
		writingKindPoint: 1, writingKindCounter: 1,
		writingKindEvidence: 2, writingKindRebuttal: 2, writingKindGap: 2,
	}
	for k, want := range cases {
		if got := writingKindDepth(k); got != want {
			t.Errorf("%s: depth = %d, want %d", k, got, want)
		}
	}
}

func TestWritingKindLabel(t *testing.T) {
	if got := writingKindLabel(writingKindEvidence, ""); got != "论据 · 你见过的事" {
		t.Errorf("evidence without source = %q", got)
	}
	if got := writingKindLabel(writingKindEvidence, "Lu 2008"); got != "论据 · 你找来的材料" {
		t.Errorf("evidence with source = %q", got)
	}
	if got := writingKindLabel(writingKindClosing, ""); got != "结尾" {
		t.Errorf("closing = %q", got)
	}
}

// 🚨 这是意见 3 的回归用例：模型把结尾挂在中心论点底下（深度 1），
// 它仍然必须是一个深度 0 的结尾，而不是「分论点 3」。
func TestWritingKindParentIgnoresModelPlacement(t *testing.T) {
	thesis := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindThesis, Depth: 0, Position: 0}
	point := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 1}
	rows := []sqlc.WritingOutline{thesis, point}

	if p := writingKindParentOf(writingKindClosing, rows); p != nil {
		t.Errorf("closing parent = %v, want nil (top level)", p.ID)
	}
	if p := writingKindParentOf(writingKindPoint, rows); p == nil || p.ID != thesis.ID {
		t.Errorf("point parent = %v, want the thesis", p)
	}
	if p := writingKindParentOf(writingKindEvidence, rows); p == nil || p.ID != point.ID {
		t.Errorf("evidence parent = %v, want the nearest point", p)
	}
	// 一条论据来了，图上还没有分论点 —— 不许挂到中心论点上冒充一条理由。
	if p := writingKindParentOf(writingKindEvidence, []sqlc.WritingOutline{thesis}); p != nil {
		t.Errorf("evidence with no point = %v, want nil", p)
	}
}

func TestWritingKindFromRoleBackfill(t *testing.T) {
	cases := []struct {
		role  string
		depth int32
		want  string
	}{
		{"结尾", 1, writingKindClosing},
		{"开头", 0, writingKindOpening},
		{"你见过的事", 2, writingKindEvidence},
		{"一份研究", 1, writingKindEvidence},
		{"反方会说的话", 1, writingKindCounter},
		{"中心论点", 0, writingKindThesis},
		{"一条理由", 1, writingKindPoint},
		{"完全不认识的东西", 0, writingKindThesis},
		{"完全不认识的东西", 1, writingKindPoint},
		{"完全不认识的东西", 2, writingKindEvidence},
	}
	for _, c := range cases {
		if got := writingKindFromRole(c.role, c.depth); got != c.want {
			t.Errorf("role=%q depth=%d: got %q want %q", c.role, c.depth, got, c.want)
		}
	}
}

func TestWritingKindGapIsNotMaterial(t *testing.T) {
	if writingKindIsMaterial(writingKindGap) {
		t.Error("gap must not count as material — it is a hole, not evidence")
	}
	if !writingKindIsMaterial(writingKindEvidence) {
		t.Error("evidence must count as material")
	}
}
```

- [ ] **Step 2: 跑它，确认失败**

```bash
export PATH="$HOME/sdk/go1.26.0/bin:$PATH"
cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestWritingKind -v
```
Expected: 编译失败，`undefined: writingKindDepth` 等。

- [ ] **Step 3: 写 `writing_kind.go`**

```go
package api

// writing_kind.go —— 图上一个节点「是什么」的闭表，以及由它决定的一切。
//
// # 为什么这一列存在（2026-09-20）
//
// 在这之前，模型自己挑 `parentId`、自己用散文写 `role`，下游再拿关键词把意思
// 猜回来。同事试用时截到的那张图里，`结尾` 挂在 `中心论点` 底下（深度 1），
// 而 slots.ts 只在 `depth === 0` 时认结尾，于是它落进最后那个 else，
// 印成 **「分论点 3」**。产品负责人的原话：「这个是总结，不是分论点」。
//
// 标题错是摆放错的下游。所以修的是摆放：**深度和父节点由 kind 算出来，
// 不采信模型给的位置**。于是那个摆法不再可表示，而不是摆错之后被纠正。
//
// 🚨 这是**单一真相源**。slots.ts 是它的 TS 孪生（outlineKind.ts），
// 两边的取值和深度必须逐条一致。下游（writing_blocks.go、writing_guide.go、
// writing_plan.go）都从这里取，不要在别处再写一张关键词表 —— 删掉的那四张
// 正是这个 bug 的来源。

import (
	"strings"

	"mindimprint/api/internal/store/sqlc"
)

// 闭表。和 apps/lite-web/src/writings/outlineKind.ts 的 OutlineKind 一致。
const (
	writingKindOpening  = "opening"  // 开篇
	writingKindThesis   = "thesis"   // 中心论点
	writingKindPoint    = "point"    // 分论点
	writingKindEvidence = "evidence" // 论据
	writingKindCounter  = "counter"  // 反方观点
	writingKindRebuttal = "rebuttal" // 对反方的回应
	writingKindGap      = "gap"      // 待补的材料
	writingKindClosing  = "closing"  // 结尾
)

var writingKindDepths = map[string]int32{
	writingKindOpening: 0, writingKindThesis: 0, writingKindClosing: 0,
	writingKindPoint: 1, writingKindCounter: 1,
	writingKindEvidence: 2, writingKindRebuttal: 2, writingKindGap: 2,
}

func writingKindValid(k string) bool {
	_, ok := writingKindDepths[k]
	return ok
}

// writingKindDepth 是这种块在图上的深度。**强制**，不采信模型给的位置。
// 不认识的 kind 当分论点处理（深度 1）—— 中间那一层是议论文里最常见的块，
// 也是错了代价最小的一层。
func writingKindDepth(k string) int32 {
	if d, ok := writingKindDepths[k]; ok {
		return d
	}
	return 1
}

// writingKindLabel 是印在她屏幕上的那个小标题。
//
// 用的是语文课上的正式词（AGENTS.md 文案规则 6：学生来这儿就是要学这套词），
// 而且是名词（规则 1）。同事的意见 2：「部分论据的标题不规范」。
//
// 论据分两种后缀，靠 source 定：非空 = 她找回来的材料，空 = 她自己见过的事。
// 这条区分承重 —— 印记 下一轮正是照着 source 去查这份材料。
func writingKindLabel(k, source string) string {
	switch k {
	case writingKindOpening:
		return "开篇"
	case writingKindThesis:
		return "中心论点"
	case writingKindPoint:
		return "分论点"
	case writingKindCounter:
		return "反方观点"
	case writingKindRebuttal:
		return "对反方的回应"
	case writingKindGap:
		return "待补的材料"
	case writingKindClosing:
		return "结尾"
	case writingKindEvidence:
		if strings.TrimSpace(source) != "" {
			return "论据 · 你找来的材料"
		}
		return "论据 · 你见过的事"
	}
	return ""
}

// writingKindParentOf 选父节点：**按 kind，不按模型给的 parentId**。
//
// 深度 0 的三种（开篇/中心论点/结尾）没有父。
// 分论点和反方观点挂在中心论点上。
// 论据和待补挂在前面最近的分论点上；对反方的回应挂在前面最近的反方观点上。
//
// 🚨 找不到该挂的那一种就返回 nil，**不退而求其次挂到中心论点上**。
// 一条没有分论点可挂的论据挂到中心论点下面，就变成了深度 1，
// 下游会把它当成一条理由 —— 这正是 2026-09-18 记下的那个毛病
//（「挂得更深的分论点被当成例子」）反过来的一面。
// 挂不上的由调用方处理（见 writing_plan.go 的 evidenceWithNoPoint）。
func writingKindParentOf(k string, rows []sqlc.WritingOutline) *sqlc.WritingOutline {
	switch k {
	case writingKindOpening, writingKindThesis, writingKindClosing:
		return nil
	case writingKindPoint, writingKindCounter:
		return lastWritingKind(rows, writingKindThesis)
	case writingKindEvidence, writingKindGap:
		return lastWritingKind(rows, writingKindPoint)
	case writingKindRebuttal:
		return lastWritingKind(rows, writingKindCounter)
	}
	return nil
}

// lastWritingKind 按 position 找最后一个该种块。
func lastWritingKind(rows []sqlc.WritingOutline, kind string) *sqlc.WritingOutline {
	var found *sqlc.WritingOutline
	for i := range rows {
		if rows[i].Kind != kind {
			continue
		}
		if found == nil || rows[i].Position > found.Position {
			found = &rows[i]
		}
	}
	return found
}

// writingKindIsMaterial：这个节点算不算「撑得住一条理由的材料」。
//
// 🚨 **gap 不算。** 截图里那个「暂时还没找到的材料」是深度 2，而
// writingPlanShapeOf 只按深度数材料，于是它被当成一条真材料计入 Material，
// 可以把 ready() 推过线 —— 她手上一条材料都没有，产品却说这份计划站得住。
func writingKindIsMaterial(k string) bool {
	return k == writingKindEvidence
}

// writingKindAppliesTo 把 kind 映射成 vocab 的三个位置桶。
// 取代 writingGuideAppliesTo 里那张对自由散文做子串匹配的关键词表。
func writingKindAppliesTo(k string) string {
	switch k {
	case writingKindOpening:
		return "opening"
	case writingKindClosing:
		return "closing"
	}
	return "body"
}

// —— 以下只服务迁移 0182 的回填，以及那之前存下的老行 ——

var (
	writingRoleWordsOpening  = []string{"开头", "引言", "开篇", "钩子", "导入", "opening", "hook", "introduction", "intro"}
	writingRoleWordsClosing  = []string{"结尾", "结论", "总结", "收尾", "落点", "结语", "closing", "conclusion", "ending"}
	writingRoleWordsCounter  = []string{"反方", "对方", "反对", "质疑", "counter", "objection"}
	writingRoleWordsRebuttal = []string{"回应", "反驳", "rebuttal", "response"}
	writingRoleWordsGap      = []string{"还没找到", "待补", "暂时没有", "缺一份"}
	writingRoleWordsEvidence = []string{
		"例", "经历", "的事", "事件", "故事", "材料", "数据", "研究", "报道", "访谈", "调查",
		"案例", "引用", "名言", "人物", "史实", "素材", "证据", "新闻", "实验", "统计", "场景", "现象",
		"example", "experience", "evidence", "data", "study", "research", "report",
		"story", "quote", "case", "survey", "statistic", "source",
	}
)

func roleContainsAny(role string, words []string) bool {
	r := strings.ToLower(role)
	for _, w := range words {
		if strings.Contains(r, w) {
			return true
		}
	}
	return false
}

// writingKindFromRole 把 0182 之前那些自由散文的 role 映射成一个 kind。
//
// 顺序有讲究：先认最具体的（待补、回应、反方），再认开篇/结尾，最后才是论据 ——
// 「反方会说的话」里含有「的事」？没有，但「对方举的例子」两条都命中，
// 而它更该是一条论据。认不出来的按深度兜底。
//
// 🚨 兜底**一行都不丢**：任何 role 都会得到一个 kind。
func writingKindFromRole(role string, depth int32) string {
	switch {
	case roleContainsAny(role, writingRoleWordsGap):
		return writingKindGap
	case roleContainsAny(role, writingRoleWordsRebuttal):
		return writingKindRebuttal
	case roleContainsAny(role, writingRoleWordsCounter):
		return writingKindCounter
	case roleContainsAny(role, writingRoleWordsOpening):
		return writingKindOpening
	case roleContainsAny(role, writingRoleWordsClosing):
		return writingKindClosing
	case roleContainsAny(role, writingRoleWordsEvidence):
		return writingKindEvidence
	}
	switch depth {
	case 0:
		return writingKindThesis
	case 1:
		return writingKindPoint
	default:
		return writingKindEvidence
	}
}
```

- [ ] **Step 4: 跑测试，确认通过**

```bash
export PATH="$HOME/sdk/go1.26.0/bin:$PATH"
cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestWritingKind -v
```
Expected: PASS（`sqlc.WritingOutline.Kind` 这一列还不存在，所以这一步会编译失败 ——
先做 Task 2 再回来跑。若按顺序执行，把本步骤推迟到 Task 2 之后。）

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/api/writing_kind.go apps/api/internal/api/writing_kind_internal_test.go
git commit -m "feat(lite-writing): kind 闭表，图上一个节点是什么由它说了算

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: 迁移 0182 + sqlc

**Files:**
- Create: `apps/api/internal/store/migrations/0182_writing_outline_kind.sql`
- Modify: `apps/api/internal/store/queries/writing_atom.sql:69-89`（ReplaceWritingOutline）、`:154-164`（InsertWritingOutlineNode）

**Interfaces:**
- Consumes: Task 1 的 `writingKindFromRole` 的映射规则（这里用 SQL 重写一遍同样的规则）。
- Produces: `sqlc.WritingOutline.Kind string`；
  `ReplaceWritingOutlineParams.Kinds []string`；
  `InsertWritingOutlineNodeParams.Kind string`。

- [ ] **Step 1: 确认 0182 还没被别人占用**

```bash
ls apps/api/internal/store/migrations | tail -3
```
Expected: 最大是 `0181_awakening_protocol.sql`。若已有 0182，顺延到下一个未占用号。

- [ ] **Step 2: 写迁移**

```sql
-- +goose Up
-- 图上一个节点「是什么」。取值是闭表，见 apps/api/internal/api/writing_kind.go。
--
-- 为什么加这一列：在这之前模型自己挑 parentId、自己用散文写 role，下游拿关键词
-- 把意思猜回来。同事 2026-09-20 截到的那张图里，结尾挂在中心论点底下（深度 1），
-- slots.ts 只在 depth=0 时认结尾，于是它被印成「分论点 3」。
--
-- 不写 CHECK 约束：同 course.category / course.audience 的理由 —— 词表在 Go 和
-- TS 两处，加 CHECK 会让加一个取值变成一次迁移。校验在 writingKindValid。
ALTER TABLE writing_outline ADD COLUMN kind text NOT NULL DEFAULT '';

-- 回填。规则与 writingKindFromRole 逐条一致：先认最具体的，再认开篇/结尾，
-- 再认论据，最后按深度兜底。
--
-- 🚨 **只写 kind，不动 depth 和 position。** 新的深度强制只作用于此后新增和
-- 她拖动的节点；在这里重排，会把一份她已经在写的稿子的卡片顺序当场换掉。
UPDATE writing_outline SET kind = CASE
  WHEN role ~ '还没找到|待补|暂时没有|缺一份' THEN 'gap'
  WHEN role ~ '回应|反驳' OR lower(role) ~ 'rebuttal|response' THEN 'rebuttal'
  WHEN role ~ '反方|对方|反对|质疑' OR lower(role) ~ 'counter|objection' THEN 'counter'
  WHEN role ~ '开头|引言|开篇|钩子|导入' OR lower(role) ~ 'opening|hook|introduction|intro' THEN 'opening'
  WHEN role ~ '结尾|结论|总结|收尾|落点|结语' OR lower(role) ~ 'closing|conclusion|ending' THEN 'closing'
  WHEN role ~ '例|经历|的事|事件|故事|材料|数据|研究|报道|访谈|调查|案例|引用|名言|人物|史实|素材|证据|新闻|实验|统计|场景|现象'
    OR lower(role) ~ 'example|experience|evidence|data|study|research|report|story|quote|case|survey|statistic|source'
    THEN 'evidence'
  WHEN depth = 0 THEN 'thesis'
  WHEN depth = 1 THEN 'point'
  ELSE 'evidence'
END;

-- +goose Down
ALTER TABLE writing_outline DROP COLUMN kind;
```

- [ ] **Step 3: 三条 SQL 语句带上 kind**

`ReplaceWritingOutline`（`writing_atom.sql:76-88`）—— INSERT 列表和 SELECT 各加一项：

```sql
INSERT INTO writing_outline (atom_id, text, role, depth, position, source, kind)
SELECT sqlc.arg(atom_id),
       unnest(sqlc.arg(texts)::text[]),
       unnest(sqlc.arg(roles)::text[]),
       unnest(sqlc.arg(depths)::int[]),
       unnest(sqlc.arg(positions)::int[]),
       unnest(sqlc.arg(sources)::text[]),
       unnest(sqlc.arg(kinds)::text[])
RETURNING *;
```

`InsertWritingOutlineNode`（`writing_atom.sql:161-163`）：

```sql
INSERT INTO writing_outline (atom_id, text, role, depth, position, source, kind)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;
```

- [ ] **Step 4: 重新生成 sqlc，确认新字段出现**

```bash
export PATH="$HOME/sdk/go1.26.0/bin:$PATH"
./.tooling/sqlc generate -f apps/api/sqlc.yaml
grep -n "Kind" apps/api/internal/store/sqlc/models.go | head -5
```
Expected: `writing_outline` 的 model 里出现 `Kind string`。
🚨 若 `sqlc.yaml` 不在那个路径，`find apps/api -name 'sqlc*.y*ml'` 找一下。

- [ ] **Step 5: 跑 Task 1 的测试（现在才编译得过）**

```bash
export PATH="$HOME/sdk/go1.26.0/bin:$PATH"
cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestWritingKind -v
```
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add apps/api/internal/store/migrations/0182_writing_outline_kind.sql \
        apps/api/internal/store/queries/writing_atom.sql \
        apps/api/internal/store/sqlc
git commit -m "feat(lite-writing): 0182 给提纲加 kind 一列，回填只写 kind 不动位置

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: 规划这一轮改成返回 kind，深度与父由服务端算

**Files:**
- Modify: `apps/api/internal/api/writing_plan.go`（`writingPlanAdd`、`parseWritingPlanReply`、`insertPlanNode`、提示词的「输出格式」一节；删除 `rootInsertPosition`）
- Test: `apps/api/internal/api/writing_plan_internal_test.go`

**Interfaces:**
- Consumes: Task 1 的 `writingKindValid` / `writingKindDepth` / `writingKindParentOf` / `writingKindLabel`；Task 2 的 `InsertWritingOutlineNodeParams.Kind`。
- Produces: `writingPlanAdd{Kind, Text, Source}`（**`ParentID` 与 `Role` 字段删除**）；
  `insertPlanNode(ctx, q, atomID, rows, kind, text, source)`（签名少了 `parent` 与 `role`）。

- [ ] **Step 1: 写失败的测试**

```go
func TestParseWritingPlanReplyDropsUnknownKind(t *testing.T) {
	raw := `{"reply":"好。","add":[
	  {"kind":"point","text":"早上省心"},
	  {"kind":"编的一个","text":"这条要丢掉"},
	  {"kind":"closing","text":"回到方便比好看值"}
	],"ready":false}`
	got, ok := parseWritingPlanReply(raw)
	if !ok {
		t.Fatal("should parse")
	}
	if len(got.Add) != 2 {
		t.Fatalf("kept %d nodes, want 2 (the invented kind is dropped)", len(got.Add))
	}
	if got.Add[1].Kind != writingKindClosing {
		t.Errorf("second kept node kind = %q", got.Add[1].Kind)
	}
}

// 🚨 意见 3 的回归用例，走到插入这一层：模型说结尾是中心论点的孩子，
// 它仍然必须落在深度 0。
func TestInsertPlanNodeForcesDepthFromKind(t *testing.T) {
	thesis := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindThesis, Depth: 0, Position: 0}
	point := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 1}
	rows := []sqlc.WritingOutline{thesis, point}

	if d := writingKindDepth(writingKindClosing); d != 0 {
		t.Fatalf("closing depth = %d, want 0", d)
	}
	if p := writingKindParentOf(writingKindClosing, rows); p != nil {
		t.Errorf("closing must be top level, got parent %v", p.ID)
	}
}
```

- [ ] **Step 2: 跑，确认失败**

```bash
export PATH="$HOME/sdk/go1.26.0/bin:$PATH"
cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestParseWritingPlanReplyDropsUnknownKind|TestInsertPlanNodeForcesDepthFromKind' -v
```
Expected: FAIL —— `writingPlanAdd` 还没有 `Kind` 字段。

- [ ] **Step 3: 改 `writingPlanAdd` 与解析**

```go
type writingPlanAdd struct {
	// Kind 是这一块**是什么**，取自 writing_kind.go 的闭表。
	//
	// 🚨 这取代了原来的 ParentID + Role 两个字段。模型不再决定摆在哪儿 ——
	// 深度和父节点由 kind 算出来（writingKindDepth / writingKindParentOf）。
	// 原来那对字段出过两类错，都是静默的：模型抄错 UUID 让节点被整个丢掉；
	// 模型把结尾挂到中心论点底下，屏幕上印成「分论点 3」。
	Kind string `json:"kind"`
	Text string `json:"text"`
	// Source 是这条材料从哪来（0158），只有她找回来的那种材料才有。
	Source string `json:"source"`
}
```

`parseWritingPlanReply` 的 clamp 循环里，把原来对 `Role` / `ParentID` 的
TrimSpace 换成：

```go
	for _, a := range got.Add {
		a.Text = strings.TrimSpace(a.Text)
		a.Kind = strings.TrimSpace(strings.ToLower(a.Kind))
		if a.Text == "" {
			continue
		}
		// 🚨 编出来的 kind 整条丢掉，不猜一个最近的。
		// 猜错的代价是她的一句话落在一个她没放的地方，而她看不出发生过什么。
		if !writingKindValid(a.Kind) {
			slog.Warn("writing plan turn: dropped node with unknown kind",
				"kind", a.Kind)
			continue
		}
		kept = append(kept, a)
		if len(kept) == writingPlanMaxNewNodes {
			break
		}
	}
```

- [ ] **Step 4: 改 `insertPlanNode`**

```go
// insertPlanNode 把一个节点放到它该在的地方。
//
// 🚨 **深度和父节点由 kind 算出来，不采信模型给的位置。** 见 writing_kind.go。
// 位置：有父的排在那个父的子树末尾（同辈保持她说出来的顺序）；
// 开篇排在最前，结尾排在最后，中心论点排在开篇之后。
func insertPlanNode(
	ctx context.Context,
	q *sqlc.Queries,
	atomID uuid.UUID,
	rows []sqlc.WritingOutline,
	kind, text, source string,
) (sqlc.WritingOutline, []sqlc.WritingOutline, error) {
	depth := writingKindDepth(kind)
	parent := writingKindParentOf(kind, rows)

	// 一条论据来了、图上还没有分论点：它自己先当一条分论点占住这一段，
	// 段落那一步的 needsPoint 会请她先说清它证明了什么。
	// 挂到中心论点底下冒充一条理由是更糟的选择 —— 那正是 2026-09-18
	// 记下的毛病：例子落在最上层，被印成「分论点 3」。
	if parent == nil && depth > 0 {
		kind = writingKindPoint
		depth = writingKindDepth(kind)
		parent = writingKindParentOf(kind, rows)
		if parent == nil {
			depth = 0
		}
	}

	var insertAt int32
	if parent != nil {
		insertAt = parent.Position + 1
		for _, r := range rows {
			if r.Position > parent.Position && r.Depth > parent.Depth {
				insertAt = r.Position + 1
			} else if r.Position > parent.Position {
				break
			}
		}
	} else {
		insertAt = topLevelInsertPosition(kind, rows)
	}
	// …以下 ShiftWritingOutlinePositions / InsertWritingOutlineNode 不变，
	// 只是 InsertWritingOutlineNodeParams 多传 Kind: kind，
	// 并且 Role 传 writingKindLabel(kind, source)（老前端和报告还读 role）。
}

// topLevelInsertPosition 取代 rootInsertPosition：开篇最前，结尾最后，
// 中心论点排在开篇之后。原来那个函数对自由散文做子串匹配，认不出来就排到末尾 ——
// 于是一个开篇会排在每一条分论点后面。
func topLevelInsertPosition(kind string, rows []sqlc.WritingOutline) int32 {
	switch kind {
	case writingKindOpening:
		return 0
	case writingKindClosing:
		return int32(len(rows))
	}
	// 中心论点：排在开篇后面。
	var at int32
	for _, r := range rows {
		if r.Kind == writingKindOpening {
			at = r.Position + 1
		}
	}
	return at
}
```

删除 `rootInsertPosition` 及其测试。

- [ ] **Step 5: 改提示词的「输出格式」一节**

把 `writingPlanSystem` 里的输出格式段换成：

```
{"reply":"你要对她说的话","add":[{"kind":"point","text":"节点文字","source":""}],"ready":false}

- reply：不超过 200 字，最多一个问题，允许不提问。
- add：这一轮要往图上加的节点，**0 到 %d 个**；没有就给空数组。
- kind：这一块**是什么**，只能是下面这八个之一，**写错整条会被丢掉**：
  - `thesis` 中心论点 —— 这篇要证明的那一句话
  - `point` 分论点 —— 支撑中心论点的一条理由
  - `evidence` 论据 —— 撑住某条分论点的一件事、一份研究、一组数据
  - `counter` 反方观点 —— 反方最强的那一点
  - `rebuttal` 对反方的回应
  - `gap` 待补的材料 —— 她知道这里缺一份材料，但还没找到
  - `opening` 开篇
  - `closing` 结尾
  🚨 **你不决定它挂在哪儿。** 位置由 kind 算出来：分论点挂在中心论点下面，
  论据挂在前面最近的那条分论点下面，开篇和结尾各在最前和最后。
  所以不要给 parentId，也不要自己起小标题 —— 标题由 kind 定。
- text：**她自己的话的精简**，不超过 30 字。
- source：…（原文不变）
- ready：…（原文不变）
```

并把提示词里其它提到 `parentId` / `role` 的句子删掉（搜 `parentId`、`role：`）。

- [ ] **Step 6: 跑全套 api 测试**

```bash
export PATH="$HOME/sdk/go1.26.0/bin:$PATH"
cd apps/api && CGO_ENABLED=0 go test ./internal/api -timeout 1800s
```
Expected: PASS。会有一批旧测试引用 `ParentID` / `Role` / `rootInsertPosition` ——
逐个改成 `Kind`，**不要把它们删掉了事**。

- [ ] **Step 7: 提交**

```bash
git add apps/api/internal/api/writing_plan.go apps/api/internal/api/writing_plan_internal_test.go
git commit -m "feat(lite-writing): 规划返回 kind，摆放由服务端算，不再采信模型给的位置

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: gap 不计入材料；下游四处改读 kind

**Files:**
- Modify: `apps/api/internal/api/writing_plan_state.go`（`writingPlanShapeOf`）
- Modify: `apps/api/internal/api/writing_blocks.go`（删 `writingRoleIsExample` / `writingNodeIsMaterial` / 两张词表，改读 kind）
- Modify: `apps/api/internal/api/writing_guide.go`（`writingGuideAppliesTo` → `writingKindAppliesTo`）
- Modify: `apps/api/internal/api/writing_outline.go`（DTO 加 `Kind`，PUT 收 `kind`）
- Test: `apps/api/internal/api/writing_plan_state_internal_test.go`、`writing_blocks_internal_test.go`

**Interfaces:**
- Consumes: Task 1 的 `writingKindIsMaterial` / `writingKindAppliesTo` / `writingKindLabel`。
- Produces: `writingOutlineItemDTO.Kind string \`json:"kind"\``；
  PUT `/outline` 的 item 增加 `kind` 字段。

- [ ] **Step 1: 写失败的测试**

```go
func TestWritingPlanShapeExcludesGap(t *testing.T) {
	rows := []sqlc.WritingOutline{
		{Text: "校服省心", Kind: writingKindThesis, Depth: 0},
		{Text: "早上不用挑", Kind: writingKindPoint, Depth: 1},
		{Text: "我常起晚", Kind: writingKindEvidence, Depth: 2},
		{Text: "没看过相关新闻", Kind: writingKindGap, Depth: 2},
	}
	s := writingPlanShapeOf(rows)
	if s.Material != 1 {
		t.Errorf("Material = %d, want 1 — a gap is a hole, not evidence", s.Material)
	}
}

// 🚨 意见 3 的回归用例，走到卡片派生这一层。
func TestWritingCardsClosingAtDepthOne(t *testing.T) {
	rows := []sqlc.WritingOutline{
		{ID: uuid.New(), Text: "校服省心", Kind: writingKindThesis, Depth: 0, Position: 0},
		{ID: uuid.New(), Text: "早上不用挑", Kind: writingKindPoint, Depth: 1, Position: 1},
		// 模型把它挂在了深度 1 —— 它仍然必须是一张结尾卡，不是分论点 2。
		{ID: uuid.New(), Text: "回到方便比好看值", Kind: writingKindClosing, Depth: 1, Position: 2},
	}
	cards := writingCards(rows, nil)
	last := cards[len(cards)-1]
	if last.Kind != writingCardClosing {
		t.Errorf("last card kind = %q, want closing", last.Kind)
	}
}
```

- [ ] **Step 2: 跑，确认失败**

```bash
export PATH="$HOME/sdk/go1.26.0/bin:$PATH"
cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestWritingPlanShapeExcludesGap|TestWritingCardsClosingAtDepthOne' -v
```
Expected: FAIL。

- [ ] **Step 3: `writingPlanShapeOf` 改按 kind 数**

```go
func writingPlanShapeOf(rows []sqlc.WritingOutline) writingPlanShape {
	var s writingPlanShape
	for _, r := range rows {
		if strings.TrimSpace(r.Text) == "" {
			continue
		}
		switch {
		case writingKindIsMaterial(r.Kind):
			s.Material++
		case r.Kind == writingKindPoint || r.Kind == writingKindCounter:
			s.Points++
		case r.Kind == writingKindGap:
			// 🚨 一个洞不是一条材料。见 writing_kind.go 的 writingKindIsMaterial。
		case r.Kind != "":
			s.Top++
		default:
			// 0182 之前的老行没有 kind —— 退回按深度数，别把老稿子的计数清零。
			switch r.Depth {
			case 0:
				s.Top++
			case 1:
				s.Points++
			default:
				s.Material++
			}
		}
	}
	return s
}
```

- [ ] **Step 4: `writing_blocks.go` 改读 kind**

删除 `writingRoleIsExample`、`writingNodeIsMaterial`、`roleHasAny`、
`writingExampleRoleWords`、`writingOpeningRoleWords`、`writingClosingRoleWords`。
`writingCards` 里原来判断种类的每一处换成 `r.Kind == writingKindOpening` 这类
直接比较；老行（`Kind == ""`）用 `writingKindFromRole(r.Role, r.Depth)` 现算一个。

同样地，`writing_guide.go` 的 `writingGuideAppliesTo(role string)` 整个删掉，
调用点换成 `writingKindAppliesTo(block.Kind)`。

- [ ] **Step 5: DTO 与 PUT 带上 kind**

`writing_outline.go`：

```go
type writingOutlineItemDTO struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	Role string `json:"role"`
	// Kind 是这一块是什么，闭表见 writing_kind.go。
	// 🚨 json 标签不能少：缺标签会让前端读到的字段名全错，而 Go 测试全绿、
	// 日志干净（[[go-nil-slice-becomes-null]]）。
	Kind     string `json:"kind"`
	Depth    int32  `json:"depth"`
	Position int32  `json:"position"`
	Source   string `json:"source,omitempty"`
	Guide    *writingGuideDTO `json:"guide,omitempty"`
}
```

`toWritingOutlineItemDTO` 里：

```go
	kind := row.Kind
	if kind == "" {
		// 0182 之前的老行：现算一个，别让前端拿到空字符串去 switch。
		kind = writingKindFromRole(row.Role, row.Depth)
	}
	dto.Kind = kind
	if lbl := writingKindLabel(kind, row.Source); lbl != "" {
		dto.Role = lbl
	}
```

PUT 的请求体 item 加 `Kind string \`json:"kind"\``，落库时
`Depth: writingKindDepth(item.Kind)`，并传 `Kinds` 数组给
`ReplaceWritingOutline`。不合法的 kind 用 `writingKindFromRole` 兜底。

- [ ] **Step 6: 加一条 marshal 测试**

```go
// 🚨 按**前端真实读的名字**取值，并断言没有 null。
// [[go-nil-slice-becomes-null]]：缺 json 标签会让报告里每张卡都空，
// 而树是对的、日志干净、Go 测试全绿。
func TestWritingOutlineDTOMarshalsKind(t *testing.T) {
	dto := toWritingOutlineItemDTO(sqlc.WritingOutline{
		ID: uuid.New(), Text: "回到方便比好看值", Kind: writingKindClosing, Depth: 1,
	})
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back["kind"] != "closing" {
		t.Errorf(`back["kind"] = %v, want "closing"`, back["kind"])
	}
	if back["role"] != "结尾" {
		t.Errorf(`back["role"] = %v, want "结尾"`, back["role"])
	}
	if strings.Contains(string(b), "null") {
		t.Errorf("payload contains null: %s", b)
	}
}
```

- [ ] **Step 7: 跑全套，提交**

```bash
export PATH="$HOME/sdk/go1.26.0/bin:$PATH"
cd apps/api && CGO_ENABLED=0 go test ./internal/api -timeout 1800s
git add apps/api/internal/api
git commit -m "feat(lite-writing): 下游改读 kind，删掉四张关键词表；待补的材料不算材料

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: TS 孪生 `outlineKind.ts`，`slots.ts` 改读 kind

**Files:**
- Create: `apps/lite-web/src/writings/outlineKind.ts`
- Create: `apps/lite-web/src/writings/outlineKind.test.ts`
- Modify: `apps/lite-web/src/writings/slots.ts`、`apps/lite-web/src/api/writingRoom.ts`
- Test: `apps/lite-web/src/writings/slots.test.ts`

**Interfaces:**
- Consumes: Task 4 的 `writingOutlineItemDTO.Kind`。
- Produces:
  - `export type OutlineKind = "opening" | "thesis" | "point" | "evidence" | "counter" | "rebuttal" | "gap" | "closing"`
  - `export const OUTLINE_KIND_DEPTH: Record<OutlineKind, number>`
  - `export function outlineKindLabel(kind: string, source?: string): string`
  - `export function outlineKindOf(item: WritingOutlineItem): OutlineKind`（老行兜底）
  - `WritingOutlineItem.kind?: string`

- [ ] **Step 1: 写失败的测试**

```ts
import { describe, expect, it } from "vitest";
import { outlineKindLabel, outlineKindOf, OUTLINE_KIND_DEPTH } from "./outlineKind";

describe("outlineKind", () => {
  it("每个 kind 的深度和 Go 侧一致", () => {
    expect(OUTLINE_KIND_DEPTH).toEqual({
      opening: 0, thesis: 0, closing: 0,
      point: 1, counter: 1,
      evidence: 2, rebuttal: 2, gap: 2,
    });
  });

  it("论据按有没有出处分两种标题", () => {
    expect(outlineKindLabel("evidence", "")).toBe("论据 · 你见过的事");
    expect(outlineKindLabel("evidence", "Lu 2008")).toBe("论据 · 你找来的材料");
  });

  it("老行没有 kind 时按 role 和深度兜底", () => {
    expect(outlineKindOf({ id: "1", text: "回到方便比好看值", role: "结尾", depth: 1, position: 2 })).toBe("closing");
    expect(outlineKindOf({ id: "2", text: "早上省心", role: "一条理由", depth: 1, position: 1 })).toBe("point");
  });
});
```

并在 `slots.test.ts` 加这条回归用例：

```ts
it("🚨 结尾挂在深度 1 也是结尾卡，不是分论点（同事 2026-09-20 意见 3）", () => {
  const outline = [
    { id: "t", text: "校服省心", role: "中心论点", kind: "thesis", depth: 0, position: 0 },
    { id: "p", text: "早上不用挑", role: "分论点", kind: "point", depth: 1, position: 1 },
    { id: "c", text: "回到方便比好看值", role: "结尾", kind: "closing", depth: 1, position: 2 },
  ];
  const slots = buildSlots(outline, []);
  expect(slots[slots.length - 1].kind).toBe("closing");
  expect(slots.some((s) => s.kind === "point" && s.outlineId === "c")).toBe(false);
});
```

- [ ] **Step 2: 跑，确认失败**

```bash
cd apps/lite-web && npx vitest run src/writings/outlineKind.test.ts src/writings/slots.test.ts
```
Expected: FAIL —— `./outlineKind` 不存在。

- [ ] **Step 3: 写 `outlineKind.ts`**

逐条对着 `apps/api/internal/api/writing_kind.go` 翻译：同样八个取值、
同样的深度表、同样的标题表、同样的 `writingKindFromRole` 兜底顺序。
文件顶上写清楚：

```ts
/**
 * outlineKind —— writing_kind.go 的 TS 孪生。
 *
 * 🚨 **两边必须逐条一致**：取值、深度、标题、老 role 的兜底顺序。
 * Go 那一侧是单一真相源（服务端决定摆放），这一侧只负责把同一套规则
 * 画到屏幕上。改一边就要改另一边，两处各有一条测试钉着。
 */
```

- [ ] **Step 4: `slots.ts` 改读 kind**

删除 `EXAMPLE_ROLE_WORDS` / `OPENING_ROLE_WORDS` / `CLOSING_ROLE_WORDS` /
`hasAny` / `roleIsExample` / `nodeIsMaterial`，`buildSlots` 里每一处种类判断
换成 `outlineKindOf(o) === "closing"` 这类直接比较。
`slotTitle` 的 `分论点 N` 编号只数 `kind === "point" | "counter"` 的卡。

- [ ] **Step 5: 跑测试 + typecheck**

```bash
cd apps/lite-web && npx vitest run src/writings/ && npm run typecheck
```
Expected: PASS。
🚨 `apps/lite-web` 的 tsconfig include 里没有 `e2e` —— typecheck 绿着也可能
漏掉 e2e 里的 ReferenceError，那部分单独看。

- [ ] **Step 6: 提交**

```bash
git add apps/lite-web/src/writings/outlineKind.ts apps/lite-web/src/writings/outlineKind.test.ts \
        apps/lite-web/src/writings/slots.ts apps/lite-web/src/writings/slots.test.ts \
        apps/lite-web/src/api/writingRoom.ts
git commit -m "feat(lite-web): 卡片派生改读 kind，删掉三张关键词表

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: 拖得出来（意见 1）

**Files:**
- Modify: `apps/lite-web/src/writings/outlineMove.ts`、`useMindMapDrag.ts`、`MindMap.tsx`
- Test: `apps/lite-web/src/writings/outlineMove.test.ts`

**Interfaces:**
- Consumes: Task 5 的 `OUTLINE_KIND_DEPTH` / `outlineKindOf`。
- Produces: `moveOutlineNode(items, draggedId, targetId, mode)` 增加 `mode: "root"`；
  `useMindMapDrag(onMove)` 的 `onMove` 第三参可为 `"child" | "after" | "root"`；
  新增 `hoverMode: OutlineMoveMode | null`。

- [ ] **Step 1: 写失败的测试**

```ts
it("🚨 挂进子层之后还能拖回最上层（同事 2026-09-20 意见 1）", () => {
  const items = [
    { id: "t", text: "主张", role: "中心论点", kind: "thesis", depth: 0, position: 0 },
    { id: "p", text: "理由一", role: "分论点", kind: "point", depth: 1, position: 1 },
    { id: "x", text: "本来是条理由", role: "分论点", kind: "point", depth: 2, position: 2 },
  ];
  const next = moveOutlineNode(items, "x", "", "root");
  expect(next).not.toBeNull();
  const moved = next!.find((r) => r.id === "x")!;
  expect(moved.depth).toBe(0);
});

it("拖到一张卡旁边是兄弟，不是孩子", () => {
  const items = [
    { id: "t", text: "主张", role: "中心论点", kind: "thesis", depth: 0, position: 0 },
    { id: "p1", text: "理由一", role: "分论点", kind: "point", depth: 1, position: 1 },
    { id: "e", text: "一件事", role: "论据", kind: "evidence", depth: 2, position: 2 },
  ];
  const next = moveOutlineNode(items, "e", "p1", "after")!;
  expect(next.find((r) => r.id === "e")!.depth).toBe(1);
});

it("挪完之后 kind 跟着新深度改写", () => {
  const items = [
    { id: "t", text: "主张", role: "中心论点", kind: "thesis", depth: 0, position: 0 },
    { id: "p1", text: "理由一", role: "分论点", kind: "point", depth: 1, position: 1 },
    { id: "e", text: "一件事", role: "论据", kind: "evidence", depth: 2, position: 2 },
  ];
  const next = moveOutlineNode(items, "e", "p1", "after")!;
  // 深度 1 上的论据不成立 —— 它现在是一条分论点，标题当场变。
  expect(next.find((r) => r.id === "e")!.kind).toBe("point");
});
```

- [ ] **Step 2: 跑，确认失败**

```bash
cd apps/lite-web && npx vitest run src/writings/outlineMove.test.ts
```
Expected: FAIL。

- [ ] **Step 3: `outlineMove.ts` 加 `"root"` 模式与 kind 改写**

```ts
export type OutlineMoveMode = "child" | "after" | "root";
```

`moveOutlineNode` 里：`mode === "root"` 时 `newDepth = 0`，整段插到清单末尾
（`targetId` 传空串）。所有分支在算完 `moved` 之后统一过一遍 kind 改写：

```ts
/**
 * 挪完之后，节点的 kind 要跟新的深度对上。
 *
 * 一条论据被拖到深度 1，它就不再是论据 —— 她做的那个动作的意思是
 * 「这其实是一条理由」。标题当场跟着变，她才看得见自己刚才做成了什么。
 * 深度对得上的就不动（把一条论据从一个分论点挪到另一个，它还是论据）。
 */
function rekind(row: WritingOutlineItem, depth: number): WritingOutlineItem {
  const kind = outlineKindOf(row);
  if (OUTLINE_KIND_DEPTH[kind] === depth) return { ...row, depth };
  const fallback: Record<number, OutlineKind> = { 0: "thesis", 1: "point", 2: "evidence" };
  // 深度 0 上已经有中心论点了，就当它是结尾 —— 一篇只有一个中心论点。
  return { ...row, depth, kind: fallback[depth] ?? "point" };
}
```

🚨 深度 0 的兜底要看图上有没有中心论点：有就给 `closing`，没有才给 `thesis`。
把这个判断写在 `moveOutlineNode` 里（它手上有整份清单），不要写在 `rekind` 里。

- [ ] **Step 4: `useMindMapDrag.ts` 三区落点 + 空白落点**

`onPointerMove` 里算 `hoverMode`：

```ts
        const over = nodeAt(e.clientX, e.clientY);
        setHoverId(over === draggingRef.current ? null : over);
        // 卡片上三分之一 = 放到它旁边（兄弟），其余 = 放到它底下（孩子）。
        // 🚨 她必须在松手**之前**看见会发生哪一种 —— 一次看不见结果的拖动，
        // 和 2026-09-12 那个「拖完弹出改字框」是同一类问题。
        if (over && over !== draggingRef.current) {
          const el = document.elementFromPoint(e.clientX, e.clientY)?.closest?.("[data-outline-node]");
          const box = el?.getBoundingClientRect();
          setHoverMode(box && e.clientY - box.top < box.height / 3 ? "after" : "child");
        } else {
          setHoverMode(null);
        }
```

`onPointerUp` 里：

```ts
        // 🚨 落在空白画布上 = 升到最上层。
        // 这之前是空操作，于是一条理由挂进子层之后就**再也出不来**
        //（同事 2026-09-20 意见 1）。`"after"` 模式在 outlineMove.ts 里躺了
        // 整整一个月，没有任何调用点。
        if (!over) {
          if (movedRef.current) onMove?.(id, "", "root");
          return;
        }
        if (over !== id) onMove?.(id, over, mode ?? "child");
```

- [ ] **Step 5: `MindMap.tsx` 显示落点提示**

被悬停的那张卡按 `drag.hoverMode` 显示 `mk-node-drop`（孩子）或在它上沿画一条
插入线（兄弟）。空白画布在拖动中显示一行 `text-mk-small text-mk-faint`：
**「放到这里：移到最上层」**（名词短语，不是句子；见文案规则 1）。

节点标题改用 `outlineKindLabel(outlineKindOf(item), item.source)`，
不再直接印 `item.role`。

- [ ] **Step 6: 跑测试 + 真浏览器看一眼**

```bash
cd apps/lite-web && npx vitest run src/writings/ && npm run typecheck
```
Expected: PASS。
然后起本地栈，用 Playwright 真的拖一次：把一条论据拖到空白处，
截图确认它升到了最上层且标题变成「分论点」。
🚨 jsdom 没有 PointerEvent，也看不见布局 —— 这一步必须用真浏览器。

- [ ] **Step 7: 提交**

```bash
git add apps/lite-web/src/writings/outlineMove.ts apps/lite-web/src/writings/outlineMove.test.ts \
        apps/lite-web/src/writings/useMindMapDrag.ts apps/lite-web/src/writings/MindMap.tsx
git commit -m "fix(lite-web): 节点能从子层拖回最上层，落点在松手前就看得见

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: 引导不再被覆盖（意见 7）

**Files:**
- Modify: `apps/api/internal/api/writing_guide.go`（`writingGuideDTO`、`guideWritingBlock`、`writingGuideQuestionRules`）
- Modify: `apps/lite-web/src/writings/GuideBox.tsx`、`apps/lite-web/src/api/writingRoom.ts`
- Test: `apps/api/internal/api/writing_guide_internal_test.go`

**Interfaces:**
- Consumes: 无新依赖。
- Produces: `writingGuideDTO` 增加 `Previous *writingGuideDTO \`json:"previous,omitempty"\``。

- [ ] **Step 1: 写失败的测试**

```go
func TestRegenerateGuideKeepsPrevious(t *testing.T) {
	old := writingGuideDTO{Job: "旧的任务", Questions: []string{"旧问题？"}}
	fresh := writingGuideDTO{Job: "新的任务", Questions: []string{"新问题？"}}
	got := writingGuideWithPrevious(fresh, &old)
	if got.Previous == nil || got.Previous.Job != "旧的任务" {
		t.Fatalf("previous not kept: %+v", got.Previous)
	}
	// 🚨 只留一层。三层之后那块板会变成一份她读不完的历史。
	if got.Previous.Previous != nil {
		t.Error("previous must not nest")
	}
}
```

- [ ] **Step 2: 跑，确认失败**

```bash
export PATH="$HOME/sdk/go1.26.0/bin:$PATH"
cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestRegenerateGuideKeepsPrevious -v
```
Expected: FAIL。

- [ ] **Step 3: 实现**

```go
// writingGuideWithPrevious 把上一份引导挂在新的那一份下面。
//
// 🚨 「卡住了？」原来直接 SetWritingOutlineGuide 覆盖，旧的那份就没了 ——
// 同事 2026-09-20 的原话：「每一次刷新就会变成新的东西」。
// 她读过的东西不该在她按一个按钮之后消失。
//
// 只留一层：再往上叠会变成一份她读不完的历史，而她要的只是
//「刚才那组问题呢」。
func writingGuideWithPrevious(fresh writingGuideDTO, old *writingGuideDTO) writingGuideDTO {
	if old != nil {
		trimmed := *old
		trimmed.Previous = nil
		fresh.Previous = &trimmed
	}
	return fresh
}
```

`guideWritingBlock` 落库前先把 `block.Guide` 解出来当 `old` 传进去。
重新生成的提示词里加上：

```
【上一组问题】（她已经读过这几条，这一次要换一个角度看同一块，不要重复）
- …
```

- [ ] **Step 4: 问题规则改写（去模板化）**

`writingGuideQuestionRules` 里加两条，删掉逼着举例的那层暗示：

```
- **举例不是唯一的路。** 一条理由可以靠一件具体的事撑住，也可以靠把道理一步
  一步推给读者看（`道理论证`）。不要每一处分析后面都要求一个例子。
- **这一块她已经写了字的时候，最多问两个问题。** 一次只解决最上面那一层；
  她盯着一段已经写完的话，收到四个问题只会读成「我写得很烂」。
```

- [ ] **Step 5: `GuideBox.tsx`**

按钮 `卡住了？` 改名 **「换一组问题」**。`guide.previous` 非空时，在
「想一想」那一节末尾加一个 `看上一组` 的切换（默认折起）。

- [ ] **Step 6: 跑，提交**

```bash
export PATH="$HOME/sdk/go1.26.0/bin:$PATH"
cd apps/api && CGO_ENABLED=0 go test ./internal/api -timeout 1800s
cd ../lite-web && npx vitest run src/writings/ && npm run typecheck
git add apps/api/internal/api/writing_guide.go apps/api/internal/api/writing_guide_internal_test.go \
        apps/lite-web/src/writings/GuideBox.tsx apps/lite-web/src/api/writingRoom.ts
git commit -m "fix(lite-writing): 换一组问题不再抹掉上一组；举例不再是唯一的路

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: 被禁止的请求先说一句说明（意见 8）

**Files:**
- Create: `apps/api/internal/api/writing_refusal.go`
- Create: `apps/api/internal/api/writing_refusal_test.go`
- Modify: `apps/api/internal/api/writing_plan.go`、`writing_turn.go`、`writing_deepen.go`（各自注入那一段并校验）

**Interfaces:**
- Consumes: 无。
- Produces:
  - `func writingAsksUsToDoIt(text string) bool`
  - `const writingRefusalBlock string`
  - `func writingReplyOwnsTheRefusal(reply string) bool`

- [ ] **Step 1: 写失败的测试**

```go
func TestWritingAsksUsToDoIt(t *testing.T) {
	yes := []string{
		"我懒得搜，你帮我找吧", "你帮我写一段吧", "你直接写好了给我",
		"帮我查一下资料", "能不能你来写", "can you write it for me",
	}
	for _, s := range yes {
		if !writingAsksUsToDoIt(s) {
			t.Errorf("%q should be detected", s)
		}
	}
	no := []string{
		// 🚨 这几条是判据的边界，全都不许命中：
		"我找到一份研究，帮我看看它撑不撑得住", // 请你看 ≠ 请你写
		"我写完了，你帮我提点意见",             // 提意见正是它该做的事
		"我不知道该怎么写这一段",               // 说自己卡住了，不是要人代劳
	}
	for _, s := range no {
		if writingAsksUsToDoIt(s) {
			t.Errorf("%q must NOT be detected", s)
		}
	}
}

func TestWritingReplyOwnsTheRefusal(t *testing.T) {
	ok := "印记不替你搜索，这一条要找的是一份关于青少年睡眠时长的调查。你先去找，找到把出处贴进来。"
	if !writingReplyOwnsTheRefusal(ok) {
		t.Error("a reply that names the boundary should pass")
	}
	bad := "这份计划已经够动笔了：主张是「学校应允许学生带手机」，底下有两条分论点。开写吧。"
	if writingReplyOwnsTheRefusal(bad) {
		t.Error("silently changing the subject must fail the check")
	}
}
```

- [ ] **Step 2: 跑，确认失败**

```bash
export PATH="$HOME/sdk/go1.26.0/bin:$PATH"
cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestWritingAsksUsToDoIt|TestWritingReplyOwnsTheRefusal' -v
```
Expected: FAIL。

- [ ] **Step 3: 实现**

```go
package api

// writing_refusal.go —— 她请印记替她做那件它不做的事时，说的那句话。
//
// 同事 2026-09-20：「对于被禁止的交互，还是回应一句类似『印记不会替代你自己
// 的思考和搜索行为』的内容，然后再转移下一个问题比较好，现在的有点太生硬了。」
//
// 截图里她说「我懒得搜，你帮我找吧」，印记 直接跳过去开始总结计划 ——
// 边界是对的，可她不知道自己被拒绝了，只看见话题变了。
//
// 🚨 **做成可验的，不是只在散文里要求。** 提示词里的「必须」是模型可以推翻的
// 「必须」（[[prompt-output-must-be-verifiable]]、[[detector-must-target-the-real-failure]]）。
// 所以：检测命中 → 注入那一段 → 校验回复里有没有那句说明 → 缺了重试一次。
//
// 🚨 判据盯的是**真失败本身**：失败是「一个字都没说就换了话题」，
// 不是「措辞不合我意」。所以校验只问一件事：这一轮有没有点出这条边界。
// 校验不通过也**只重试一次**，然后照旧把模型说的话交出去 ——
// 为了一个检测项让她看见一个死掉的终端，是更大的毛病。

import "strings"

// 请我们代劳的说法。命中一条就算。
var writingDoItForMePhrases = []string{
	"你帮我找", "帮我找一下", "帮我搜", "你帮我搜", "帮我查一下资料", "帮我查查资料",
	"你帮我写", "帮我写一", "你来写", "你直接写", "你写好", "能不能你来写",
	"替我写", "代我写", "你写吧", "懒得搜", "懒得找", "懒得写",
	"write it for me", "write this for me", "do it for me", "find it for me",
}

// writingAsksUsToDoIt：她这一句是不是在请我们替她做搜索或撰写。
//
// 🚨 **子串比对，而且词表要窄。** 「你帮我看看」「你帮我提点意见」都是它
// 该做的事，一个都不许命中 —— 判错的方向不对称：把一次正常的求助判成越界，
// 换来的是一句她没道理挨的说明。
func writingAsksUsToDoIt(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	for _, p := range writingDoItForMePhrases {
		if strings.Contains(t, p) {
			return true
		}
	}
	return false
}

// writingRefusalBlock 是命中那一轮加进 prompt 的一段。
//
// 🚨 **一次性，不做常驻。** 2026-09-05 的教训：这类提示做成常驻，模型会一直
// 去处理那条提示、把该做的事挤掉（那次是六轮里一直在补一张卡）。
const writingRefusalBlock = `
【她刚才请你替她做一件你不做的事】
她请你替她搜索，或者替她写正文。先说明，再往下走，一共三句以内：

1. 一句话说明：印记不替你搜索，也不替你写正文。说一次就够。
2. 紧接着说你**会**做的那件事，而且要具体：这一条该找什么样的材料
   （不是「去查查资料」，是「一份关于青少年睡眠时长的调查」）；
   或者这一段该怎么自己动手。
3. 然后往下走，当这件事已经说完了。

不要说教，不要解释我们的教育理念，下一轮不要再提这件事。
`

// 那句说明的痕迹。命中任一条就算她被告知了。
var writingRefusalMarkers = []string{
	"不替你", "不会替你", "不能替你", "不替学生", "替你写", "替你搜", "替你找",
	"得你自己", "要你自己", "由你自己", "你自己来",
}

// writingReplyOwnsTheRefusal：这一轮有没有点出那条边界。
//
// 只问这一件事，不判措辞 —— 见文件顶上那条「判据盯的是真失败本身」。
func writingReplyOwnsTheRefusal(reply string) bool {
	t := strings.ToLower(reply)
	for _, m := range writingRefusalMarkers {
		if strings.Contains(t, m) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: 三处调用点接上**

`writing_plan.go` 的 `buildWritingPlanPrompt` 里，在 `writingPlanStalledBlock`
那一段旁边：

```go
	if writingAsksUsToDoIt(studentText) {
		b.WriteString(writingRefusalBlock)
	}
```

落库之前加校验重试：

```go
	// 检测命中却一个字都没说 —— 重试一次，然后照旧交出去。
	// 🚨 绝不因为这个检测让她看见一个死掉的终端。
	if writingAsksUsToDoIt(studentText) && !writingReplyOwnsTheRefusal(parsed.Reply) {
		slog.Warn("writing plan turn: refusal not owned, retrying once", "atom_id", at.ID)
		// …重跑一次同一个调用，仍然不合格就用第一次的结果。
	}
```

`writing_turn.go` 和 `writing_deepen.go` 照同样两处接。

- [ ] **Step 5: 跑全套，提交**

```bash
export PATH="$HOME/sdk/go1.26.0/bin:$PATH"
cd apps/api && CGO_ENABLED=0 go test ./internal/api -timeout 1800s
git add apps/api/internal/api/writing_refusal.go apps/api/internal/api/writing_refusal_test.go \
        apps/api/internal/api/writing_plan.go apps/api/internal/api/writing_turn.go apps/api/internal/api/writing_deepen.go
git commit -m "feat(lite-writing): 被禁止的请求先说一句说明再往下走，且这句可验

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: 真模型验一遍 + 线上走查

**Files:**
- Modify: `apps/api/internal/api/writing_plan_live_test.go`（没有就新建）

- [ ] **Step 1: 写一条 LIVE_LLM 测试**

```go
//go:build !race

func TestLiveWritingPlanReturnsLegalKinds(t *testing.T) {
	if os.Getenv("LIVE_LLM") == "" {
		t.Skip("LIVE_LLM not set")
	}
	// 用同事那个真实题目：「学校应不应该允许学生带手机」。
	// 断言：每一个 add 的 kind 都在闭表里，且至少有一轮长出 thesis 和 point。
}
```

- [ ] **Step 2: 跑它**

```bash
export PATH="$HOME/sdk/go1.26.0/bin:$PATH"
cd apps/api && LIVE_LLM=1 CGO_ENABLED=0 go test ./internal/api -run TestLiveWritingPlan -v -timeout 600s
```
Expected: PASS。真模型回的 kind 必须全部合法；出现编造的 kind 说明提示词那一节
写得还不够硬（把八个取值的说明再收紧，不要放宽校验）。

- [ ] **Step 3: 线上走查，至少两趟**

```bash
cd apps/lite-web
E2E_BASE_URL=https://mind-lite.uni-robot.cn E2E_API_BASE=https://mind-api.uni-robot.cn \
E2E_JOIN_CODE=3VMV-3TZD node e2e/shootWriting.mjs /tmp/walk-1
```
每趟用新账号。**至少两趟** —— 2026-09-18 的记录写着模型的摆放逐次不同，
一趟走通不算数。逐张截图确认：没有一张卡片印着「分论点 N」而内容是结尾；
论据的标题是「论据 · …」。

- [ ] **Step 4: 提交**

```bash
git add apps/api/internal/api/writing_plan_live_test.go
git commit -m "test(lite-writing): 真模型回的 kind 必须在闭表里

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## 自查

**规格覆盖**（对着 `2026-09-20-lite-writing-polish-design.md` 的 R1 部分）：

| 规格节 | 任务 |
|---|---|
| §2.1 kind 闭表 + 强制深度/父 | Task 1、2、3 |
| §2.2 gap 不计入材料 | Task 4 |
| §2.3 老行回填 | Task 2（SQL）+ Task 1（`writingKindFromRole`）|
| §2.4 下游四处改读 kind | Task 4（Go 三处）、Task 5（TS 一处）|
| §3 拖得出来 | Task 6 |
| §4 引导不再被覆盖 + 去模板化 | Task 7 |
| §5 被禁请求说出那句话 | Task 8 |
| §8 测试（LIVE_LLM + 两趟线上） | Task 9 |

**占位符扫描：** 无 TBD / TODO。Task 3 Step 4 和 Task 8 Step 4 用 `…` 标出
「其余不变」的部分，每一处都指明了改哪几行、改成什么。

**类型一致性：** `writingKind*` 系列（Go）与 `outlineKind*` 系列（TS）成对；
`OutlineMoveMode` 在 Task 6 扩成三个取值，`useMindMapDrag` 的 `onMove` 第三参
同步；`writingGuideDTO.Previous` 只在 Task 7 出现。
Task 1 Step 4 依赖 Task 2 的 sqlc 产物 —— 已在该步骤内注明。
