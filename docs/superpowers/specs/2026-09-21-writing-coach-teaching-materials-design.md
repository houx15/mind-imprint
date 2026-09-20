# 写作陪练 R4：把语文老师的那套教法装进去

> 输入：`docs/reference/writing-teaching/` 六份讲义（产品负责人从一位语文老师处拿到）。
> 产品负责人 2026-09-21：「the split, suggestion, and feedback of our current
> writing coach is also not good enough……maybe this is not so good for english
> writing I guess(that may need some insights from toefl or ielts writing)。
> but is very insightful for our **chinese** writing coach.」
>
> 前三轮：R1 `b2dbcdce`（卡片种类闭集）、R2 `539de37e`（整篇上下文 + 反馈分级）、
> R3 `947ecc13`（行文这一步）。本轮接着它们，不重做它们。

---

## 0 · 先把上三轮的账结清

R4 之前先对着同事的十条和 `general-suggestions.md` 逐条走了一遍。**八条已闭，
两条没闭**，这两条并进本轮：

| 缺口 | 应在哪 | 现状 |
|---|---|---|
| 主体段少一句 | `paragraphShape.ts` 的 `point` | 现在是四步（分论点句／论据／分析／回扣）。讲义（五）的五句型里有 **阐释句**，我们没有。R2 线上实测里模型自己说的那句「例子讲完就结束了，还差一句把它和主张连起来的话」，缺的就是它和分析句。 |
| 连着两轮卡住要换帮法 | `general-suggestions.md` 交互策略 + 验收第 5 条 | `writingPlanStalled` 只接在**立题**那条路（`writing_plan.go:338`）。段落陪练 `writing_guide.go` 和请印记看一看 `writing_comment.go` 两条路上**一个都没有**。 |

另外 R3 的线上 e2e（`apps/lite-web/e2e/flow-stage.spec.ts`）当时是未跟踪文件，
没进 main。本轮先补上。

---

## 1 · 这一轮改什么

产品负责人点了三处：**split（怎么拆）、suggestion（给什么建议）、feedback（怎么评）**。
讲义正好一一对上：

| 她说的 | 讲义里的东西 | 落在哪 |
|---|---|---|
| split | 主体段**五句型**；记叙文的四步 | `paragraphShape.ts` / `writing_sentence.go` |
| suggestion | **分析句三法**（因果／假设／归纳）带句式；分论点**四角度三原则**；结尾**四技法**；细节描写 | `methods.json` + 立题 prompt |
| feedback | 用上面这套词去判，而不是用「还差一句话」 | `writingCommentBlockJob` + 停滞升级 |

**只做中文。** 英文那套（`en_*`）原样不动 —— 产品负责人说英文要另外找
托福雅思的路子，本轮不猜。判据是 `wr.Lang == "zh"`。

---

## 2 · 主体段五句型（split）

讲义（五）第一节，逐句标了功能：

| 句 | 功能 | 讲义原话（摘） |
|---|---|---|
| ①观点句 | 段首提出 | 是对中心论点的分解，要准确鲜明。**一定要扣中心论点，扣住关键词** |
| ②阐释句 | 紧跟观点句 | 用道理／比喻／对比**把观点句里的抽象词说开**，为选材找准角度 |
| ③材料句 | 紧接阐释句 | 事例要**简洁**、多样化，叙述讲究简明扼要 |
| ④分析句 | 紧扣主题 | 对事实进行剖析，**挖掘意义**。可用归纳／假设／因果 |
| ⑤结论句 | 呼应分论点 | 紧扣关键词，联系实际，回应段首观点句 |

`paragraphShape.ts` 的 `point` 从四步改成这五步。**新增的是阐释句** ——
它正是学生最容易跳过的那一句（观点句写完直接跳到例子，于是例子和主张之间
没有桥）。

同一份表 Go 侧也要有一份（`writing_sentence.go`），因为 `writingCommentBlockJob`
判一段的时候要按这五句去查「缺了哪一句」，而不是笼统说「还差一句」。
两份之间由一条测试钉住（同 `TestEmbeddedCopyMatchesSourceOfTruth` 的路子）。

**🚨 这一节是确定性渲染，不花模型调用**（沿用 R3 定下的那条）。

