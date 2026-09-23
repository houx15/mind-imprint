# 四期 · 批改 —— 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 学生端把一个笼统的结论换成按四层分开的等级与颜色；教师端告诉老师每一条意见**是怎么来的**（哪一维、命中哪条毛病、引的是她哪一句）；补上英文记叙那张缺的毛病表；把数得出来的那几维交给服务端数。

**Architecture:** 学生端 `Comment.Verdict`（一条评语一个结论）扩成按 `Layer` 分的等级，等级由服务端从这条评语自己的 points 算出来，不问模型。教师端 `litegrade.Point` 加上「这条来自哪一维、命中哪条 symptom」，按和 `CommentPoint.Symptom` 同一套闭表校验，渲染成批改卡上的一个弹层。毛病表按 `internal/guidance` 的 `Pick` 多登记一张英文记叙。可数的那几维用一个新的共享 `splitSentences` 算出来，当事实喂给批改提示词。

**Tech Stack:** Go 1.26（`~/sdk/go1.26.0/bin/go`，`CGO_ENABLED=0`）、React + TypeScript + Vite、vitest。

**Spec:** `docs/superpowers/specs/2026-09-22-teaching-guidance-registry-design.md` §6「四期 · 批改」、§6.5

---

## Global Constraints

- **不给学生看分数、不给范文。** 产品负责人：「I don't suggest to give exact scores. but we can give them some levels/colored feedbacks.」等级与颜色可以，数字不行。整篇重写的范文**本期不做，要做需要单独点头**。
- **教师端要知道理由。** 产品负责人：「we also need to tell teacher the rationale or the real logic of our comment there (maybe a modal?)」
- **模板照给。** 「so templates are totally ok」—— 批改里提出更好的句子形式是这一期的正事，她自己把它用到自己的文章里。
- **门只有一句：不替她写、不替她改完她的文章。** 教师端批改提示词第一句已经写着这条（`litegrade_prompt.go:21`：「你只给反馈，绝不替学生改：不要重写、不要润色、不要续写，不要给出可以直接替换原文的句子。」）—— 本期不放宽它。
- **服务端数得出来的事实，别让模型每轮自己数。** 这条在 memory 和 `writing_repeats.go` 里都有先例。
- **教法照学，文字自己写。** `docs/reference/writing-teaching/` 下那几份是别人发布的作品，`英语作文批改/SKILL.md` 里埋着 12 处水印与授权指纹。教学内容照收，句子不许整段粘进 `internal/prompts`。
- **界面文案：标签是名词，状态用「已/待/中」，报错「动词+失败」+后台原话。不写文学腔。**
- **术语只从闭表来。**

## 🚨 门（期末那一次不许带 `-run` 过滤）

二期 b 的教训：过滤器 `-run 'Writing|Grade|Class'` 漏掉了 `TestClarityPersonalPlanReady`，整包红着而七次评审都没看见。

每个任务可以跑过滤版本省时间，**收尾必须跑全包**：

```
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go build ./...
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -count=1 ./... -timeout 1800s
cd apps/lite-web && npm run typecheck
cd apps/lite-web && npm run test
cd apps/web && npm run typecheck
cd apps/web && npm run build
cd apps/web && npx vitest run
```

🚨 **判断绿红要看真实退出码。** `go test … | tail` 的退出码是 tail 的 —— 这个仓库栽过两次，二期 b 的终审自己又踩了一次。

## 🚨 侦察查实的八件事（别再查一遍）

