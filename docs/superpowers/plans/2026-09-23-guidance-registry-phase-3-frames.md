# 三期 · 句式与例句 —— 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `methods.json` 里那 28 条句式从「一行英文骨架」变成「骨架 + 中文读法 + 一句例句」，并让写英文的学生在卡住求助时也能拿到句式 —— 今天那条路上只给中文的。

**Architecture:** `vocab.Pattern` 加两个字段（`gloss`、`example`），两份 `methods.json` 同步改，Go 结构体和 lite 的 TS 类型跟着加，`VocabExamples.tsx` 多渲染两行。求助那条路改两处：`writingHelpFrames` 收 lang，调用点去掉 `if lang == "zh"` 那道闸。

**Tech Stack:** Go 1.26（`~/sdk/go1.26.0/bin/go`，`CGO_ENABLED=0`）、React + TypeScript + Vite、vitest。

**Spec:** `docs/superpowers/specs/2026-09-22-teaching-guidance-registry-design.md` §6「三期 · 句式与例句」

---

## Global Constraints

- **模板照给。** 产品负责人 2026-09-23：「so templates are totally ok」。句式、模板、更好的句子形式都给她，她自己用到自己的文章里。spec §7 那条「不收整句填空模板」已经撤掉了。
- **门只有一句：不替她写、不替她改完她的文章。** 说这句话本身，不要引条款、不要展开。
- **例句就写一句，标签写「例句：」。** 产品负责人 2026-09-23：「just say 例句： is ok」。方法级的 `examples` 带着 `topic` 标签是那一处的做法，句式**不照搬** —— 一句话的例子不需要一段免责声明。所以句式的 `example` 是**一个字符串**，不是 `{topic, text}`。
- **例句仍然写学生不会拿来写这一篇的话题**（`internal/vocab/vocab.go` 开头那段注释里的规矩），但那是**写的时候的分寸**，不是要渲染出来的东西。
- **术语只从注册表来。** 不在散文里造词。
- **界面文案：标签是名词，不写文学腔。**
- **两份 `methods.json` 必须一起改。** `packages/contracts/vocab/methods.json` 是真相源，`apps/api/internal/vocab/methods.json` 是 `go:embed` 用的逐字节副本（go:embed 出不了自己的包目录）。`TestEmbeddedCopyMatchesSourceOfTruth`（`apps/api/internal/vocab/vocab_test.go:206`）会比对两者。**只改一份，测试会红，而且红得莫名其妙。**

## 🚨 门

碰前端：

```
cd apps/lite-web && npm run typecheck
cd apps/lite-web && npm run test
cd apps/web && npm run typecheck
cd apps/web && npm run build
cd apps/web && npx vitest run
```

碰后端：

```
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go build ./...
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -count=1 ./internal/vocab/... ./internal/guidance/... ./internal/prompts/... ./cmd/promptinspect/... ./cmd/routebench/...
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -count=1 ./internal/api -run 'Writing|Vocab|Guide|Help|Stall' -timeout 1800s
```

## 🚨 侦察查实的五件事（别再查一遍）

1. **`methods.json` 没有任何 Zod / TS 契约。** `packages/contracts/` 里没有校验它的 schema（`registry.ts` 引的 `lens-methods.json` 是同名的另一个东西）。今天唯一的校验是 Go 的结构体 + `encoding/json`，而且**不是 strict** —— 加字段不会被任何东西挡住，也没有 TS schema 要更新。
2. **28 条句式的分布**：`en` 12 个方法 23 条，`zh` 3 个方法 5 条。所以 `gloss`（中文读法）主要是给英文那 23 条用的。
3. **句式今天就已经摆在学生面前了**，不是这一期新开的面：`GuideBox.tsx` 的「查看例子」→ `VocabExamples.tsx`，标题「常用的句式（横线上的内容要你自己填）：」，渲染 `p.label` 和 `p.frame`。这一期是**在这个已有的面上多渲染两行**。
4. **求助那条路要改两处，不是一处。** spec 只说「把 `if lang == "zh"` 闸拿掉」。但 `writingHelpFrames(genre)`（`writing_stall_prompt.go:53`）自己写死了 `vocab.ForLang("zh", genre)` —— **只去掉闸，英文学生仍然一条句式都拿不到**，只是 helpShow 那段指令下面空着。必须同时让它收 lang。
5. **`vocab.ForLang(lang, genre)`**（`vocab.go:212`）：`speaks` 认 `m.Lang == lang || m.Lang == "any"`；`suits` 认 `genre == "" || m.Genre == "" || m.Genre == genre`；并且会跳掉 `AppliesTo == WholePiece` 的条目。