---

## 3 · 分析句三法（suggestion）

讲义（五）第三节。这是这批材料里最值钱的一条 —— 它给「分析」这个动作
起了**三个名字**和**三句可套的句式**，而我们现在只会说「这件事凭什么证明上面那句话」。

进 `methods.json`，`category: "analysis"`，`applies_to: "body"`：

| id | 名字 | 什么时候用 | 句式（`patterns`） |
|---|---|---|---|
| `analysis_cause` | 因果分析法 | 材料**只摆了现象**，还没说出为什么 | 是什么……？是……。因为……，所以…… |
| `analysis_suppose` | 假设分析法 | 材料**已经有结论**了，要加力度 | 假如……，那么……？／若不是……，…… |
| `analysis_induce` | 归纳分析法 | **列了好几个例子**，要找共性 | 这些……，真正表现了…… |

讲义把三者的分工写得很干净，照抄：
> 1. 只摆现象材料 → 追原因 —— 因果分析法
> 2. 已有结论材料 → 问假如 —— 假设分析法
> 3. 列举事实材料 → 找共性 —— 归纳分析法

**`category: "analysis"` 而不是 `"method"`**，所以它们**不会**漏进行文那一步
每一块的「论证方法」下拉（`writingValidMethodID` 只收 `category=="method"`）。
论证方法回答「这一段怎么证」，分析法回答「例子摆完之后那一句怎么写」，
是两层东西。但它们照常进 `For("body", "zh")`，陪练可以点名递给她。

### 结尾四技法

讲义（六）。同样进 `methods.json`，`applies_to: "closing"`，`category: "closing"`：
首尾呼应法／名言警句法／总结归纳法／修辞收束法。讲义的两条硬话一并带上：
**结尾不超过 200 字**、**忌空喊口号**。

---

## 4 · 分论点四角度与三原则（suggestion）

讲义（四）。立题那条路的 prompt 现在只会催她「再来一条理由」，不会告诉她
**从哪个方向去想第二条**。

**四角度**（并列式议论文，整篇选定一个角度）：
是什么（概念分类）／为什么（因果分析）／怎么样、怎么办（途径）／会怎样（结果作用）。

**三原则**：
- **扣得住** —— 分论点里要嵌进中心论点的关键词，保证每一段都扣题
- **分得开** —— 分论点之间不交叉、不重复
- **排得顺** —— 逻辑顺序合理（由小到大／由浅入深／从古至今）

**🚨「扣得住」服务端数得出来，就不要让模型每轮自己数**
（[[hardcoded-thresholds-vs-user-set-scale-2026-09-12]] 那条的正面用法）。
新 `writing_points_check.go`：

```go
// pointsOffThesis 交出「哪几条分论点一个中心论点的词都没沾上」。
func pointsOffThesis(thesis string, points []sqlc.WritingOutline) []string
```

取中心论点里的实词（去停用词、去标点、长度 ≥2 的连续汉字串），逐条分论点看
有没有交集。**结果当事实喂进 prompt，不当规矩写进 prompt。** 空集就一个字都不加
（沿用 [[pbl-refeed-one-produce-slot-2026-09-05]]：这类提示只在真的成立时出现，
不做常驻）。

「分得开」已经有 `writing_repeats.go` 在数了，不重复造。
「排得顺」只有模型判得了，写进 prompt。

---

## 5 · 连着两轮卡住就换帮法（feedback，补 §0 的欠账）

`general-suggestions.md`：
> 缺信息时问一个具体问题；**连续两轮无新增信息时，改用选项、句式或简短示范**。

验收第 5 条：「连续两轮卡住后改变帮助方式」。

新 `writing_stall.go`，三档：

```go
type helpMode int
const (
	helpAsk   helpMode = iota // 问一个具体问题（默认）
	helpOffer                 // 给两个选项让她挑
	helpShow                  // 给一句句式 / 半句示范
)

// writingHelpMode 从**存下来的行**里数，不问模型。
//   - 这一块上连着两条意见指的是同一个症状，且她的正文没变 → helpOffer
//   - 连着三条 → helpShow
func writingHelpMode(prior []sqlc.WritingComment, snippetID, currentText string) helpMode
```

