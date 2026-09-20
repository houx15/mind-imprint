# 写作面 R2 实施计划 —— 段落级的每一次调用都看得见整篇

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让「请印记看看这一段」「深入一层」「写作引导」三处**都看得见整篇** ——
这一块是什么、别的块写了什么、她已经收到过什么意见 —— 并把反馈契约换成
`verdict: pass ｜ polish ｜ revise` 打头的那个形状。

**Architecture:** 一个新函数 `buildWritingPieceContext` 组装整篇上下文，三处调用点
共用。反馈的输出契约加 `verdict`，`good` 由「必填且排第一」改成可选。两项减法
（后文已承接、她已改过）在服务端做，渲染之前丢掉。按 `kind` 分派每一块该查什么。

**Tech Stack:** Go 1.26（`~/sdk/go1.26.0/bin`）、pgx/sqlc v1.27.0、React + TS、vitest。

**Spec:** `docs/superpowers/specs/2026-09-20-lite-writing-polish-design.md` §6

## Global Constraints

- 每个 shell 先 `export PATH="$HOME/sdk/go1.26.0/bin:$PATH"`。sqlc 用
  `./.tooling/sqlc`（arm64；`~/go/bin/sqlc` 是 x86_64，Rosetta 没了跑不了）。
- Go 测试：`CGO_ENABLED=0 go test ./... -timeout 1800s`。
  🚨 **跑 `./...`，不是只跑 `./internal/api`** —— R1 有五处回归是跑全量才露出来的。
- 🚨 **Go 原始字符串（反引号）里不能再出现反引号。** 提示词里写 `` `pass` ``
  会当场截断那个字符串，报一个和内容毫不相干的 syntax error。用「」。R1 踩了两次。
- 界面文案：标签是名词、按钮写「做什么／做完了」、状态用「已／待／处理中」、
  不写文学腔。见 AGENTS.md「界面文案怎么写」。
- 只写逻辑测试；UI 用真浏览器看。
- Go→TS 边界：新字段必须有 json 标签；nil 切片 marshal 成 `null` 会让前端崩 ——
  返回 `[]T{}`，并写一条 marshal 测试按前端真实读的名字取值。
- 提交信息结尾带 `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`。
- 🚨 **prompt 块顺序是成本契约**（2026-09-20 记的那条）：把**易变的块放在后面**。
  整篇上下文里她的正文会每轮变，必须排在稳定的系统提示词和方法表**之后**，
  否则 DashScope 的隐式缓存整轮落空、全价计费。

## 文件结构

**新建**
- `apps/api/internal/api/writing_piece_context.go` —— 整篇上下文块这一个概念的全部。
- `apps/api/internal/api/writing_piece_context_test.go`
- `apps/api/internal/api/writing_verdict.go` —— `verdict` 闭表 + 两项减法。
- `apps/api/internal/api/writing_verdict_test.go`

**修改**
- `apps/api/internal/api/writing_comment.go` —— 契约加 `verdict`，`good` 改可选，
  系统提示词按 kind 分派检查表，取消「优点+不足」模板。
- `apps/api/internal/api/writing_deepen.go` —— `buildDeepenBrief` 吃整篇上下文。
- `apps/api/internal/api/writing_guide.go` —— `buildWritingGuidePrompt` 同上。
- `apps/api/internal/api/writing_plan_state.go` —— 材料要求跟篇幅/教师要求走。
- `apps/lite-web/src/api/writingRoom.ts` —— `Comment.verdict`。
- `apps/lite-web/src/writings/CommentPanel.tsx` —— verdict 打头，good 可选。

---

### Task 1: 整篇上下文块

**Files:**
- Create: `apps/api/internal/api/writing_piece_context.go`
- Test: `apps/api/internal/api/writing_piece_context_test.go`

