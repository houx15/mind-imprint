# 教学内容按「面 × 语言 × 文体 × 学段」选取 —— 设计

> 2026-09-22。产品负责人提出要接入英文写作、英文批改、中文批改，并问：
> 我们是不是需要一套 skill 系统？这份 spec 回答那个问题，并给出实现方案。

## 1 · 现状：同一件事有五套做法

「这一篇该用哪份教学内容」今天由五处各自回答，互不知道对方存在：

| # | 位置 | 做法 | 它自己的缺口 |
|---|---|---|---|
| 1 | `internal/api/reading_routines.go` | 每套读法一行，带 `Lang` 与 `Genres[]`；`pickRoutineForGenre` 纠正模型挑错的那一套 | 无。这一套是对的，下面的注册表照它长 |
| 2 | `internal/api/reading_genre.go` | `map[genre]string` 的带读说明 + 各体裁的标注板 | 教学正文住在 `internal/api`，不在 `internal/prompts` |
| 3 | `internal/api/writing_plan_prompt.go` `writingPlanSystemFor` | `switch` 选 `@@KINDS@@` / `@@MATERIAL@@` / `@@SKELETON@@` | 文体靠 `narrativeIdeaMarkers` 关键词**猜**，不是判出来的 |
| 4 | `packages/contracts/vocab/methods.json` + `vocab.ForLang` | JSON 注册表，三条轴 | 12 条英文句式在「她卡住了」那条路上取不到 |
| 5 | `internal/api/writing_symptoms.go` `writingSymptomTable` | 三个 Go 切片 | 有 `NarrativeZH`，**没有英文记叙** |

阅读面因此能分四种体裁，写作面只有两种；阅读面的体裁由模型判、服务端纠，
写作面的体裁由一张关键词表猜。加一个体裁要动四到六个文件。

`docs/reference/writing-teaching/reading-suggestion.md` 其实**已经实现**了 ——
`reading_genre.go` 的文件头逐字引用了它。找不到它是因为它没住在 `internal/prompts`。

## 2 · 产品负责人 2026-09-22 的四条决定

1. **学段在建班时设。** 不按每篇推断。目前只做**中学**；`小学作文` 那份资料
   只作参考，用来找「怎么把学生的兴趣点起来」的做法，不作为要兼容的学段。
2. **不给确切分数。** 给学生等级／颜色。教师端（也有 AI 批改）同样给等级，
   并且要**告诉教师这条意见背后的真实逻辑**。
3. **模板放在批改的时候给。** 我们提出更好的句式，**由她自己**落到她的文章里。
   她卡住并**主动求助**时也给（那是她要的，不是我们塞的）。
4. **模板要带中文注解和例句。** 光给 `Although [反方的事实], [你的主张]` 这种
   带括号的抽象骨架不好懂。要写成 `Although(尽管) ___, ___.`，再给一句例句。

## 3 · 架构：一张注册表，一个纯函数

新增 `apps/api/internal/guidance`。它只回答一件事：这一次该用哪几段教学内容。

```go
package guidance

// Key 是「这一次是什么情况」。四条轴，缺省值都表示「不限」。
type Key struct {
	Surface string // "read" | "write" | "comment"
	Lang    string // "zh" | "en"
	Genre   string // argument | narrative | report | explain
	Grade   string // "" 不限 | junior1..3 | senior1..3，见 §4
}

// Slot 是提示词模板上的一个洞。
type Slot string

const (
	SlotKinds    Slot = "kinds"    // 节点类型闭表
	SlotMaterial Slot = "material" // 材料怎么选
	SlotSkeleton Slot = "skeleton" // 常见文章结构
	SlotCoach    Slot = "coach"    // 这一篇怎么带
	SlotSymptoms Slot = "symptoms" // 毛病表
	SlotRubric   Slot = "rubric"   // 评价维度
	SlotFrames   Slot = "frames"   // 句式与例句
)

// Source 指向一段文本。两种来源并存是故意的：已有的 Go 常量登记进来即可，
// 正文不必在这一次搬家（AGENTS.md「重构时逐字保留」）。
type Source struct {
	Const string // internal/prompts 里的常量名
	File  string // go:embed 的 markdown 路径
}

// Resolve 取这一次要用的每个槽。
//
// 命中顺序：完全匹配 → 去掉 Grade → 去掉 Genre。三步都取不到就报错。
// 🚨 取不到必须报错，不能返回空串：2026-09-22 那次「按语言挑」的教训是
// 挡住中文之后英文那边空了 —— 少给一整块，而线上看起来只是印记话少了。
func Resolve(k Key, slots ...Slot) (map[Slot]string, error)
```

### 🚨 真正的原语是泛型的 Pick，Resolve 只是它上面的一层壳

写实施计划时发现的：五处选取里有两处携带的**不是文本**——
读法是 `readingRoutine`，毛病表是 `[]writingSymptom`。把它们硬塞成字符串
是为了迁就一个只会处理字符串的 `Resolve`，那是本末倒置。

所以核心原语是：

