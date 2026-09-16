# 教师端工作台 + 布置作业 AI 模式（Part D1）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建起「对话 + 画布」的壳与服务端工具循环，并在布置作业页上做出 AI 模式：左边对话（回复以选项为主），右边作业卡（每一格可直接改），点一下发布。

**Architecture:** 新增一个所有教师面共用的端点 `POST /api/v1/lite/teacher/workspace/turn`。纯逻辑（工具闭集、名字校验、时间换算、选项截断）放在新包 `apps/api/internal/liteworkspace`，handler 放 `apps/api/internal/api/lite_teacher_workspace.go`。前端纯逻辑放 `apps/lite-web/src/teacher/workspace/workspaceLogic.ts`，壳组件 `WorkspacePanel.tsx` 只管布局与对话，画布由调用方以 children 传入——D2／D3 复用同一个壳。

**Tech Stack:** Go（`net/http` + `pgx`/`sqlc`，`internal/gateway` 的 tool-call 循环）、React + TypeScript + Vite。

**Spec:** `docs/superpowers/specs/2026-09-16-lite-teacher-workspace-design.md`（§3–§6、§5.1、§8）

## Global Constraints

- **每次 LLM 调用声明能力档并计量。** 对话轮走 `gateway.ClassDialogue`，**不使用 `ClassAssess`、不使用旗舰档**。调用后用 `a.recordLiteLLMCall(ctx, u.ID, uuid.Nil, purpose, resolved, usage)` 记一行（`atomID` 传 `uuid.Nil`，工作台不属于任何 atom）。
- **上下文按页面与登录隔离。** 一轮只带：教师身份、当前班级、画布对象、该线程最近 `TurnsWindow = 8` 轮。绝不携带另一个页面的线程。
- **数字与姓名不由模型写**（§6）：具体人数与姓名由工具结果渲染成卡，模型的话只指向卡。唯一校验是姓名校验，**不要加数字检测器**。
- **教师可见性边界：** 工具可读学生产出的一切，**绝不读学生与印记的对话记录**。边界写在工具里，不写在 prompt 里。
- **错误必须显形：** 模型失败、解析失败、工具失败一律「动词+失败：{后台原话}」，不得返回兜底话。
- **界面文案守 AGENTS.md §界面文案怎么写**（标签是名词；按钮写做什么／做完了；状态用已／待／处理中；不写文学腔，含代码注释与 commit message）。
- **只写逻辑测试**，不写前端渲染测试。UI 用一次性 Playwright harness 看截图，看完删掉。
- **lite 不得破坏 pro：** 不动 `apps/web`；新建文件前先确认同名文件不存在。
- `mk-*` token 不用 Tailwind alpha 语法（用 `color-mix`／`linear-gradient`）；不加左侧色条。
- Go 测试：`CGO_ENABLED=0 go test ./internal/api -count=1 -timeout 1800s`。
- 时间一律按**东八区固定偏移**换算，不用 `time.LoadLocation`（distroless 镜像没有 tzdata，会静默退回 UTC）。

---

## File Structure

| 文件 | 责任 |
|---|---|
| `apps/api/internal/liteworkspace/workspace.go` | 常量、类型、闭集、纯函数（筛学生、搜库、名字校验、时间换算、选项截断） |
| `apps/api/internal/liteworkspace/workspace_test.go` | 上述纯函数的单元测试 |
| `apps/api/internal/liteworkspace/tools.go` | 工具的 JSON schema（`[]gateway.ChatTool`）与系统提示词 |
| `apps/api/internal/api/lite_teacher_workspace.go` | 端点：鉴权、归属、工具循环、计量、错误 |
| `apps/api/internal/api/lite_teacher_workspace_test.go` | 端点测试（stub provider + testcontainers） |
| `apps/api/internal/api/lite_teacher_routes.go` | 挂一条路由 |
| `apps/lite-web/src/api/teacherWorkspace.ts` | 客户端 + 响应 normalizer |
| `apps/lite-web/src/teacher/workspace/workspaceLogic.ts` | 纯逻辑：`applyPatch`、`trimTurns`、`clampChoices` |
| `apps/lite-web/src/teacher/workspace/workspaceLogic.test.ts` | 上述的 vitest |
| `apps/lite-web/src/teacher/workspace/WorkspacePanel.tsx` | 壳：左对话右画布，画布走 children |
| `apps/lite-web/src/teacher/AssignmentAIMode.tsx` | AI 模式页：壳 + 作业卡 + 发布 |
| `apps/lite-web/src/teacher/AssignmentForm.tsx` | 模式切换，两模式共用同一个 `AssignmentDraft` |

---

### Task 1: `liteworkspace` 包的纯逻辑

**Files:**
- Create: `apps/api/internal/liteworkspace/workspace.go`
- Create: `apps/api/internal/liteworkspace/workspace_test.go`

**Interfaces:**
- Produces: 下列常量、类型与函数。Task 2、3 全部消费它们，名字与签名不要改。

- [ ] **Step 1: 写失败的测试**

`apps/api/internal/liteworkspace/workspace_test.go`：