判据用**存下来的 `symptom` 字段逐字比**，不用模型的自述。
「她的正文没变」沿用 R2 的 `writingSheActedOn`（`SourceText` 逐字比对）。

🚨 [[observation-tool-is-the-bug-2026-09-12]] 那条：**判「有没有变」必须逐字**，
不许拿长度当代理。

接两处：`writing_comment.go`（请印记看一看）和 `writing_guide.go`（段落陪练）。

`helpShow` 那一档给的是**句式**（`Pattern.Frame`，如「因为……，所以……」），
不是替她写的正文 —— 铁律①。§3 那三条句式正好是这一档手里的东西。

---

## 6 · 记叙文（split + suggestion）

### 6.1 文体是**推断**出来的，不问她

`writing_setup.go:15` 写着这是产品的明确要求：
> **没有文体单选** —— 学生未必知道「文体」……由模型去判断这是议论还是记叙。

所以新增 `writingGenreOf(wr, outline) string` → `genreArgument` | `genreNarrative`，
**默认 `argument`**。理由是错的代价不对称：这间屋子是按议论文搭的，把一篇
议论文当记叙文处理，她会拿到一整套用不上的引导；反过来只是少拿到几条。

判据（先确定性、后兜底）：
1. 板上已经有 `thesis` 或 `point` → `argument`（她已经在按议论文摆了）
2. 板上有记叙文的 kind → `narrative`
3. 都没有 → 看 `writing.idea` 里有没有记叙文的题目特征词，没有就 `argument`

### 6.2 四个记叙文 kind

`writing_kind.go` 的闭集加四个（R1 定下的那条：种类是闭集，深度由服务端派生）：

| kind | 标签 | 深度 |
|---|---|---|
| `scene` | 场景 | 1 |
| `detail` | 细节 | 2（挂在 scene 下） |
| `turn` | 转折 | 1 |
| `feeling` | 感悟 | 1 |

0182 的回填正则不管它们 —— 那是给老行写的，新行才会是这四种。

### 6.3 抑扬转情法

讲义给了一张四步表，直接就是一个 `struct_*`（`applies_to: "whole"`）：

| 步 | 位置 | 写法 |
|---|---|---|
| 抑 | 开头 | 1-2 件具体小事，写对这个人的不满（对比写法更出彩） |
| 渡 | 中间 | 平淡叙事，用「直到有一天」引出关键事件 |
| 转 | 核心 | 环境烘托＋细节描写＋心理变化 |
| 扬 | 结尾 | 升华主题，呼应开头和标题 |

### 6.4 细节描写

讲义的三类（人物／环境／物件）进 `methods.json`，`category: "detail"`。
讲义的三禁忌（真实性／典型性／独特性）和「**精准动词**」进记叙文的
段落检查表。

---

## 7 · 症状表按文体分岔（feedback）

`writing_symptoms.go` 现在是一张表，按议论文写的。记叙文另起一段：

| 症状 | 判什么 |
|---|---|
| `detail_vague` | 只有「很感动」「特别好」，没有动作、神态、语言 |
| `no_turn` | 平铺直叙，从头到尾一个调子 |
| `turn_abrupt` | 从抑直接跳到扬，中间没有过渡和触发点 |
| `verb_generic` | 动词笼统（「走过去」「拿着」），讲义要的是精准动词 |

---

## 8 · 验收

1. `paragraphShapeOf("point")` 交出五步，第二步是阐释句；Go 和 TS 两份一致（测试钉住）。
2. 三条分析法读得到、带句式；**不出现**在行文那一步每一块的方法下拉里。
3. 中心论点「读书要读慢」＋分论点「人应该多运动」→ `pointsOffThesis` 指出后者；
   分论点「慢读才能发现问题」→ 不指出。
4. 同一块上连着两条同症状的意见且正文没变 → `writingHelpMode` 返回 `helpOffer`；
   三条 → `helpShow`。正文改过 → 回 `helpAsk`。
5. 板上有 `thesis` → `writingGenreOf` 返回 `argument`，哪怕题目像记叙文。
6. `wr.Lang == "en"` 时上面这些一条都不加进 prompt。
7. `LIVE_LLM=1`：一段「观点句＋例子」的正文，陪练点得出缺的是**分析句**，
   并递一条三法之一的句式。