**Interfaces:**
- Consumes: R1 的 `writingKindOf` / `writingKindLabel` / `writingCards`。
- Produces:
  `func buildWritingPieceContext(wr sqlc.Writing, outline []sqlc.WritingOutline, snippets []sqlc.WritingSnippet, comments []sqlc.WritingComment, focus *sqlc.WritingOutline) string`

- [ ] **Step 1: 写失败的测试**

```go
// 🚨 同事 2026-09-20 的意见 9：分析第一段时，它不知道例子写在第二、三段，
// 也不知道开头只是一个引子，于是给出了错误的分析结果。
func TestPieceContextShowsTheOtherBlocks(t *testing.T) {
	wr := sqlc.Writing{Lang: "zh", Title: "学校应不应该允许学生带手机"}
	o1 := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindThesis, Depth: 0, Position: 0, Text: "应该允许"}
	o2 := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 1, Text: "放学能联系家长"}
	o3 := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 2, Text: "学习上能查题"}
	outline := []sqlc.WritingOutline{o1, o2, o3}
	snippets := []sqlc.WritingSnippet{
		{OutlineID: pgtype.UUID{Bytes: o2.ID, Valid: true}, Text: "有一次我放学等车，用手机给我妈打了电话。"},
		{OutlineID: pgtype.UUID{Bytes: o3.ID, Valid: true}, Text: "英语单词不会，我查了一下就懂了。"},
	}

	got := buildWritingPieceContext(wr, outline, snippets, nil, &o1)

	// 这一块是什么
	if !strings.Contains(got, "开头") {
		t.Errorf("没说清这一块是开头：\n%s", got)
	}
	// 🚨 别的块**写了什么**，不只是标题 —— 意见 9 的核心就是这个。
	for _, want := range []string{"有一次我放学等车", "英语单词不会"} {
		if !strings.Contains(got, want) {
			t.Errorf("别的段写的内容没进上下文（%q）：\n%s", want, got)
		}
	}
}

// 她这一块之前收到过的意见，以及她做到了没有。
func TestPieceContextCarriesPriorComments(t *testing.T) {
	o := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Position: 0, Text: "放学能联系家长"}
	sn := sqlc.WritingSnippet{
		ID: uuid.New(), OutlineID: pgtype.UUID{Bytes: o.ID, Valid: true},
		Text: "有一次我放学等车，用手机给我妈打了电话，她来接我。",
	}
	points, _ := json.Marshal([]CommentPoint{{
		Kind: "issue", Symptom: "example_no_detail",
		Text: "这件事没有时间地点。", Action: "把那天几点、在哪儿写进去。", Quote: "有一次我放学等车",
	}})
	prior := []sqlc.WritingComment{{
		ID: uuid.New(), Scope: "block", SnippetID: pgtype.UUID{Bytes: sn.ID, Valid: true},
		Summary: "这一段有事，但看不见。", Points: points,
		SourceText: "有一次我放学等车。", // 她后来改过了：现在比这一版长
	}}

	got := buildWritingPieceContext(sqlc.Writing{Lang: "zh"},
		[]sqlc.WritingOutline{o}, []sqlc.WritingSnippet{sn}, prior, &o)

	if !strings.Contains(got, "把那天几点、在哪儿写进去") {
		t.Errorf("上一条意见没进上下文：\n%s", got)
	}
	if !strings.Contains(got, "她已经改过") {
		t.Errorf("没说她已经照着改过了 —— 于是它会再说一遍：\n%s", got)
	}
}

// 🚨 成本契约：她的正文每轮都变，必须排在稳定的块后面。
func TestPieceContextPutsVolatileLast(t *testing.T) {
	o := sqlc.WritingOutline{ID: uuid.New(), Kind: writingKindPoint, Depth: 1, Text: "理由"}
	got := buildWritingPieceContext(sqlc.Writing{Lang: "zh", Title: "题目"},
		[]sqlc.WritingOutline{o}, nil, nil, &o)
	title := strings.Index(got, "题目")
	focus := strings.Index(got, "她现在停在这一块")
	if title < 0 || focus < 0 || title > focus {
		t.Errorf("稳定的块要排在易变的块前面（title=%d focus=%d）：\n%s", title, focus, got)
	}
}
```