---

## 文件清单

- `packages/contracts/vocab/methods.json` —— 28 条句式加 `gloss` + `example`
- `apps/api/internal/vocab/methods.json` —— 同上，逐字节一致
- `apps/api/internal/vocab/vocab.go` —— `Pattern` 加两个字段
- `apps/api/internal/vocab/vocab_test.go` —— `latinFrame` 要看新字段
- `apps/lite-web/src/api/writingRoom.ts:90` —— `WritingGuidePattern` 加两个字段
- `apps/lite-web/src/writings/VocabExamples.tsx` —— 多渲染两行
- `apps/lite-web/test/guideBox.test.tsx` —— 夹具补字段 + 新断言
- `apps/api/internal/api/writing_stall_prompt.go` —— 去闸 + 收 lang
- `apps/api/internal/api/writing_stall_test.go:155` —— 那条钉着「英文拿不到中文句式」的测试要改

---

### Task 1: 数据模型先动，内容先不动

**Files:**
- Modify: `apps/api/internal/vocab/vocab.go`（`Pattern` 结构体）
- Modify: `apps/lite-web/src/api/writingRoom.ts:90`（`WritingGuidePattern`）
- Test: `apps/api/internal/vocab/vocab_test.go`

**Interfaces:**
- Produces: `vocab.Pattern{Label, Frame, Gloss, Example string}` —— `Example` 是**一个字符串**，不是这个包里那个 `{Topic, Text}` 结构体。
- Produces: TS `WritingGuidePattern = { label: string; frame: string; gloss: string; example: string }`

🚨 **这一任务不改任何一条 `methods.json` 的内容。** 加完字段之后，28 条句式的 `gloss` 是空串、`example` 是零值 —— Go 的 `encoding/json` 对缺失字段就是这么填的，不会报错（侦察查实：没有 `DisallowUnknownFields`）。这样做是为了让「加字段」和「写内容」分成两次能各自评审的改动。

- [ ] **Step 1: 写失败的测试**

在 `apps/api/internal/vocab/vocab_test.go` 里加：

```go
// 句式的两个新字段要真的从 JSON 里读进来。这一条在内容写进去之前会红，
// 那是对的 —— Task 2 才填内容。
func TestPatternCarriesGlossAndExample(t *testing.T) {
	m, ok := ByID("en_concession")
	if !ok {
		t.Fatal("en_concession 不在库里")
	}
	if len(m.Patterns) == 0 {
		t.Fatal("en_concession 一条句式都没有")
	}
	p := m.Patterns[0]
	if p.Gloss == "" {
		t.Errorf("%s 的第一条句式没有中文读法", m.ID)
	}
	if p.Example == "" {
		t.Errorf("%s 的第一条句式没有例句", m.ID)
	}
}
```

🚨 先确认 `ByID` 的真实签名（`vocab.go` 里），照它写。如果它只返回一个值，去掉 `ok`。

- [ ] **Step 2: 跑，确认它红**

```
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -count=1 ./internal/vocab/... -run TestPatternCarriesGlossAndExample
```

预期：先是**编译不过**（`p.Gloss` 不存在）。那也算红。

