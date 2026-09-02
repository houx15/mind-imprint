# 兴趣模型 P1 · 双模型 + 路由地基 —— 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 后端长出「她的关键词树」「共用的学科表」和两者之间的路由，并在她真的读完 / 写完 / 复盘时自动采集关键词。

**Architecture:** 学科表是 `packages/contracts` 里的静态 JSON（Go 用同目录副本 `go:embed`，靠一条 drift 测试守住不漂移）；她的关键词、来源、以及词→学科的边进 Postgres（迁移 `0116`）。纯逻辑（归一化 / 合并 / 三档路由 / 采集解析）全在 `internal/interest`，**不碰 DB**，照抄 `internal/pbl` 的分层；HTTP + sqlc 胶水在 `internal/api/interest*.go`。

**Tech Stack:** Go 1.2x · pgx/v5 + sqlc + goose · `internal/gateway`（`Collect` + `resolveEval`）· Zod（`packages/contracts`）

**Spec:** `docs/superpowers/specs/2026-09-02-interest-model-and-exploration-design.md`

## Global Constraints

- **不破坏 pro。** 只新建 lite 自己的文件；迁移从 `0116` 起（现存最新 `0115`）；**绝不创建已存在的文件**——先 `ls` 再写。
- **密钥只在 `apps/api`**；采集与 T3 路由的每一次模型调用都必须 `a.recordLiteLLMCall(...)`，`surface="lite"`。
- **模型失败绝不编内容。** 解析失败 = 不长词 + 记 `slog.Warn`，永远不返回一个像样的假关键词。
- **`evidence` 为空的来源不入库。** 树的断言是「每个词都来自你真做过的事」。
- Go 测试：`CGO_ENABLED=0 go test ./... -timeout 1800s`。
- sqlc 重新生成：`cd apps/api && CGO_ENABLED=0 go tool sqlc generate`（即 `make sqlc`）。
- 七根主枝的 id：`formal` `science` `making` `society` `humanities` `arts` `self`。

---

## File Structure

| 文件 | 责任 |
|---|---|
| `packages/contracts/disciplines/disciplines.json` | **唯一真相源**：42 门学科 |
| `packages/contracts/src/discipline.ts` | Zod 契约 + 类型 + 前端 loader |
| `apps/api/internal/disciplines/disciplines.json` | 逐字节副本（`go:embed` 出不了包目录） |
| `apps/api/internal/disciplines/disciplines.go` | embed + 解析 + `ByID` / `ByField` / `MatchAlias` |
| `apps/api/internal/disciplines/disciplines_test.go` | 结构校验 + drift 守卫 |
| `apps/api/internal/store/migrations/0116_interest_model.sql` | 三张表 |
| `apps/api/internal/store/queries/interest.sql` | sqlc 查询 |
| `apps/api/internal/interest/normalize.go` | 归一化 + 相似判定（纯） |
| `apps/api/internal/interest/router.go` | T1 别名 / T2 共现 / T3 prompt + 解析（纯） |
| `apps/api/internal/interest/harvest.go` | 采集 prompt + 解析（纯） |
| `apps/api/internal/api/interest.go` | 落库 + `GET /api/v1/interest/tree` |
| `apps/api/internal/api/interest_harvest.go` | 三个完成点的采集钩子 |

---

### Task 1: 学科表（契约 + 42 门数据 + 校验）

**Files:**
- Create: `packages/contracts/disciplines/disciplines.json`
- Create: `packages/contracts/src/discipline.ts`
- Create: `apps/api/internal/disciplines/disciplines.json`（副本）
- Create: `apps/api/internal/disciplines/disciplines.go`
- Test: `apps/api/internal/disciplines/disciplines_test.go`

**Interfaces:**
- Produces: `disciplines.All() []Discipline` · `disciplines.ByID(id string) (Discipline, bool)` · `disciplines.ByField(field string) []Discipline` · `disciplines.MatchAlias(normalized string) (Discipline, bool)` · `type Discipline{ID,Field,Zh,En,Asks,Method,Exemplar string; Aliases []string; Syllabus []SyllabusRef}` · `type SyllabusRef{Board,Code,Label,Level string}`

42 门 = 七根主枝 × 6：

