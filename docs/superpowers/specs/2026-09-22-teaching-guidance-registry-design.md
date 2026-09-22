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
	Stage   string // "" 不限 | junior1..3 | senior1..3，见 §4
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
// 命中顺序：完全匹配 → 去掉 Stage → 去掉 Genre。三步都取不到就报错。
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

`classes` 加一列 `stage text not null default ''`，建班时设。迁移号取 **0186**
（现最高 0185）—— 分支开久了迁移号会撞，撞了 goose 直接起不来，所以这一号要尽早落。

取值到**年级**这一层，不是 junior / senior 两档：

	'' | junior1 | junior2 | junior3 | senior1 | senior2 | senior3

🚨 这一条 2026-09-22 读完资料之后改过。原来定的是 junior / senior 两档，
而 `初中语文作文批改` 的标准是**按年级**给的 —— 初一 500–600 字「叙事完整、
语句通顺」，初二 550–650 字「描写生动、结构完整」，初三 600–700 字「立意深刻、
语言优美」。两档表达不了这三行，而一个班本来就是一个年级。

`Resolve` 的回退链因此多一级：年级 → 学段（junior / senior）→ 不限。
`guidance.StageBand("junior2") == "junior"`，让「整个初中通用」的内容只写一份。

学段经 `enrollments` 流到学生，再流到她的 writing / reading。取不到就是 `''`，
`Resolve` 退到不限学段那一行。**本期不产出任何小学内容。**

## 5 · 判据：一条现在写不出来的测试

做这件事的回报就是这条测试：

```
TestEveryCombinationResolves —— 枚举所有真的会出现的
(Surface × Lang × Genre × Stage)：
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

- `classes.stage` 迁移 0186 + 建班表单那一格。
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
- 不收整句填空模板。

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

1. **加第一行带 `Stages` 的登记时，`TestEveryCombinationResolves` 会骗你。**
   它今天对 7 个学段值断言的是**同一份**期望内容。二期写下
   `{write, zh, Stages:["junior2"]}`（26 分）时，它会输给已经在的
   `{write, zh, Genres:["narrative"]}`（28 分）—— 年级专用的内容一次都不会
   出现，而测试照样绿，因为拿到的仍然是它期望的通用内容。
   **动 Stages 之前先把 want 表也按学段分开。** 这正是本 spec §8 写的
   「加了轴但到不了深处」，只是挪到了隔壁那条轴上。

2. **`Scope{Genres: nil}` 和 `readingRoutine.serves()` 读法相反。**
   `Matches` 把空 Genres 读成「不限」，`serves()` 把它读成「谁都不服务」。
   一期已在 `pickRoutineForGenre` 里两条都查来挡住这件事。
   要加一套通用读法（`Genres: nil`）之前，先决定这个字段到底是哪个意思。

### 🚨 唯一一处依赖打分次序的地方

打分是 **Surface(16) > Lang(8) > Genre(4) > Stage(2/1)** 的字典序，
不是「轴越多越具体」（`{Lang}`=8 就压过 `{Genres,Stage}`=6）。

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

## 9 · 参考

- `docs/reference/writing-teaching/reading-suggestion.md` —— 已实现于 `reading_genre.go`
- `docs/reference/writing-teaching/english-writing/思维印记-英文写作逻辑框架搭建.md`
- `docs/reference/writing-teaching/english-comment/`、`chinese-comment/`
- `docs/reference/writing-teaching/chinese-writing/小学作文/` —— 只作参考
- AGENTS.md「提示词怎么写」七条