- [ ] **Step 3: 加字段**

`apps/api/internal/vocab/vocab.go` 的 `Pattern`：

```go
// Pattern 是一条句式：骨架（她照着填）、中文读法、以及一句写好的例句。
//
// Gloss 是骨架的中文读法。产品负责人 2026-09-22 的原话：
// 「`Although [反方的事实], [你的主张]` is not a good thing that easy to
// understand. Although(尽管) xxx, xxx, and gives an example?」——
// 一行英文骨架对着一个中学生等于没说，她需要知道这几个词是什么意思。
// 中文那几条句式本来就读得懂，Gloss 留空。
//
// Example 是把这个骨架填满的一整句，写的时候挑一个学生不会拿来写这一篇的
// 话题。屏幕上它前面就一个「例句：」，不摆话题标签 —— 产品负责人
// 2026-09-23：「just say 例句： is ok」。
type Pattern struct {
	Label   string `json:"label"`
	Frame   string `json:"frame"`
	Gloss   string `json:"gloss"`
	Example string `json:"example"`
}
```

`apps/lite-web/src/api/writingRoom.ts` 第 90 行那个类型：

```ts
// 镜像 apps/api/internal/vocab.Pattern。
export type WritingGuidePattern = {
  label: string;
  frame: string;
  gloss: string;
  example: string;
};
```

🚨 **`writing_guide.go:196` 把 `m.Patterns` 整个抄进 DTO**，所以 Go 加了字段，wire 上就有了，DTO 那边不用另外改。确认一下那行是不是真的整体赋值（`Patterns: m.Patterns` 之类），是的话什么都不用动。

- [ ] **Step 4: 跑，确认还是红（但已经是内容红，不是编译红）**

```
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go build ./...
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -count=1 ./internal/vocab/... -run TestPatternCarriesGlossAndExample
```

预期：编译过了，测试红在「没有中文读法 / 没有例句」上。**这就是 Task 1 该停的地方** —— Task 2 填内容。

- [ ] **Step 5: 跑全门（除了那条故意红着的）**

```
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -count=1 ./internal/vocab/... -run 'TestLoad|TestFor|TestEmbedded'
cd apps/lite-web && npm run typecheck
cd apps/lite-web && npm run test
```

🚨 lite 的 typecheck **可能会红** —— `guideBox.test.tsx` 里的夹具（第 24、102、189、222 行）构造的是 `{label, frame}` 字面量，加了必填字段之后它们不完整。把那四处夹具补上 `gloss: ""` 和 `example: ""`，**先不写真内容**，Task 3 再写。

- [ ] **Step 6: 提交**

信息：`feat(vocab): 句式加上中文读法和例句两个字段（内容下一步）`

---

### Task 2: 把 28 条句式的中文读法和例句写出来

**Files:**
- Modify: `packages/contracts/vocab/methods.json`
- Modify: `apps/api/internal/vocab/methods.json`（**逐字节一致**）
- Modify: `apps/api/internal/vocab/vocab_test.go`（`latinFrame` 与新的完整性测试）

这一步是**写内容**，不是写代码。28 条，英文 23 条、中文 5 条。

- [ ] **Step 1: 先写完整性测试**

```go
// 每一条英文句式都要有中文读法 —— 一行英文骨架对着中学生等于没说。
// 中文句式本来读得懂，Gloss 允许为空。例句一条都不能少。
func TestEveryPatternIsTeachable(t *testing.T) {
	for _, m := range All() {
		for i, p := range m.Patterns {
			where := fmt.Sprintf("%s 第 %d 条句式（%s）", m.ID, i+1, p.Label)
			if strings.TrimSpace(p.Frame) == "" {
				t.Errorf("%s 没有骨架", where)
			}
			if m.Lang == "en" && strings.TrimSpace(p.Gloss) == "" {
				t.Errorf("%s 是英文句式却没有中文读法", where)
			}
			if strings.TrimSpace(p.Example) == "" {
				t.Errorf("%s 没有例句", where)
			}
		}
	}
}
```