```go
type Row[T any] struct { Scope Scope; Value T }
func Pick[T any](k Key, rows []Row[T]) (T, bool)   // 最具体的那一行胜出
```

`Resolve` 是 `Pick[string]` 上面的一层壳（多做一件事：取不到时报错）。
一条规则、一份测试、五个调用点各带各的类型。

**`vocab.ForLang` / `vocab.Structures` 例外，它们不搬。** 那两个是**过滤器**
（返回一组方法，一页上摆四条论证结构），不是选择器（返回最合适的一条）。
塞进 `Pick` 会把「全都要」变成「只要一条」—— 那是行为改变，不是搬家。
两边的体裁词表由 `TestVocabGenreFieldStaysInTheClosedSet` 钉住一致。

### 为什么不做成模型自己去取的 skill 系统

同事那几份资料是 `SKILL.md` + `references/` 的形状，靠模型按需读文件。
我们这边不采用那个取法，理由有三条，都是这个栈特有的：

1. **网关一轮只发一份拼好的提示词**，陪练那条路上没有 tool-use 循环。
   要让模型自己取，就得多一次检索调用 —— 每一轮都加一次延迟和一次钱。
2. **提示词块的顺序是成本契约**（AGENTS.md）。百炼隐式缓存让阅读陪练从第二轮
   起命中 91–99%。内容随模型当轮决定取哪一份而变，前缀就碎了，那一轮全价。
3. **`TestMainRequestParity` 和 `cmd/promptinspect` 只钉得住离线拼得出来的提示词。**
   换成模型自己取，parity 这道兜底就没有了。

采用的是它的**写法**（markdown 文件、一份内容一个文件），不采用它的**取法**。
选取由服务端确定性地做：不烧调用、可测、可 parity。

## 4 · 闭表的扩充

### 体裁

`reading_outline.go` 那张四值闭表（argument / report / narrative / explain）
升为两个面共用。写作面今天只用 argument / narrative，注册表里其余两个体裁
在 `Surface: "write"` 下没有行 —— `Resolve` 会退到 argument，和今天一致。

**写作面的体裁改成判出来的，不再靠关键词猜。** `writingGenreOf` 的第 1 条
（板上已有的节点类型）保留，它是硬证据；第 2 条那张 `narrativeIdeaMarkers`
关键词表退为兜底，正路改成和阅读面一样由模型在立题那一轮报一个 `genre`，
服务端按闭表校验。理由：那张表收的是「记一次／难忘的／我和」，说明文和
应用文进来时它一条都收不到，而默认 argument 会把一篇说明文按议论文带。

### 学段

`classes` 加一列 `grade text not null default ''`，建班时设。迁移号取 **0186**
（现最高 0185）—— 分支开久了迁移号会撞，撞了 goose 直接起不来，所以这一号要尽早落。

🚨 **叫 `grade`，不叫 `stage`；`guidance.Key.Stage` 同时改名成 `Grade`。**
2026-09-22 二期开工前查出来的：`writing.stage` 在这个仓库里**已经**是另一个
意思 —— 写作**流程**阶段（`ideate|outline|snippets|draft|finished`，
`0099_writing_tables.sql:12-14`），而且 `sqlc.Writing.Stage` 到处在用。
再让 `sqlc.Class.Stage` 和 `guidance.Key.Stage` 表示「几年级」，一个词在同一个
包里就有两个意思。改名现在是零成本（四处 `guidance.Key{...}` 一处都没设过
这个字段），等二期 b 往里填内容之后就不是了。

取值到**年级**这一层，不是 junior / senior 两档：

	'' | junior1 | junior2 | junior3 | senior1 | senior2 | senior3

🚨 这一条 2026-09-22 读完资料之后改过。原来定的是 junior / senior 两档，
而 `初中语文作文批改` 的标准是**按年级**给的 —— 初一 500–600 字「叙事完整、
语句通顺」，初二 550–650 字「描写生动、结构完整」，初三 600–700 字「立意深刻、
语言优美」。两档表达不了这三行，而一个班本来就是一个年级。

`Resolve` 的回退链因此多一级：年级 → 学段（junior / senior）→ 不限。
`guidance.GradeBand("junior2") == "junior"`，让「整个初中通用」的内容只写一份。

年级经 `enrollments` 流到学生，再流到她的 writing / reading。取不到就是 `''`，
`Resolve` 退到不限学段那一行。**本期不产出任何小学内容。**

## 5 · 判据：一条现在写不出来的测试

做这件事的回报就是这条测试：

```
TestEveryCombinationResolves —— 枚举所有真的会出现的
(Surface × Lang × Genre × Grade)：
  1. 模板声明的每一个槽都取得到；
  2. 渲染出来的提示词里不许残留字面的 @@SLOT@@。
```

这正是 AGENTS.md 提示词第 6 条那个事故（`@@KINDS@@` 原样发到线上）在结构上
不再可能发生。今天写作面有四条 文体×语言 分支，parity 基线只覆盖一条；
再加一条学段轴，分支数增长得比人手写样例快。

另外三条：