- [ ] **Step 2: 跑，确认失败**

```bash
export PATH="$HOME/sdk/go1.26.0/bin:$PATH"
cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestPieceContext -v
```
Expected: 编译失败，`undefined: buildWritingPieceContext`。

- [ ] **Step 3: 实现**

`writing_piece_context.go` 渲染这几段，**顺序固定**（稳定的在前）：

```
【这一篇】题目 / 语言 / 目标篇幅 / 老师的要求
【整篇的结构】按卡片顺序，每一块：第 N 张 · 名字（kind 的标题）· 她定的要点
                 ← **她在那一块写下的正文**（截到 300 字，标注「（后面还有）」）
                 ← 当前这一块标 ← **她现在停在这一块**
【她这一块之前收到过的意见】每条：说了什么 + 让她做什么 + 她做到了没有
【她写的这一段】正文（调用方自己接，这个函数不含）
```

关键实现点，逐条写进注释：

```go
// 🚨 **别的块要带着正文**，不只是标题。
// 同事 2026-09-20 的意见 9：分析第一段时它不知道例子写在第二、三段，
// 于是判「没有一件具体的事」——而那件事就在下一段里。只给标题解决不了这件事，
// 因为标题是她的要点，不是她写的字。
//
// 🚨 但**不能整篇原样塞**：一篇 3000 字的稿子会让每一次「看看这一段」都按
// 整篇计费。每一块截到 writingPieceBlockRunes；截断要**说出来**，不然模型会
// 以为她那一段就写了这么多，然后指责她写得短（[[observation-tool-is-the-bug]]：
// 她的字被切掉，而产品照着被切的那一版评价她）。
const writingPieceBlockRunes = 300
```

- [ ] **Step 4: 跑测试，确认通过**

```bash
export PATH="$HOME/sdk/go1.26.0/bin:$PATH"
cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestPieceContext -v
```

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/api/writing_piece_context.go apps/api/internal/api/writing_piece_context_test.go
git commit -m "feat(lite-writing): 整篇上下文块 —— 段落级的调用终于看得见别的段

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: 三处调用点接上整篇上下文

**Files:**
- Modify: `apps/api/internal/api/writing_comment.go`（`buildWritingCommentPrompt` + `commentOnSnippet` 加载提纲/片段/历史意见）
- Modify: `apps/api/internal/api/writing_deepen.go`（`buildDeepenBrief`）
- Modify: `apps/api/internal/api/writing_guide.go`（`buildWritingGuidePrompt`）
- Test: `apps/api/internal/api/writing_comment_test.go`

**Interfaces:**
- Consumes: Task 1 的 `buildWritingPieceContext`。
- Produces: `buildWritingCommentPrompt(wr, label, text, piece string)` —— 多一个参数。

- [ ] **Step 1: 写失败的测试**

```go
// 🚨 断言的是**喂给模型的那份 prompt 本身**，不是「它能解析」。
// 2026-09-11 的教训：提示词写着「从表里挑」而那张表根本没进 prompt，
// 单元测试全绿，真模型造了两个假 id。
func TestCommentPromptCarriesTheWholePiece(t *testing.T) {
	wr := sqlc.Writing{Lang: "zh", Title: "学校应不应该允许学生带手机"}
	piece := "【整篇的结构】\n- 第 2 张 · 分论点：放学能联系家长\n  她写的：有一次我放学等车……\n"
	got := buildWritingCommentPrompt(wr, "她写的这一段", "开头这一段的字。", piece)
	if !strings.Contains(got, "有一次我放学等车") {
		t.Errorf("整篇上下文没进 prompt：\n%s", got)
	}
}
```