- `formal` — statistical-inference 统计推断 · probability 概率 · calculus 微积分与变化率 · logic-proof 逻辑与证明 · discrete-networks 离散数学与网络 · modelling 数学建模
- `science` — climate-ocean 气候与海洋 · ecology 生态学 · genetics 遗传与进化 · neuroscience 神经科学 · matter-energy 物质与能量 · astronomy 天文与宇宙学
- `making` — algorithms 算法与计算 · ai-ml 人工智能与机器学习 · materials 材料科学 · engineering-design 工程设计 · energy-systems 能源系统 · interaction-design 交互设计
- `society` — economics 经济学 · sociology 社会学 · political-economy 政治经济与制度 · anthropology 人类学 · geography-urban 地理与城市 · public-health 公共卫生
- `humanities` — history 历史学 · philosophy 哲学 · linguistics 语言学 · literary-criticism 文学批评 · rhetoric-argument 修辞与论证 · media-literacy 媒介素养
- `arts` — art-history 艺术史 · music-theory 音乐理论 · film-narrative 影像叙事 · visual-design 视觉设计 · performance 表演与剧场 · creative-writing 创意写作
- `self` — cognitive-psychology 认知心理学 · developmental-psychology 发展心理学 · ethics 伦理学 · education-learning 学习科学 · wellbeing 身心健康 · career-futures 职业与未来

一条完整样例（其余 41 条同形）：

```json
{
  "id": "statistical-inference",
  "field": "formal",
  "zh": "统计推断", "en": "Statistical inference",
  "asks": "一个样本，能替多少人说话？",
  "method": "抽样 · 置信区间 · 显著性检验 · 效应量",
  "exemplar": "4 平方公里的珊瑚，能不能代表一片海？",
  "aliases": ["统计", "统计学", "抽样", "样本", "代表性", "置信区间", "显著性",
              "statistics", "sampling", "significance", "confidence interval"],
  "syllabus": [
    { "board": "IB",      "code": "MAA-4",        "label": "IB 数学 AA · 统计与概率", "level": "HL/SL" },
    { "board": "ALevel",  "code": "9709-S1",      "label": "剑桥 A-Level 数学 · 概率与统计 1", "level": "AS" },
    { "board": "AP",      "code": "AP-STAT-U4",   "label": "AP Statistics · Unit 4 抽样分布", "level": "" },
    { "board": "University", "code": "STAT-101",  "label": "大学 · 统计学导论 / 数据科学", "level": "" }
  ]
}
```

- [ ] **Step 1: 写失败的测试** —— `apps/api/internal/disciplines/disciplines_test.go`

```go
package disciplines

import (
	"bytes"
	"os"
	"testing"
)

var fields = map[string]bool{"formal": true, "science": true, "making": true,
	"society": true, "humanities": true, "arts": true, "self": true}

func TestEveryFieldHasSix(t *testing.T) {
	count := map[string]int{}
	for _, d := range All() {
		if !fields[d.Field] {
			t.Fatalf("%s: unknown field %q", d.ID, d.Field)
		}
		count[d.Field]++
	}
	if len(count) != 7 {
		t.Fatalf("want 7 fields, got %d", len(count))
	}
	for f, n := range count {
		if n != 6 {
			t.Errorf("field %s has %d disciplines, want 6", f, n)
		}
	}
}

// 别名必须全局唯一：两门学科抢同一个别名，T1 就会按文件顺序静默选一个，
// 而那个顺序对读代码的人是不可见的。
func TestAliasesAreGloballyUnique(t *testing.T) {
	owner := map[string]string{}
	for _, d := range All() {
		for _, a := range d.Aliases {
			n := Normalize(a)
			if prev, dup := owner[n]; dup {
				t.Errorf("alias %q claimed by both %s and %s", a, prev, d.ID)
			}
			owner[n] = d.ID
		}
	}
}

func TestEveryDisciplineIsComplete(t *testing.T) {
	for _, d := range All() {
		if d.Zh == "" || d.En == "" || d.Asks == "" || d.Method == "" || d.Exemplar == "" {
			t.Errorf("%s: incomplete", d.ID)
		}
		if len(d.Aliases) < 3 {
			t.Errorf("%s: %d aliases, want >= 3 (T1 是免费那一档，别名少就全掉到模型上)", d.ID, len(d.Aliases))
		}
	}
}

func TestMatchAliasFindsDiscipline(t *testing.T) {
	d, ok := MatchAlias(Normalize("代表性"))
	if !ok || d.ID != "statistical-inference" {
		t.Fatalf("MatchAlias(代表性) = %v, %v", d.ID, ok)
	}
}

// 见 vocab_test.go：go:embed 出不了包目录，所以副本靠这条测试守住。
func TestEmbeddedCopyMatchesSourceOfTruth(t *testing.T) {
	src, err := os.ReadFile("../../../../packages/contracts/disciplines/disciplines.json")
	if err != nil {
		t.Fatalf("read source of truth: %v", err)
	}
	if !bytes.Equal(src, disciplinesJSON) {
		t.Fatal("apps/api/internal/disciplines/disciplines.json 已与 packages/contracts/disciplines/disciplines.json 漂移——把真相源复制过来重跑")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/disciplines/ -run Test -v`
