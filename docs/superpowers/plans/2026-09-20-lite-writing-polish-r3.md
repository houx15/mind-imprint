# 写作面 R3 实施计划 —— 行文这一步 + 方法库规范化 + 段落页的图

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 同事的意见 4 和 5。写之前多一步**只想组织方式、不填内容**的「行文」；
方法库按语文课的正式分类规范化；段落页左栏顶部留着那张思维导图。

**Architecture:** 行文这一步**不新建表**：它就是那张结构图，重新排序 + 每一块标一个
方法。顺序用已有的 `writing_outline.position`（拖动那套 R1 已经建好），
每块的方法存进新列 `writing_outline.method`，整篇的论证结构存进
`writing.structure_key`（那一列自 2026-08-27 模板选择器被删之后就没人写过）。

**Tech Stack:** Go 1.26（`~/sdk/go1.26.0/bin`）、sqlc v1.27.0（`./.tooling/sqlc`，
arm64；worktree 里要重新下）、goose、React + TS、vitest、Playwright。

**Spec:** `docs/superpowers/specs/2026-09-20-lite-writing-polish-design.md` §7

## Global Constraints

- 每个 shell 先 `export PATH="$HOME/sdk/go1.26.0/bin:$PATH"`。
- 🚨 **跑 `go test ./...`，不是只跑 `./internal/api`**（R1 有五处回归只有全量能露出来）。
- 🚨 **Go 原始字符串里不许出现反引号**，会当场截断并报一个不相干的 syntax error。用「」。
- 🚨 **方法名的 🔑 规矩**：提示词散文里 `『』` 引的每一个名字都必须逐字存在于
  `packages/contracts/vocab/methods.json` 的某个 `name` 或 `formal_name`。
  `TestWritingPlanSystem_NamesOnlyRealMethods` 守着这条，而且它显式钉住
  **`并列论证` 必须存在且出现在 `writingGuideTeachingRules` 里** —— 所以
  **这一轮不许改任何一个现有的 name / formal_name**，只许新增。
- 🚨 **methods.json 有两份**（`packages/contracts/vocab/` 是源，
  `apps/api/internal/vocab/` 是 go:embed 的逐字节副本）。改了要两份一起改，
  `TestEmbeddedCopyMatchesSourceOfTruth` 守着。
- 界面文案：标签是名词、按钮写「做什么／做完了」、不写文学腔。
- Go→TS：新字段要有 json 标签；nil 切片返回 `[]T{}`。
- 提交信息结尾带 `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`。

## 文件结构

**新建**
- `apps/api/internal/store/migrations/0184_writing_flow.sql` —— `writing_outline.method`
  + 清掉 `writing.structure_key` 的陈年脏值。
- `apps/api/internal/api/writing_flow.go` —— 论证结构的闭表 + 这一步的校验。
- `apps/api/internal/api/writing_flow_test.go`
- `apps/lite-web/src/writings/FlowStage.tsx` —— 行文那块板。
- `apps/lite-web/src/writings/flowStructures.ts` + `.test.ts` —— 闭表的 TS 孪生。
- `apps/lite-web/src/writings/MiniMap.tsx` —— 段落页左栏顶上那张缩略图。

**修改**
- `packages/contracts/vocab/methods.json` + `apps/api/internal/vocab/methods.json`
- `apps/api/internal/vocab/vocab.go` —— 加 `Category`，`ForLang` 排除整篇层
- `apps/api/internal/api/writing_stage.go` —— 接受 `flow`
- `apps/api/internal/api/writing_outline.go` —— DTO / PUT 带 `method`
- `apps/lite-web/src/writings/StageMap.tsx` —— 四步
- `apps/lite-web/src/writings/WritingRoomHost.tsx` —— 挂上 FlowStage
- `apps/lite-web/src/writings/SnippetsStage.tsx` —— 左栏顶上放 MiniMap
- `apps/lite-web/src/writings/GuideBox.tsx` —— 「常见的几种写法」改成段落内结构
- `apps/api/internal/api/writing_guide.go` —— 段落内结构那一节

---

### Task 1: 方法库规范化（意见 5 的第三点）

**Files:**
- Modify: `packages/contracts/vocab/methods.json`、`apps/api/internal/vocab/methods.json`（两份逐字节一致）
- Modify: `apps/api/internal/vocab/vocab.go`
- Test: `apps/api/internal/vocab/vocab_test.go`