1. **`CommentPoint`** —— `apps/api/internal/api/writing_comment.go:97-123`。字段 `Kind`（issue/good）、`Symptom`（闭表 id，只有 issue 有）、`Layer int`（**服务端从 Symptom 算出来，模型给的值从不采信**，注释在 :110-112）、`Method`、`Text`、`Action`、`Quote`。
2. **四层的常量** —— `apps/api/internal/api/writing_symptoms.go:55-60`：`writingLayerClaim=1` 立意 / `writingLayerMaterial=2` 材料 / `writingLayerStructure=3` 结构 / `writingLayerSentence=4` 字句。名字表在 :65-70。注释写着「这四个数字会进 JSON、进数据库、进统计，**不要重新编号**」。
3. **`verdict`** —— `apps/api/internal/api/writing_verdict.go:30-37`，闭集 `pass`/`polish`/`revise`，不认识的值归一到 `polish`**而不是** `revise`（:47-57）。**它挂在 `Comment` 上（`writing_comment.go:137`），一条评语一个**，不是每层一个 —— 这正是本期要扩的地方。
4. **学生端渲染** —— `apps/lite-web/src/writings/CommentPanel.tsx`，`VERDICT_CHIP` 在 :60-79（`polish` 特意**不是**危险色，:54-55 有注释说明「不能长得像错误」），结论 chip 渲染在 :235-242。每条 issue 已经有一个 layer 名字的小标签（:274-278），**但没有按层的颜色**。TS 镜像类型在 `apps/lite-web/src/api/writingRoom.ts`：`CommentPoint`(:164-172)、`COMMENT_LAYER_NAMES`(:175-180)、`CommentVerdict`(:198)、`Comment`(:200-216)。
5. **`liteassign.Rubric`** —— `apps/api/internal/liteassign/rubric.go`：`Rubric{Scale, Max, Dimensions, Focus}`(:13-18)、`RubricDimension{Name, Note}`(:20-23)。`DefaultRubric(lang)`(:40-55)：英文是雅思四维（Task Response / Coherence and Cohesion / Lexical Resource / Grammatical Range and Accuracy），中文是内容/结构/语言/书写规范。🚨 **没有任何字母等级的计算** —— `GradeInScale`(:118-131) 只校验模型给的等级在不在 `LetterGrades`(:37) 里。
6. **教师端批改卡** —— `apps/lite-web/src/teacher/GradingCard.tsx`（`GradingPreview` :113-181、`GradingEditor` :183-301）。弹层有两套现成的：`controls/Popover.tsx`（锚定小面板，Select/DateField 在用）和 `ReturnDialog.tsx`(:65,72) 那种居中 `fixed inset-0 role="dialog"` + Esc 关闭。**理由弹层要放一维 + 一条毛病 + 一句引文，用居中那套。**
7. **毛病表** —— 全在 `apps/api/internal/api/writing_symptoms.go`：`writingSymptomsZH`(:89-135) 16 条、`writingSymptomsEN`(:147-183) 13 条（**一张表，不分文体**）、`writingSymptomsNarrativeZH`(:194-210) 5 条（是**加在** zh 那张上面，不是替换）。选取用 `writingSymptomTable(lang, genre)`(:218-244) 走 `guidance.Pick`。🚨 :216-217 的注释明写着「英文今天只有一张表，不分文体。补英文记叙那一张属于四期」，并且有 `TestWritingSymptomTableENIgnoresGenreForNow` 钉着现状 —— **那条测试是本期计划内的报废**。
8. **可数的那几维：一件现成的都没有。** 没有 type-token、没有平均句长、没有复杂句占比、没有连接词密度。有的是两个**互相独立**的 `CountWords`（`internal/agent/wordcount.go` 和 `internal/evalreport/countwords.go`），以及**六处各写各的**句末标点判断（`writing_plan.go:250`、`reading_coach_board_build.go:165`、`reading_coach.go:2095`、`reading_coach_card.go:319,795`、`teacher/weekly.go:119`、`atom_report.go:583`）—— **没有共享的 `splitSentences`**。本期要新建一个。
9. **教师端的「理由」链路今天不存在。** `litegrade.Point`（`apps/api/internal/litegrade/content.go:46-52`）只有 `Kind/Quote/Text/Action/Source` —— 没有 symptom、没有维度、没有 layer。提示词确实把 `SymptomCatalog` 渲染进去了（`prompt.go:27`）并允许模型在散文里提毛病名，但**没有任何结构化的东西被返回或存下来**。学生端那边 `CommentPoint` 反而已经有 `Symptom` + `Layer` 了。

---

### Task 1: 学生端 —— 一个结论换成四层的等级