Expected: FAIL — package 不存在

- [ ] **Step 3: 写 `disciplines.go`**

```go
// Package disciplines 是学科表——自上而下那一张图。
//
// 它是内容，不是用户数据，所以它是一份 JSON 而不是一张表。学生的关键词通过
// internal/interest 的路由连到这里的 id 上。
//
// 「学科为骨、课程为投影」：Syllabus 是可以为空的投影层。考试局改考纲时改的是
// 这个数组，学生的关键词连的是 id（"statistical-inference"），所以不受影响；
// 空的 Syllabus 表示「这门学科高中不教」——那是真话，也是有吸引力的真话。
//
// disciplines.json 是故意重复的：可编辑的真相源在
// packages/contracts/disciplines/disciplines.json，go:embed 出不了本包目录，
// 所以这里放一份逐字节副本，由 TestEmbeddedCopyMatchesSourceOfTruth 守着。
// 不要"简化"掉其中任何一份。
package disciplines

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed disciplines.json
var disciplinesJSON []byte

type SyllabusRef struct {
	Board string `json:"board"` // IB | ALevel | AP | IGCSE | University
	Code  string `json:"code"`
	Label string `json:"label"`
	Level string `json:"level,omitempty"`
}

type Discipline struct {
	ID       string        `json:"id"`
	Field    string        `json:"field"`
	Zh       string        `json:"zh"`
	En       string        `json:"en"`
	Asks     string        `json:"asks"`
	Method   string        `json:"method"`
	Exemplar string        `json:"exemplar"`
	Aliases  []string      `json:"aliases"`
	Syllabus []SyllabusRef `json:"syllabus"`
}

var (
	once   sync.Once
	all    []Discipline
	byID   map[string]Discipline
	byAlia map[string]Discipline
)

func load() {
	once.Do(func() {
		if err := json.Unmarshal(disciplinesJSON, &all); err != nil {
			panic("disciplines.json is malformed: " + err.Error())
		}
		byID = make(map[string]Discipline, len(all))
		byAlia = make(map[string]Discipline, len(all)*8)
		for _, d := range all {
			byID[d.ID] = d
			for _, a := range d.Aliases {
				byAlia[Normalize(a)] = d
			}
			// 学科自己的中英文名也是别名，不必在数据里重复写一遍。
			byAlia[Normalize(d.Zh)] = d
			byAlia[Normalize(d.En)] = d
		}
	})
}

func All() []Discipline { load(); return all }

func ByID(id string) (Discipline, bool) { load(); d, ok := byID[id]; return d, ok }

func ByField(field string) []Discipline {
	load()
	out := make([]Discipline, 0, 6)
	for _, d := range all {
		if d.Field == field {
			out = append(out, d)
		}
	}
	return out
}

func MatchAlias(normalized string) (Discipline, bool) {
	load()
	d, ok := byAlia[normalized]
	return d, ok
}

// Normalize 把一个词压成可比较的形式：去掉空白与常见标点、转小写。
// 路由的 T1 档和别名唯一性测试共用它，所以两边不可能对"相同"有不同看法。
func Normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch r {
		case ' ', '\t', '\n', '·', '、', '，', ',', '。', '.', '「', '」', '"', '\'', '-', '_', '/':
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
```

- [ ] **Step 4: 写 42 门数据**，先写 `packages/contracts/disciplines/disciplines.json`，再 `cp` 到 `apps/api/internal/disciplines/disciplines.json`
- [ ] **Step 5: 写 `packages/contracts/src/discipline.ts`**（Zod schema 镜像上面的 Go 结构 + `export const DISCIPLINES`，前端构建期 import）
- [ ] **Step 6: 跑测试确认通过** — `cd apps/api && CGO_ENABLED=0 go test ./internal/disciplines/ -v`
- [ ] **Step 7: 提交**