- [ ] **Step 2: 跑，确认失败**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestCommentPromptCarries -v
```

- [ ] **Step 3: `commentOnSnippet` 加载它需要的三样**

它已经有 `at.ID` 和 `wr`，加三次查询（都已存在，不用新 query）：

```go
	outline, oerr := a.d.Queries.ListWritingOutline(turnCtx, at.ID)
	if oerr != nil { /* 读不到就给空 —— 少一份上下文，不该让她连意见都拿不到 */ }
	snippets, _ := a.d.Queries.ListWritingSnippets(turnCtx, at.ID)
	prior, _ := a.d.Queries.ListWritingComments(turnCtx, at.ID)
	focus := blockOfSnippet(outline, snippet) // 可能是 nil（自由段落）
	piece := buildWritingPieceContext(wr, outline, snippets, prior, focus)
```

🚨 三次查询失败都**降级为空上下文**，不报错：上下文是让意见更准的东西，
不是它成立的条件；为了少一份上下文让她按下按钮拿到一个 502，是更糟的交换。

`buildDeepenBrief` 和 `buildWritingGuidePrompt` 同样多收一个 `piece string`，
把它们各自那段「整篇的结构」换掉（那两处现在只列标题，不带正文）。

- [ ] **Step 4: 跑全套，提交**

```bash
export PATH="$HOME/sdk/go1.26.0/bin:$PATH"
cd apps/api && CGO_ENABLED=0 go test ./... -timeout 1800s
git add apps/api/internal/api
git commit -m "feat(lite-writing): 意见 / 深入一层 / 引导 三处都吃整篇上下文

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: `verdict` 分级 + 取消「优点+不足」模板

**Files:**
- Create: `apps/api/internal/api/writing_verdict.go` + test
- Modify: `apps/api/internal/api/writing_comment.go`（`Comment`、`writingCommentSystem`、`parseWritingComment`）

**Interfaces:**
- Produces: `Comment.Verdict string \`json:"verdict"\``；
  `func writingVerdictValid(v string) bool`；常量 `writingVerdictPass/Polish/Revise`。

- [ ] **Step 1: 写失败的测试**

```go
func TestWritingVerdict(t *testing.T) {
	for _, v := range []string{writingVerdictPass, writingVerdictPolish, writingVerdictRevise} {
		if !writingVerdictValid(v) {
			t.Errorf("%q 应当合法", v)
		}
	}
	if writingVerdictValid("很好") {
		t.Error("编出来的 verdict 不该合法")
	}
}

// 🚨 同事 2026-09-20 的意见 10：「取消固定『优点＋不足』模板」。
// good 由「必填且排第一」改成**可选**：确实有一处用对了就点出来，没有就不点，
// 不再为了凑模板去找一条。
func TestCommentWithoutAGoodIsStillValid(t *testing.T) {
	src := "手机可以帮助我们联系家长，也能用来学习。"
	points := []CommentPoint{{
		Kind: "issue", Symptom: "claim_no_evidence",
		Text: "这一句是判断，后面跟着的还是判断。", Action: "挑一件你亲身经历的事写下来。",
		Quote: src,
	}}
	got := validateCommentPoints(points, src, "zh", writingBlockCommentMaxIssues)
	if len(got) != 1 {
		t.Fatalf("没有 good 的一条意见也该活下来，得到 %d 条", len(got))
	}
}

// 🚨 pass 的时候不该硬凑一条 issue 出来。
func TestPassNeedsNoIssue(t *testing.T) {
	if !writingVerdictValid(writingVerdictPass) {
		t.Fatal("pass 不合法？")
	}
	got := validateCommentPoints(nil, "随便什么", "zh", writingBlockCommentMaxIssues)
	if len(got) != 0 {
		t.Fatalf("pass 那一轮可以一条 point 都没有，得到 %d", len(got))
	}
}
```

