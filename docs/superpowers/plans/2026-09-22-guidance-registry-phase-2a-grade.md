# 年级这条轴接到班级上（二期 a）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让「她在哪个年级」从建班那一刻起有地方存，并且一路送到 `guidance.Key`，为二期 b 的英文写作内容备好那条轴。

**Architecture:** 班级加一列 `grade`（闭表在 Go，不写 DB CHECK，照 `0146_writing_origin.sql` 的先例）。一篇 writing 经 `atom.user_id → enrollments → classes` 反查年级；**查不准就当不知道**，落回今天的行为。`guidance` 里那条轴同时从 `Stage` 改名成 `Grade`，因为这个仓库里 `writing.stage` 已经是另一个意思。

**Tech Stack:** Go 1.26 + pgx/sqlc + goose；前端是 pro 控制台（React + Vite，`apps/web`）。

**Spec:** `docs/superpowers/specs/2026-09-22-teaching-guidance-registry-design.md`（§4 学段、§8.5 一期留下的东西）

## Global Constraints

- **这一期学生看不到任何变化。** 没有一行年级专属的教学内容 —— 那是二期 b。本期做完，`Resolve` 拿到的年级只会让它退回今天那一行。
- 🚨 **parity 是闸**：`CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./cmd/promptinspect` 必须绿。红了就是提示词正文被动了。
- Go 用 `~/sdk/go1.26.0/bin/go`（系统 `go` 是死的 x86），一律 `CGO_ENABLED=0`，在 `apps/api` 下跑；`./internal/api` 要 `-timeout 1800s`。
- 🚨 **sqlc 用仓库自带那个**：在 `apps/api` 下跑 `../../.tooling/sqlc generate`（v1.27.0，arm64）。**不要**用 `sqlc`（不在 PATH）或 `~/go/bin/sqlc`（x86，已废）。当前生成结果与 schema 一致，跑一次不该产生无关 diff。
- 迁移号取 **0186**（现最高 0185）。照 `0146_writing_origin.sql`：`ADD COLUMN ... NOT NULL DEFAULT ''`，**不写 CHECK**，加 `COMMENT ON COLUMN`，附 `-- +goose Down`。
- 注释写中文，讲**为什么**不讲是什么，是交付物的一部分。
- 不加任何第三方依赖。
- 🚨 **不要动 `writing.stage`**。它是写作**流程**阶段（ideate/outline/snippets/draft/finished），和本期的年级毫无关系。

## File Structure

| 文件 | 职责 |
|---|---|
| `apps/api/internal/guidance/guidance.go` | `Key.Stage`→`Key.Grade`、`Scope.Stages`→`Scope.Grades`、`StageBand`→`GradeBand` |
| `apps/api/internal/guidance/text.go` | 报错信息里的 `k.Stage`→`k.Grade` |
| `apps/api/internal/guidance/{guidance_test,coverage_test}.go` | 跟着改名 |
| `apps/api/internal/store/migrations/0186_class_grade.sql` | **新增**：`classes.grade` |
| `apps/api/internal/store/queries/org.sql` | `CreateClass` 带上 grade；新增 `SetClassGrade` |
| `apps/api/internal/store/queries/auth.sql` | 新增 `ListStudentClassGrades`（按 user 反查年级） |
| `apps/api/internal/store/sqlc/**` | `../../.tooling/sqlc generate` 的产物，不手改 |
| `apps/api/internal/api/class_grade.go` | **新增**：年级闭表、校验、界面上的说法 |
| `apps/api/internal/api/classes.go` / `classes_dto.go` | 建班收 grade、DTO 出 grade |
| `apps/api/internal/api/writing_grade.go` | **新增**：一篇 writing 的年级怎么查出来（含「拿不准就当不知道」） |
| `apps/api/internal/api/writing_plan_prompt.go` / `writing_symptoms.go` | 把年级填进 `guidance.Key` |
| `apps/web/src/api/classes.ts` / `src/console/ClassesView.tsx` | 建班表单上的年级选择 |

---

### Task 1: guidance 那条轴改名 Stage → Grade