**Interfaces:**
- Produces: `Method.Category string \`json:"category"\``（`"structure"` ｜ `"method"` ｜ `""`）；
  `vocab.Structures() []Method`；`ForLang` 不再返回 `applies_to == "whole"` 的条目。

- [ ] **Step 1: 写失败的测试**

```go
// 同事 2026-09-20 的意见 5：「几种写法不太规范，我找了一些论证结构的思维导图
// 和教辅，可以参考一下这些论证方法规范一下语言」。
//
// 照他给的那两页教辅：论证方法有七种，论证结构有四种，库里各缺一半。
func TestVocabHasTheStandardArgumentMethods(t *testing.T) {
	want := []string{"举例论证", "引用论证", "对比论证", "比喻论证", "因果论证", "类比论证", "归谬论证"}
	have := map[string]bool{}
	for _, m := range vocab.All() {
		have[m.FormalName] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("论证方法缺 %q", w)
		}
	}
}

func TestVocabHasTheFourArgumentStructures(t *testing.T) {
	got := vocab.Structures()
	want := map[string]bool{"总分式": true, "并列式": true, "层进式": true, "对照式": true}
	for _, m := range got {
		delete(want, m.Name)
		if m.Category != "structure" {
			t.Errorf("%s 的 category 该是 structure，得到 %q", m.Name, m.Category)
		}
	}
	if len(want) != 0 {
		t.Errorf("论证结构缺：%v", want)
	}
}

// 🚨 整篇层的那四条**不许漏进按块取方法的那几条路** —— 「这一段用总分式」
// 是句错话，而 ForLang 正是喂给规划和批量引导两个 prompt 的那一份。
func TestWholePieceStructuresStayOutOfPerBlockLists(t *testing.T) {
	for _, m := range vocab.ForLang("zh") {
		if m.AppliesTo == "whole" {
			t.Errorf("整篇层的 %q 漏进了 ForLang", m.Name)
		}
	}
	for _, pos := range []string{"opening", "body", "closing"} {
		for _, m := range vocab.For(pos, "zh") {
			if m.AppliesTo == "whole" {
				t.Errorf("整篇层的 %q 漏进了 For(%q)", m.Name, pos)
			}
		}
	}
}

// 🚨 一个都不许改名：TestWritingPlanSystem_NamesOnlyRealMethods 钉着
// 提示词散文里那几个名字，改名会让那条测试在一个和方法库毫不相干的地方红掉。
func TestVocabKeepsTheNamesPromptsAlreadyUse(t *testing.T) {
	for _, n := range []string{"并列论证", "递进论证", "对比论证", "举个例子", "讲道理",
		"先承认，再反驳", "说清前因后果", "留个悬念", "先抛一个问题", "开门见山",
		"从一件事讲起", "结尾回到开头", "最后提个建议"} {
		found := false
		for _, m := range vocab.All() {
			if m.Name == n {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("现有的名字 %q 不见了 —— 提示词里引着它", n)
		}
	}
}
```

- [ ] **Step 2: 跑，确认失败**

```bash
export PATH="$HOME/sdk/go1.26.0/bin:$PATH"
cd apps/api && CGO_ENABLED=0 go test ./internal/vocab -run 'TestVocabHas|TestWholePiece|TestVocabKeeps' -v
```

- [ ] **Step 3: 加 `Category`，加 `Structures()`，`ForLang`/`For` 排除 whole**

```go
	// Category 把 body 那一层再分成两类，照语文课的分法（同事 2026-09-20 给的
	// 两页教辅）：
	//
	//   "structure" 论证结构 —— 整篇的分论点之间是什么关系（总分式/并列式/
	//                层进式/对照式）。它是**行文那一步**的东西，不是某一段的。
	//   "method"    论证方法 —— 一个看法怎么被证明（举例/引用/对比/比喻/
	//                因果/类比/归谬）。
	//   ""          开篇和结尾的那几种开法收法，以及英文的句式 —— 它们不在
	//                这条轴上。
	//
	// 🚨 加这个字段没有动任何一个现有的 name / formal_name：提示词散文里引着
	// 它们，而 TestWritingPlanSystem_NamesOnlyRealMethods 逐字钉着。
	Category string `json:"category"`
```

`For` 和 `ForLang` 各加一句 `if m.AppliesTo == wholePiece { continue }`。
新增 `Structures()` 只返回 `applies_to == "whole"` 的那四条。

- [ ] **Step 4: 两份 methods.json 一起加条目**