🚨 `All()` 是占位名 —— 用 `vocab.go` 里真实的那个「取全部方法」的函数名（读一下文件，可能叫 `All`、`Methods`、或者直接用包级变量 `loaded`；它在 `package vocab` 内部测试里可见）。

- [ ] **Step 2: 跑，确认它红 28 次左右**

```
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -count=1 ./internal/vocab/... -run TestEveryPatternIsTeachable
```

- [ ] **Step 3: 写内容**

给 `packages/contracts/vocab/methods.json` 里每一条 `patterns` 条目加 `gloss` 和 `example`。

**写法，照这个样子：**

```json
{
  "label": "Admit then limit",
  "frame": "While it is true that ___, this does not mean ___.",
  "gloss": "尽管……，但这并不意味着……",
  "example": "While it is true that online classes save commuting time, this does not mean they suit every subject."
}
```

**四条硬要求：**

1. **`gloss` 是这个骨架的中文读法，不是翻译练习。** 把英文里那几个起结构作用的词说清楚就够：`While it is true that…, this does not mean…` → 「尽管……，但这并不意味着……」。骨架里的 `___` 在中文读法里写成 `……`。
2. **例句写一个学生不会拿来写这一篇的话题。** 参照库里现有的方法级例句是怎么挑话题的（`南极科考`、`手写 vs 打字` 之类）—— 具体、离她的作业远。这是**写的时候的分寸**，屏幕上不写话题标签。
3. **例句必须是把这个骨架真的填满的一整句。** 半句、还留着 `___` 的，都不算。
4. **中文那 5 条（`analysis_cause` / `analysis_suppose` / `analysis_induce`）`gloss` 留空串**，但**例句照给**。它们的骨架本来就读得懂。

- [ ] **Step 4: 把副本同步过去**

```
cd /Users/houyuxin/08Coding/mind-imprint/.claude/worktrees/writing-polish-r3-0920
cp packages/contracts/vocab/methods.json apps/api/internal/vocab/methods.json
shasum -a 256 packages/contracts/vocab/methods.json apps/api/internal/vocab/methods.json
```

两行哈希必须一样。

- [ ] **Step 5: 补一条会漏的守卫**

`vocab_test.go:369` 那个 `latinFrame` 辅助函数只看 `p.Frame` 里有没有拉丁字母，`TestFor_NeverOffersEnglishWordingToAChinesePiece`（第 152 行）靠它。现在 `Gloss` 和 `Example` 也可能带英文 —— 把这两个字段也纳入它扫描的范围，否则一条英文例句漏进中文那一篇，这条测试看不见。

读一遍 `latinFrame` 现在的实现再改，**别改它的判定口径，只扩大它看的范围**。

- [ ] **Step 6: 跑门**

```
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -count=1 ./internal/vocab/...
```

`TestEveryPatternIsTeachable`、`TestEmbeddedCopyMatchesSourceOfTruth`、`TestFor_NeverOffersEnglishWordingToAChinesePiece`、`TestLoad_EveryMethodIsUsable` 全绿。

- [ ] **Step 7: 提交**

信息：`feat(vocab): 28 条句式补上中文读法和例句`

---

### Task 3: 把这两样渲染给她看

**Files:**
- Modify: `apps/lite-web/src/writings/VocabExamples.tsx`
- Test: `apps/lite-web/test/guideBox.test.tsx`

- [ ] **Step 1: 写失败的测试**

在 `apps/lite-web/test/guideBox.test.tsx` 里，现有那条 `"shows a method's sentence patterns, which is all an English method carries"`（约第 89 行）旁边加：