**Files:**
- Modify: `apps/api/internal/api/writing_verdict.go`
- Modify: `apps/api/internal/api/writing_comment.go`（`Comment` 加一个按层的等级表）
- Modify: `apps/lite-web/src/api/writingRoom.ts`
- Modify: `apps/lite-web/src/writings/CommentPanel.tsx`
- Test: `apps/api/internal/api/writing_verdict_test.go`（或该文件现有的测试文件）、`apps/lite-web/test/commentPanel.test.tsx`

**Interfaces:**
- Produces: `Comment.LayerVerdicts map[int]string` —— 键是 1..4（`writingLayerClaim` 等），值是 `pass`/`polish`/`revise` 闭集里的一个。**服务端算，不问模型。**
- Produces: TS `Comment.layer_verdicts: Record<string, CommentVerdict>`

**算法（写进 Go 注释，不进提示词）：** 一层的等级由**落在这一层的 points** 决定 ——
这一层一条 issue 都没有 → `pass`；有 issue 但都不带 `Action` 的硬毛病 → `polish`；
否则 `revise`。**具体判据由实施者按 `writing_verdict.go` 现有的语义定，但必须满足：
只看这条评语自己的 points，不看别的评语，也不问模型。**

🚨 **保留 `Comment.Verdict` 这个字段，不要删。** 它是整条评语的结论，今天所有地方都在用；本期是在它**旁边**加一张按层的表，不是替换它。删掉它会牵动一大片，而且老数据里没有新字段。

- [ ] **Step 1: 写失败的测试**

在 Go 侧先钉住算法：

```go
// 一层的等级只看落在这一层的 points。四层各自独立 —— 立意没问题、
// 字句一堆问题，她该看到的是「立意 pass，字句 revise」，而不是一个
// 笼统的 polish 把两件事拌在一起。
func TestLayerVerdictsComeFromThatLayerOnly(t *testing.T) {
	c := Comment{Points: []CommentPoint{
		{Kind: "issue", Layer: writingLayerSentence, Action: "把这句拆成两句"},
		{Kind: "good", Layer: writingLayerClaim},
	}}
	got := layerVerdictsOf(c)
	if got[writingLayerClaim] != verdictPass {
		t.Errorf("立意那层没有 issue，应该 pass，拿到 %q", got[writingLayerClaim])
	}
	if got[writingLayerSentence] == verdictPass {
		t.Error("字句那层有一条带 action 的 issue，不该是 pass")
	}
	// 没有任何 point 的那两层也要有值 —— 屏幕上四个格子都要有东西。
	if got[writingLayerMaterial] == "" || got[writingLayerStructure] == "" {
		t.Error("没有 point 的层也要给一个等级，不能是空")
	}
}

// 不认识的值归一到 polish 而不是 revise —— writing_verdict.go 已有的
// 非对称兜底，按层算的时候不许把它改成对称的。
func TestLayerVerdictNeverInventsRevise(t *testing.T) {
	c := Comment{Points: []CommentPoint{{Kind: "issue", Layer: 99}}}
	for _, v := range layerVerdictsOf(c) {
		if v == verdictRevise {
			t.Error("一个认不出层号的 point 把某一层判成了 revise")
		}
	}
}
```

🚨 `verdictPass` / `verdictRevise` / `Comment` / `CommentPoint` 的真实标识符先读 `writing_verdict.go:30-37` 和 `writing_comment.go:97-152` 确认，照它们写。

- [ ] **Step 2: 跑，确认它红**

```
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -count=1 ./internal/api -run 'LayerVerdict' -timeout 1800s
```

- [ ] **Step 3: 实现 + 挂到 Comment 上**

写 `layerVerdictsOf(c Comment) map[int]string`，在组装 `Comment` 的地方调用它填进新字段。**四层都要有值**，哪怕那层一个 point 都没有。

- [ ] **Step 4: TS 类型 + 渲染**

`writingRoom.ts` 的 `Comment` 加 `layer_verdicts`。

`CommentPanel.tsx`：在现有那个整体结论 chip 的**下面**，摆四个按层的小 chip（用现成的 `COMMENT_LAYER_NAMES` 取名字，颜色照 `VERDICT_CHIP` 那套）。