- `TestGenreClosedSetMatchesReadingAndVocab` —— 三处体裁词表逐字一致
  （照 `TestWritingGenreWordsMatchVocab` 的样子写）。
- `TestResolveNeverReturnsEmpty` —— 任何一个槽取不到都要是 error，不是空串。
- `go test ./cmd/promptinspect` 在每一阶段前后各跑一次，wire request 的 sha256
  必须一致；确实要改措辞就单独一次提交、单独跑一次 `LIVE_LLM`。

## 6 · 分四期

每一期自己能上线、自己能验。

### 一期 · 注册表（不改任何一个字的教学内容）

把上面那五处选取搬到 `Resolve` 上，正文**一个字都不动**（已有常量用
`Source.Const` 登记）。`reading_genre.go` 的 `genreCoachGuide` 正文搬进
`internal/prompts`，因为 AGENTS.md 说教学正文住在那里。

验收：parity 全绿、`TestEveryCombinationResolves` 绿、线上走查不变。
这一期学生看不到任何变化 —— 这是它对的样子。

### 二期 · 英文写作

- `classes.grade` 迁移 0186 + 建班表单那一格（二期 a）。
- 英文议论文的**题目拆解**进 `SlotCoach`：TOPIC + TASK 两段式；四类 TASK
  （agree / discuss / advantage / reason&solution）都可归约成 two tasks。
  这一条来自 `思维印记-英文写作逻辑框架搭建.md`，是那份资料里最有价值的部分。
- **不收那份资料里的填空模板。** `In contemporary society, xxx has become an
  increasingly widely discussed issue on social media.` 这类整句开头违反铁律①，
  而且雅思考官对背诵式开头本来就扣分。它的九维评价标准照收，落在四期。

### 三期 · 句式与例句

`methods.json` 的 `patterns` 今天是 `{label, frame}`，加两个字段：

```json
{
  "label": "Admit then limit",
  "frame": "Although ___, ___.",
  "gloss": "尽管……，但……",
  "example": {
    "topic": "手写 vs 打字",
    "text": "Although typing is faster, handwriting helps children remember what they write."
  }
}
```

渲染成她看得懂的样子：`Although(尽管) ___, ___.` 再跟一句例句。

🚨 **守住铁律① 的是 `example.topic`，不是一条规矩。** 例句的话题必须**不是
她这一篇的话题** —— 她能从中学到句子的形状，但粘不进自己的文章。
`TestExampleTopicDiffersFromWriting` 钉住这条；话题撞了就不给例句，只给句式。

同时把英文接进「她卡住并求助」那条路：`writing_stall_prompt.go` 的
`if lang == "zh"` 闸拿掉 —— 12 条英文句式今天在那条路上取不到。
**只在她主动求助时给**；没求助的模板留在批改里。

### 四期 · 批改

**同一份教学内容，两个读者，能给的东西不一样。** 这是读完那几份批改资料之后
最重要的一条：雅思那份的核心产出是 `提升后范文参考`（一篇 7.5 分的重写）
加 `范文及原文中英对比`，还有 `band score X.X`。

**学生端**：不给分数、不给范文。按 `CommentPoint.Layer`（1 立意 / 2 材料 /
3 结构 / 4 字句）给等级与颜色 —— `verdict`（pass / polish / revise）已经在，
扩成每层一个等级。整篇重写就是代写（铁律①）。

**教师端**：字母等级与每维等级**已经是**要的样子（`liteassign.Rubric`，
英文默认维度已经是雅思四项）。要补的是**理由**：这条意见依据哪一维、命中哪条
`symptom`、引的是她哪一句。做成批改卡上的一个弹层。

🚨 **范文（`提升后范文参考`）这一项本期不做，要做需要单独点头。** 今天教师端的
`GradingSystemTemplate` 第一句就写着「不要重写、不要润色、不要续写，不要给出
可以直接替换原文的句子」—— 那条规矩现在**连教师端也管**。收雅思那份的整篇
重写等于推翻它，那是一次单独的产品决定，不在这份 spec 里替他做。

雅思那份**除范文之外**的部分本期都收：逐段反馈、四维表格、
「先 task response 后语法」的轻重次序（正好对上我们的 Layer 1→4）、
以及「只给分不解释分」属于禁止行为这一条 —— 那和产品负责人要的「告诉教师
真实逻辑」是同一件事。

这一期还要补两件：

- 补上缺的那张表：英文记叙的 symptom（今天只有 `writingSymptomsNarrativeZH`）。
- 九维评价标准里**数得出来的那几维由服务端算**，不让模型每轮自己数：
  词汇多样性（type-token）、平均句长、复杂句占比、连接词数／句数。
  模型只判需要判断力的那几维。理由见 memory：服务端数得出来的事实别交给模型。

## 6.5 · 第三方资料怎么用

`docs/reference/writing-teaching/` 下的四份技能是**别人发布的作品**，
`_meta.json` 里带着 `ownerId` 和 `publishedAt`。其中 `英语作文批改/SKILL.md`
还埋着 12 处水印与授权指纹：

	<!-- license-fingerprint: AMBER-IWF-[ORDER-ID]-[BUYER-ID] -->
	<!-- watermark: 英语作文批改老师 | AMBER-IWF-TRACE-A01 -->
	<!-- audit-token: … AMBER-IWF-FLOW-[ORDER-ID] -->