```tsx
it("每条句式都带着中文读法和一句例句", async () => {
  // 照这个文件现有的 render 方式来，夹具里那条 pattern 填成：
  //   { label: "Admit then limit",
  //     frame: "While it is true that ___, this does not mean ___.",
  //     gloss: "尽管……，但这并不意味着……",
  //     example: "While it is true that online classes save commuting time, this does not mean they suit every subject." }
  expect(await screen.findByText(/尽管……，但这并不意味着……/)).toBeTruthy();
  expect(screen.getByText(/例句：/)).toBeTruthy();
  expect(screen.getByText(/online classes save commuting time/)).toBeTruthy();
});
```

**照这个文件现有的 render/夹具写法来**，上面只是要断言什么。Task 1 已经把四处夹具补成了空字段，这一步把其中一处填成真内容。

- [ ] **Step 2: 跑，确认它红**

```
cd apps/lite-web && npx vitest run test/guideBox.test.tsx
```

- [ ] **Step 3: 改渲染**

`VocabExamples.tsx` 里那个 `m.patterns.map(...)` 的块，现在是：

```tsx
                <div key={i} className="flex flex-col gap-0.5 border-l-2 pl-2.5" style={{ borderColor: "var(--mk-accent-300)" }}>
                  <span className="text-mk-small text-mk-muted">{p.label}</span>
                  <p className="text-mk-body text-mk-ink">{p.frame}</p>
                </div>
```

改成（骨架下面接中文读法，再接例句；例句照方法级例句那一套，把话题摆在旁边）：

```tsx
                <div key={i} className="flex flex-col gap-0.5 border-l-2 pl-2.5" style={{ borderColor: "var(--mk-accent-300)" }}>
                  <span className="text-mk-small text-mk-muted">{p.label}</span>
                  <p className="text-mk-body text-mk-ink">{p.frame}</p>
                  {p.gloss && (
                    <p className="text-mk-small text-mk-muted">{p.gloss}</p>
                  )}
                  {p.example && (
                    <p className="text-mk-body text-mk-ink">例句：{p.example}</p>
                  )}
                </div>
```

🚨 `p.gloss` 为空时**整行不渲染**（中文那 5 条句式就是这种情况），不要摆一个空的灰条。

- [ ] **Step 4: 跑，确认它绿**

```
cd apps/lite-web && npx vitest run test/guideBox.test.tsx
```

- [ ] **Step 5: 跑全门**

```
cd apps/lite-web && npm run typecheck
cd apps/lite-web && npm run test
cd apps/web && npm run typecheck
cd apps/web && npm run build
cd apps/web && npx vitest run
```

- [ ] **Step 6: 提交**

信息：`feat(lite-writing): 句式下面摆出中文读法和例句`

---

### Task 4: 写英文的学生卡住时也给句式

**Files:**
- Modify: `apps/api/internal/api/writing_stall_prompt.go`
- Test: `apps/api/internal/api/writing_stall_test.go`

**🚨 这里要改两处，不是一处。** spec 只写了「把 `if lang == "zh"` 闸拿掉」，但 `writingHelpFrames(genre)` 自己写死了 `vocab.ForLang("zh", genre)` —— 只去掉闸，英文学生仍然一条都拿不到，helpShow 那段指令下面空着。

- [ ] **Step 1: 改那条钉着旧行为的测试**

`apps/api/internal/api/writing_stall_test.go:155` 的 `TestHelpModeBlockIsSilentByDefault` 现在断言 `writingHelpModeBlock(helpShow, "en", genreArgument)` **不含**「假如」（中文句式的字样）。那条断言本身**仍然要留着** —— 英文学生不该拿到中文句式。但要补上正面的那一半：英文学生该拿到**英文**句式。