新增四条论证方法（`applies_to: "body"`, `category: "method"`, `lang: "any"`）：

| id | name | formal_name | definition |
|---|---|---|---|
| `point_quote` | 引一句话 | 引用论证 | 引一句名言、一条权威数据来撑住你的看法。引之前先说清它是谁说的，否则读者不知道该信几分。 |
| `point_metaphor` | 打个比方 | 比喻论证 | 拿一件读者熟悉的事来说明一件他不熟悉的事。比方只负责讲清楚，不负责当证据。 |
| `point_analogy` | 类比 | 类比论证 | 两件事在关键的那一点上相同，那么在结论上也该相同。要先说清相同的是哪一点。 |
| `point_absurd` | 顺着他的话往下推 | 归谬论证 | 先假设对方说得对，顺着推下去推出一个荒唐的结果，反过来说明他不对。 |

新增四条论证结构（`applies_to: "whole"`, `category: "structure"`, `lang: "any"`）：

| id | name | formal_name | definition |
|---|---|---|---|
| `struct_total_part` | 总分式 | 总分式 | 先用一段把主张说清楚，后面每一段各撑住它的一面。收尾再回到那句主张就是总—分—总。 |
| `struct_parallel` | 并列式 | 并列式 | 几条分论点地位相同，从不同角度说同一件事。顺序换了不影响读者理解。 |
| `struct_progressive` | 层进式 | 层进式 | 后一条建立在前一条上：是什么 → 为什么 → 怎么办。顺序不能换。 |
| `struct_contrast` | 对照式 | 对照式 | 把正反两面摆在一起，一面写得重、一面作陪衬，差别本身就是论证。 |

并给现有的 body 条目补上 `category`：`point_parallel`/`point_progressive` 仍是
`"method"`（它们说的是**一段**怎么展开，和整篇的并列式／层进式不是一回事，
这一点写进注释），其余 `point_*` 也是 `"method"`；`opening_*` / `closing_*` /
`en_*` 留空。

- [ ] **Step 5: 跑 vocab 全部测试（含两份副本一致那条），提交**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/vocab -v
```
🚨 `TestEmbeddedCopyMatchesSourceOfTruth` 会红，直到两份 json 逐字节一致。

---

### Task 2: 迁移 0184 + `method` 列

**Files:**
- Create: `apps/api/internal/store/migrations/0184_writing_flow.sql`
- Modify: `apps/api/internal/store/queries/writing_atom.sql`（两条写提纲的语句带 `method`）
- Modify: `apps/api/internal/api/writing_outline.go`（DTO + PUT）

- [ ] **Step 1: 迁移**

```sql
-- +goose Up
-- 这一块打算用哪一个论证方法（vocab 的 id，''=还没定）。
-- 行文那一步她给每条分论点标一个；段落那一步的引导据此说「这一段你打算用
-- 举例论证」，而不是每次重新猜。
ALTER TABLE writing_outline ADD COLUMN method text NOT NULL DEFAULT '';

-- writing.structure_key 改作「整篇的论证结构」（struct_* 的 id）。
--
-- 🚨 这一列自 2026-08-27 模板选择器被删之后就**没有任何代码写过它**，
-- 留下的是那之前存的骨架 id。那套 id 和新的闭表没有交集，留着只会让
-- 行文那一步一进去就显示一个不存在的选择。清掉。
UPDATE writing SET structure_key = '' WHERE structure_key <> '';

-- +goose Down
ALTER TABLE writing_outline DROP COLUMN method;
```

- [ ] **Step 2-4:** sqlc 重新生成、DTO 加 `Method string \`json:"method"\``、
  PUT 收 `method`（不在 vocab 里的 id 落库前清空），跑 `go test ./...`，提交。

---

### Task 3: 行文这一步（服务端）

**Files:**
- Create: `apps/api/internal/api/writing_flow.go` + test
- Modify: `apps/api/internal/api/writing_stage.go`

**Interfaces:**
- Produces: `writingFlowStructures()`（从 vocab 取那四条）；
  `PUT /api/v1/writings/{id}/flow` 收 `{structureKey, methods: {outlineId: methodId}}`。

- [ ] **Step 1: 写失败的测试**