规矩：**教法照学，文字自己写。** 那几份资料的教学内容照收（那是产品负责人
挑它们的理由），但不要把它们的句子整段粘进 `internal/prompts`。

这不是额外的工作 —— AGENTS.md「提示词怎么写」本来就要求陈述句、不打比喻、
术语只从注册表来，而那几份资料的行文（emoji 小标题、`📝 作文批改报告`、
`✨ 亮点赏析`）一条都不符合，照搬也得重写一遍。

## 7 · 不做

- 不做运行时模板引擎、不做远程提示词服务、不做第二套模型配置。
- 不做模型自取的 skill 检索（理由见 §3）。
- 本期不产出小学内容。
- 不给学生看分数。
- 不替她写、不替她改完她的文章。**这条门就是这么一句话，不要展开成别的。**

🚨 **2026-09-23 产品负责人更正：「so templates are totally ok」。**
原来这里写着「不收整句填空模板」—— 那是我把一句有分寸的话收紧成了禁令，
而且和产品的现状**相反**：句式今天就已经摆在学生面前了
（`GuideBox` →「查看例子」→ `VocabExamples.tsx`，标题写着
「常用的句式（横线上的内容要你自己填）：」）。

产品负责人原来的原话是分寸，不是禁令：「for english writing, maybe sometimes
students need templates. but I would suggest templates only work when we comment
on their writing. we propose a better sentence form etc. and **they themselves
implement this template to their writing**.」

所以规矩是：**句式、模板、更好的句子形式，都给。** 她自己把它用到自己的
文章里。我们不动她的文稿，也不替她把哪一段写完 —— 就这一条。

## 8 · 风险

| 风险 | 处理 |
|---|---|
| 一期搬家搬坏了教学内容 | parity 逐条比 sha256；正文用 `Source.Const` 登记，不重新排版 |
| 迁移号 0186 被别的会话抢走 | 尽早落这一号；goose 起不来时按 memory 那次的办法整体后移 |
| 体裁改判后议论文那条路变样 | 照 `reading_genre.go` 的纪律：argument 一个字不动，新增只在另外几种体裁上生效 |
| 例句被她整句抄走 | `example.topic` 与她的题目不同才给；撞了只给句式 |
| 第三方技能被整段抄进提示词 | 教法照学、文字自己写，见 §6.5；水印本身就是为查抄袭存在的 |
| 加了轴但到不了深处 | `TestEveryCombinationResolves` 枚举真会出现的组合，不靠抽样 |

## 8.5 · 一期做完之后留下的东西（2026-09-22）

一期已实现，九个提交加一次终审修复（`93431172..255e7f0e`）。parity 全程绿，
学生看不到任何变化 —— 那是它对的样子。下面几条是**二期开工前必须先读的**。

### 🚨 二期一定会踩的两个坑

1. **加第一行带 `Grades` 的登记时，`TestEveryCombinationResolves` 会骗你。**
   它今天对 7 个学段值断言的是**同一份**期望内容。二期写下
   `{write, zh, Grades:["junior2"]}`（26 分）时，它会输给已经在的
   `{write, zh, Genres:["narrative"]}`（28 分）—— 年级专用的内容一次都不会
   出现，而测试照样绿，因为拿到的仍然是它期望的通用内容。
   **动 Grades 之前先把 want 表也按学段分开。** 这正是本 spec §8 写的
   「加了轴但到不了深处」，只是挪到了隔壁那条轴上。

2. **`Scope{Genres: nil}` 和 `readingRoutine.serves()` 读法相反。**
   `Matches` 把空 Genres 读成「不限」，`serves()` 把它读成「谁都不服务」。
   一期已在 `pickRoutineForGenre` 里两条都查来挡住这件事。
   要加一套通用读法（`Genres: nil`）之前，先决定这个字段到底是哪个意思。

### 🚨 唯一一处依赖打分次序的地方

打分是 **Surface(16) > Lang(8) > Genre(4) > Grade(2/1)** 的字典序，
不是「轴越多越具体」（`{Lang}`=8 就压过 `{Genres,Grade}`=6）。

生产里真正依赖它的只有一条：**`Lang(8) > Genre(4)`**。英文记叙文靠它留在
英文毛病表（24 分）上，而不是掉进那张无语言的记叙文兜底（20 分）。
**把这两条轴的次序对调，每一篇英文记叙文都会换成中文的毛病表。**

### 带着走的几条小账

- `vocab.Structures` 在 narrative/zh 和 narrative/en 上各只有**一条**方法
  （`struct_yiyang`、`struct_en_narrative_arc`）。测试钉得住漂移，但一条数据
  是很薄的钉子 —— 二期补英文记叙内容时顺手加宽。
- `internal/api` 里有**五个**已经没人读的别名，不是三个；其中两个仍然被
  `writingPlanSystemFor` 的兜底用着。**不要凭印象删。**