```go
// 英文那一篇，helpShow 要给英文句式，而且不许混进中文句式。
func TestHelpShowOffersFramesInThePieceOwnLanguage(t *testing.T) {
	en := writingHelpModeBlock(helpShow, langEnglish, genreArgument)
	if !strings.Contains(en, "While it is true that") {
		t.Error("英文议论文卡住求助，却一条英文句式都没给")
	}
	if strings.Contains(en, "假如") {
		t.Error("英文那一篇里混进了中文句式")
	}

	zh := writingHelpModeBlock(helpShow, "zh", genreArgument)
	if !strings.Contains(zh, "假如") {
		t.Error("中文议论文那条路本来就有句式，不该被这次改动弄丢")
	}
	if strings.Contains(zh, "While it is true that") {
		t.Error("中文那一篇里混进了英文句式")
	}
}
```

🚨 `"While it is true that"` 和 `"假如"` 是**今天库里真有的字**（`en_concession` 的第一条；中文那三个 `analysis_*` 方法之一）。动手前先 `grep` 一遍 `methods.json` 确认，别钉一个不存在的字符串。

- [ ] **Step 2: 跑，确认它红**

```
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -count=1 ./internal/api -run 'HelpShow|HelpMode' -timeout 1800s
```

- [ ] **Step 3: 改两处**

`writing_stall_prompt.go` 第 53 行那个函数收 lang：

```go
// writingHelpFrames 把库里带句式的那几条摆出来，供 helpShow 那一档引用。
//
// 🚨 lang 是参数，不是写死的。2026-09-23 之前这里写死 "zh"，而调用点外面
// 还套着一道 `if lang == "zh"` —— 两处叠在一起，写英文的学生在这条路上
// 一条句式都拿不到，而库里给她备着 23 条。
func writingHelpFrames(lang, genre string) string {
	var b strings.Builder
	for _, m := range vocab.ForLang(lang, genre) {
		for _, p := range m.Patterns {
			b.WriteString("- ")
			b.WriteString(m.Name)
			b.WriteString("：")
			b.WriteString(p.Frame)
			if p.Gloss != "" {
				b.WriteString("（")
				b.WriteString(p.Gloss)
				b.WriteString("）")
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}
```

调用点（第 40 行附近）去掉那道闸：

```go
		frames := writingHelpFrames(lang, genre)
		if frames != "" {
			b.WriteString("\n\n可以给的句式：\n")
			b.WriteString(frames)
		}
```

🚨 `if frames != ""` 那层**留着** —— 某个 lang/genre 组合底下一条句式都没有时，不要摆一个空标题。

🚨 **例句不进这条路。** 这里给模型的是骨架加中文读法，让它挑一条说给她；整句例句由 `VocabExamples` 那个面直接摆给她看。提示词里塞 23 条例句只会把这一轮撑长。

- [ ] **Step 4: 跑，确认它绿**

```
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -count=1 ./internal/api -run 'HelpShow|HelpMode|Stall' -timeout 1800s
```

- [ ] **Step 5: 🚨 parity**

这条路进的是 `writing_guide.go:556` 和 `writing_comment_prompt.go:40` 两处装配。**英文那两条的装配会变**（多出句式），中文那两条**不许变**。

```
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test -count=1 ./cmd/promptinspect/...
```

绿就说明基线里那几条没被碰到。**真红了先看是哪个 id** —— 如果红的是中文那条，说明改错了；如果红的是英文那条，那是这次的功能，按 Task 5b 那次的办法**只重录那一条**，并证明其余的没动。

- [ ] **Step 6: 跑全后端门 + 提交**

信息：`feat(writing): 写英文的学生卡住时也给句式 —— 两道闸都拿掉`

---

## 收尾（控制者做）

- [ ] 全门一遍：前端五条 + 后端三条。
- [ ] 整支终审，模型用最强的那一档。重点：28 条例句的话题是不是真的离学生的作业远、`gloss` 读起来是不是人话、例句是不是真的把骨架填满了。
- [ ] 🚨 真的打开那一屏看一眼：`GuideBox` →「查看例子」，看骨架、中文读法、例句三行的层次对不对，中文那 5 条句式没有多出一条空灰线。
- [ ] 把三期留下的东西写进 spec §8.8。