---

### Task 2: 迁移 0116 + sqlc 查询

**Files:**
- Create: `apps/api/internal/store/migrations/0116_interest_model.sql`
- Create: `apps/api/internal/store/queries/interest.sql`

**Interfaces:**
- Produces: sqlc 方法 `UpsertInterestKeyword` · `AddKeywordSource` · `ListInterestKeywords` · `ListKeywordSources` · `UpsertKeywordDiscipline` · `ListKeywordDisciplines` · `CountKeywordSources`

```sql
-- +goose Up
-- 兴趣模型：她的树。
--
-- 断言很强，必须挣得到：树上每一个词都来自她真做过的一件事。所以
-- keyword_source 里的 evidence（她自己的那句话）是 NOT NULL——没有原话的
-- 词是装饰，抽屉里也没东西可给她看，落库这一层就把它挡掉。
--
-- strength 不手填：它由不同来源的条数推出来（见 internal/interest）。
CREATE TABLE interest_keyword (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  text_zh     text NOT NULL,
  text_en     text NOT NULL DEFAULT '',
  norm        text NOT NULL,            -- 归一化后的 text_zh，去重就靠它
  field       text NOT NULL CHECK (field IN
                ('formal','science','making','society','humanities','arts','self')),
  strength    int  NOT NULL DEFAULT 1 CHECK (strength BETWEEN 1 AND 5),
  note        text NOT NULL DEFAULT '',
  first_seen_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, norm)
);

CREATE TABLE keyword_source (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  keyword_id  uuid NOT NULL REFERENCES interest_keyword(id) ON DELETE CASCADE,
  kind        text NOT NULL CHECK (kind IN ('reading','writing','project','news','quiz')),
  ref_id      uuid,                     -- 对应的 atom；quiz/news 可以没有
  label       text NOT NULL,
  evidence    text NOT NULL,            -- 她自己的一句话。空串由应用层挡掉。
  happened_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (keyword_id, kind, ref_id)     -- 同一篇不重复计入强度
);

-- 词 → 学科的边。how 记下这条边是怎么来的，因为便宜档和模型档的可信度不一样，
-- 而 student 档（她自己改的）永远压过前三档。
CREATE TABLE keyword_discipline (
  keyword_id    uuid NOT NULL REFERENCES interest_keyword(id) ON DELETE CASCADE,
  discipline_id text NOT NULL,
  confidence    real NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
  how           text NOT NULL CHECK (how IN ('alias','cooccur','llm','student')),
  rationale     text NOT NULL DEFAULT '',
  PRIMARY KEY (keyword_id, discipline_id)
);

CREATE INDEX interest_keyword_user_idx ON interest_keyword (user_id);
CREATE INDEX keyword_source_keyword_idx ON keyword_source (keyword_id);

-- +goose Down
DROP TABLE keyword_discipline;
DROP TABLE keyword_source;
DROP TABLE interest_keyword;
```

- [ ] **Step 1:** 写迁移；`ls apps/api/internal/store/migrations/ | tail -3` 确认 `0116` 没被别人占用
- [ ] **Step 2:** 写 `queries/interest.sql`（`-- name: X :one|:many|:exec` 形式，见 `queries/writing.sql`）
- [ ] **Step 3:** `cd apps/api && CGO_ENABLED=0 go tool sqlc generate`
- [ ] **Step 4:** `CGO_ENABLED=0 go build ./...` 确认生成物能编译
- [ ] **Step 5:** 提交

---

### Task 3: `internal/interest` —— 归一化与合并（纯逻辑）

**Files:** Create `apps/api/internal/interest/merge.go` · Test `apps/api/internal/interest/merge_test.go`

**Interfaces:**
- Consumes: `disciplines.Normalize`
- Produces: `Strength(sourceCount int) int` · `SameKeyword(a, b string) bool`

规则（来自 spec §2.1）：1 源→1，2→2，3-4→3，5-7→4，8+→5。

- [ ] **Step 1: 写失败的测试**