- `writingSymptomTable` 每次调用都会拼一次 24 条的记叙文合表，包括英文和
  中文议论文那两条根本用不上它的路径。`lookupWritingSymptom` 每条意见调一次。
- `vocab.All()` 返回的是包级切片本身，没有复制，调用方改得动整个方法库。
- `guidance.Default()` 把同一个可变 `*Registry` 交给每个调用方，`Add` 不加锁。

### 终审对这次重构本身的评价（原样记下来）

> 匹配规则今天并不划算，划算的是那条测试。

四个调用点的生产代码**都变长了**，而本 spec §1 说的「各有各的兜底」其实
**没有解决** —— 四处仍然各留各的兜底。真正变好的是
`TestEveryCombinationResolves`：全组合枚举 + 逐字内容断言，而且用 mutation
证明过它抓得住 en/narrative 那个缺口 —— 那正是当年放 `@@KINDS@@` 上线的
同一种缺口。**那条测试离不开一张能枚举的注册表。**

终审的建议：二期的学段轴要是没做成，就把读法表和阅读带读说明这两处迁回去
（它们为了一条自己用不上的规则担了风险），只留写作立题这一处 —— 那条测试
住在那儿。

## 8.6 · 二期 a 做完之后留下的东西（2026-09-22）

年级那条轴已经通了：`classes.grade` → `ListClassGradesForUser` →
`gradeFromClasses` → `guidance.Key.Grade` → `Resolve`。八个提交加一次终审修复
（`98800b36..e67d5bb1`）。学生看不到任何变化 —— 那是它对的样子。

### 🚨 二期 b 开工前必须先做的一件事

**没有任何界面能改一个已有班级的年级。** `patchClass` 收 grade、有测试、
空串能清掉，但 `ClassDetailView` 只有改名和换邀请码，`apps/web/src/api/classes.ts`
里也没有对应的客户端方法。

这件事卡住的是二期 b 的全部价值：线上两个班都是**批量导入**建的
（`adminImport` 永久写空串），所以今天每一个真实班级的年级都是空的。
**二期 b 的内容做出来，一个学生也收不到。** 把「改一个已有班级的年级」
列成二期 b 的第一个任务。

### 🚨 二期 b 会踩的三个坑

1. **`TestEveryCombinationResolves` 会骗你**（§8.5 那条仍然逐字有效）：它按
   `lang/genre` 建 want 表，然后拿七个年级去套同一份期望。第一行带 `Grades`
   的登记（26 分）会输给已经在的文体行（28 分），内容一次不出现而测试全绿。
   **动 Grades 之前先把 want 表也按年级分开。**
2. **`TestGradeDoesNotChangeAnythingYet` 必须删掉。** 它是二期 a 的验收
   （「填不填年级，结果一模一样」），二期 b 一旦登记年级内容它就必然红。
   这是计划内的报废，不是回归。
3. **`writingPlanSystemFor` 的 Resolve 兜底退到中文议论文**。一期终审补了
   四行无语言的兜底行之后这条路已经取不到 error（取不到就是登记漏了），
   但二期 b 会把组合数乘开。真要走到那儿，症状是一个写英文的学生拿到语文
   高考的材料 —— 先看一眼再动。

### 🚨 门要开得比一个 app 宽

二期 a 的终审抓到两条**红着的门**，而七个任务的评审一条都看不见：

- `apps/lite-web` 的 typecheck 红了。`ClassSummary` 多了两个必填字段，而
  lite 的 `@/*` 解析到 `apps/web/src` —— **pro 的类型改动会打断 lite**。
  计划里的门只写了 `cd apps/web && npm run typecheck`。
- `apps/web` 有两条测试红着。整个二期 a **没有一个任务跑过 vitest**。

以后碰 `apps/web` 或 `apps/lite-web` 的计划，门至少是这四条：

	cd apps/lite-web && npm run typecheck
	cd apps/web && npm run typecheck
	cd apps/web && npm run build     # vite build 不做类型检查
	cd apps/web && npx vitest run

### 带着走的几条小账

- 给 sqlc 的 params 结构体加字段**不是编译期强制的**：Go 的具名字段字面量会
  把缺的字段悄悄补成零值。`CreateClassParams{}` 漏了 `Grade` 照样编译，写进去
  的是空年级。加字段之后要 **grep 字面量**，别信绿色的 build。
- 一个学生可能在不止一个班里。两个班的年级对不上时 `gradeFromClasses` 返回
  「不知道」，**绝不挑一个**。终审确认这是对的产品行为而不是过度谨慎：猜错
  是整篇按错误年级教而屏幕上毫无异常，不猜只是少一条线索、落回今天的内容。
  🚨 但这条分支**一声不响** —— 二期 b 让年级真正起作用之后，值得在这里加一行
  `slog.Info`，否则一个在两个班里的学生永远收不到年级内容而没人知道为什么。
- `gradeTestTeacher` / `gradeTestSignIn` 是 package api 里第二份
  createTeacher/signInAs。再有第三份就该提到 `testdb_internal_test.go` 里去。