```go
package liteworkspace

import (
	"testing"
	"time"
)

func TestFilterStudentsClosedSet(t *testing.T) {
	rows := []Student{
		{ID: "a", Name: "林知遥", ActiveDaysThisWeek: 0, OverdueAssignments: 1, WritingsDone: 0},
		{ID: "b", Name: "陈屿", ActiveDaysThisWeek: 3, OverdueAssignments: 0, WritingsDone: 2},
		{ID: "c", Name: "周未", ActiveDaysThisWeek: 0, OverdueAssignments: 0, WritingsDone: 1},
	}
	for _, tc := range []struct {
		filter StudentFilter
		want   []string
	}{
		{FilterAll, []string{"a", "b", "c"}},
		{FilterInactiveThisWeek, []string{"a", "c"}},
		{FilterHasOverdue, []string{"a"}},
		{FilterNoWritingYet, []string{"a"}},
	} {
		got := FilterStudents(rows, tc.filter)
		if len(got) != len(tc.want) {
			t.Fatalf("%s: got %d rows, want %d", tc.filter, len(got), len(tc.want))
		}
		for i, id := range tc.want {
			if got[i].ID != id {
				t.Fatalf("%s: row %d is %q, want %q", tc.filter, i, got[i].ID, id)
			}
		}
	}
}

func TestFilterStudentsRejectsUnknownFilter(t *testing.T) {
	if _, ok := ParseStudentFilter("below_tier"); ok {
		t.Fatal("below_tier has no query behind it and must not parse")
	}
	if f, ok := ParseStudentFilter("has_overdue"); !ok || f != FilterHasOverdue {
		t.Fatalf("has_overdue did not parse, got %q ok=%v", f, ok)
	}
}

// 姓名校验：花名册里的名字出现在回复里，就必须是这一轮工具给过的，
// 或者老师自己打过的。模型自己上一轮说过的话不算数据来源。
func TestUngroundedNames(t *testing.T) {
	roster := []string{"林知遥", "陈屿", "周未"}
	got := UngroundedNames("林知遥和陈屿这周都没有写作。", roster, []string{"林知遥"})
	if len(got) != 1 || got[0] != "陈屿" {
		t.Fatalf("got %v, want [陈屿]", got)
	}
	if n := UngroundedNames("这三位学生本周还没有写作。", roster, nil); len(n) != 0 {
		t.Fatalf("a reply that names nobody is grounded, got %v", n)
	}
}

func TestBeijingWallToUTC(t *testing.T) {
	got, err := BeijingWallToUTC("2026-09-20T18:00")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	want := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
	if _, err := BeijingWallToUTC("周五下午"); err == nil {
		t.Fatal("a relative phrase must fail, not silently become now")
	}
}

func TestClampChoices(t *testing.T) {
	in := []Choice{{ID: "1", Label: "论证结构"}, {ID: "2", Label: ""}, {ID: "3", Label: "证据使用"},
		{ID: "4", Label: "语言表达"}, {ID: "5", Label: "篇幅"}, {ID: "6", Label: "体裁"}}
	got := ClampChoices(in)
	if len(got) != MaxChoices {
		t.Fatalf("got %d choices, want %d", len(got), MaxChoices)
	}
	for _, c := range got {
		if c.Label == "" {
			t.Fatal("an empty label renders as a blank button; it must be dropped")
		}
	}
}

func TestTrimTurnsKeepsTheMostRecent(t *testing.T) {
	var in []Turn
	for i := 0; i < 20; i++ {
		in = append(in, Turn{Role: "teacher", Text: string(rune('a' + i))})
	}
	got := TrimTurns(in)
	if len(got) != TurnsWindow {
		t.Fatalf("got %d turns, want %d", len(got), TurnsWindow)
	}
	if got[len(got)-1].Text != in[len(in)-1].Text {
		t.Fatal("trimming must keep the LAST turns, not the first")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/liteworkspace -count=1`
Expected: FAIL，报 `undefined: Student` 等

- [ ] **Step 3: 写实现**

`apps/api/internal/liteworkspace/workspace.go`：