```go
func TestStrengthLaddersBySourceCount(t *testing.T) {
	for _, c := range []struct{ sources, want int }{
		{0, 1}, {1, 1}, {2, 2}, {3, 3}, {4, 3}, {5, 4}, {7, 4}, {8, 5}, {40, 5},
	} {
		if got := Strength(c.sources); got != c.want {
			t.Errorf("Strength(%d) = %d, want %d", c.sources, got, c.want)
		}
	}
}

func TestSameKeywordIgnoresPunctuationAndCase(t *testing.T) {
	if !SameKeyword("例外与代表性", "例外 与 代表性") {
		t.Error("空格不该让同一个词变成两个")
	}
	if !SameKeyword("Sampling", "sampling") {
		t.Error("大小写不该让同一个词变成两个")
	}
	if SameKeyword("气候与海洋", "记忆怎么形成") {
		t.Error("两个不同的词被判成同一个")
	}
}
```

- [ ] **Step 2:** 跑，确认 FAIL
- [ ] **Step 3:** 实现（`Strength` 用查表；`SameKeyword` = `Normalize(a) == Normalize(b)`）
- [ ] **Step 4:** 跑，确认 PASS
- [ ] **Step 5:** 提交

---

### Task 4: 路由 T1 + T2（纯逻辑，不调模型）

**Files:** Create `apps/api/internal/interest/router.go` · Test `router_test.go`

**Interfaces:**
- Produces:
```go
type Route struct { DisciplineID string; Confidence float32; How string; Rationale string }
type Known struct { KeywordNorm string; DisciplineIDs []string; SourceRefs []string }
// RouteCheap 先试别名（T1，置信 1.0），再试共现（T2，置信 0.6）。
// 两档都没命中就返回 nil ——调用方据此决定要不要花那一次模型调用。
func RouteCheap(text string, sourceRefs []string, known []Known) []Route
```

- [ ] **Step 1: 写失败的测试**

```go
func TestT1AliasHitIsFreeAndCertain(t *testing.T) {
	got := RouteCheap("代表性", nil, nil)
	if len(got) != 1 || got[0].DisciplineID != "statistical-inference" {
		t.Fatalf("别名没命中：%+v", got)
	}
	if got[0].How != "alias" || got[0].Confidence != 1.0 {
		t.Errorf("want alias/1.0, got %s/%v", got[0].How, got[0].Confidence)
	}
}

func TestT2InheritsFromAKeywordSharingTwoSources(t *testing.T) {
	known := []Known{{
		KeywordNorm:   "例外与代表性",
		DisciplineIDs: []string{"statistical-inference"},
		SourceRefs:    []string{"r-coral", "w-coral", "n-0829"},
	}}
	got := RouteCheap("样本量够不够", []string{"r-coral", "w-coral"}, known)
	if len(got) != 1 || got[0].How != "cooccur" {
		t.Fatalf("共现没继承：%+v", got)
	}
	if got[0].Confidence != 0.6 {
		t.Errorf("want 0.6, got %v", got[0].Confidence)
	}
}

// 只共享一条来源不够——一篇文章里同时出现的两个词经常毫无关系。
func TestT2NeedsTwoSharedSources(t *testing.T) {
	known := []Known{{KeywordNorm: "例外与代表性",
		DisciplineIDs: []string{"statistical-inference"}, SourceRefs: []string{"r-coral"}}}
	if got := RouteCheap("完全无关的词", []string{"r-coral"}, known); len(got) != 0 {
		t.Fatalf("一条共享来源就继承了：%+v", got)
	}
}

func TestRouteCheapReturnsNilOnMiss(t *testing.T) {
	if got := RouteCheap("一个谁也不认识的词", nil, nil); got != nil {
		t.Fatalf("want nil so the caller knows to spend an LLM call, got %+v", got)
	}
}
```

- [ ] **Step 2:** 跑，确认 FAIL
- [ ] **Step 3:** 实现 `RouteCheap`
- [ ] **Step 4:** 跑，确认 PASS
- [ ] **Step 5:** 提交

---

### Task 5: 路由 T3（prompt + 容错解析）

**Files:** Modify `apps/api/internal/interest/router.go` · Test `router_test.go`

**Interfaces:**
- Produces: `BuildRoutePrompt(text, evidence, field string) (system, user string)` · `ParseRouteReply(raw, field string) ([]Route, error)`

**候选只带同一根主枝下的 6 门**，不带 42 门（spec §2.3）。

- [ ] **Step 1: 写失败的测试**