🚨 **`polish` 不能长得像错误** —— `VERDICT_CHIP` 在 :54-55 有注释专门说这件事，按层的那四个也照办。
🚨 **不要摆数字。** 没有分数、没有 4/5、没有百分比。

- [ ] **Step 5: 前端测试 + 全门**

- [ ] **Step 6: 提交**

`feat(writing): 学生端的结论按立意/材料/结构/字句分开`

---

### Task 2: 补上英文记叙那张毛病表

**Files:**
- Modify: `apps/api/internal/api/writing_symptoms.go`
- Test: 同文件的测试（`TestWritingSymptomTableENIgnoresGenreForNow` 在那儿）

- [ ] **Step 1: 那条钉着现状的测试是本期计划内的报废**

`TestWritingSymptomTableENIgnoresGenreForNow` 钉的是「英文不分文体」。本期就是要让它分。**删掉它**，换成一条正面的：

```go
// 英文记叙有自己的毛病，和英文议论文不是同一批。
func TestWritingSymptomTableENSplitsByGenre(t *testing.T) {
	arg := writingSymptomTable(langEnglish, genreArgument)
	nar := writingSymptomTable(langEnglish, genreNarrative)
	if len(nar) == 0 {
		t.Fatal("英文记叙一条毛病都没有")
	}
	sameIDs := func(a, b []writingSymptom) bool { /* 逐 id 比 */ }
	if sameIDs(arg, nar) {
		t.Error("英文记叙和英文议论文拿到的是同一张表")
	}
	// 🚨 英文记叙那张是**加在**英文那张上面的，不是替换 ——
	// 照 writingSymptomsNarrativeZH 的做法（:191-193 的注释）。
	if len(nar) <= len(arg) {
		t.Error("记叙那张应该是在通用英文表之上再加几条")
	}
}
```

- [ ] **Step 2: 写 `writingSymptomsNarrativeEN`**

照 `writingSymptomsNarrativeZH`（:194-210，5 条：detail_vague / verb_generic / no_turn / turn_abrupt / feeling_unearned）的**形状**写英文记叙的那几条。id 用英文记叙自己的说法，`Layer` 用那四个常量。

🚨 **不要把中文那 5 条直译过去。** 英文记叙的常见毛病不完全一样（时态跳动、show-don't-tell、对话标点）。教法可以照学，条目自己定。

- [ ] **Step 3: 在 `writingSymptomTable` 里登记**

照现有那几行 `guidance.Row[[]writingSymptom]` 的写法加一行 `{write, en, narrative}`。

🚨 **打分**：`Surface(16)+Lang(8)+Genre(4)=28`，压过现有那条只有 lang 的 `{en}`(24)。和二期 a 的 `Pick` 语义一致。

- [ ] **Step 4..6: 跑门、提交**

`feat(writing): 补上英文记叙的毛病表`

---

### Task 3: 教师端 —— 告诉老师这条意见是怎么来的

**Files:**
- Modify: `apps/api/internal/litegrade/content.go`（`Point` 加两个字段）
- Modify: `apps/api/internal/prompts/litegrade_prompt.go`（让模型返回这两样）
- Modify: `apps/api/internal/litegrade/check.go`（校验）
- Modify: `apps/lite-web/src/api/gradings.ts`（TS 镜像）
- Modify: `apps/lite-web/src/teacher/GradingCard.tsx`（弹层）
- Test: `apps/api/internal/litegrade/` 的测试、`apps/lite-web/test/` 里对应的

**这是产品负责人点名要的那一件。**

- [ ] **Step 1: `Point` 加两个字段**

```go
// Dimension 是这条意见挂在 rubric 的哪一维上（逐字来自 Rubric.Dimensions
// 的 Name）。Symptom 是它命中了毛病闭表里的哪一条（和学生端
// CommentPoint.Symptom 同一套 id）。
//
// 这两样是给**老师**看的：批改卡上点开一条意见，她要看到这条是从哪一维、
// 哪条毛病来的、引的是学生哪一句。产品负责人 2026-09-22：
// 「we also need to tell teacher the rationale or the real logic of our
// comment there」。
//
// 🚨 两个都允许为空 —— 模型给不出来时宁可没有，不要编一个。校验时
// 不在闭表里的直接清空（照 CommentPoint.Symptom 的 lookupWritingSymptom 那套）。
Dimension string `json:"dimension"`
Symptom   string `json:"symptom"`
```