```go
// Package liteworkspace holds the teacher workspace's pure logic: the closed
// sets a tool call may name, and the checks that keep a reply grounded.
// It touches no database and no model, so every rule here is unit-testable.
package liteworkspace

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"mindimprint/api/internal/library"
)

// TurnsWindow bounds the transcript one turn carries. lite has no compaction
// layer, so this window is the only thing bounding prompt growth.
const TurnsWindow = 8

// ToolLoopMax bounds tool round-trips inside one turn. Hitting it fails the
// turn with a visible error rather than truncating silently.
const ToolLoopMax = 4

// MaxChoices bounds the option buttons a reply may carry.
const MaxChoices = 4

// BeijingOffset is a fixed offset, not a named zone: the distroless runtime
// image ships no tzdata and LoadLocation there fails back to UTC in silence.
var BeijingOffset = time.FixedZone("UTC+8", 8*60*60)

type Turn struct {
	Role string `json:"role"` // "teacher" | "ai"
	Text string `json:"text"`
}

type Choice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Student is the workspace's view of one roster row — only the fields a
// closed-set filter reads. The api layer maps its roster DTO into this.
type Student struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	ActiveDaysThisWeek int    `json:"activeDaysThisWeek"`
	OverdueAssignments int    `json:"overdueAssignments"`
	WritingsDone       int    `json:"writingsDone"`
}

// StudentFilter is the closed set list_students accepts. Every member must be
// answerable from a roster row above. Do NOT add a filter whose data we do not
// already have — adding the query comes first.
type StudentFilter string

const (
	FilterAll              StudentFilter = "all"
	FilterInactiveThisWeek StudentFilter = "inactive_this_week"
	FilterHasOverdue       StudentFilter = "has_overdue"
	FilterNoWritingYet     StudentFilter = "no_writing_yet"
)

func ParseStudentFilter(s string) (StudentFilter, bool) {
	switch StudentFilter(s) {
	case FilterAll, FilterInactiveThisWeek, FilterHasOverdue, FilterNoWritingYet:
		return StudentFilter(s), true
	}
	return "", false
}

// FilterStudents keeps the roster's order so the card reads the same way twice.
func FilterStudents(rows []Student, f StudentFilter) []Student {
	out := make([]Student, 0, len(rows))
	for _, r := range rows {
		keep := false
		switch f {
		case FilterAll:
			keep = true
		case FilterInactiveThisWeek:
			keep = r.ActiveDaysThisWeek == 0
		case FilterHasOverdue:
			keep = r.OverdueAssignments > 0
		case FilterNoWritingYet:
			keep = r.WritingsDone == 0
		}
		if keep {
			out = append(out, r)
		}
	}
	return out
}

// UngroundedNames returns the roster names a reply states that this turn's
// tools never returned and the teacher never typed.
//
// The check runs over the roster — a closed, exactly-comparable set — instead
// of trying to spot "a name" in free text. `grounded` is what the tools
// returned plus what the teacher herself wrote; it deliberately excludes the
// model's own earlier turns, since those are where a fabrication comes from.
//
// Only names are checked, never digits: an article title or a year would make
// a digit check fire on correct output.
func UngroundedNames(reply string, roster, grounded []string) []string {
	ok := make(map[string]bool, len(grounded))
	for _, g := range grounded {
		ok[g] = true
	}
	var bad []string
	for _, name := range roster {
		if name == "" || ok[name] {
			continue
		}
		if strings.Contains(reply, name) {
			bad = append(bad, name)
		}
	}
	sort.Strings(bad)
	return bad
}

// BeijingWallToUTC reads a wall-clock time the model wrote as Beijing time
// ("2026-09-20T18:00") and returns the instant. A relative phrase is an error:
// the model is told today's Beijing date and resolves 周五 itself, so anything
// that is not an absolute time is a bug we surface rather than guess at.
func BeijingWallToUTC(wall string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02T15:04", strings.TrimSpace(wall), BeijingOffset)
	if err != nil {
		return time.Time{}, fmt.Errorf("截止时间不是绝对时刻：%q", wall)
	}
	return t.UTC(), nil
}

// ClampChoices drops blank labels and keeps at most MaxChoices.
func ClampChoices(in []Choice) []Choice {
	out := make([]Choice, 0, MaxChoices)
	for _, c := range in {
		if strings.TrimSpace(c.Label) == "" {
			continue
		}
		out = append(out, c)
		if len(out) == MaxChoices {
			break
		}
	}
	return out
}

// TrimTurns keeps the most recent TurnsWindow turns.
func TrimTurns(in []Turn) []Turn {
	if len(in) <= TurnsWindow {
		return in
	}
	return in[len(in)-TurnsWindow:]
}

// SearchLibrary filters the embedded catalogue. query matches either title
// case-insensitively; tier 0 means "any tier".
func SearchLibrary(arts []library.Article, query string, disciplines []string, tier, limit int) []library.Article {
	q := strings.ToLower(strings.TrimSpace(query))
	want := make(map[string]bool, len(disciplines))
	for _, d := range disciplines {
		want[d] = true
	}
	out := make([]library.Article, 0, limit)
	for _, a := range arts {
		if q != "" && !strings.Contains(strings.ToLower(a.Title), q) && !strings.Contains(strings.ToLower(a.ZhTitle), q) {
			continue
		}
		if len(want) > 0 {
			hit := false
			for _, d := range a.Disciplines {
				if want[d] {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
		}
		if tier != 0 {
			if _, ok := a.LevelAt(tier); !ok {
				continue
			}
		}
		out = append(out, a)
		if len(out) == limit {
			break
		}
	}
	return out
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/liteworkspace -count=1 && gofmt -l internal/liteworkspace && go vet ./internal/liteworkspace`
Expected: 测试 PASS；`gofmt -l` 不输出文件名；vet 无输出

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/liteworkspace/workspace.go apps/api/internal/liteworkspace/workspace_test.go
git commit -m "feat(api): teacher workspace closed sets, name grounding and Beijing time"
```

---

### Task 2: 工具 schema 与系统提示词

**Files:**
- Create: `apps/api/internal/liteworkspace/tools.go`
- Create: `apps/api/internal/liteworkspace/tools_test.go`

**Interfaces:**
- Consumes: Task 1 的 `StudentFilter` 闭集、`MaxChoices`。
- Produces: `AssignmentTools() []gateway.ChatTool`、`AssignmentSystem(ctx SystemContext) string`、`type SystemContext struct{ ClassName string; TodayBeijing string; StudentCount int }`。

- [ ] **Step 1: 写失败的测试**

`apps/api/internal/liteworkspace/tools_test.go`：

```go
package liteworkspace