**Files:**
- Modify: `apps/api/internal/guidance/guidance.go`（第 18、28、33、43–55、68–70、78、99–103 行附近）
- Modify: `apps/api/internal/guidance/text.go:40`
- Modify: `apps/api/internal/guidance/guidance_test.go`、`apps/api/internal/guidance/coverage_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `guidance.Key{Surface, Lang, Genre, Grade string}`；`guidance.Scope{Surface, Lang string; Genres, Grades []string}`；`func GradeBand(grade string) string`

**为什么改名。** 这个仓库里 `writing.stage` 已经存在，指的是写作**流程**阶段
（`ideate|outline|snippets|draft|finished`，见 `0099_writing_tables.sql:12-14`），
而且生成的 `sqlc.Writing.Stage` 到处在用。再让 `sqlc.Class.Stage` 和
`guidance.Key.Stage` 表示「几年级」，同一个词在一个包里就有两个意思。
**现在改零成本**：今天没有任何调用点设置过这个字段（四处 `guidance.Key{...}`
一处都没写 Stage），改完行为一模一样。等二期 b 往里填内容之后再改就不是了。

- [ ] **Step 1: 先确认现状，再动手**

```bash
cd apps/api && grep -rn "Stage\|Stages\|StageBand" internal/guidance/
cd apps/api && grep -rn "guidance.Key{" internal/api/ | grep -v _test
```

预期：`internal/guidance` 里有 12 处非测试命中；`internal/api` 的四处
`guidance.Key{...}` **一处都不含 Stage**。第二条不成立就停下来报告 ——
说明有人在这期之前就用上了它，改名不再是零成本。

- [ ] **Step 2: 改名**

在 `internal/guidance` 全包范围内做这三个替换，**只改这三个标识符**：

- `Key.Stage` 字段 → `Grade`
- `Scope.Stages` 字段 → `Grades`
- `func StageBand` → `func GradeBand`

同时把注释里的说法理顺：`Grades` 那一行写成
`// 可以写年级（junior2），也可以写学段（junior）`，`GradeBand` 的文档说明它
把年级折成学段，好让「整个初中通用」的内容只写一份。
`text.go:40` 的报错格式串里 `stage=%q` 保持这个字面词不变（它是给人读的日志，
说「stage」或「grade」都能懂），但取值改成 `k.Grade`。
**不要**改 `specificity` 的权重，也不要改任何一行取值（`junior1`…`senior3`）。