```go
func TestWritingStageAcceptsFlow(t *testing.T) {
	if !writingStageValid("flow") {
		t.Error("flow 该是一个合法的 stage")
	}
}

// 🚨 编出来的结构 id 落库前要丢掉 —— 同 kind、同 verdict，一个道理。
func TestFlowRejectsAnInventedStructure(t *testing.T) {
	if writingFlowStructureValid("我编的一种") {
		t.Error("不在闭表里的结构 id 不该合法")
	}
	if !writingFlowStructureValid("struct_total_part") {
		t.Error("总分式该合法")
	}
}
```

- [ ] **Step 2-4:** 实现、跑、提交。
  `writing_stage.go` 的枚举加 `"flow"`；
  `PUT /flow` 校验 structureKey 在闭表里、每个 method 在 vocab 里且
  `category == "method"`，不合格的清空而不是整份拒绝。

---

### Task 4: 行文那块板（前端）

**Files:**
- Create: `apps/lite-web/src/writings/flowStructures.ts` + `.test.ts`
- Create: `apps/lite-web/src/writings/FlowStage.tsx`
- Modify: `StageMap.tsx`、`WritingRoomHost.tsx`、`api/writingRoom.ts`

- [ ] **Step 1-5**

板上三件事，**都不填内容**（同事：「并非填充内容」）：

1. **顺序**：分论点可以拖着换先后 —— 复用 R1 的 `moveOutlineNode`（`"after"` 模式）。
2. **整篇的论证结构**：四张卡选一张（总分式／并列式／层进式／对照式），
   每张卡底下一句话说它是什么。🚨 这不是「从库里挑一副骨架往里填」那个被
   2026-08-27 否掉的东西 —— 那次否的是**在她想之前**让她挑；这一步发生在
   她的分论点都有了之后，选的是**她已经摆出来的那些点之间是什么关系**。
3. **每条分论点用哪个论证方法**：一个下拉（`teacher/controls/Select` 那套，
   不用原生 select —— 2026-09-17 的裁定）。

底栏一句「这一步只定组织方式，正文在下一步写」，按钮「完成，去写段落」。

- [ ] **Step 6: 真浏览器看一遍**（看图台那套，`e2e/harness/`）。

---

### Task 5: 段落页左栏顶上那张图（意见 5 的 UI）

**Files:**
- Create: `apps/lite-web/src/writings/MiniMap.tsx`
- Modify: `apps/lite-web/src/writings/SnippetsStage.tsx`

- [ ] **Step 1-4**

同事截图里的红箭头指着左栏顶部：「我觉得可以在这里保留刚刚的思维导图，
然后把引导往下放」。

- 缩略图：复用 `MindMap`，只读（不传 `onMove` / `onRemove` / `onEdit`），
  高度约 180px，**当前这张卡高亮**。
- 点开放大：一个全屏浮层，里面是正常尺寸的 `MindMap`（仍然只读 —— 这一步
  不是改结构的地方，改结构回「结构」那一步）。
- 引导整体下移到图下面。左栏本来就能折起，不占正文宽度。

---

### Task 6: 引导框改成「段落内结构」（意见 5 的第二点）

**Files:**
- Modify: `apps/api/internal/api/writing_guide.go`、`apps/lite-web/src/writings/GuideBox.tsx`

- [ ] **Step 1-4**

同事：「到了分段写作的界面，需要组织的内容为一个段落内的文本结构」。

「常见的几种写法」那一节改成**这一段里的几步**，按块的种类给：

| kind | 段内结构 |
|---|---|
| `opening` | 把话题带出来 → 亮出中心论点 → 告诉读者后面要讲几件事 |
| `point`/`counter` | 分论点句 → 论据 → 分析（它凭什么证明）→ 回扣中心论点 |
| `rebuttal` | 承认对方那一点 → 指出它的适用范围 → 回到自己的结论 |
| `closing` | 回到中心论点 → 说得比开头更准 → 落到一件具体的事上 |

依据是同事给的那页教辅的「分析段内层次」。
这一节是**确定性渲染**的（按 kind 查表），不花模型调用；
模型那一份只管 `job` / `method_ids` / `questions`。

---

## 自查

| 规格节（spec §7） | 任务 |
|---|---|
| §7.1 第四步「行文」 | Task 2、3、4 |
| §7.2 方法库拆成两张表 + 补四种论证方法 | Task 1 |
| §7.3 段落页留图 | Task 5 |
| §7.3 引导改成段落内结构 | Task 6 |

**不许做的**（写在这里免得手滑）：改任何现有 name / formal_name；
把论证结构做成「她还没想就先挑一副骨架」；给段落页的图加编辑能力。