- [ ] **Step 2: 跑，确认失败**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestWritingVerdict|TestCommentWithoutAGood|TestPassNeedsNoIssue' -v
```

- [ ] **Step 3: 契约与提示词**

`Comment` 加 `Verdict`；输出 JSON 改成：

```
{"verdict":"polish","summary":"…","points":[…]}
```

系统提示词里那一节整段重写（**删掉**「一条 good 打头，后面跟 N 条 issue」）：

```
## 先说这一段现在算什么

verdict 只能是这三个之一：
- 「pass」  这一段站得住了，可以去写下一段。
- 「polish」还可以更好，但**不挡着她往下走**。
- 「revise」必须改：说清是哪一处文字、它让读者产生了什么问题、改到什么程度算完成。

🚨 **可选的优化不要判成必改。** 同事 2026-09-20 指的就是这件事
（「将可选优化判为必改」）。问自己一句：这一处不改，读者读到这一段会不会
真的读不下去、或者得出相反的结论？不会，就是 polish。

## 一段话长什么样

当前判断 → 必要原因 → 下一步动作。**不要「优点＋不足」那个固定模板。**
她确实有一处用对了就点出来（kind 是 good），没有就不点 ——
为了凑模板去找一条，那条肯定是空话，而空话会让她把真话也一起不信。
```

- [ ] **Step 4: 跑，提交**

```bash
cd apps/api && CGO_ENABLED=0 go test ./... -timeout 1800s
git add apps/api/internal/api
git commit -m "feat(lite-writing): 反馈分成通过／可优化／需修改，取消优点+不足模板

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: 两项减法（后文已承接 / 她已改过）

**Files:**
- Modify: `apps/api/internal/api/writing_verdict.go`
- Test: `apps/api/internal/api/writing_verdict_test.go`

**Interfaces:**
- Produces:
  `func dropIssuesLaterBlocksAnswer(points []CommentPoint, focusKind string, laterText string) []CommentPoint`
  `func dropIssuesSheAlreadyFixed(points []CommentPoint, prior []sqlc.WritingComment, snippetID uuid.UUID, now string) []CommentPoint`

- [ ] **Step 1: 写失败的测试**

```go
// 🚨 同事 2026-09-20 的验收标准第一条：
// 「开头预告、正文举例时，不要求开头重复补例子」。
func TestOpeningNotAskedForAnExampleTheBodyCarries(t *testing.T) {
	points := []CommentPoint{{
		Kind: "issue", Symptom: "claim_no_evidence",
		Text: "这一句是判断，没有一件具体的事。", Action: "举一件你经历过的事。", Quote: "手机能帮我们联系家长",
	}}
	later := "有一次我放学等车，用手机给我妈打了电话，她来接我。"
	got := dropIssuesLaterBlocksAnswer(points, writingKindOpening, later)
	if len(got) != 0 {
		t.Fatalf("开头是引子，例子在后面的段里 —— 这一条该丢掉：%+v", got)
	}
	// 正文段里同样的毛病**不能**丢：那一段就是该带例子的地方。
	if got := dropIssuesLaterBlocksAnswer(points, writingKindPoint, later); len(got) != 1 {
		t.Fatalf("正文段的「缺例子」不该被丢掉：%+v", got)
	}
}

// 她照着上一条改过了，就不要再说一遍（意见 10：「排除……已解决和重复的问题」）。
func TestDropIssuesSheAlreadyFixed(t *testing.T) {
	sid := uuid.New()
	old, _ := json.Marshal([]CommentPoint{{
		Kind: "issue", Symptom: "example_no_detail",
		Text: "这件事没有时间地点。", Action: "把那天几点、在哪儿写进去。", Quote: "有一次我放学等车",
	}})
	prior := []sqlc.WritingComment{{
		Scope: "block", SnippetID: pgtype.UUID{Bytes: sid, Valid: true},
		Points: old, SourceText: "有一次我放学等车。",
	}}
	// 她改过了：现在这一版比当初长，而且多了时间地点。
	now := "上周三五点半我放学等车，用手机给我妈打了电话。"
	fresh := []CommentPoint{{
		Kind: "issue", Symptom: "example_no_detail",
		Text: "这件事还是没有时间地点。", Action: "把那天几点、在哪儿写进去。", Quote: "我放学等车",
	}}
	got := dropIssuesSheAlreadyFixed(fresh, prior, sid, now)
	if len(got) != 0 {
		t.Fatalf("她已经改过的那一条不该再说一遍：%+v", got)
	}
	// 🚨 她**没**改的时候不能丢 —— 判错的方向不对称：
	// 她没做完而产品说「过了」，比多说一次更糟。
	same := "有一次我放学等车。"
	if got := dropIssuesSheAlreadyFixed(fresh, prior, sid, same); len(got) != 1 {
		t.Fatalf("她一个字都没改，这一条必须留着：%+v", got)
	}
}
```