- `guidance` 的测试名里还留着 `Stage`（`TestStageBandFoldsGradeToBand` 等），
  下次碰那两个文件时顺手改掉。

## 8.7 · 二期 b 做完之后留下的东西（2026-09-23）

15 个提交 `a1731fa2..5f6fbbb0`，七个任务全部通过评审。交付了三件：
老师能改一个已有班级的年级了（pro 控制台 + lite 教师端**两块**界面）；
写英文议论文的学生在立题那一步先拿到 TOPIC + TASK 的拆解；
以及一条本来不在计划里、但比计划内那些都重要的修复（见下）。

### 🚨 最重要的一条：routebench 一直在拿一份没装配过的提示词挑模型

`benchcases_lite_writing.go` 的 `writingPlanCase()` 把**原始模板常量**当系统
提示词发了出去 —— `@@KINDS@@` / `@@MATERIAL@@` / `@@SKELETON@@` / `%d` 一个
都没替换。生产走的是 `writingPlanSystemFor(...)`。

`cmd/routebench` 拿这批用例去花钱跑候选模型、决定哪个模型绑到哪一档，而这条
用例按它自己的注释是「compose 档在 lite 里最贵的一个调用点（占 compose
46%）」。**给最贵那个调用点挑模型的那次实测，量的是一份没有节点类型表、
没有材料那一节、没有骨架的提示词。**

已修，并留下一道闸：`cmd/routebench/benchcases_assembled_test.go` 的
`TestBenchCasesSendAssembledPrompts` —— 三个来源包、所有 role、四种占位符
（`@@` / `%s` / `%d` / `%LENS%`）全查，且三个包各自不许为空。这道闸当面红过
一次（故意塞一个 `%s`，它点名了那条用例）。

🚨 **连带更正 spec 的一个前提**：`bench/compose/lite-writing-plan` 这条 parity
样例**从来没有守过装配** —— 它守的是原始模板的字节。有意重录了这一条，
现在它守的是装配后的那一份。

### 🚨 parity 基线过去只盖了 28/36 条

`examples()` 吐 36 条，基线里只有 28 条，而 `TestMainRequestParity` **只遍历
基线那张表** —— 另外 8 条（含**四条 `writing/plan/*` 装配**）一次都没被比过。
AGENTS.md §6 记的那次事故就是这个形状。八条已全部补进基线（纯新增，原 28 条
哈希一个没动），并加了一条判据：`examples()` 里每一条都必须在基线里有位置。

### 🚨 过滤器是七次评审都看不见的那个洞

计划里的后端门是 `go test ./internal/api -run 'Writing|Grade|Class'`。
`TestClarityMergeWriting` 名字里有 Writing，被跑到了；
**`TestClarityPersonalPlanReady` 三个词一个都不含，没被跑到** ——
它的 en 快照因为这次改动变了却没被重录，**不带过滤器跑整包就是红的**。
终审跑无过滤版本才抓到。

教训两条，下次照办：
1. **期末那一次门不许带 `-run` 过滤。** 每个任务跑过滤版本省时间可以，
   收尾必须 `go test ./...` 跑全。
2. **`go test … | tail` 的退出码是 tail 的。** memory 里记过一次，这次终审
   自己又踩了一次并当场发现。判断绿红要看真实退出码。

### 带着走的几条小账

- `SlotMaterial` / `SlotSkeleton` 登记在 `{write, en}` 上**不分文体**，所以
  写**英文记叙文**的学生会收到一节标题写着「英文议论文的材料与分析」的内容，
  以及 Thesis–body–conclusion 那套骨架，而三屏之后的块名表又告诉她
  「这是记叙文，不使用议论文的 thesis、point 等节点类型」—— 同一份文档里
  两条指令打架。**先于二期 b 就存在**，`TestWritingPlanSystemFor_EveryGenreAndLangAssembles`
  的「文体不许串台」只查 `「thesis」`/`「scene」` 这种方括号块名，看不见散文里的串台。
  **留给三期。**
- `## 教学术语` 后面跟着一段本该在 `## 输出格式` 底下的输出格式碎片。
  **中文议论文那一份也有同样的形状**（`## 区分观点与材料` 夹在 `- kind：`
  和 `- text：` 之间），所以这是「把 `##` 小标题放进 `@@KINDS@@` 常量」的
  长期后果，不是二期 b 弄出来的。要改就是 `WritingPlanSystem` 的分节重构，
  按 AGENTS.md §7 属于「单独一次提交、单独跑一次 LIVE_LLM」。**没有排期。**
- 年级那条轴到二期 b 为止仍然**是通的但不起作用**：生产注册表里一行
  `Grades` 都没有。`TestGradeDoesNotChangeAnythingYet` 仍然成立。
  第一个登记年级内容的人要同时做三件事：拆 `coverage_test.go` 的 want 表
  （两轴→三轴）、删掉那条验收测试、检查 `benchcases_lite_writing.go` 传的
  grade（今天是 `""`）。