---

## 9 · 不做

- **英文那一套**。产品负责人说要托福雅思的路子，本轮不猜。
- **新卡片交互**。[[card-form-interaction-rejected-2026-09-01]] 仍然有效。
- **改 R1/R2/R3 已定的东西**。五句型是把 `point` 那一格改写，不动 kind 闭集
  的议论文那六个，不动分级三档，不动行文那一步的四个结构。

---

## 10 · 建完之后订正（2026-09-21）

建的过程里有四处和上面写的不一样，以**实际建成的**为准。

### 10.1 「扣得住」加了一个**时机**门槛（§4）

spec 里写的是「数出来当事实喂进去」，没说什么时候数。LIVE_LLM 实测撞上一下：
她刚说出第一条理由「青少年生物钟本来就晚」（中心论点是「上学时间该往后推
一小时」），判据响了，于是那一轮陪练不去帮她想，改成请她重新措辞 ——
正是产品负责人说的「吹毛求疵」。

那句话按讲义的标准确实没扣住，判据本身没算错，**错的是时机**。讲义里
「扣得住」是分论点都摆出来之后回头检查的一条，不是写第一条时的门槛。
改成有 ≥2 条分论点之后才查（`writingPointsCheckFrom`）。

### 10.2 「扣得住」数的是**字**，不是词（§4）

第一版切「连续两个以上的实字串」再比子串，在
「读书要读慢」↔「慢读才能发现问题」上判错：两句共用「读」和「慢」，
但一句里是「读慢」另一句里是「慢读」—— **中文的语序会翻**。
改成数共用的实字个数（≥2，顺序不参与）。

### 10.3 五句型要明说「不是一张验收清单」（§2）

加完五句型之后，一段有时间地点人物的亲身经历被判成 `revise`，理由是
「没有观点句、没有分析句」—— 检查表被当成了逐项验收。补了两句：
少一两句 ⇒ polish；观点句可能写在这一块的**卡片**上，卡片上有就不算她没有。

### 10.4 立题的 kind 清单也要按文体分（§6.2）

spec 只说了「闭表加四种」。但立题那份提示词写着「只能是下面这十个之一」——
也就是模型**永远开不出**那四种，记叙文那半边是死代码。拆成两份清单，
装配时按文体选（`writingPlanSystemFor`）。

### 10.5 顺带修的一个线上真 bug（不在 §1-9 里）

LIVE_LLM 抓到：模型回了**不止一个** JSON 对象（先写一份，用大白话跟自己
商量，再写一份改好的）。共用的提取函数把「第一个 `{` 到最后一个 `}`」整段
夹出来，得到的不是合法 JSON，于是整轮静默作废 —— 她那边是一个转不动的终端。

和 [[model-json-half-arrived-2026-09-08]] 同一类，而**提示词越长模型越爱这样
自言自语**，R4 把检查表加长了正好撞上。改成：老路子失败之后按括号配平切出
每一段 `{...}`，从后往前试。真坏掉的还是失败。

### 10.6 §8 验收的实测结果

| # | 结果 |
|---|---|
| 1 | ✅ `TestBodyParagraphIsTheFiveSentenceShape` + `TestParagraphShapeMatchesFrontend`（改一个字验过它会响） |
| 2 | ✅ `TestVocabNeverCrossesGenres` + R3 那条 e2e |
| 3 | ✅ `TestWritingPointsOffThesis`（含 10.1 的时机用例） |
| 4 | ✅ `TestWritingHelpModeLadder` / `TestGuideHelpModeLadder` |
| 5 | ✅ `TestWritingGenreOf` |
| 6 | ✅ 英文那几条都在各自的测试里 |
| 7 | ✅ LIVE_LLM：「例子摆完就停了，这一段缺的是**分析句**。……用**假设分析法**：假如他不是九岁起几十年不间断地练，而是练几年就停，他的字还会不会被奉为瑰宝？」 |
| — | LIVE_LLM 记叙文立题：题目「记一次难忘的经历」，开出的是 `kind=scene`，问的是「雨是什么样子？你等了多久」，不是「你要证明什么」 |