```go
func TestRoutePromptCarriesOnlyThatFieldsSix(t *testing.T) {
	_, user := BuildRoutePrompt("样本量", "我去查了那片珊瑚有多大", "formal")
	if !strings.Contains(user, "statistical-inference") {
		t.Error("同枝的候选没进 prompt")
	}
	if strings.Contains(user, "art-history") {
		t.Error("别的主枝漏进了候选——prompt 该只带 6 门")
	}
	if !strings.Contains(user, "我去查了那片珊瑚有多大") {
		t.Error("她的原话没进 prompt，模型就只能猜词面")
	}
}

func TestParseRouteReplyToleratesCodeFences(t *testing.T) {
	raw := "```json\n{\"routes\":[{\"id\":\"statistical-inference\",\"why\":\"她在问代表性\"}]}\n```"
	got, err := ParseRouteReply(raw, "formal")
	if err != nil || len(got) != 1 || got[0].DisciplineID != "statistical-inference" {
		t.Fatalf("got %+v, err %v", got, err)
	}
	if got[0].How != "llm" {
		t.Errorf("want how=llm, got %s", got[0].How)
	}
}

// 模型报一个不存在的 id，绝不能凭空造一门学科出来。
func TestParseRouteReplyDropsUnknownIDs(t *testing.T) {
	got, err := ParseRouteReply(`{"routes":[{"id":"astrology","why":"…"}]}`, "formal")
	if err == nil && len(got) != 0 {
		t.Fatalf("不存在的 id 被收下了：%+v", got)
	}
}

// 解析不了就报错，让调用方不长边——绝不返回一个像样的假路由。
func TestParseRouteReplyErrorsOnGarbage(t *testing.T) {
	if _, err := ParseRouteReply("模型今天不想说话", "formal"); err == nil {
		t.Fatal("want an error so the caller records nothing")
	}
}
```

- [ ] **Step 2:** 跑，确认 FAIL
- [ ] **Step 3:** 实现（解析沿用 `reading_questions.go:130` 的办法：剥 code fence、取首 `{` 到末 `}`、`json.Unmarshal`，再用 `disciplines.ByID` 过滤，并校验 field 一致）
- [ ] **Step 4:** 跑，确认 PASS
- [ ] **Step 5:** 提交

---

### Task 6: 采集器（prompt + 解析）

**Files:** Create `apps/api/internal/interest/harvest.go` · Test `harvest_test.go`

**Interfaces:**
- Produces: `type Harvested struct{ TextZh, TextEn, Field, Note, Evidence string }` · `BuildHarvestPrompt(kind, title, body string) (system, user string)` · `ParseHarvestReply(raw string) ([]Harvested, error)`

- [ ] **Step 1: 写失败的测试**

```go
// 树的断言全靠这条：没带她原话的词直接丢掉。
func TestParseHarvestDropsKeywordsWithoutEvidence(t *testing.T) {
	raw := `{"keywords":[
	  {"zh":"例外与代表性","en":"Exception vs representative","field":"formal",
	   "note":"你现在会先问这个例子能代表多少","evidence":"我读到面积那一段才反应过来它有多小"},
	  {"zh":"气候","en":"Climate","field":"science","note":"","evidence":""}]}`
	got, err := ParseHarvestReply(raw)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	if len(got) != 1 || got[0].TextZh != "例外与代表性" {
		t.Fatalf("没带证据的词没被丢掉：%+v", got)
	}
}

func TestParseHarvestRejectsUnknownField(t *testing.T) {
	raw := `{"keywords":[{"zh":"占星","en":"Astrology","field":"magic","note":"n","evidence":"e"}]}`
	got, _ := ParseHarvestReply(raw)
	if len(got) != 0 {
		t.Fatalf("野生 field 被收下了：%+v", got)
	}
}

func TestParseHarvestCapsAtThree(t *testing.T) {
	// 一次完成最多长三个词：一篇文章长出八个词，树一周就成了灌木。
	var b strings.Builder
	b.WriteString(`{"keywords":[`)
	for i := 0; i < 8; i++ {
		if i > 0 { b.WriteString(",") }
		fmt.Fprintf(&b, `{"zh":"词%d","en":"w%d","field":"science","note":"n","evidence":"e"}`, i, i)
	}
	b.WriteString(`]}`)
	got, _ := ParseHarvestReply(b.String())
	if len(got) != 3 {
		t.Fatalf("want 3, got %d", len(got))
	}
}