- [ ] **Step 3: 跑测试，确认行为一个字没变**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./internal/guidance/ -v
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go build ./...
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./cmd/promptinspect
```

预期：**先数一下改名前有几条通过**（`go test ./internal/guidance/ -v 2>&1 | grep -c '^--- PASS'`，
在 `6295e76c` 上是 **12** 条），改名后必须是**同样多、同样的名字**，build 干净，
parity 绿。这是一次纯改名 —— 条数少一条就是有测试没跟着改名而被悄悄漏掉了，
红一条就是改到了不该改的东西。

- [ ] **Step 4: 提交**

```bash
git add apps/api/internal/guidance/
git commit -m "refactor(guidance): 那条轴改叫 Grade —— stage 在这个仓库里已经是别的意思"
```

---

### Task 2: 迁移 0186，班级加一列 grade

**Files:**
- Create: `apps/api/internal/store/migrations/0186_class_grade.sql`
- Modify: `apps/api/internal/store/queries/org.sql`（`CreateClass`，第 24–27 行）
- Modify: `apps/api/internal/store/sqlc/**`（生成物）

**Interfaces:**
- Consumes: 无
- Produces: `classes.grade` 列；`sqlc.Class.Grade string`；`sqlc.CreateClassParams.Grade string`

- [ ] **Step 1: 写迁移**

`apps/api/internal/store/migrations/0186_class_grade.sql`：

```sql
-- 0186_class_grade.sql —— 这个班是几年级。
--
-- 产品负责人 2026-09-22：「during a class creating we can add 学段。」
-- 不按每篇作文推断，也不问学生 —— 建班的人知道这件事，问一次就够了。
--
-- 为什么要到年级这一层，而不是初中／高中两档：`初中语文作文批改` 的标准
-- 是按年级给的（初一 500–600 字「叙事完整」、初二 550–650「描写生动」、
-- 初三 600–700「立意深刻」），两档表达不了这三行。一个班本来就是一个年级。
--
-- 🚨 **不写 CHECK 约束**，和 writing.origin（0146）、writing.stage（0100）
-- 同一个理由：闭表的执行点在 Go（internal/api/class_grade.go），而一个还
-- 容得下将来某个值的数据库不花钱；立了 CHECK，将来加一个年级就要重建约束，
-- 那是一次会锁表的迁移。
--
-- 🚨 **和 writing.stage 不是一回事**。那一列是写作流程走到哪儿
-- （ideate/outline/…），这一列是她读几年级。名字不同是故意的。
--
-- additive：老班全部回填空串 = 不知道年级，行为和这次迁移之前一模一样。

-- +goose Up
ALTER TABLE classes
    ADD COLUMN grade text NOT NULL DEFAULT '';

COMMENT ON COLUMN classes.grade IS
  '这个班几年级：junior1..3 / senior1..3；空串 = 没填。闭表在 internal/api/class_grade.go，不在 DB 上设 CHECK。';

-- +goose Down
ALTER TABLE classes
    DROP COLUMN grade;
```

- [ ] **Step 2: 让建班的那条 query 带上它**

`apps/api/internal/store/queries/org.sql`，把 `CreateClass` 改成：

```sql
-- name: CreateClass :one
INSERT INTO classes (school_id, name, join_code, created_by, grade)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;
```

再在同一个文件末尾加一条，给「建完之后改年级」用：

```sql
-- name: SetClassGrade :one
UPDATE classes SET grade = $2 WHERE id = $1 RETURNING *;
```

- [ ] **Step 3: 重新生成，并确认只多了该多的东西**

```bash
cd apps/api && ../../.tooling/sqlc generate
cd /Users/houyuxin/08Coding/mind-imprint/.claude/worktrees/writing-polish-r3-0920 && git status --porcelain apps/api/internal/store/sqlc/
```

预期：`models.go`（`Class` 多一个 `Grade string`）、`org.sql.go`
（`CreateClassParams` 多一个 `Grade`，新增 `SetClassGrade`）有改动。
**看一眼 diff**：除了 `Class`、`CreateClass*`、`SetClassGrade` 之外还有别的
结构变了，就说明这次生成捎带了别的东西 —— 停下来报告，不要提交。

- [ ] **Step 4: 让它编译**

`CreateClass` 现在多一个参数，唯一的两个调用点要补上空串（不知道年级）：
`internal/api/classes.go` 的 `createClass`（约第 68 行）和
`internal/api/import.go` 的 `adminImport`（约第 75 行）。两处都先写
`Grade: ""`，各配一句注释说明原因（Task 4 会把 `createClass` 那处接上真值；
`adminImport` 那处**保持空串**，因为批量导入的行里没有年级）。

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go build ./...
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/
```

预期：build 干净，`./internal/api` 全绿。

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/store/migrations/0186_class_grade.sql apps/api/internal/store/queries/org.sql apps/api/internal/store/sqlc/ apps/api/internal/api/classes.go apps/api/internal/api/import.go
git commit -m "feat(class): 班级记下年级 —— 建班的人知道，问一次就够"
```

---

### Task 3: 年级闭表与校验

**Files:**
- Create: `apps/api/internal/api/class_grade.go`
- Test: `apps/api/internal/api/class_grade_test.go`

**Interfaces:**
- Consumes: `guidance.GradeBand`（Task 1）
- Produces: `func validateClassGrade(s string) (string, bool)`；`func classGradeLabel(grade string) string`；`var classGrades []string`

- [ ] **Step 1: 写下会红的测试**

`apps/api/internal/api/class_grade_test.go`：

```go
package api

import "testing"

func TestValidateClassGradeAcceptsTheClosedSetAndEmpty(t *testing.T) {
	for _, ok := range []string{"", "junior1", "junior2", "junior3", "senior1", "senior2", "senior3"} {
		if got, valid := validateClassGrade(ok); !valid || got != ok {
			t.Errorf("%q 该被接受，拿到 got=%q valid=%v", ok, got, valid)
		}
	}
}

// 🚨 空串是「没填」，是合法的；别的不认识的字一律拒掉，不要悄悄存进去。
// 存进去的那一刻它就会被当成年级去挑教学内容，而没有任何一行登记服务它。
func TestValidateClassGradeRejectsAnythingElse(t *testing.T) {
	for _, bad := range []string{"junior", "senior", "JUNIOR1", "初二", "primary4", "junior0", "junior4", " junior1"} {
		if _, valid := validateClassGrade(bad); valid {
			t.Errorf("%q 不该被接受", bad)
		}
	}
}

// 🚨 年级要能折成学段，「整个初中通用」的内容才只写一份。
// 这一条同时钉住：闭表里的每一个年级，guidance 那边都折得出学段来。
func TestEveryClassGradeFoldsToABand(t *testing.T) {
	for _, g := range classGrades {
		if band := guidance.GradeBand(g); band != "junior" && band != "senior" {
			t.Errorf("年级 %q 折出来的学段是 %q，既不是 junior 也不是 senior", g, band)
		}
	}
}

func TestClassGradeLabelIsChineseAndEmptyStaysEmpty(t *testing.T) {
	if got := classGradeLabel(""); got != "" {
		t.Errorf("没填年级时标签该是空串，拿到 %q", got)
	}
	if got := classGradeLabel("junior2"); got != "初二" {
		t.Errorf("junior2 的标签该是「初二」，拿到 %q", got)
	}
	if got := classGradeLabel("senior3"); got != "高三" {
		t.Errorf("senior3 的标签该是「高三」，拿到 %q", got)
	}
}
```

测试文件要 `import "mindimprint/api/internal/guidance"`。

- [ ] **Step 2: 跑一次，确认它红**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/ -run TestValidateClassGrade
```

预期：`undefined: validateClassGrade` 之类的编译错误。

- [ ] **Step 3: 写实现**

`apps/api/internal/api/class_grade.go`：

```go
package api

import "strings"

// class_grade.go —— 一个班是几年级。
//
// # 为什么闭表在这儿，不在数据库上
//
// 和 writing.origin、writing.stage 同一条纪律（见 0186 迁移里的说明）：
// 执行点在 Go，数据库那边留着余地。将来要加「初四」或者小学，改这一张表
// 就够了，不用做一次锁表的迁移。
//
// # 🚨 空串是「没填」，而且它必须一路是合法的
//
// 老班全是空串，批量导入建的班也是空串。空串到了 guidance 那边匹配不上任何
// 一行带年级的登记，于是退回不限年级的那一行 —— 也就是今天的行为。
// **所以「不知道年级」永远不是错误，只是少一条线索。**
var classGrades = []string{"junior1", "junior2", "junior3", "senior1", "senior2", "senior3"}

// classGradeLabels 是她和老师在屏幕上看见的说法。
//
// 名词，不是句子（AGENTS.md 界面文案第 1 条）。
var classGradeLabels = map[string]string{
	"junior1": "初一", "junior2": "初二", "junior3": "初三",
	"senior1": "高一", "senior2": "高二", "senior3": "高三",
}

// validateClassGrade 收下一个年级。第二个返回值是「这个值能不能存」。
//
// 🚨 不认识的值一律拒掉，不做大小写或中文的宽容匹配：存进去的那一刻它就会
// 被当成年级去挑教学内容，而没有任何一行登记服务它 —— 症状是她悄悄少拿到
// 一块内容，而日志上什么都看不见。
func validateClassGrade(s string) (string, bool) {
	if s == "" {
		return "", true
	}
	for _, g := range classGrades {
		if s == g {
			return g, true
		}
	}
	return "", false
}

// classGradeLabel 是界面上那个词。没填就还它一个空串。
func classGradeLabel(grade string) string {
	return classGradeLabels[strings.TrimSpace(grade)]
}
```

- [ ] **Step 4: 跑测试**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/ -run 'TestValidateClassGrade|TestEveryClassGrade|TestClassGradeLabel'
```

预期：四条全绿。

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/api/class_grade.go apps/api/internal/api/class_grade_test.go
git commit -m "feat(class): 年级闭表 —— 不认识的值一律拒掉"
```

---

### Task 4: 建班收年级，DTO 出年级

**Files:**
- Modify: `apps/api/internal/api/classes.go`（`createClass` 第 18–89 行、`patchClass` 第 143–196 行）
- Modify: `apps/api/internal/api/classes_dto.go`（第 9–25 行）
- Test: `apps/api/internal/api/classes_grade_test.go`（新建）

**Interfaces:**
- Consumes: `validateClassGrade`（Task 3）、`sqlc.CreateClassParams.Grade`、`sqlc.SetClassGrade`（Task 2）
- Produces: `createClass` 请求体多一个 `grade`；`classDTO` 多一个 `grade`

- [ ] **Step 1: 写下会红的测试**

`apps/api/internal/api/classes_grade_test.go`：

```go
package api

import "testing"

// DTO 要把年级带出去，否则老师那边建完就再也看不到自己填了什么。
func TestClassDTOCarriesGrade(t *testing.T) {
	dto := toClassDTO(sqlcClassWithGrade("junior2"))
	if dto.Grade != "junior2" {
		t.Errorf("classDTO.Grade = %q，想要 junior2", dto.Grade)
	}
	if dto.GradeLabel != "初二" {
		t.Errorf("classDTO.GradeLabel = %q，想要「初二」", dto.GradeLabel)
	}
}

// 🚨 没填年级的班（老班、批量导入建的）要原样出去，不要在这儿编一个默认值。
// 编一个出来，她就会按一个没人填过的年级拿教学内容。
func TestClassDTOKeepsUnknownGradeEmpty(t *testing.T) {
	dto := toClassDTO(sqlcClassWithGrade(""))
	if dto.Grade != "" || dto.GradeLabel != "" {
		t.Errorf("没填年级时该是两个空串，拿到 %q / %q", dto.Grade, dto.GradeLabel)
	}
}
```

同文件里写这个小助手（放在测试文件里，生产代码不需要它）：

```go
func sqlcClassWithGrade(grade string) sqlc.Class {
	return sqlc.Class{Name: "试用班级", JoinCode: "AAAA-BBBB", Grade: grade}
}
```

测试文件要 `import "mindimprint/api/internal/store/sqlc"`。

- [ ] **Step 2: 跑一次，确认它红**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/ -run TestClassDTO
```

预期：`dto.Grade undefined`。

- [ ] **Step 3: 改 DTO 和 handler**

`classes_dto.go` 的 `classDTO` 加两个字段，并在 `toClassDTO` 里填上：

```go
	// Grade 是闭表里的值（junior2）；GradeLabel 是屏幕上那个词（初二）。
	// 两个都给：前端不必自己维护一份中文对照表，那是第二份会漂的真相。
	Grade      string `json:"grade"`
	GradeLabel string `json:"grade_label"`
```

`createClass` 的请求结构体加 `Grade string \`json:"grade"\``，在校验 `Name`
之后校验它：

```go
	grade, gradeOK := validateClassGrade(strings.TrimSpace(body.Grade))
	if !gradeOK {
		// 不认识的年级是前端的 bug，不是老师填错了字 —— 直接回 400，
		// 不要悄悄存成空串，那样他以为自己填了。
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "年级不在可选范围内", nil))
		return
	}
```

（这一行照抄本文件第 29–31 行校验班级名那句的写法：
`httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "班级名称不能为空", nil))`。
请求体的变量名在这个 handler 里叫 `body`，不叫 `req`。）

把 `grade` 传进 `qtx.CreateClass(...)` 的 `Grade` 字段（Task 2 那里先写的
`Grade: ""` 换成它）。

`patchClass` 同样收一个可选的 `grade`：请求里带了就校验并调
`qtx.SetClassGrade`，没带就不动。照该函数现有的「字段给了才改」写法来。

- [ ] **Step 4: 跑测试**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/ -run 'TestClassDTO|TestCreateClass|TestPatchClass'
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/
```

预期：全绿。已有的建班测试没传 `grade`，会走空串那条路 —— 它们必须仍然绿。

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/api/classes.go apps/api/internal/api/classes_dto.go apps/api/internal/api/classes_grade_test.go
git commit -m "feat(class): 建班收年级，DTO 带年级出去"
```

---

### Task 5: 一篇 writing 的年级怎么查出来

**Files:**
- Create: `apps/api/internal/api/writing_grade.go`
- Modify: `apps/api/internal/store/queries/auth.sql`（文件末尾加一条 query）
- Test: `apps/api/internal/api/writing_grade_test.go`

**Interfaces:**
- Consumes: `sqlc` 生成的新 query（本任务加）
- Produces: `func gradeFromClasses(grades []string) string`

**这一步唯一难的地方：一个学生可能在不止一个班里。** `enrollments` 只在
`(user_id, class_id)` 上唯一，`ListClassesForUser` 是 `:many`，`GET /me` 那边
已经按列表在渲染。今天没有哪条路会给一个已有账号再加一个学生身份的 enrollment，
但那是「还没做这个功能」，不是「数据库不让」。

- [ ] **Step 1: 写下会红的测试**

`apps/api/internal/api/writing_grade_test.go`：

```go
package api

import "testing"

func TestGradeFromClassesTakesTheOnlyOne(t *testing.T) {
	if got := gradeFromClasses([]string{"junior2"}); got != "junior2" {
		t.Errorf("只有一个班时该直接用它，拿到 %q", got)
	}
}

// 没进班、或者班上没填年级 —— 都是「不知道」，不是错误。
func TestGradeFromClassesUnknownWhenNothingToGoOn(t *testing.T) {
	for _, in := range [][]string{nil, {}, {""}, {"", ""}} {
		if got := gradeFromClasses(in); got != "" {
			t.Errorf("%v 该是不知道，拿到 %q", in, got)
		}
	}
}

// 🚨 两个班两个年级 —— **不猜**。
//
// 猜错的代价是她整篇拿到另一个年级的教学内容，而屏幕上什么异常都没有。
// 退回「不知道」只是少一条线索：她拿到的是今天那一份通用内容。
func TestGradeFromClassesRefusesToGuessWhenClassesDisagree(t *testing.T) {
	if got := gradeFromClasses([]string{"junior2", "senior1"}); got != "" {
		t.Errorf("两个年级对不上时该退回不知道，拿到 %q", got)
	}
}

// 两个班但年级一样（或者其中一个没填）—— 那就没有歧义，用那个年级。
func TestGradeFromClassesAgreesIsFine(t *testing.T) {
	if got := gradeFromClasses([]string{"junior2", "junior2"}); got != "junior2" {
		t.Errorf("两个班同一个年级时该用它，拿到 %q", got)
	}
	if got := gradeFromClasses([]string{"", "senior1"}); got != "senior1" {
		t.Errorf("一个没填一个填了，该用填了的那个，拿到 %q", got)
	}
}
```

- [ ] **Step 2: 跑一次，确认它红**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/ -run TestGradeFromClasses
```

预期：`undefined: gradeFromClasses`。

- [ ] **Step 3: 写纯函数 + 那条 query**

`apps/api/internal/api/writing_grade.go`：

```go
package api

// writing_grade.go —— 这一篇是几年级的学生写的。
//
// 路径：writing.atom_id → atom.user_id → enrollments → classes.grade。
// writing 表自己没有 owner 列，归属挂在 atom 上（0092_atom_substrate.sql）。
//
// # 🚨 一个学生可能在不止一个班里
//
// `enrollments` 只在 (user_id, class_id) 上唯一，`ListClassesForUser` 是
// `:many`，`GET /me` 那边早就按列表渲染。今天没有哪条路会给一个已有账号再加
// 一个学生 enrollment（只有注册时按 join code 加一次），但那是「还没做」，
// 不是数据库拦着。
//
// 所以这里的规矩是：**两个班的年级对不上就当不知道，绝不挑一个。**
// 挑错的代价是她整篇拿到另一个年级的教学内容，而屏幕上一点异常都没有；
// 退回「不知道」只是少一条线索 —— 她拿到的是今天那份不分年级的内容。
func gradeFromClasses(grades []string) string {
	found := ""
	for _, g := range grades {
		if g == "" {
			continue
		}
		if found == "" {
			found = g
			continue
		}
		if found != g {
			return "" // 对不上 —— 不猜。
		}
	}
	return found
}
```

`apps/api/internal/store/queries/auth.sql` 末尾加：

```sql
-- name: ListClassGradesForUser :many
SELECT c.grade FROM enrollments e
JOIN classes c ON c.id = e.class_id
WHERE e.user_id = $1 AND e.role_in_class = 'student';
```

然后重新生成：

```bash
cd apps/api && ../../.tooling/sqlc generate
```

- [ ] **Step 4: 跑测试**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/ -run TestGradeFromClasses
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go build ./...
```

预期：五条全绿，build 干净。

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/api/writing_grade.go apps/api/internal/api/writing_grade_test.go apps/api/internal/store/queries/auth.sql apps/api/internal/store/sqlc/
git commit -m "feat(writing): 反查她的年级 —— 两个班对不上就当不知道"
```

---

### Task 6: 把年级填进 guidance.Key

**Files:**
- Modify: `apps/api/internal/api/writing_plan_prompt.go`（`writingPlanSystemFor`，第 51 行起）
- Modify: `apps/api/internal/api/writing_plan.go:589`（handler 里那个调用点）
- Modify: `apps/api/internal/api/prompt_examples.go:47`
- Test: `apps/api/internal/api/writing_grade_key_test.go`

**Interfaces:**
- Consumes: `gradeFromClasses`（Task 5）、`guidance.Key.Grade`（Task 1）
- Produces: `writingPlanSystemFor(genre, lang, grade string) string`

🚨 **本期只接 `writingPlanSystemFor` 这一处，别的一概不接。**

- **毛病表不接。** `writingSymptomTable` 的调用链是
  `writingSymptomTable → writingSymptomCatalog → buildWritingCommentSystem`，
  底下挂着 `writing_compose.go:297`、`writing_comment.go:583`、
  `lite_grading_run.go:40`、`prompt_examples.go:57`、
  `benchcases_lite_writing.go:139` 五个调用点。把年级穿过这一整条链要改六个
  文件，**而二期 b 一行都用不上** —— 它要加的英文内容落在立题那份提示词里，
  也就是 `writingPlanSystemFor` 服务的地方。等四期真要按年级分毛病表的时候
  再穿，那时这条路已经趟熟了。
- **阅读面不接。** `reading_genre.go`、`reading_routines.go` 一行年级内容都
  没有，接上去只是多一个恒为空的参数。

`writingPlanSystemFor` 今天只有两个调用点（`writing_plan.go:589` 和
`prompt_examples.go:47`），这就是它先接的理由。

- [ ] **Step 1: 写下会红的测试**

`apps/api/internal/api/writing_grade_key_test.go`：

```go
package api

import "strings"
import "testing"

// 🚨 这一期不加任何一行年级专属的内容，所以**填不填年级，结果必须一模一样**。
// 这条测试就是这期的验收：轴通了，而她看到的东西没变。
func TestGradeDoesNotChangeAnythingYet(t *testing.T) {
	for _, lang := range []string{"zh", langEnglish} {
		for _, genre := range []string{genreArgument, genreNarrative} {
			base := writingPlanSystemFor(genre, lang, "")
			for _, grade := range classGrades {
				if got := writingPlanSystemFor(genre, lang, grade); got != base {
					t.Errorf("%s/%s：填了年级 %s 之后提示词变了，但这一期还没有任何年级内容",
						lang, genre, grade)
				}
			}
		}
	}
}

// 提示词里不许出现内部的年级取值 —— 那是给代码看的标记，她的屏幕上没有。
func TestGradeCodeNeverLeaksIntoThePrompt(t *testing.T) {
	for _, grade := range classGrades {
		s := writingPlanSystemFor(genreArgument, "zh", grade)
		if strings.Contains(s, grade) {
			t.Errorf("提示词里出现了内部取值 %q", grade)
		}
	}
}
```

- [ ] **Step 2: 跑一次，确认它红**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/ -run TestGrade
```

预期：参数个数不对的编译错误。

- [ ] **Step 3: 加参数、填进 Key、把调用点补齐**

`writingPlanSystemFor` 改成收第三个参数 `grade string`，并填进 Key：

```go
func writingPlanSystemFor(genre string, lang string, grade string) string {
	k := guidance.Key{Surface: guidance.SurfaceWrite, Lang: lang, Genre: genre, Grade: grade}
```

两个调用点这样改：

**`prompt_examples.go:47`** —— 离线渲染样例，没有学生也没有库，传空串：

```go
				{Role: gateway.RoleSystem, Content: writingPlanSystemFor(genre, lang, "")}, {Role: gateway.RoleUser, Content: doc.Text},
```

**`writing_plan.go:589`** —— 真正的那一轮。这里有 `r`、有 `a.d`、有 `at`
（atom，带着 `at.UserID`），所以在这一行之前把年级查出来：

```go
	// 她在哪个年级 —— 用来挑年级对得上的教学内容。查不到、或者她在两个年级
	// 对不上的班里，就当不知道（gradeFromClasses），落回不分年级的那一份。
	// 🚨 查不出来不是错误，所以这里只记一行日志，不中断这一轮。
	grade := ""
	if rows, gerr := a.d.Q.ListClassGradesForUser(r.Context(), at.UserID); gerr != nil {
		slog.Warn("writing plan turn: 年级查不出来，按不分年级办",
			"err", gerr, "atom_id", at.ID,
			"request_id", httpx.RequestIDFromContext(r.Context()))
	} else {
		grade = gradeFromClasses(rows)
	}
	system := writingPlanSystemFor(writingGenreOf(wr, rows2), wr.Lang, grade)
```

> **实现者注意：** 上面那段里 `rows2` 是个占位写法 —— 这一行原本写的是
> `writingPlanSystemFor(writingGenreOf(wr, rows), wr.Lang)`，而 `rows` 在那个
> 作用域里已经是**大纲行**了。给你查年级拿到的那个切片换一个名字
> （例如 `gradeRows`），**不要覆盖已有的 `rows`** —— 覆盖了，
> `writingGenreOf` 就会拿着一串年级去判文体，静默判错。
> 改完把那一行恢复成 `writingGenreOf(wr, rows)`。
>
> `a.d.Q` 是这个包访问 sqlc 查询的习惯写法；如果本文件用的是别的名字
> （例如 `a.d.Queries`），**用本文件已有的那个**。`grep -n "a\.d\.Q" internal/api/writing_plan.go`
> 先看一眼。

- [ ] **Step 4: 跑测试与 parity**

```bash
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -timeout 1800s ./internal/api/
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./cmd/promptinspect
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go build ./...
```

预期：全绿。**parity 红了就是有哪个调用点把年级传成了非空**，而这一期不该有
任何东西因为年级而改变。

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/api/
git commit -m "feat(guidance): 年级填进 Key —— 轴通了，她看到的还是原来那份"
```

---

### Task 7: 建班表单上的年级

**Files:**
- Modify: `apps/web/src/api/classes.ts`（第 38–45 行附近）
- Modify: `apps/web/src/console/ClassesView.tsx`（第 9–121 行）

**Interfaces:**
- Consumes: `POST /api/v1/classes` 的 `grade` 字段、`classDTO.grade` / `grade_label`（Task 4）
- Produces: 无（这是最后一层）

🚨 **这是 pro 控制台（`apps/web`），不是 lite。** 建班表单只在这儿。改动限于
这个表单：加一个下拉、把值放进请求体、在班级列表里显示 `grade_label`。
**不要顺手重构这个文件的别的部分**，也不要碰 lite。

- [ ] **Step 1: 先看清楚现状**

```bash
sed -n 1,60p apps/web/src/console/ClassesView.tsx
sed -n 30,50p apps/web/src/api/classes.ts
```

记下：表单现在提交哪几个字段、`Select` 是从哪儿导入的（这个文件用的是
`@/ui` 里的 `Input` 和 `Select`，**不是** lite 的 `teacher/controls/`）、
班级列表每一行现在显示什么。

- [ ] **Step 2: 客户端类型加上 grade**

`apps/web/src/api/classes.ts`：`createClass` 的入参类型加
`grade?: string`，请求体原样带上；班级的响应类型加
`grade: string` 和 `grade_label: string`。

- [ ] **Step 3: 表单加一个年级下拉**

`ClassesView.tsx`：用这个文件已经在用的 `Select`（和管理员那个教师下拉同一个
组件），选项如下，第一项是「不填」：

```tsx
const GRADE_OPTIONS = [
  { value: "", label: "未填写" },
  { value: "junior1", label: "初一" },
  { value: "junior2", label: "初二" },
  { value: "junior3", label: "初三" },
  { value: "senior1", label: "高一" },
  { value: "senior2", label: "高二" },
  { value: "senior3", label: "高三" },
];
```

提交时把选中的值放进 `createClass({name, grade, teacher_user_id?})`。
班级列表里每一行显示 `grade_label`，空的时候**什么都不显示**（不要写
「未填写」当占位 —— 列表里一列重复的「未填写」只是噪音）。

选择器的说明文字就写「年级」两个字（名词，不是句子）。

- [ ] **Step 4: 编译与本地检查**

```bash
cd apps/web && npm run typecheck
cd apps/web && npm run build
```

🚨 **两条都要跑，顺序别反。** `build` 是 `vite build`，它**不做类型检查**；
类型错误只有 `typecheck`（`tsc --noEmit`）抓得到。只跑 build 就上，等于把
`grade` 拼错的那种错留到运行时。

预期：两条都通过。这个仓库的前端不写渲染断言测试（AGENTS.md：「测试只写逻辑
测试，不堆前端渲染测试」），所以这一步看的就是类型和编译。

- [ ] **Step 5: 提交**

```bash
git add apps/web/src/api/classes.ts apps/web/src/console/ClassesView.tsx
git commit -m "feat(console): 建班时选年级"
```

---

## 这一期做完之后

- 学生看不到任何变化（Task 6 的两条测试就是钉这件事的）。
- 老师建班时能选年级，班级列表看得到。
- `guidance.Key.Grade` 一路通了，二期 b 只需要往注册表里加带 `Grades` 的行。
- 🚨 **二期 b 开工前先读 spec §8.5 的那条警告**：加第一行带 `Grades` 的登记时，
  `TestEveryCombinationResolves` 会骗你 —— 它对 7 个年级断言的是同一份期望。
  那张 want 表必须同时按年级分开，否则年级专属的内容一次都不出现而测试照样绿。