- [ ] **Step 2: 提示词要这两样**

`litegrade_prompt.go` 的输出格式里，`points` 的每一条加 `dimension` 和 `symptom`。
**措辞照 AGENTS.md：陈述句、不打比喻、术语只从闭表来。**
🚨 `dimension` 必须逐字是 rubric 那几维的名字 —— 提示词里已经有一条「names must match rubric exactly」（:26-28），沿用它的说法。

- [ ] **Step 3: 校验 —— 认不出就清空，不要编**

`check.go` 里：`Dimension` 不在这次 rubric 的维度名里 → 清空；`Symptom` 不在闭表里 → 清空。**不要拒绝整条意见**，那会让老师少看到一条真实的反馈。

🚨 这条和 `writing_comment.go` 的 `lookupWritingSymptom` 是同一条纪律，去读一眼它怎么做的。

- [ ] **Step 4: 弹层**

`GradingCard.tsx`：每条意见旁边一个「依据」按钮，点开一个居中弹层（照 `ReturnDialog.tsx` 那套 `fixed inset-0` + Esc 关闭），里面三行：

	维度：Task Response
	对应毛病：论点没有回应题目要求的第二件事
	学生原句：「……」

🚨 **三样都没有的时候不显示那个按钮** —— 一个点开是空的弹层比没有按钮更糟。
🚨 文案是名词标签（「维度」「对应毛病」「学生原句」），不写成句子。

- [ ] **Step 5..7: 测试、全门、提交**

`feat(litegrade): 批改卡能看到一条意见的依据`

---

### Task 4: 数得出来的那几维交给服务端

**Files:**
- Create: `apps/api/internal/textstat/textstat.go` + `textstat_test.go`
- Modify: `apps/api/internal/litegrade/prompt.go`（把算出来的事实喂进去）
- Modify: `apps/api/internal/prompts/litegrade_prompt.go`

- [ ] **Step 1: 一个共享的 `splitSentences`**

🚨 侦察查实：**这个仓库没有共享的句子切分**，只有六处各写各的句末标点判断。新建 `internal/textstat`，先把切分写对，并在注释里列出那六处，留给以后收编（**本期不去改那六处**，那是另一件事）。

中英都要能切：中文 `。！？…`，英文 `.!?` 且要避开 `Mr.` `e.g.` 这类。

- [ ] **Step 2: 四个数**

```go
// TypeTokenRatio 不重复词数 / 总词数。
// MeanSentenceLength 平均句长（词）。
// ComplexSentenceRatio 含从句的句子占比。
// ConnectiveDensity 连接词数 / 句数。
```

🚨 **词数不要再造第三个。** 仓库里已经有两个互相独立的 `CountWords`（`internal/agent/wordcount.go`、`internal/evalreport/countwords.go`）。**挑一个复用**，并在注释里写明挑了哪个、为什么，以及另一个的存在。

- [ ] **Step 3: 当事实喂给批改提示词**

算出来的四个数进提示词的一个事实块，并明写一句：**这几项由系统数出，不要自己再数一遍**。模型只判需要判断力的那几维。

🚨 **不要把这四个数变成给学生看的分数。** 它们是给模型和老师的依据。

- [ ] **Step 4..6: 测试、全门、提交**

`feat(litegrade): 可数的那几维由服务端算`

---

## 收尾（控制者做）

- [ ] **全门一遍，`go test ./...` 不带 `-run`，看真实退出码。**
- [ ] 整支终审，模型用最强的那一档。
- [ ] 🚨 真的打开两屏看：学生端的四个层级 chip（`polish` 不能长得像错误）、教师端的依据弹层。
- [ ] 把四期留下的东西写进 spec §8.9。
- [ ] **范文（提升后范文参考）本期没做** —— 要做需要产品负责人单独点头，写进 spec 备着。