import "testing"

// 工具名与闭集必须和 Task 1 的解析器对得上。工具 schema 里写了一个
// ParseStudentFilter 不认的值，模型就会照着 schema 交出一个我们会拒绝的调用。
func TestAssignmentToolsDeclareOnlyParsableFilters(t *testing.T) {
	var listTool *struct {
		found bool
		enum  []any
	}
	_ = listTool
	for _, tool := range AssignmentTools() {
		if tool.Name != "list_students" {
			continue
		}
		props, _ := tool.Parameters["properties"].(map[string]any)
		filter, _ := props["filter"].(map[string]any)
		enum, _ := filter["enum"].([]any)
		if len(enum) == 0 {
			t.Fatal("list_students must constrain filter with an enum")
		}
		for _, v := range enum {
			s, _ := v.(string)
			if _, ok := ParseStudentFilter(s); !ok {
				t.Fatalf("schema offers filter %q that ParseStudentFilter rejects", s)
			}
		}
		return
	}
	t.Fatal("list_students tool is missing")
}

func TestAssignmentToolsAreNamedOnce(t *testing.T) {
	seen := map[string]bool{}
	for _, tool := range AssignmentTools() {
		if seen[tool.Name] {
			t.Fatalf("tool %q declared twice", tool.Name)
		}
		seen[tool.Name] = true
	}
	for _, want := range []string{"set_fields", "search_library", "set_material", "list_students", "set_recipients", "ask_choice"} {
		if !seen[want] {
			t.Fatalf("tool %q is missing", want)
		}
	}
}