- [ ] **Step 2: 跑，确认失败**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestOpeningNotAsked|TestDropIssuesShe' -v
```

- [ ] **Step 3: 实现**

```go
// writingOpeningExcusedSymptoms 是**开头段不该被要求**的那几种毛病。
//
// 🚨 同事 2026-09-20：「这个第一段的分析，这个具体的举例写在了第二段和第三段，
// 但是 ai 在分析第一段的时候没有进行关联，也不知道开头段只是一个引子的作用，
// 给出了错误的分析结果。」
//
// 开头的活是「让读者愿意读下去 + 亮出主张」，不是把全文的证据先摆一遍。
// 所以：这一块是开头、而后面的段里**确实**有具体的事，这一类毛病就丢掉。
// 后面的段里也没有，就不丢 —— 那时候它是真的缺。
var writingOpeningExcusedSymptoms = map[string]bool{
	"claim_no_evidence": true, "example_no_detail": true, "no_example": true,
}
```

`dropIssuesSheAlreadyFixed` 复用 0147 已有的 `SourceText` 逐字比：
同一个 symptom + 这一段的正文**和当初那一版不一样了** ⇒ 丢掉。
🚨 一模一样就留着。

- [ ] **Step 4: 接到 `commentOnSnippet` 上；跑，提交**

---

### Task 5: 按 kind 分派检查表

**Files:**
- Modify: `apps/api/internal/api/writing_comment.go`（`buildWritingCommentSystem` 多收一个 kind）
- Test: `apps/api/internal/api/writing_comment_test.go`

- [ ] **Step 1: 写失败的测试**

```go
func TestCommentSystemDispatchesByKind(t *testing.T) {
	opening := buildWritingCommentSystem("zh", 1, writingKindOpening)
	if !strings.Contains(opening, "不要求开头自带完整事例") {
		t.Errorf("开头那一份没说清它的活：\n%s", opening)
	}
	point := buildWritingCommentSystem("zh", 1, writingKindPoint)
	for _, want := range []string{"理由", "证据", "解释", "限制"} {
		if !strings.Contains(point, want) {
			t.Errorf("正文那一份缺 %q", want)
		}
	}
	closing := buildWritingCommentSystem("zh", 1, writingKindClosing)
	if !strings.Contains(closing, "收束全文") {
		t.Errorf("结尾那一份没说清它的活")
	}
}
```

- [ ] **Step 2-4: 实现、跑、提交**

每一块的检查表（`docs/2026-08-09-all-statuses.md` §6 补了「限制」和
「与其他段的关系」两项）：

| kind | 查什么 |
|---|---|
| `opening` | 观点与题意是否对上。**不要求开头自带完整事例。** |
| `point`/`counter` | 理由、证据、解释三者的关联；这条理由的**限制**；它和别的段的关系 |
| `rebuttal` | 回应是不是针对反方说的那一点 |
| `closing` | 是否收束全文 |

---

### Task 6: 材料要求跟着任务走（同事的 500 字那条）

**Files:**
- Modify: `apps/api/internal/api/writing_plan_state.go`（`writingPlanNeedOf`）
- Modify: `apps/api/internal/api/writing_plan.go`（「材料有两种」那一节）
- Test: `apps/api/internal/api/writing_plan_ready_internal_test.go`

- [ ] **Step 1: 写失败的测试**

```go
// 🚨 同事 2026-09-20 的验收标准：
// 「500字任务不默认强制多条理由和调查数据」。
func TestShortPieceDoesNotDemandFoundMaterial(t *testing.T) {
	short := sqlc.Writing{Lang: "zh"}
	w := int32(500)
	short.TargetWords = &w
	need := writingPlanNeedOf(short)
	if need.Wider != 0 {
		t.Errorf("500 字的短文不该强制一条「她找来的」材料，need.Wider=%d", need.Wider)
	}
	// 长一点的仍然要 —— 一篇 1600 字只拿自己两件事去撑，老师读到的是「我觉得」。
	long := sqlc.Writing{Lang: "zh"}
	lw := int32(1600)
	long.TargetWords = &lw
	if writingPlanNeedOf(long).Wider < 1 {
		t.Error("1600 字仍然要至少一条社会/历史上的材料")
	}
}
```

- [ ] **Step 2-4: 实现、跑、提交**

门槛：`writingPlanWiderFrom = 800`（中文字 / 英文词按 `writingPlanWordsPerPoint`
同一比例换算）。低于它 `Wider = 0`。
提示词里那句「牵涉到人群、趋势、政策、因果的话题**必须**有找来的材料」
加一个前提：**篇幅到得了、或者老师要求了**才必须。

---

### Task 7: 前端接住 verdict

**Files:**
- Modify: `apps/lite-web/src/api/writingRoom.ts`、`src/writings/CommentPanel.tsx`
- Test: `apps/lite-web/src/writings/commentStale.test.ts`（或新建）

- [ ] **Step 1-4**

`Comment` 加 `verdict?: "pass" | "polish" | "revise"`（老评论没有 ⇒ 不渲染那一行）。
面板顶上一枚状态标签，用「已/待/处理中」那套成对词：

| verdict | 标签 | 颜色 |
|---|---|---|
| `pass` | 已通过 | accent |
| `polish` | 可优化 | muted |
| `revise` | 需修改 | danger |

🚨 `polish` **不能长得像错误**：它不挡着她往下走，用 muted 不用 danger。

---

### Task 8: 真模型 + 线上

- [ ] **Step 1: LIVE_LLM**

用同事那个真实用例（500 字带手机那一篇，开头段已经写完、例子在后面两段），
逐条对他给的七条验收标准：

```bash
LIVE_LLM=1 go test ./internal/api -run TestLiveWritingComment -v -count=1
```

断言：开头段**不**被要求补例子；verdict 合法；`polish` 的那一轮不出现
「必须」「需要修改」这类话。

- [ ] **Step 2: 线上走查两趟**，新账号，逐条看截图。

---

## 自查

**规格覆盖**（spec §6）：

| 规格节 | 任务 |
|---|---|
| §6.1 整篇上下文块 | Task 1、2 |
| §6.2 verdict + good 可选 | Task 3、7 |
| §6.3 两项减法 | Task 4 |
| §6.4 按角色分派检查表 | Task 5 |
| §6.5 卡住换帮法 | 复用 `writingPlanStalled`，在 Task 3 的提示词里接 |
| §6.6 材料要求跟任务走 | Task 6 |
| §6.7 不许编 | 已有 `validateCommentPoints`；Task 1 的上下文块里重申一次 |
| §6.8 对话与卡片同步 | Task 3（verdict=pass ⇒ 卡片标「已完成」）+ Task 7 |

**类型一致性：** `Verdict` 在 Go（`Comment.Verdict`）与 TS（`Comment.verdict`）成对；
`buildWritingPieceContext` 的签名在 Task 1 定，Task 2 的三处调用点照它写。