- **合并前还欠两条验证**（二期 b 这个环境里做不了，不是取消）：
  真的打开 pro 和 lite 那两屏看一眼（lite 那块没有任何测试挡着，
  控件的**接线**终审核过了，**布局**没有）；以及跑一次 `LIVE_LLM`，
  判据是拿一道 discuss-both-views 的英文题走一轮立题，看印记有没有先把题目
  拆成两项、有没有把整句开头塞给她。

## 8.8 · 三期做完之后留下的东西（2026-09-23）

7 个提交 `cfb9b4d0..e404b24e`。28 条句式各自补上中文读法和一句例句，
渲染在她点「查看例子」之后那一屏；写英文的学生卡住求助时也拿得到句式了。

### 🚨 spec 那条只说对了一半

「把 `if lang == "zh"` 闸拿掉」—— 实际上有**两道闸**。
`writingHelpFrames` 自己写死了 `vocab.ForLang("zh", genre)`，只拿掉外面那道，
英文学生拿到的是「这一轮给她一句句式」加下面一片空白。

### 🚨 评审三轮抓到的，按严重程度

1. **讲「用真证据」的那个方法，例句自己编了一份研究**（`A 2019 study of
   thirty-four mountain glaciers…`）。这个仓库为幻引造过两次检测器。
   换成了 EHT 2019 对 M87 的实测（6.5±0.7×10⁹ 太阳质量，由阴影角大小推出），
   **上线前用网络核过**。
2. **同一个方法连着两条归纳不成立。** 先是「燕子低飞/蚂蚁搬家/雁群南下 →
   对气压变化的敏感」（雁群走的是温度和日照）；改完之后它的**姐妹条**又写了
   「高原上长大的人……皆来自血液里偏高的红细胞数量」—— 对藏族是反的
   （EPAS1/EGLN1 压低促红反应，高红细胞是安第斯型，在藏族身上是病）。
   教「检查归纳在每个例子上都成立」的方法，自己的例子两次不成立。
3. **四条例句落在它们本该避开的作文题上。** 最重的一条是
   「这些在洪水里守堤的人、在雪夜里通车的人、在地震后第一时间进山的人，
   真正表现了……担当」—— 写「担当」的学生等于被递了结尾段。
4. **刚打开的那条路给英文议论文学生发记叙文句式。** `ForLang` 不看位置，
   而所有 `en_*` 的 `genre` 是空串 ⇒ genre 轴在英文侧根本不起作用。
   改用 `vocab.For(appliesTo, lang, genre)`。

### 🚨 摘两道闸之前先数还剩几道

第 3 条是我一条裁定的直接后果：例句的话题标签（产品负责人说不用）和计划里
那条运行时检查（我判过度设计）**各自摘掉都有道理，一起摘掉**之后，
「写的时候挑远话题」成了唯一屏障 —— 28 条里滑了 4 条。

修法是改内容不是加机制（加机制正是产品负责人当天纠正过两次的毛病），
但**把「远离哪些题材」的名单写进了 `vocab.go`** —— 这是那条裁定的另一半，
原来只活在一个没人会打开的报告里。

### 带着走的几条（我自己量过的数字）

	en/argument  opening 15 / body 20 / closing 16
	en/narrative opening 15 / body 20 / closing 16   ← 和议论文一模一样
	zh/argument  opening  0 / body  5 / closing  0
	zh/narrative opening  0 / body  0 / closing  0   ← 一条都没有

- **英文侧 genre 轴是死的**（所有 `en_*` 的 `genre` 都是空串）。位置过滤之后
  真正错配的只剩 `en_story_turn` 那 2 条落在议论文正文段上。根治要给
  `en_story_*` 打 `genre:"narrative"`，那会同时改变段落引导和深化两个面
  （改了其实也对），属于**数据决定**，没排期。
- **20 条句式对着一句「这一轮给她一句句式」。** 其中 17 条是
  `applies_to:"any"` 的语言风格类，不算错配，但密度偏高。
  **合并前那次 `LIVE_LLM` 要专门看这一条**：模型递一条，还是把表甩给她挑。
- **中文记叙文一条句式都没有。** 那条路上模型被告知「给她一句句式」却拿不到
  任何一条（`if frames != ""` 只挡住列表，指令本身是无条件的）。先于本期存在。

## 8.9 · 四期做完之后留下的东西（2026-09-23）

4 个提交 `bd601064..5d6b394b`。

- **学生端**：一条笼统的结论拆成四层各自的等级（立意/材料/结构/字句），
  服务端从这条评语自己的 points 算，不问模型。**不给分数**，只有等级和颜色。
- **英文记叙的毛病表**补上了（13 → 18）：show-don't-tell、时态跳变、
  filtering、对话标点、and-then 连接单一 —— **不是中文那 5 条的翻译**。
- **教师端的依据弹层**（产品负责人点名要的）：`litegrade.Point` 加
  `Dimension` + `Symptom`，批改卡上一个「依据」按钮开出
  维度 / 对应毛病 / 学生原句。
- **可数的那几维交给服务端**：新建 `internal/textstat`，含**这个仓库第一个
  共享的句子切分**，加 type-token、平均句长、复杂句占比、连接词密度。

### 🚨 一条差点上线的判据错误