// 系统提示词必须带上今天的北京日期——模型要靠它把「周五」算成绝对时刻，
// 而 BeijingWallToUTC 只收绝对时刻。
func TestAssignmentSystemCarriesToday(t *testing.T) {
	s := AssignmentSystem(SystemContext{ClassName: "初三二班", TodayBeijing: "2026-09-16", StudentCount: 12})
	if !contains(s, "2026-09-16") {
		t.Fatal("system prompt must state today's Beijing date")
	}
	if !contains(s, "初三二班") {
		t.Fatal("system prompt must name the class")
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/liteworkspace -count=1 -run TestAssignment`
Expected: FAIL，报 `undefined: AssignmentTools`

- [ ] **Step 3: 写实现**

`apps/api/internal/liteworkspace/tools.go`。六个工具的 schema 用 `map[string]any` 写 JSON Schema；`SystemContext` 与提示词如下。

提示词要点（照写，不要改成散文）：

```go
// SystemContext is what the assignment prompt needs to know that the tools
// cannot tell it.
type SystemContext struct {
	ClassName    string
	TodayBeijing string // "2006-01-02"
	StudentCount int
}

const assignmentSystemTemplate = `你在帮一位老师布置作业。你的输出会填进右边的作业卡，老师看一眼就发布。

现在是北京时间 %s。班级是%s，共 %d 名学生。

## 你怎么问

- **一轮只问一个问题。** 问的时候尽量给选项，让老师点，而不是让她打字。
  给选项就调 ask_choice，一次 2 到 4 个。选项是名词或短动宾，不要写成句子。
- 老师已经说清楚的事不要再问。她说「这周读气候变化写议论文周五交」，
  你就直接把这些填进去，只问她还没说的那一件。
- 说话要短。不超过 120 个字。

## 硬规矩

- **不要在回复里写学生人数和学生姓名。** 它们会显示在卡片上，由系统查出来。
  你要指代的时候就说「这些学生」「名单上的学生」。
- **截止时间必须是绝对时刻**，格式 2006-01-02T15:04，按北京时间写。
  老师说「周五」，你按上面的今天算出是哪一天，自己写成绝对时刻。
- 材料只能从分级阅读库里选（先 search_library 再 set_material），
  或者设成个性化阅读。不要编造文章标题。
- 你改不了的事不要说你改了。`

func AssignmentSystem(c SystemContext) string {
	return fmt.Sprintf(assignmentSystemTemplate, c.TodayBeijing, c.ClassName, c.StudentCount)
}
```

工具 schema 逐个写：

- `set_fields`：`kind`（enum `reading`/`writing`/`reading_writing`，取值以 `AssignmentKind` 在 `packages/contracts` 里的定义为准，**打开确认后照抄**）、`title`、`instructions`、`dueAt`（string，格式说明写进 description）。
- `search_library`：`query`（string）、`disciplines`（string 数组）、`tier`（integer 1–5，0 表示不限）。
- `set_material`：`source`（enum `library`/`personalized`）、`slug`（string）、`tier`（integer，可空）。
- `list_students`：`filter`（enum，取值**必须**是 `ParseStudentFilter` 认的四个：`all`、`inactive_this_week`、`has_overdue`、`no_writing_yet`）。
- `set_recipients`：`userIds`（string 数组）。
- `ask_choice`：`question`（string）、`options`（对象数组，每项 `id` + `label`），description 里写明最多 4 个。

- [ ] **Step 4: 跑测试确认通过**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/liteworkspace -count=1 && gofmt -l internal/liteworkspace`
Expected: PASS；gofmt 无输出

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/liteworkspace/tools.go apps/api/internal/liteworkspace/tools_test.go
git commit -m "feat(api): assignment tool schemas and the prompt that keeps names off the reply"
```

---

### Task 3: 端点与工具循环

**Files:**
- Create: `apps/api/internal/api/lite_teacher_workspace.go`
- Create: `apps/api/internal/api/lite_teacher_workspace_test.go`
- Modify: `apps/api/internal/api/lite_teacher_routes.go`

**Interfaces:**
- Consumes: Task 1、2 的一切；`a.authTeacherClass(w, r) (sqlc.Class, bool)`；`a.routeE(ctx, gateway.ClassDialogue) (gateway.Resolved, error)`；`a.recordLiteLLMCall(ctx, userID, atomID uuid.UUID, purpose string, resolved gateway.Resolved, usage gateway.ChatUsage)`。
- Produces: `POST /api/v1/lite/teacher/workspace/turn` 与 §4.4 的请求／响应形状。

- [ ] **Step 1: 挂路由**

在 `registerLiteTeacherRoutes` 的作业那一段后面加一行，并按该文件既有的风格写一句注释说明它**会**调模型（相邻两段注释分别写了「A GET never calls a model」与「No route calls a model」，这条是例外，要说清楚）：

```go
	// 教师工作台。这是教师端唯一一条同步调模型的路由：一轮对话 + 最多
	// liteworkspace.ToolLoopMax 次工具往返，档位 dialogue。
	mux.Handle("POST /api/v1/lite/teacher/workspace/turn", liteTeacher(a.postLiteTeacherWorkspaceTurn))
```

🚨 班级归属不在路径里，由请求体的 `classId` 带。`authTeacherClass` 读的是路径参数 `{id}`，**这条路由没有 `{id}`**，所以本 handler 要自己解析 `classId` 并做同样的归属校验——打开 `lite_weekly.go:731` 的 `authTeacherClass` 照它的查询写一个按 body 取 id 的版本，不要把它改成兼容两种来源（那会动到四个已上线的调用点）。

- [ ] **Step 2: 写失败的测试**

`apps/api/internal/api/lite_teacher_workspace_test.go`。用仓库既有的 stub provider 辅助（参考 `reading_coach_test.go` 的 `liteHandlerWithProvider`，照它的用法写）。至少覆盖：

```go
// 未登录 401；学生 403；不教这个班的老师 403。
func TestWorkspaceTurnRejectsOutsiders(t *testing.T) { /* … */ }

// 工具循环打满仍不收敛 → 整轮失败并带可读原话，不截断、不返回半个答案。
func TestWorkspaceTurnFailsWhenToolLoopExhausted(t *testing.T) {
	// stub provider：每一轮都回一个 tool_call，永不给最终答复。
	// 断言：状态码非 2xx，且响应里带「工具调用次数超出上限」。
}

// 模型说了一个这一轮工具没给过、老师也没打过的学生姓名 → 整轮失败，不渲染。
func TestWorkspaceTurnRejectsUngroundedName(t *testing.T) {
	// 班里有「林知遥」；stub 回复里写「林知遥这周没写作」，且没调 list_students。
	// 断言：非 2xx；响应里点名是哪个名字没有依据。
}

// 选项超过 4 个被截断到 4，空 label 被丢掉。
func TestWorkspaceTurnClampsChoices(t *testing.T) { /* … */ }

// 每一轮都记一行 llm_call，档位是 dialogue，不是 assess。
func TestWorkspaceTurnMetersOnDialogue(t *testing.T) { /* … */ }
```

- [ ] **Step 3: 跑测试确认失败**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -count=1 -timeout 1800s -run TestWorkspaceTurn`
Expected: FAIL（handler 还不存在）

- [ ] **Step 4: 写 handler**

`apps/api/internal/api/lite_teacher_workspace.go` 的骨架与不变量：

1. 解析请求体 → `surface`、`classId`、`artifact`、`turns`、`text`／`choiceId`。`surface` 目前只接受 `"assignment"`，其余返回 400（D2／D3 再开）。
2. 归属校验（Step 1 那个按 body 取 id 的版本）。
3. `turns = liteworkspace.TrimTurns(turns)`。
4. `resolved, err := a.routeE(ctx, gateway.ClassDialogue)`；失败 → `httpx.ErrInternal()`。
5. 取花名册一次，映射成 `[]liteworkspace.Student`；`roster` 的姓名列表留着做 Step 6 的校验。
6. 工具循环，最多 `liteworkspace.ToolLoopMax` 次：
   - 调 `a.d.Provider` 的 Complete，带 `AssignmentTools()`；
   - `StopToolCall` → 逐个执行工具，把结果作为 `RoleTool` 消息追加，继续；
   - 否则收尾。
   - **每一次调用都要 `recordLiteLLMCall`**，包括没产出的那次——它一样花钱。
   - 循环打满仍是 `StopToolCall` → 返回错误「对话失败：工具调用次数超出上限」。
7. 姓名校验：`grounded` = 这一轮 `list_students` 返回的姓名 ∪ 老师本轮与历史里打过的字里出现的花名册姓名。`UngroundedNames` 非空 → 整轮失败，错误里点名是哪几个名字。
8. `ClampChoices`。
9. 组装响应：`reply`、`choices`、`patch`（只含工具真的写过的字段）、`cards`（`list_students` 的结果）。

🚨 `set_material` 选 `personalized` 时**只写 patch**，不在这里算每个学生的文章——个性化的选人走已有的 `previewLitePersonalizedReading`（C 部分），不要在这里重复实现。

🚨 工具执行绝不返回学生与印记的对话记录。本任务用到的工具都不碰那张表；后续加工具时这条仍然成立。

- [ ] **Step 5: 跑测试确认通过**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -count=1 -timeout 1800s -run TestWorkspaceTurn && gofmt -l internal/api && go vet ./internal/api`
Expected: 全 PASS；gofmt 无输出

- [ ] **Step 6: 跑整包回归**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -count=1 -timeout 1800s`
Expected: PASS（确认没碰坏已上线的教师路由）

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/api/lite_teacher_workspace.go \
        apps/api/internal/api/lite_teacher_workspace_test.go \
        apps/api/internal/api/lite_teacher_routes.go
git commit -m "feat(api): teacher workspace turn endpoint with a bounded tool loop"
```

---

### Task 4: 前端纯逻辑

**Files:**
- Create: `apps/lite-web/src/teacher/workspace/workspaceLogic.ts`
- Create: `apps/lite-web/src/teacher/workspace/workspaceLogic.test.ts`

**Interfaces:**
- Produces: `applyPatch<T>(current: T, snapshot: T, patch: Partial<T>): { next: T; kept: (keyof T)[] }`、`trimTurns(turns: Turn[]): Turn[]`、`clampChoices(cs: Choice[]): Choice[]`、`TURNS_WINDOW = 8`、`MAX_CHOICES = 4`。

- [ ] **Step 1: 写失败的测试**

```ts
import { describe, expect, it } from "vitest";
import { applyPatch, clampChoices, trimTurns, TURNS_WINDOW, MAX_CHOICES } from "./workspaceLogic";

describe("applyPatch", () => {
  // 一轮在飞的时候老师改了截止时间：patch 不能把她的修改抹掉。
  it("keeps a field the teacher edited while the turn was in flight", () => {
    const snapshot = { title: "旧标题", dueInput: "2026-09-20T18:00" };
    const current = { title: "旧标题", dueInput: "2026-09-21T09:00" }; // 老师手改过
    const { next, kept } = applyPatch(current, snapshot, { title: "新标题", dueInput: "2026-09-22T18:00" });
    expect(next.title).toBe("新标题");
    expect(next.dueInput).toBe("2026-09-21T09:00");
    expect(kept).toEqual(["dueInput"]);
  });

  it("applies every field when the teacher changed nothing", () => {
    const snapshot = { title: "旧", dueInput: "" };
    const { next, kept } = applyPatch(snapshot, snapshot, { title: "新" });
    expect(next.title).toBe("新");
    expect(kept).toEqual([]);
  });

  it("is identity for an empty patch", () => {
    const cur = { title: "甲", dueInput: "" };
    expect(applyPatch(cur, cur, {}).next).toEqual(cur);
  });
});

describe("trimTurns", () => {
  it("keeps the most recent TURNS_WINDOW turns", () => {
    const turns = Array.from({ length: 20 }, (_, i) => ({ role: "teacher" as const, text: String(i) }));
    const got = trimTurns(turns);
    expect(got).toHaveLength(TURNS_WINDOW);
    expect(got[got.length - 1].text).toBe("19");
  });

  it("leaves a short thread alone", () => {
    const turns = [{ role: "teacher" as const, text: "a" }];
    expect(trimTurns(turns)).toEqual(turns);
  });
});

describe("clampChoices", () => {
  it("drops blank labels and caps at MAX_CHOICES", () => {
    const got = clampChoices([
      { id: "1", label: "论证结构" }, { id: "2", label: "  " }, { id: "3", label: "证据使用" },
      { id: "4", label: "语言表达" }, { id: "5", label: "篇幅" }, { id: "6", label: "体裁" },
    ]);
    expect(got).toHaveLength(MAX_CHOICES);
    expect(got.every((c) => c.label.trim() !== "")).toBe(true);
  });
});
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd apps/lite-web && npx vitest run src/teacher/workspace/workspaceLogic.test.ts`
Expected: FAIL，模块不存在

- [ ] **Step 3: 写实现**

`applyPatch` 的规则（这是本任务唯一有分量的逻辑）：逐个 patch 字段比对 `current[k]` 与 `snapshot[k]`；不等 = 老师在这一轮进行中手改过 → 丢弃该字段的 patch，记进 `kept`；相等 → 应用。

- [ ] **Step 4: 跑测试确认通过**

Run: `cd apps/lite-web && npx vitest run src/teacher/workspace/workspaceLogic.test.ts && npx tsc --noEmit`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/src/teacher/workspace/workspaceLogic.ts apps/lite-web/src/teacher/workspace/workspaceLogic.test.ts
git commit -m "feat(lite-web): workspace patch keeps the field the teacher just edited"
```

---

### Task 5: 客户端与壳组件

**Files:**
- Create: `apps/lite-web/src/api/teacherWorkspace.ts`
- Create: `apps/lite-web/src/teacher/workspace/WorkspacePanel.tsx`

**Interfaces:**
- Consumes: Task 4 的 `trimTurns`、`clampChoices`；Task 3 的端点。
- Produces: `postWorkspaceTurn(input): Promise<WorkspaceTurn>`；`WorkspacePanel({ turns, busy, error, choices, onSend, onChoose, children })`——`children` 就是画布，D2／D3 传别的东西进来。

- [ ] **Step 1: 客户端**

`apps/lite-web/src/api/teacherWorkspace.ts`：按 `apps/lite-web/src/api/` 既有文件的写法（`apiFetch` + 一个 normalizer）实现，响应按 §4.4 归一化。错误沿用既有的 `errorText`／`failText`，不要自己拼错误字符串。

- [ ] **Step 2: 壳组件**

`WorkspacePanel.tsx`：
- 外层 `TeacherPage width="full"`（这一档的注释写着它就是「报告编辑器的两栏」，形状一样，不新增测量档）。
- 宽屏两栏：左栏 `min-width:320px; max-width:420px`，右栏吃剩下的宽。窄屏（`< 900px`）改上下两段，**画布在上、对话在下**。
- 左栏：消息列表（复用 `apps/lite-web/src/readings/LiteChatMarkdown.tsx` 渲染 AI 的话，不要另写一个 markdown 渲染器）、选项按钮行、输入框 + 发送。
- 选项按钮：点一下即发出该选项，发出后这一行选项消失（她已经答过了）。
- `busy` 时输入框与选项禁用，显示「处理中」。
- `error` 用「动词+失败：{后台原话}」，并给一颗重试。
- 右栏：直接渲染 `children`。

🚨 不要把画布做成只读——`children` 由调用方给，调用方负责让每一格可编辑。

- [ ] **Step 3: typecheck**

Run: `cd apps/lite-web && npx tsc --noEmit && npx vitest run`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add apps/lite-web/src/api/teacherWorkspace.ts apps/lite-web/src/teacher/workspace/WorkspacePanel.tsx
git commit -m "feat(lite-web): workspace shell with the conversation left and the canvas right"
```

---

### Task 6: AI 模式页与模式切换

**Files:**
- Create: `apps/lite-web/src/teacher/AssignmentAIMode.tsx`
- Modify: `apps/lite-web/src/teacher/AssignmentForm.tsx`

**Interfaces:**
- Consumes: Task 5 的 `WorkspacePanel`、`postWorkspaceTurn`；Task 4 的 `applyPatch`；既有的 `AssignmentDraft`、`buildCreateInput`、`createAssignment`、`Field`、`SettingsFields`、`RecipientChecklist`。

- [ ] **Step 1: 模式切换**

`AssignmentForm.tsx` 顶部（标题下面）加一个两档切换，用既有的 `Segmented`（`formParts.tsx` 已有，不要新写）：

```tsx
<Segmented value={mode} onChange={setMode} options={[{ value: "traditional", label: "传统" }, { value: "ai", label: "AI" }]} />
```

- 默认值：`localStorage` 里这位教师上次选的；没有记录时默认 `"ai"`。读写照抄同文件里 `readLastClassId`／`writeLastClassId` 的写法，key 用 `lite.teacher.assignmentMode`。
- **两个模式共用同一个 `draft` state**，切换时草稿内容保留——`setMode` 只改 `mode`，绝不碰 `draft`。
- `mode === "traditional"` 时渲染今天这张表单，**一个字都不改**。

- [ ] **Step 2: AI 模式页**

`AssignmentAIMode.tsx`：拿 `draft`／`setDraft` 与班级列表作为 props，渲染 `WorkspacePanel`，`children` 是作业卡。

作业卡直接复用既有组件拼：`Field` 包 标题／说明／截止时间，`SettingsFields` 管材料，`RecipientChecklist` 管学生，底部 `发布作业` 走既有的 `buildCreateInput` + `createAssignment`。**不要重写一套表单控件。**

每一轮：
1. 发起前记下 `snapshot = draft`；
2. `postWorkspaceTurn({ surface: "assignment", classId, artifact: draft, turns, text })`；
3. 回来后 `const { next, kept } = applyPatch(draft, snapshot, res.patch)`；`setDraft(next)`；
4. `kept` 非空时在画布上显示一行：`{字段中文名} 已保留你的修改`。

字段中文名用一张常量表映射（`title` → 标题，`dueInput` → 截止时间，…），不要把英文字段名显示给老师。

- [ ] **Step 3: typecheck + 既有测试**

Run: `cd apps/lite-web && npx tsc --noEmit && npx vitest run`
Expected: PASS（`assignmentLogic.test.ts` 的 72 条必须仍然全绿）

- [ ] **Step 4: Commit**

```bash
git add apps/lite-web/src/teacher/AssignmentAIMode.tsx apps/lite-web/src/teacher/AssignmentForm.tsx
git commit -m "feat(lite-web): homework AI mode beside the form, sharing one draft"
```

---

### Task 7: 真模型与真浏览器验收

**Files:** 无产物（验收任务；发现的缺陷就地修，修完补测试）

- [ ] **Step 1: 真模型跑一条完整路径**

按 `prompt-output-must-be-verifiable-2026-09-03` 的规矩，新提示词上线前必须过一次真模型。

写一个 `LIVE_LLM=1` 的 Go 测试，走完：老师说「这周读一篇气候变化的报道，写一篇议论文，周五交」→ 断言模型
(a) 调了 `search_library` 再 `set_material`，没有编造标题；
(b) `dueAt` 是绝对时刻且 `BeijingWallToUTC` 解析得动；
(c) 回复里**没有**学生姓名与人数。

Run: `cd apps/api && LIVE_LLM=1 CGO_ENABLED=0 go test ./internal/api -count=1 -timeout 1800s -run TestLiveWorkspace`

🚨 只跑一次就过不算数——同一条跑 3 次。模型在这里不稳是常态（`model-json-half-arrived-2026-09-08`：本地 6/6 过、线上 4 次错 3 次）。

- [ ] **Step 2: 真浏览器看一遍**

一次性 Playwright harness：登录教师账号 → 作业页 → 切到 AI 模式 → 打一句话 → 截图（回复 + 选项 + 作业卡）→ 点一个选项 → 截图 → 手改截止时间后立刻发下一轮 → 截图确认「截止时间 已保留你的修改」真的出现。

🚨 lite 的滚动在 `<main class="overflow-y-auto">` 里，`fullPage: true` 会裁——先把视口调大。
🚨 截图要**人眼看过**，不能只断言元素存在（`test-logic-not-endless-frontend`：344 个测试全绿而导出的 PNG 是全白的）。

再看窄屏 375px：画布在上、对话在下，不横向滚动。

看完删掉 harness。

- [ ] **Step 3: 全量回归**

Run: `cd apps/api && CGO_ENABLED=0 go test ./... -count=1 -timeout 1800s`
Run: `cd apps/lite-web && npx tsc --noEmit && npx vitest run`
Expected: 全绿

🚨 `go test ./... | grep | tail` 的退出码是 tail 的——不要用管道判断成败（`pbl-refeed-one-produce-slot-2026-09-05`）。

- [ ] **Step 4: Commit**

```bash
git commit -m "test(api): live model check for the assignment workspace turn" # 若 Step 1 留下了测试文件
```

---

## Self-Review

**Spec coverage：**
§3.1 能力档 + 计量 → Task 3 Step 4.6 与 `TestWorkspaceTurnMetersOnDialogue` ✓
§3.2 上下文隔离 → `TrimTurns`（Task 1）+ `trimTurns`（Task 4）+ 每轮只带本页画布 ✓
§3.3／§6 数字姓名不由模型写 → `UngroundedNames`（Task 1）、提示词硬规矩（Task 2）、端点校验（Task 3 Step 4.7）✓
§3.4 可见性边界 → Task 3 Step 4 的 🚨 ✓
§3.5 错误显形 → Task 3（工具循环打满）+ Task 5（`WorkspacePanel` 的 error 行）✓
§4.1 布局 → Task 5 Step 2 ✓
§4.2 线程不入库 → 全程无迁移，Task 3 不写任何表 ✓
§4.4 接口 → Task 3 ✓
§4.5 patch 与手改保留 → Task 4 `applyPatch` + Task 6 Step 2 ✓
§4.6 工具循环上限 4 → `ToolLoopMax` + `TestWorkspaceTurnFailsWhenToolLoopExhausted` ✓
§5.1 两模式、闭集、个性化不重复实现、时间东八区 → Task 6 Step 1、Task 1 `FilterStudents`/`BeijingWallToUTC`、Task 3 Step 4 的 🚨 ✓
§8 测试与上线前真模型 → Task 7 ✓

**未覆盖（有意）：** §5.2 D2 与 §5.3 D3 不在本 plan 内；Task 3 的 `surface` 只接受 `"assignment"`，其余 400。

**Placeholder scan：** Task 3 Step 2 的测试体是签名 + 断言说明而非完整实现，这是有意的——stub provider 的用法必须照抄仓库里 `reading_coach_test.go` 的现成写法，凭空写一份会与既有 harness 打架。其余步骤都给了可直接用的代码。

**Type consistency：** Go 侧 `Student`／`StudentFilter`／`Choice`／`Turn` 在 Task 1 定义，Task 2、3 按同名同形使用；`TurnsWindow`／`ToolLoopMax`／`MaxChoices` 三个常量在 Go（Task 1）与 TS（Task 4 `TURNS_WINDOW`／`MAX_CHOICES`）两侧取值一致（8／4／4）。`applyPatch` 的返回形状 `{ next, kept }` 在 Task 4 定义、Task 6 消费。