func TestParseHarvestErrorsOnGarbage(t *testing.T) {
	if _, err := ParseHarvestReply("抱歉，我没能理解"); err == nil {
		t.Fatal("want an error so nothing is planted")
	}
}
```

- [ ] **Step 2:** 跑，确认 FAIL
- [ ] **Step 3:** 实现
- [ ] **Step 4:** 跑，确认 PASS
- [ ] **Step 5:** 提交

---

### Task 7: 落库 + `GET /api/v1/interest/tree`

**Files:** Create `apps/api/internal/api/interest.go` · Modify `apps/api/internal/api/api.go`（注册路由，`liteOnly`）

**Interfaces:**
- Consumes: Task 1-6 全部；`a.d.Queries`、`a.resolveEval`、`gateway.Collect`、`a.recordLiteLLMCall`
- Produces: `func (a *API) plantKeywords(ctx context.Context, userID uuid.UUID, kind string, refID uuid.UUID, label string, hs []interest.Harvested) error` · `func (a *API) getInterestTree(w http.ResponseWriter, r *http.Request)`

响应形状（P2 的前端按它写）：

```json
{ "fields": [{ "id":"formal", "keywordCount": 3 }],
  "keywords": [{
     "id":"…", "textZh":"例外与代表性", "textEn":"Exception vs representative",
     "field":"formal", "strength":4, "note":"…", "firstSeenAt":"2026-08-29T…",
     "sources":[{"kind":"reading","refId":"…","label":"红海北端那片不白化的珊瑚",
                 "evidence":"我读到面积那一段才反应过来它有多小","happenedAt":"…"}],
     "disciplines":[{"id":"statistical-inference","zh":"统计推断","en":"Statistical inference",
                     "asks":"一个样本，能替多少人说话？","method":"…","exemplar":"…",
                     "confidence":1.0,"how":"alias","rationale":"",
                     "syllabus":[{"board":"IB","code":"MAA-4","label":"…","level":"HL/SL"}]}]
  }]}
```

- [ ] **Step 1:** 写 `plantKeywords`：一个事务里 upsert 关键词 → 插来源（`evidence == ""` 跳过）→ 重算 `strength = interest.Strength(CountKeywordSources)` → 对**新词**跑 `RouteCheap`，miss 时 `resolveEval` + `Collect` + `ParseRouteReply` + `recordLiteLLMCall`
- [ ] **Step 2:** 写 `getInterestTree`（读三张表 + `disciplines.ByID` 补全静态字段）
- [ ] **Step 3:** 在 `api.go` 的 `Handler()` 里注册 `mux.Handle("GET /api/v1/interest/tree", liteOnly(a.getInterestTree))`
- [ ] **Step 4:** `CGO_ENABLED=0 go test ./internal/api/ -run Interest -timeout 1800s`
- [ ] **Step 5:** 提交

---

### Task 8: 三个采集钩子

**Files:** Create `apps/api/internal/api/interest_harvest.go` · Modify 阅读完成 / 写作定稿 / 项目进入复盘三处

**Interfaces:**
- Consumes: `a.plantKeywords`
- Produces: `func (a *API) harvestFromAtom(ctx context.Context, userID, atomID uuid.UUID, kind, title, body string)`

- [ ] **Step 1:** 写 `harvestFromAtom`：`BuildHarvestPrompt` → `resolveEval` → `Collect` → `recordLiteLLMCall` → `ParseHarvestReply` → `plantKeywords`。**解析失败只 `slog.Warn`，不长词，也不让学生的请求失败。**
- [ ] **Step 2:** 三处触发点各加一行调用（`go a.harvestFromAtom(...)` 用一个脱离请求生命周期的 context，采集绝不能拖慢她的响应）
- [ ] **Step 3:** `CGO_ENABLED=0 go test ./... -timeout 1800s`
- [ ] **Step 4:** 提交

---

## Self-Review

**Spec coverage:** §2.1 关键词模型 → Task 2/3/7；§2.2 学科模型 + 课程投影 → Task 1；§2.3 路由三档 + 学生覆盖 → Task 4/5（`how="student"` 的写入路径在 P2 抽屉里接，P1 只备好列枚举与主键）；§3 采集器五个触发点 → Task 6/8（news/quiz 两个触发点属于 P4/P3，本期不接）；§4 P1 条目 → Task 1-8 全覆盖。

**已知的 P1 缺口（有意）：** `how="student"` 无写入端点（P2）· news/quiz 采集（P4/P3）· 七根主枝的几何（P2）。