Task 1 的第一版按「那条 issue 带没带 Action」判等级。但
`validateCommentPoints`（`writing_comment.go:304`）会把没有 Action 的 point
**整条丢掉**，所以活下来的 issue 全都带 Action ⇒ 三档塌成两档，
polish 那一支是走不到的死代码，而一处小的用词问题就把「字句」判成
revise（危险色），正好撞上 `CommentPanel.tsx:54-55` 那条
「polish 不能长得像错误」。

改成按**条数**：0 条 pass / 1 条 polish / ≥2 条 revise ——
三档都够得着，而且条数是服务端数得出来的事实。
`TestLayerVerdictUsesAllThreeLevels` 钉住中间那一档。

### 🚨 终审抓到的两条阻塞，**其中一条是控制者自己写的**

1. **四个格子里有三个在骗她。** `validateCommentPoints` 只保留最上面那一层，
   其余层的 issue 由 `dropLowerLayer` 丢掉；而 `layerVerdictsOf` 跑在**已经
   过滤完**的 points 上，所以「被查出问题然后被刻意压后」和「本来就干净」
   长得一模一样 —— 两者都画成「已通过」。
   她的段落立意弱外加三处长句，模型两样都报了，服务端留下立意、丢掉字句，
   屏幕上写着**字句·已通过**；她改完立意再点一次，字句忽然变成「可优化」，
   而没有任何东西解释这一下。
   修法：加第四个状态 `unchecked`／「本轮未看」（**只给分层那张图用**，
   `Comment.Verdict` 的闭表仍然是三个值）。一条 issue 都没有时四层才全 pass。
2. **老师每存一次，对应毛病就被抹掉一次。** `SanitizeProvenance` 不幂等：
   生成时把 symptom **id** 解析成中文名存下来，老师保存时客户端把名字发回来，
   `lookupWritingSymptom` 只认 id ⇒ 清空。
   修法：**存 id，在 DTO 边界解析成名字**（仓库自己的先例就是这样 ——
   `CommentPoint.Symptom` 存的是 id），并让 lookup 同时认 id 和名字。
   存名字还毁掉 join key：「这个班里这条毛病出现过多少次」从此答不出来。

🚨 **两条都是绿着的门抓不到的，因为两边的夹具描述的都是系统造不出来的数据**：
分层那两条测试手搓了「两个层同时非 pass」的 Comment（服务端只留一层）；
老师那条测试在 PATCH 里发的是原始 id，而真实客户端发的是名字。
这正是 memory `judge-the-delivered-artifact-2026-09-21` 那条 —— 判据站在
她真正收到的那一份的上游。

3. **事实块插在系统提示词中间，把成本契约打碎了。** 批改的系统提示词在这一期
   之前对同一次作业的每个学生都完全稳定；按学生变的事实块插进中间之后，
   它后面的一切都失去跨学生的前缀缓存。已挪进 user 消息，系统提示词恢复
   100% 稳定（zh 7,972 字节 / en 7,662 字节）。

4. **两个度量在真实学生作文上是反的**，而提示词告诉模型它们「不是估计」：

	"I like that book. After school I play. Before dinner I read."  复杂句占比 1.0 → 0.00
	"我起床了。然后我吃饭。然后我上学。然后我回家。"                连接词密度 0.0 → 0.75

   （`that` 当限定词、`after`/`before` 当介词都误命中；中文连接词表里没有
   `然后` —— 而那正是流水账最常见的样子，`connector_monotony` 就在毛病表里。）
   同时把那句「不是估计」去掉了：词表近似就说成近似。

### 带着走的几条

- **`internal/textstat` 是第一个共享的句子切分**，而仓库里还有**六处**各写
  各的句末标点判断（`writing_plan.go:250`、`reading_coach_board_build.go:165`、
  `reading_coach.go:2095`、`reading_coach_card.go:319,795`、
  `teacher/weekly.go:119`、`atom_report.go:583`）。新包的注释里列了这六处，
  **本期一处都没动** —— 收编它们是另一件事。
- 连接词和复杂句标记那两张表是**自己编的**（仓库里没有现成的词表），
  注释里写明不完备，值得找人调。
- **范文（提升后范文参考）没做**，spec §6 四期写着要做需要单独点头。
  教师端批改提示词第一句「不要重写、不要润色、不要续写」本期一字未动。
- `litegrade.Point.Symptom` 存的是**解析后的中文名**不是 id ——
  模型交 id（好匹配闭表），服务端换成人看得懂的名字再存，
  所以老师屏幕上不会出现 `tense_drift`。代价：毛病改名之后老批改仍是旧名。

## 9 · 参考

- `docs/reference/writing-teaching/reading-suggestion.md` —— 已实现于 `reading_genre.go`
- `docs/reference/writing-teaching/english-writing/思维印记-英文写作逻辑框架搭建.md`
- `docs/reference/writing-teaching/english-comment/`、`chinese-comment/`
- `docs/reference/writing-teaching/chinese-writing/小学作文/` —— 只作参考
- AGENTS.md「提示词怎么写」七条
