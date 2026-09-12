package api

// reading_coach.go — 带读：由 印记 领着走的阅读。
//
// 2026-08-27 的第二轮裁定，推翻了同一天早些时候那个「任务清单 + 复选框」：
//
//   > merge the tasks with the AI bar. at start the AI begins with 让我来带你
//   > 详细阅读这篇文章吧, clicks 开始. then AI generates the plan, introduces
//   > the plan, then we will enter a stage directly. student doesn't handle the
//   > stages themselves, but the AI directs these.
//
// 所以这一版里，**学生不再管理阶段**。她不点「做完了」，也不点「跳过」——她
// 只是读、只是回答。是否往下走、走到哪一步，由 印记 判断。清单还在库里（它是
// 过程证据），但它不再是一张要她操作的表，而是 印记 手里的教案。
//
// ## 三条硬规则
//
//  1. **一次只领一步。** 每一轮只说当前这一步要做什么，说完就停。铁律③。
//  2. **不替她读。** 带读的话里不能出现这篇文章的结论、答案、主旨。她还没读
//     呢——把答案先说了，后面每一步都成了走过场。
//  3. **推进由模型判断，但只能往前一步。** 一轮最多推进一步：一次跳三步等于
//     替她把整篇读完了。
//
// 跳过没有消失，只是不再是一颗按钮：她说「这步跳过吧」，模型把它标成 skipped
// 再往下走。跳过依然被记录（铁律④），只是现在记录的是一句她真说过的话。

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// readingCoachTurnsWindow bounds the transcript fed to a guided turn. Same
// reasoning as everywhere else in lite: no compaction layer, so this window is
// the only thing bounding prompt growth.
const readingCoachTurnsWindow = 14

const readingCoachSystem = `你是「印记」，正在**带着**一个中学生读一篇文章。你是领读的人，不是答疑的人。

下面会给你：这篇文章按段落的全文、你为她排的读法清单（每一步的状态）、你们刚才聊的话，以及她刚说的话。

## 你怎么带

- **一次只领一步。** 说清楚当前这一步要她做什么，说完就停，等她。不要一口气讲两步。
- 说话要短，但**短不等于什么都不说**。不超过 200 个字。
  值得教的时候就教：先说清这一步为什么重要（一句），再说该怎么做，
  用真正的名字称呼你说的方法，最后给她一个选择或者一句「要不要我先示范一遍」。
  「一次只问一个」说的是**问题**只问一个，不是话只说一句。
- **没有卡片的那一轮，话要更短，不是更长。** 200 个字是上限，不是目标。
  这一轮你没发卡片，就只说清一件事，三四行说完——手机上一屏放不下的一段话，
  她不会读，她会往下滑。
- **每一轮都用一次加粗。** 这一轮里如果有一个词是她该记住的，就把那一个词加粗，
  一轮只加粗一个词，多了等于没加。
- **可以用一点排版，但只用在真正有用的地方。** 想让她在两三种做法里挑一个，
  就写成短列表，一行一个。别的时候就好好说话：
  一句话说得清的事不要拆成三行，标题几乎永远用不上——这是对话，不是文档。
  🚨 列表里并排的是**选项**，不是问题；三个问题分成三行，它还是三个问题。
- **段落要用「第几段」来说，绝对不要说 b1/b2 这种编号。** 那是给你看的内部标记，
  她的屏幕上没有这个东西——说了她只会一脸茫然地找。
- 要具体到这篇文章：不要照着念这一步的标题（「精读重点段落第3段」），
  要说「往下翻到第三段，那段里有三个数字，先把它们圈出来」。
- 她答完一步之后，先接住她说的（一句就够），再领下一步。
- 她问问题的时候先回答她，回答完再把她带回当前这一步。

## 你绝对不能做的事

- **不要替她读。** 不要说出这篇文章的结论、主旨、答案、要点总结。她还没读呢——你先说了，后面每一步都成了走过场。
- 不要一次问好几个问题。
- 不要催她、不要评价她读得快慢。
- **不要训她。** 她没做到你要她做的那件事，绝大多数时候是她没看懂该点哪儿、
  该往哪看，不是她不肯做——多半是你把她指向了屏幕的另一边。
  所以规矩是正面的这一条：**不要去评论「她还没做到」这件事本身**，
  一个字都不要花在它上面。（换个说法绕不过去：「还没完」「先别往下」
  和「还没做完」是同一件事——它们都是在说她，而你该说的是那件事怎么做。）
  把该怎么做说得**更具体、更小步**，比如这样说：
  - 「在第 4 段里点一下讲『两把钥匙』那句——点完它会出现在下面。」
  - 「这一段里有三个数字，先看带百分号的那个，它跟前一句是不是对得上？」
  - 「不用整段都想好：先说你看到的第一个变化就行。」
  **一次说不通就换个说法，不要把同一句指令再讲一遍。**
- 不要说"作为AI"、不要空夸。

## 她指的东西有多大，决定你问她什么

她点了一段、划了一句、还是什么都没指，服务端会告诉你。**问题的大小必须跟上**：

- **一到两句** → 给它贴一个角色（主张 / 证据 / 限制 / 背景 / 对比），
  或者说清楚这两句之间是什么关系（发现→意义 / 主张→根据 / 主张→限制 / 问题→解决）。
  🚨 **不要拿一两句话去要一句概括**，除非你就是在练压缩。
- **一整段** → 十五个字以内的大意，或者「作者在这里的主张 + 一个他自己承认的限制」。
- **一整篇** → 三句话，而且**只在前面那些小的都做完之后**。

## 她卡住的时候，一级一级来

她说「不会」「太难了」「给点提示」——**这是要提示，不是要答案。** 按这个顺序，
一轮只给一级：

1. **方向**：「先不给答案。这里要找的是一处转折。」
2. **位置**：「看第二段 however 后面那句。」
3. **结构**：「前一句说的是预期，后一句说的是限制。」
4. **局部线索**：「关键的词是 is associated with——它比 cause 弱。」
5. **完整答案**。

🚨 **只有她明确要，才给第 5 级。** 「直接告诉我答案」「我放弃」「给我看范例」
是明确要；「不会」「太难了」「给点提示」不是。这两组不要混。

要她自己写点什么的时候，脚手架也是一级一级的：
先给**约束**（「用上『相关』这个词，不要写成因果」），再给**句子开头**
（「这项研究发现……之间存在相关」），最后才是填空框和词库。
🚨 **起手不要给填空框**——句子开头逼她自己决定填什么，填空框已经替她决定完了。

## 你指出问题之后，必须再要一次她的产出

一条以「正确的说法是 X，因为 Y」结束的纠正，是把她的下一步收走了。
每一条纠正都要以一句祈使收尾，产出她的**再一次尝试**：
「请据此改写这一句」「请用同一个结构，换一个话题再写一句」。
一次重来通常就够了，最多两次。

**她说的那句英文用词不地道的时候，不要直接把地道说法写给她。**
说清楚问题在哪儿，然后指向文章里已经有的那个说法：
「第 3 段作者用的是 may，请照那个语气改你这一句。」——她眼前就有的东西，
不要替她抄一遍。

## 一轮只修一件事，而且修最上面的那一件

她一次答得不对的地方往往不止一处。按这个顺序，**只修排在最前面的那一条**，
其余的这一轮一个字都不要提：

1. 她把作者的主张读错了
2. 证据或者作者自己写着的限制，她漏掉了
3. 她把「相关」当成了「因果」，或者把「可能」当成了「已经证明」
4. 长句子里她丢了主干
5. 转折、让步她没读出来
6. 词汇、搭配
7. 语法、文体

🚨 **词汇排在第六。** 上面五条全是「有没有读懂这个论证」。
一上来就改她的用词，等于绕过了真正要紧的那五件事。

## 什么时候往下走

- 她确实做完了当前这一步（哪怕做得粗糙）→ advance 给 "done"。
- 她说想跳过、说这步没意思、说她已经会了 → advance 给 "skipped"。**不要劝她**。
- 她还没做、或者答得完全没碰到这一步要她做的事 → advance 给 ""，留在原地，把这一步再说一遍（换个说法，别重复原话）。
- 一轮最多往前一步。

## 输出格式

只输出一个 JSON 对象：

{"reply":"你要对她说的话","advance":"","focusBlock":"","tool":"","lens":"","card":null}

- advance：""（留在当前步）/ "done"（当前步完成）/ "skipped"（她想跳过当前步）。
- focusBlock：如果这一步要她看某一段，给出段落编号（b1/b2/…）；否则留空。必须是真实存在的段落。
  **它必须和 reply 里你说的那一段是同一段。** 每段后面都标了「第几段」，照着填，别自己数。
  你嘴上说「第三段」、focusBlock 却给了 b4，她屏幕上跳开的就是另一段。
- tool：见下。不用就留空。
- lens：透镜卡的 id。你要她**亲手做一遍某种分析**的时候用它，见下。不用就留空。
- card：一张她可以直接点的卡片，见下。不发卡就整个省略这个键，或者给 null。

## 段落工具：你手上的教具

每一段都能用下面这些工具拆开。它们不是给她自己乱点的菜单——**该用哪一件、什么时候用，
由你决定**。你在 tool 里写一个 id，她屏幕上那一段就会自动展开这件工具的结果。

%s

怎么用：
- 她说这段读不懂、卡在某个句子上 → 用讲解类的工具（翻译 / 关键单词 / 语法 / 成语修辞 / 案例）。
- 她读懂了字面意思，但没看出作者的手法 → 用 craft / structure。
- 你想让她自己往深里想一层，而不是听你讲 → 用 questions（想一想）。
- 这一段的写法值得她自己练一遍 → 用 imitate（仿写）。**这是把读转成写的那一步**，
  遇到写法特别的段落别浪费。
- 一轮最多用一件。不确定就留空——工具是拿来推她一把的，不是拿来填满屏幕的。
- 用了工具，reply 里要说一句你为什么给她这个（一句就够），别让它凭空冒出来。

## 透镜：让她自己做一遍

段落工具是你讲给她听；透镜是她自己动手。用法只有一种，但这一种很重要：

先在 reply 里挑出这一段里的**某一句**，当着她的面把这种分析做一遍——
这一句为什么可疑 / 为什么有力 / 它在干什么——然后在 lens 里写下那张卡的 id。
她的屏幕上会出现这副透镜，先给她看你刚才的示范，再请她**在文章别的地方
自己找一句**做同样的事。

可用的透镜：

%LENS%

规矩：
- 🚨 **方法名照说，但说完要跟一句白话。**
  「传播学」「科学方法论」这些名字她要学，所以照说；但一个名字后面不跟一句
  「它就是看……」，那个名字对她就只是一个生词。实测她逐字报的：
  「它让我用传播学的角度分析……我真的不懂这些词是什么意思，我只是个高中生。」
  说法是：先给名字，再一句白话，然后当场拿这一段的某一句做一遍。
- 🚨 **先问自己：这篇文章撑得住这副透镜吗？**
  挑透镜要看**这一篇有没有那种东西**，不是看哪副听起来更深。
  实测出过这么一次：一篇讲打仗和救援物资的新闻，你召了「科学方法论」那副，
  一遍遍要她找「样本不够、测量有偏差」的句子 —— 那篇文章里根本没有做实验。
  她连着说了三遍「这篇没有这种句子」，越说越烦，而她是对的。
  **她说文章里没有这种句子的时候，先信她**：回去看一眼，真没有就换一副，
  或者干脆不用透镜，直接往下走。不要让她为一个不存在的东西找第四遍。
- 🚨 **格子名不要自己编。** 标注板的格子永远是那五个（主张/证据/限制/背景/对比），
  由服务端填。你在话里说「按因果链分成原因和结果两格」「分成正方反方」之类的，
  她屏幕上出现的仍然是那五个格子 —— 于是她照着你的话去找，找不到，就以为板没出来。
  实测她逐字报的：「它让我把第2段那三句话重新分类拖进格子里（因果链），但我屏幕上
  根本没有出现那个分类的板子和卡片。」说板的时候就说「把这几句各自放进它的角色里」。
- **给 lens 就必须同时给 focusBlock**，而且是你 reply 里刚讲的那一段。
  没有落点的透镜等于没有——她自己去透镜库点也是一样的东西。
- 一轮最多一副。屏幕上已经开着一副的时候，不要再给。
- **这一轮已经给了 lens，就不要再给卡片**（card 留空或省略，见下）。
  透镜和卡片都是把这一步交回她手上，两个一起弹出来，她只会先纠结做哪个。
- 读法清单走到 lens 那一步的时候，这是首选动作；别的时候，只有在她卡住、
  或者某一段特别值得她自己做一遍时才用。

### 她刚做完一副透镜的那一轮

prompt 里出现【她刚做完一副透镜】的时候，这一轮**是她交作业**，不是她在闲聊。
她刚刚自己在文章里找了一句、做了一遍分析，屏幕上那副透镜已经收起来了。
这一轮你必须做三件事，而且**只做这三件**：

1. 说出她选的那一句**哪里选得准**——具体到那一句本身，别说「很好」「不错」这种
   谁都能说的话。她要是选偏了，就说清楚偏在哪，但仍然承认她动手做了。
2. 把这次的发现接回**这篇文章要回答的问题**上：这一句让我们对这篇的判断有了
   什么变化。这是透镜存在的理由，不是装饰。
3. 领她进下一步。当前这一步已经用这副透镜做完了，就在 advance 里给 "done"。

绝对不要：
- **不要再给一副透镜，也不要给卡片**（lens、card 都留空）。她刚做完一件事，
  紧接着又被塞一件，等于这次动手没有被看见。
- 不要重复你上一轮说过的话。她已经读过了。
- 不要问她「感觉怎么样」。看她做了什么，然后往下走。

## 卡片：把这一步递到她手上，让她点

**带一步的默认方式就是给她一张卡片。** 一步的指令写成一段散文、末尾缀一句
「读完告诉我一声」，她要么随口应一声，要么得先自己组织语言——两种都不是读。
同一步写成一张能点的卡片，她不必先组织语言，但**不把文章读一遍就点不下去**。
所以每一轮先想一件事：这一步能不能交给一张卡片？能就发。只有这一步确实没法用
卡片承担（比如你这一轮主要是在回答她刚问的问题）才纯说话。
**第一轮也一样**：把路线介绍完之后，第一步要用一张卡片把她领进去，
不要以「先通读全文，读完告诉我」收尾。
要发卡就在 card 里给一个对象，不发就省略这个键。

五种卡片。前三种是**她说**，后两种是**她摆**——一块能用手拖的板：

- {"type":"choose_span","prompt":"一句话的问题","options":[{"blockId":"b3","quote":"文章里的原话"}]}
  从文章里的几句原话中点一句。options 给 2 到 4 条，**至少来自两个不同的段落**。
- {"type":"pick_in_article","prompt":"一句话的问题"}
  请她自己到正文里划出一句。没有 options ——「自己去找」就是这张卡的全部内容。
- {"type":"short_text","prompt":"一句话的问题"}
  请她用自己的话写一小段。没有 options。
- {"type":"label_roles","prompt":"一句话的问题","options":[{"blockId":"b3","quote":"文章里的原话"}]}
  **标注板**：几句原话摆在板上，她把每一句拖到一个角色格子里。
  格子是固定的五个（主张 / 证据 / 限制 / 背景 / 对比），**你不用给，也给不了**。
  options 给 2 到 4 条，**每一句都必须逐字抄自文章**（系统会核对，对不上就整张丢掉）。
  🚨 和 choose_span 不同：这几句**可以来自同一段**。同一段里的「后果」和「原因」
  正是关系最紧、也最值得让她分辨的一对。
  什么时候用：读法清单走到「标注论证」那一步，或者她说得出这篇在讲什么、
  却说不清哪句在撑着哪句的时候。**这是一次不问「你懂了吗」的理解检查**——
  贴不出来就是没读懂，而她一个字都不用写。
- {"type":"word_bank","prompt":"一句话的问题","words":[{"blockId":"b3","term":"scrambling"}]}
  **生词板**：这一段里的几个词摆在板上，她把每个拖进「认识 / 不确定 / 不认识」。
  words 给 3 到 6 个，**每个都必须逐字出现在它那个段落里**（系统会核对，
  核不上就丢掉）。挑真正值得学的：学术高频词、熟词僻义、地道搭配、
  这篇的话题核心词。**不要挑最长的那几个，也不要挑初中就学过的。**
  🚨 **不要在这一轮解释这些词。** 她分完之后你才知道该讲哪几个——
  下一轮只讲她划到「不确定」和「不认识」的那些，认识的一个字都别讲。
  这块板存在的全部理由就是让「讲哪几个词」这件事由她决定，而不是由你猜。

硬规矩：

- **问题必须是一个 5W1H 形状的真问题：问的是文章的内容。**
  作者想说明什么？作者是**怎么**让你相信这笔账划算的？这一段里**发生了**什么变化？
  作者**为什么**在这里放一个数字？哪一句你读着最不服气？——
  这些她都得先把那几句读懂，才答得出来。
  反过来，这些都不行：「哪一句最让你觉得作者在讲『为什么』」——她扫一眼哪条里带
  「因为」「所以」就点了，一个字都没读懂；「哪一句提到了数字」同理，扫阿拉伯数字就行；
  「最想xx的那处代价」根本不是一个人会问出口的话。
  🚨 出卡片之前先自问一句：这个问题能不能靠扫关键词答出来？能，就换一个。
  ⚠️ 这一条和下面那条同时守，不冲突：5W1H 管的是**问题的形式**（问的是内容），
  「不能有唯一正解」管的是**答案的空间**（站得住的答法有很多种）。
  「作者是如何让你相信这笔账划算的？」两条都满足。
- 🚨 **给板的那一轮，不要把答案说出来。**
  「第一句是主张，第二句是证据，你摆一下」—— 这样一说，这块板就只剩搬运了。
  走查里她逐字说过：「既然它都直接告诉我答案了我就照着搬吧……」「其实我不太
  分得清主张和证据的区别，但上面都告诉我答案了。」
  她分不清这几个角色是**正常的**（英语课上不讲论证成分），格子底下已经各有一句
  白话给她照着判断。你要做的是把板递出去，然后闭嘴等她摆完 —— 她摆错的那一张，
  正是下一轮你要讲的那件事。摆之前讲，你就没有东西可讲了。
- 🚨 **题目就是那道题：不写怎么操作，也不写有几句。**
  怎么拖、怎么点，卡片下面那行字一直在说；你再写一遍，出来的就是
  「这三句各自在算账的哪一步？拖到角色各自里。」这种句子 —— 产品负责人
  2026-09-12 逐字指过这一张。
  **数目更不要写。** 你先写题目再写选项，而选项要逐字核对原文，对不上的会被
  刷掉 —— 于是「这三句」剩下两句，她数得出来。用「下列句子」，不要用「这三句」。
  书面一点，像一道真的分析题：**「分析下列句子，判断它们各自属于哪一类论证成分。」**
  而不是「这三句各自在算账的哪一步？」
- **问题要问她的判断，不能有唯一正解。**「哪一句你读着最不服气」可以，
  「哪一句是作者的结论」不行——两个都逼她把几句都读一遍，但后一个是考试。
  我们不考她，她自己的想法才是这里最值钱的东西。
  所以卡片上没有正确答案，你下一轮也不要说她点得对不对。
- 🚨 **choose_span 的选项必须跨段落取：至少来自两个不同的段落。**
  把一段话按原文顺序剁成它的几句、摆成三个选项，那不是「从文章里挑几句让她选」——
  **选项就是那一段**：她不必读别的段落，扫一眼选项里的名词就能点。
  所以「第 X 段里，哪一句……」这种卡片一张都不要出。
  要出就在整篇文章里挑：第二段一句、第四段一句，让她非把两处放在一起比不可。
  系统会核对这件事：**存活的选项全部来自同一段，整张卡片会被丢掉**，她这一轮就什么也收不到。
- **不要连着出两张几乎一样的卡片。** 上一张卡她已经答过了，就不要再拿同一批句子
  问几乎同样的事——同样的三个选项、只换了一个词的问题，在她眼里就是
  「你答错了，再选一次」，哪怕你一个对错都没说。
- **上一张卡片问过的那件事，这一张就换一件事问。** 判据是**问题在问什么**，
  不是选项一不一样：「哪一句最能看出钱流向了谁」和「哪一句让你最清楚地看到钱去了哪里」
  换了一整批选项，在她眼里仍然是同一个问题被问了两遍。
  下一张卡要么换一段，要么换一种 5W1H（从「哪一句…」换成「作者是怎么…」／
  「这里发生了什么变化」），要么这一轮干脆不发卡、直接往下走。
- **choose_span 的每个 quote 必须逐字抄自文章**：一个字都不许改、不许缩写、
  不许把两句拼在一起、不许自己顺一遍。系统会拿它回原文里逐字核对，
  对不上就把整张卡片丢掉——她那一轮就什么卡也收不到。
  blockId 要写这句话真正所在的那一段；挂错段落一样作废。
- **引文要从一个标点后面开始、到一个标点为止**，不要从半句话中间截。
  一整句可以，用「，」「、」「；」隔开的一个完整从句也可以。
  举个真出过事的例子：原文是「所以近年来很多城市在做的事情，是把灰色的屋顶改成绿色的。」，
  只引「把灰色的屋顶改成绿色的」就是从「事情，是|把」中间切开的半句——
  它确确实实是原文里的字，但不是一句话，系统照样把这个选项丢掉。
  存活的选项不足 2 条，整张卡片就没了。
- **这几种问法一个都不要出，系统会把整张卡丢掉**：「什么是 X？」「X 是什么？」
  「X 有几个步骤/几个部分？」「X 重要吗？」「我们应当如何看待 X？」
  「X 的优缺点是什么？」「这段讲了什么？」——它们一句定义就能打发，不承重。
  换成动作、对比、因果、边界这四种里的一种：
  「他是怎么做到的？」「为什么是 A 不是 B？」「这一步凭什么成立？」「它在什么时候不成立？」
- prompt 是一句话，不超过 60 个字。
- blockId 只出现在 options 里，是给系统看的。**prompt 和 reply 里绝不能出现 b1/b2**，
  要说段落就说「第几段」。
- 一轮最多一张卡，而且**这一轮已经给了 lens，就不要再给卡片**：
  系统会把卡片丢掉、只留透镜。想让她点卡片，这一轮就别给透镜。
- 几个选项之间不要互相包含。「白天吸热、夜里放热」和「夜里放热」摆在一起是套娃，
  她根本没法「挑一句」；系统会把短的那条丢掉。
- **卡片已经把这一步要她做的事说清楚了，reply 就不要再把它复述一遍**，
  更不要照抄这一步的标题或说明。reply 这时候只说一句你对她刚才做的事的真实回应——
  接住她说的那句话，或者说清你为什么现在把这张卡给她。

### 她刚把一块板摆完的那一轮

板（label_roles / word_bank）摆完之后，她的作答会原样回到【她刚刚说的】里：
标注板是「角色：」加上她放进去的那一句，生词板是「词 — 她放的那一格」。

**这一轮和她刚做完一副透镜是同一件事：她交作业了，不是在闲聊。** 所以：

1. **先接住她摆的东西，而且要具体。** 说的是**某一句为什么摆在那儿**，
   不是「摆得不错」。她摆得跟你想的不一样，先认真看她的理由 ——
   一句话在不同的读法下确实可以既是证据又是限制。
   要是她确实摆偏了，说清楚偏在哪儿，**只说最要紧的那一处**（见上面那份优先级）。
2. **生词板还多一件事：只讲她划到「不确定」和「不认识」的那几个词。**
   她说认识的，一个字都不要讲 —— 这块板存在的全部理由就是让「讲哪几个词」
   由她决定。讲词的时候给的是**这一句里的意思**，不是词典释义。
3. **然后往下走。** 这一步已经用这块板做完了，advance 给 "done"。

绝对不要：
- **不要紧接着再给一块板或者一张卡片**（card 留空）。她刚动完手，
  马上又被塞一件，等于这次动手没有被看见。
- 不要把她摆的东西再复述一遍。她刚摆完，她记得。
- 🚨 **不要再让她动那块板。** 她一交上来，那块板就从屏幕上收走了 ——
  「把这句挪到主张旁边」「再拖一张过去」这类话，她照着做的时候会发现屏幕上
  什么都没有（实测她逐字说：「屏幕上没有板、没有卡片，也没有可以拖拽的地方」）。
  想让她再摆一次，就重新发一块**新的**板；否则就用说的。

## 三种特别的步骤

**通读（read）** —— 🚨 **不要问她「读完了吗」。**

你没有办法知道她读没读，她说「读完了」你也没有办法核实。所以这一步**不靠她
报告完成**，靠一张她不读就答不出来的卡片：出一张 choose_span，选项从**离得很远
的两三段**里取（比如第 2 段一句、第 11 段一句）。她答了，这一步就算做完
（advance 给 "done"）——答得出来就说明她扫过全文了。

线上真实发生过的死锁，别再来一次：印记 说「读完告诉我一声」→ 她回「好，我读完
了」→ 印记 说「你说的是哪一段？我要的是全文，第 1 段到第 18 段」→ 她再回一句
→ 印记 又问一遍 —— **连着七轮，一轮比一轮硬**，最后在问她「你现在读到第几段了？
给我一个数字」。那是审问，不是带读。

所以这一步只有两种走法：

- 给卡片，她答了 → advance "done"。
- 她说她读完了 → **就当她读完了**，advance "done"，往下走。下一步自然会暴露她
  有没有真读，而那时候你手上有具体的东西可以说。

## 两种特别的步骤

**链接经验（connect）** —— 这一步没有标准答案，也没有什么要检查的。
她说什么都算。你的活儿是接住她说的，问一句让她多说一点，然后往下走。
**不要评价她的经历，不要把她的话拉回文章的「正确理解」上。** 这一步存在的理由
就是让这篇文章跟她本人有关系；你一纠正，它就变回了阅读理解。

**找出关键句（hunt）** —— 这一步她必须**真的在文章里点出一句**。
她点出来的句子会单独给你（【她在文章里点出来的句子】）。
- 她点了 → 接住那一句，说说它好在哪儿 / 站不站得住，advance 给 "done"。
- 她只是说「我觉得是第三段那句」，却没有点 → 那是说的，不是点的。
  advance 留空，告诉她在文章里把那句划出来或者点一下，它会自己出现在对话里。
- 她点的句子跟你想的不一样 → **那不是错**。先认真看她点的这一句，
  很多时候她的理由比你预设的更有意思。

不要输出对象以外的任何文字或代码块标记。`

// readingPick is one sentence she pointed at in the article, rather than
// typed. Same shape, and the same reason, as the writing room's comment-quote
// validator: a guarantee you can check (literal substring of a real
// paragraph) beats one you asked the model to honor.
type readingPick struct {
	BlockID string `json:"blockId"`
	Quote   string `json:"quote"`
}

// validateReadingPicks keeps only picks that point at a real paragraph AND
// quote it literally. Anything else — an unknown block id, an empty quote, a
// paraphrase, or words that are real but belong to a different paragraph —
// is dropped silently rather than passed on for the model to sort out.
func validateReadingPicks(picks []readingPick, blocks []Block) []readingPick {
	byID := make(map[string]string, len(blocks))
	for _, b := range blocks {
		byID[b.ID] = b.Text
	}
	out := make([]readingPick, 0, len(picks))
	for _, p := range picks {
		q := strings.TrimSpace(p.Quote)
		if q == "" {
			continue
		}
		body, ok := byID[p.BlockID]
		if !ok || !strings.Contains(body, q) {
			continue
		}
		out = append(out, readingPick{BlockID: p.BlockID, Quote: q})
	}
	return out
}

// readingPickOrdinal finds the paragraph ordinal (第几段) for a pick's block
// id, counting position in blocks the same way readingBlockTag does — so the
// number shown here always matches the number the paragraph listing above it
// uses for the same block.
func readingPickOrdinal(blocks []Block, blockID string) (int, bool) {
	for i, blk := range blocks {
		if blk.ID == blockID {
			return i + 1, true
		}
	}
	return 0, false
}

// hasHuntPickEvidence is the F3 guard: a hunt step may only settle when a
// valid pick (a literal quote of a real paragraph) was seen THIS turn, or in
// the immediately preceding student turn. The one-turn lookback exists
// because she may point on turn N and the coach may legitimately settle on
// turn N+1 — without it, a coach that says "good, noted" one turn late would
// read as her having asserted rather than pointed.
//
// This needs no schema change: validateReadingPicks already guarantees
// `picks` covers this turn, and ReadingCoachPanel.tsx's send() always inlines
// every surviving quote into the student message content as `> ` blockquote
// lines BEFORE it is persisted — so re-validating the previous student
// message's quoted lines against the article's real paragraphs is an honest
// reconstruction of "did she point last turn", not a guess.
func hasHuntPickEvidence(picks []readingPick, msgs []sqlc.AtomMessage, blocks []Block) bool {
	if len(picks) > 0 {
		return true
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != "student" {
			continue
		}
		return quotedLinesCiteArticle(msgs[i].Content, blocks)
	}
	return false
}

// quotedLinesCiteArticle reports whether content contains at least one `> `
// blockquote line that is a literal substring of some real paragraph — the
// same "real paragraph, quoted literally" bar validateReadingPicks holds
// structured picks to, applied to the transcript's own `> ` convention.
func quotedLinesCiteArticle(content string, blocks []Block) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, ">") {
			continue
		}
		quote := strings.TrimSpace(strings.TrimPrefix(line, ">"))
		if quote == "" {
			continue
		}
		for _, blk := range blocks {
			if strings.Contains(blk.Text, quote) {
				return true
			}
		}
	}
	return false
}

// quoteIsArticleText reports whether s appears literally inside SOME
// paragraph — the broad form of the check validateReadingPicks makes against
// one declared paragraph. It exists for one decision only: does this string
// belong to the article, and therefore have to reach the transcript behind a
// `> ` prefix? That question must be answered by looking at the article, not
// by trusting a `type` field the client sent along with the text.
//
// It normalizes CRLF for the same reason SplitBlocks does: the article it is
// compared against is already LF-only, so a quote that kept its "\r\n" would
// fail this test on line endings alone and be filed as her own words.
func quoteIsArticleText(s string, blocks []Block) bool {
	s = normalizeCardAnswerText(s)
	if s == "" {
		return false
	}
	for _, blk := range blocks {
		if strings.Contains(blk.Text, s) {
			return true
		}
	}
	return false
}

// normalizeCardAnswerText brings a client-sent string onto the SAME line
// endings SplitBlocks (reading_blocks.go) already normalized the article to.
// Without it a quote carrying CRLF — a Windows browser, a PDF paste — can
// never be a literal substring of any block, so it fails every "are these the
// article's words?" test and lands in the transcript bare.
func normalizeCardAnswerText(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}

// cardAnswerChoiceParts classifies her tapped choice against the ARTICLE and
// splits it into the two halves composeCardAnswerMessage keeps apart: `quote`
// (the article's words, every line of which reaches the transcript behind a
// `> `) and `own` (hers, left bare). It also returns the picks the tap earns.
//
// 🚨 Classified by CHECKING, never by the declared type. A client that
// mislabels an article sentence as short_text would otherwise drop the
// article's words, unprefixed, into the corpus of "her own words".
func cardAnswerChoiceParts(choice, blockID string, blocks []Block) (quote, own string, picks []readingPick) {
	choice = normalizeCardAnswerText(choice)
	if choice == "" {
		return "", "", nil
	}
	if !quoteIsArticleText(choice, blocks) {
		// Not the article's words WHOLE — which is not the same as "none of it
		// is the article's". composeCardAnswerMessage classifies this half line
		// by line; a cross-paragraph selection lands here (a block never
		// contains a blank line) and still reaches the transcript quoted.
		return "", choice, nil
	}
	// Tapping a sentence IS pointing at it. Fed through the same validator as
	// a drag-selection so it earns the same standing: the coach sees it under
	// 【她在文章里点出来的句子】, and a hunt step may settle on it
	// (hasHuntPickEvidence).
	//
	// 🚨 …but only above the same floor card OPTIONS have to clear
	// (coachCardMinQuoteRunes). Two characters are a substring of the article
	// too, and promoting a two-character short_text answer into `picks` would
	// let a hunt step settle on words she never pointed at — exactly what F3
	// of the system prompt refuses ("她只是说…却没有点 → 那不是点的").
	if utf8.RuneCountInString(choice) < coachCardMinQuoteRunes {
		return choice, "", nil
	}
	return choice, "", validateReadingPicks([]readingPick{{BlockID: blockID, Quote: choice}}, blocks)
}

// dedupeReadingPicks drops a pick that repeats one already in the list — same
// paragraph, same sentence. Two paths produce picks in one turn (the panel
// inlines every drag-selection, and the tapped choice is promoted here), and
// 【她在文章里点出来的句子】 would otherwise show her one sentence twice.
func dedupeReadingPicks(picks []readingPick) []readingPick {
	seen := make(map[readingPick]bool, len(picks))
	out := make([]readingPick, 0, len(picks))
	for _, p := range picks {
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// composeCardAnswerMessage turns her tap into the one thing the transcript
// stores: a student message whose every line of ARTICLE text carries a `> `
// prefix, and whose own-words half carries none.
//
// 🚨 This is R4's third enforcement point, and the reason it is a named
// function with its own test rather than three lines inside the handler.
// On 2026-08-29 the same invariant was broken in ReadingCoachPanel.send():
// it prefixed only the FIRST line of a drag-selected quote. SplitBlocks
// splits the article on blank lines only, so a hard-wrapped paragraph (a PDF
// paste, a poem, a stretch of dialogue) is ONE block containing newlines —
// the article's later lines landed in a role='student' row with no prefix,
// sailed through report_facts.go's stripQuotedLines, and became eligible to
// be printed under her name on a shareable picture. Card options ARE article
// sentences, so they are that same landmine's second chance: every line gets
// the prefix, including the card's own question (印记's words, not hers —
// they must not count as her prose either).
//
// `own` is whatever is CLAIMED to be hers this turn — a short_text answer, or
// anything she typed alongside the tap. Claimed, not trusted: it is classified
// LINE BY LINE against the article, and only the lines that are not the
// article's reach the transcript bare.
//
// 🚨 Per line, because the classification upstream is whole-string, and a
// whole-string test is all-or-nothing: a selection that spans a blank line
// (blocks never contain one) or that carried CRLF matches NO block, so under
// the old rule EVERY line of it landed bare. Line by line is exactly the test
// stripArticleLines (atom_report.go) makes when the report is built — moved to
// write time, so it no longer depends on the source row still being there.
// That second net degrades to a no-op when the source is gone; this one
// cannot. A line of her own prose that happens to be a literal substring of
// the article is prefixed too, the same trade stripArticleLines already makes
// and for the same reason: a coincidental drop costs her nothing a report
// needs, a false keep is the leak this code exists to close.
func composeCardAnswerMessage(prompt, quote, own string, blocks ...Block) string {
	lines := make([]string, 0, 8)
	// 印记's own question, on ONE line. The prompt arrives from the client and
	// a card's question may quote a sentence; folded onto a second line, that
	// sentence would be a `> ` line that reads back as HER pointing at the
	// article (quotedLinesCiteArticle → hasHuntPickEvidence), letting a hunt
	// step settle on 印记's words. Behind 【印记问】 on a single line it can
	// never be a literal substring of any paragraph.
	if p := collapseCardPrompt(prompt); p != "" {
		lines = append(lines, "> 【印记问】"+p)
	}
	if q := normalizeCardAnswerText(quote); q != "" {
		// Every line, unconditionally — an internal "\n" inside one quote is
		// the whole reason this loop exists.
		for _, line := range strings.Split(q, "\n") {
			lines = append(lines, "> "+line)
		}
	}
	if o := normalizeCardAnswerText(own); o != "" {
		if len(lines) > 0 {
			// The same blank-line separation ReadingCoachPanel.send() uses
			// between a quote block and what she typed under it.
			lines = append(lines, "")
		}
		for _, line := range strings.Split(o, "\n") {
			if quoteIsArticleText(line, blocks) {
				lines = append(lines, "> "+line)
				continue
			}
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// collapseCardPrompt readies the client-sent card question for the one `> `
// line it is allowed: newlines folded into spaces, and anything longer than
// the cap validateCoachCard holds 印记's OWN cards to dropped outright. A
// prompt over 60 runes did not come from a card this coach wrote, and the
// transcript is not the place to find out what it did come from.
func collapseCardPrompt(prompt string) string {
	parts := make([]string, 0, 2)
	for _, line := range strings.Split(normalizeCardAnswerText(prompt), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			parts = append(parts, line)
		}
	}
	p := strings.Join(parts, " ")
	if p == "" || utf8.RuneCountInString(p) > coachCardPromptMaxRunes {
		return ""
	}
	return p
}

// readingLensDone is what she just finished doing with a lens, when this turn
// is the room reporting a completed one rather than her saying something.
//
// ## Why this exists (2026-09-03, colleague trial)
//
// Reported as 「透镜应用完毕之后，没有响应，没有推进到下一步。没有和透镜选句
// 打通」. It was all three, and none of them were the model's fault:
//
// The lens loop is shared with pro (apps/web/src/studio/reading/readingLoop.ts).
// Its confirm() saved the outcome, appended a canned confirmation line to
// loop.messages, and called clearCard(). But lite does not RENDER
// loop.messages — it renders ReadingCoachPanel, a different thread over the
// same atom_message table. So the one acknowledgement the loop produced went
// into an array nothing on screen reads, no turn was ever posted, and 印记
// therefore never reacted and never advanced the step. From the student's side
// the lens just vanished.
//
// 🚨 The fix could NOT be an empty-text turn. That is the exact shape that bit
// the PBL room (an empty-text turn after finishing a tool made 印记 repeat
// itself verbatim AND re-summon the tool she had just done), and this builder
// has the same landmine at the bottom: studentText == "" prints 「她刚点了
// 「开始」，还没说话」, which would make 印记 re-introduce the whole reading
// plan the moment she finished a lens. So a completed lens arrives as its own
// labelled section, and it SUPPRESSES that fallback line.
//
// Nothing here is re-asked of a model: Quote is the sentence she picked and
// Finding is the evaluation agent.EvaluateSelection already returned when she
// submitted the card — the same two values reportLensNote reads at report
// time.
type readingLensDone struct {
	// CardName is the lens's display name (「溯源体检」), not its id: this
	// string is for the model to say back to her.
	CardName string `json:"cardName"`
	// Quote is the sentence SHE found in the article. The selecting is the
	// thinking, so this is the part 印记 must respond to specifically.
	Quote string `json:"quote"`
	// Finding is what the room already concluded from her pick — 印记's OWN
	// prior words, never presented to it as something she said.
	Finding string `json:"finding"`
}

// clean reports whether this outcome carries enough to be worth a turn. A
// lens with no quote is not a completed lens, and refeeding one would spend a
// flagship call to say "nice work" about nothing.
func (l *readingLensDone) clean() bool {
	return l != nil && strings.TrimSpace(l.Quote) != ""
}

func buildReadingCoachPrompt(
	title string,
	blocks []Block,
	outline readingOutline,
	tasks []sqlc.ReadingTask,
	msgs []sqlc.AtomMessage,
	picks []readingPick,
	studentText string,
	lensDone *readingLensDone,
) string {
	var b strings.Builder
	if t := strings.TrimSpace(title); t != "" {
		b.WriteString("文章标题：" + t + "\n")
	}

	// 导读。它是排读法那一次就算出来的（reading_outline.go），学生屏幕上也摆着
	// 同一份 —— 所以这里给它，是为了让你和她看的是同一张地图，不是为了让你把它
	// 念一遍。
	if strings.TrimSpace(outline.OneLine) != "" || strings.TrimSpace(outline.Shape) != "" {
		b.WriteString("\n【导读（她屏幕上也有这一份，不要复述）】\n")
		if v := strings.TrimSpace(outline.OneLine); v != "" {
			b.WriteString("这篇在问：" + v + "\n")
		}
		if v := strings.TrimSpace(outline.Shape); v != "" {
			b.WriteString("结构：" + v + "\n")
		}
		b.WriteString("每段后面标着承重。**核心段停下来问一轮；支撑段连着过，" +
			"过完说一句它们在撑哪一段；过渡段一句带过。** 这是你安排节奏的依据。\n")
	}

	b.WriteString("\n【文章，按段落】\n")
	total := 0
	for i, blk := range blocks {
		text := strings.TrimSpace(blk.Text)
		if text == "" {
			continue
		}
		tag := readingBlockTag(i, blk.ID)
		if label := loadLabels[outline.Load[blk.ID]]; label != "" {
			tag += "·" + label
		}
		runes := []rune(text)
		if total+len(runes) > readingPlanArticleRuneBudget {
			b.WriteString(tag + "：（这一段没放进来，但它存在）\n")
			continue
		}
		total += len(runes)
		b.WriteString(tag + "：" + text + "\n")
	}

	b.WriteString("\n【你排的读法】\n")
	current := currentReadingTask(tasks)
	for _, t := range tasks {
		mark := "待办"
		switch t.Status {
		case "done":
			mark = "已完成"
		case "skipped":
			mark = "已跳过"
		}
		// The kind rides on every line, in the same English identifiers ("hunt",
		// "connect", …) the system prompt's own "## 两种特别的步骤" section names
		// them by — so a task line and the instructions that govern it are
		// actually joined up, instead of the model reverse-inferring a kind from
		// a Chinese label. Harmless to show her-facing paragraph tags too: like
		// the block-id tags above, this is an internal marker for the model, not
		// prose it is told to repeat to her.
		line := "- [" + mark + "] (" + t.Kind + ") " + t.Label
		if t.Detail != "" {
			line += "：" + t.Detail
		}
		if t.BlockID != "" {
			line += "（这一步看 " + t.BlockID + "）"
		}
		if current != nil && t.ID == current.ID {
			line += "   ← **她现在在这一步**"
		}
		b.WriteString(line + "\n")
	}
	if current == nil {
		b.WriteString("\n所有步骤都走完了。跟她说一句收尾的话，别再领新的一步。\n")
	}

	b.WriteString("\n【你们刚才聊的】\n")
	tail := msgs
	if len(tail) > readingCoachTurnsWindow {
		tail = tail[len(tail)-readingCoachTurnsWindow:]
	}
	any := false
	for _, m := range tail {
		var who string
		switch m.Role {
		case "student":
			who = "她"
		case "ai":
			who = "你"
		default:
			continue
		}
		if s := strings.TrimSpace(m.Content); s != "" {
			b.WriteString(who + "：" + s + "\n")
			any = true
		}
	}
	if !any {
		b.WriteString("（还没聊过。）\n")
	}

	// 🚨 她屏幕上现在摆着的那张卡片/板，原样给它看。
	//
	// 模型只看得见自己说过的**话**，看不见随那句话发出去的 card —— 而那张卡有时
	// 根本不是它写的（标注论证那一步由服务端兜底摆板，见
	// reading_coach_board_build.go）。于是它会对着一块自己没见过的板提要求：
	// 实测「把主张那张换成文章里某个人亲口说的话」，而板上四句全是叙述句，
	// 一句引语都没有 —— 她照着做不到，当场卡死。
	//
	// 递出去的东西要让它知道，这和「没送到要告诉它」是同一条闭环的两半。
	if card := lastOpenCard(tail); card != nil {
		b.WriteString("\n【她屏幕上现在摆着这张卡片，你看不到，所以照着它说话】\n")
		b.WriteString("类型：" + card.Type + "　问题：" + card.Prompt + "\n")
		for i, o := range card.Options {
			ord, ok := readingPickOrdinal(blocks, o.BlockID)
			where := ""
			if ok {
				where = "（第" + itoaSmall(ord) + "段）"
			}
			b.WriteString("  " + itoaSmall(i+1) + ". " + where + "「" + o.Quote + "」\n")
		}
		for i, w := range card.Words {
			b.WriteString("  " + itoaSmall(i+1) + ". " + w.Term + "\n")
		}
		if len(card.Labels) > 0 {
			b.WriteString("格子：" + strings.Join(card.Labels, " / ") + "\n")
		}
		b.WriteString("🚨 **只能要求她用板上真有的东西。** 板上没有的句子、没有的词，" +
			"不要让她去找 —— 她手上只有上面这几样。\n")
	}

	// 🚨 上一轮那张卡片没发出去的话，当面告诉它为什么。
	//
	// 它自己发现不了：写完就交出去了，下一轮的上文里只有它说过的话。线上实测
	// 连着六轮在说「点这张卡」而卡片每轮都被丢掉 —— 她屏幕上是一句句指着空气的
	// 话。这是「闭环」的失败那一侧：AI 递出去的东西没送到，也得让它知道。
	if why := lastDroppedCard(tail); why != "" {
		fix := cardFixIt[cardReject(why)]
		if fix == "" {
			fix = "这一轮换一件事做，或者直接把这一步说清楚。"
		}
		b.WriteString("\n【你上一轮递出去的东西没有到她屏幕上】\n" +
			"原因：" + why + "\n怎么改：" + fix + "\n" +
			"她那边只有你说的话，没有卡片、也没有透镜 —— 所以**不要再提「这张卡」" +
			"「这副透镜」「上面那块板」**，她看不到。\n" +
			"这一轮要么按上面的规矩重新给一次，要么就什么都不给，用一句具体的指令" +
			"把这一步说清楚。\n" +
			"🚨 **这件事不要说给她听。** 她不需要知道我们这边有校验、有规则、" +
			"有什么「系统不收」——那是我们的事，说出来只会让她觉得这个房间在出故障。\n")
	}

	// 🚨 这一步是不是卡住了（同一步带了三轮以上还没动）。卡住了才加这一节，
	// 没卡住一个字都不加 —— 常驻的提示会抢掉这一轮真正该做的事。
	// 见 reading_coach_repeat.go。
	if coachStepStuck(tasks, tail) {
		b.WriteString(coachStuckNudge)
	}

	// Structurally distinct from what she typed: a pick is a pointer at a real
	// paragraph, never spoken to the model as a block id (only 第几段, same as
	// the paragraph listing above) — the id is an internal marker, not
	// something the model should ever try to repeat back to her.
	if len(picks) > 0 {
		b.WriteString("\n【她在文章里点出来的句子】\n")
		for _, p := range picks {
			ord, ok := readingPickOrdinal(blocks, p.BlockID)
			if !ok {
				continue
			}
			b.WriteString("第" + itoaSmall(ord) + "段：「" + p.Quote + "」\n")
		}
	}

	// A completed lens is HER WORK, so it gets its own section rather than
	// being folded into 【她刚刚说的】 — the finding is 印记's own earlier
	// evaluation and must never read as a sentence she uttered.
	if lensDone.clean() {
		b.WriteString("\n【她刚做完一副透镜】\n")
		if n := strings.TrimSpace(lensDone.CardName); n != "" {
			b.WriteString("透镜：" + n + "\n")
		}
		b.WriteString("她自己在文章里找的那一句：「" + strings.TrimSpace(lensDone.Quote) + "」\n")
		if f := strings.TrimSpace(lensDone.Finding); f != "" {
			b.WriteString("你当时对这一句的复核（这是你自己的话，不是她说的）：" + f + "\n")
		}
	}

	if studentText != "" {
		b.WriteString("\n【她刚刚说的】\n" + studentText + "\n")
	} else if lensDone.clean() {
		// 🚨 NOT the 「她刚点了「开始」」 line below. She did not press 开始 —
		// she just finished a lens, and telling the model otherwise makes it
		// re-introduce the reading plan from the top (the PBL repeat-itself
		// bug, in this room). See readingLensDone's own comment.
		b.WriteString("\n【她刚刚说的】\n（这一轮她没打字——她是把透镜做完了。按「她刚做完一副透镜」那一节的三件事回应她。）\n")
	} else {
		// 🚨 这里原来写的是「介绍一下你排的读法」。那是一道自相矛盾的题：
		// 要它介绍，又不许它说这篇文章的内容，它只能去说「这篇大概在讲什么」
		// ——也就是内容。真模型给出的那一份把文章讲了一遍、替她减压、还以
		// 「读完告诉我一声」收尾，三条规矩一句话里全违反了。
		//
		// 导读卡（一句话 + 结构 + 段落承重）现在由前端确定性地渲染，摆在正文
		// 顶上。所以这一轮它**没有内容可讲**，只剩下递第一张卡片这一件事。
		// 见 reading_outline.go 和 docs/2026-09-10-reading-guidance-redesign.md。
		b.WriteString("\n【她刚刚说的】\n（她刚点了「开始」，还没说话。" +
			"导读已经摆在她面前了——这篇在问什么、它怎么组织、哪几段承重，她都看得见，" +
			"**不要再复述一遍**，也不要讲这篇文章的内容。" +
			"直接领她进第一步，并且用一张卡片把她领进去。）\n")
	}
	return b.String()
}

// answeredBoard —— 这一轮她交上来的是不是一块摆完了的板。
//
// 只认两块板的类型，且作答非空。她在输入框里打一句「我摆好了」不算：
// 这条判据的全部价值就在于它认的是**动作**，不是一句声明。
func answeredBoard(a *coachCardAnswer) bool {
	if a == nil || strings.TrimSpace(a.Choice) == "" {
		return false
	}
	switch strings.TrimSpace(a.Type) {
	case coachCardLabelRoles, coachCardWordBank:
		return true
	}
	return false
}

// dropReason —— 这一轮有什么东西没送到她屏幕上。卡片优先（它更具体）；
// 卡片没问题的时候，透镜那条也要说。
func (r readingCoachReply) dropReason() cardReject {
	if r.cardWhy != cardOK && r.cardWhy != cardRejectNoCard {
		return r.cardWhy
	}
	if r.lensWhy != "" {
		return cardReject(r.lensWhy)
	}
	return cardOK
}

// spokenParagraph —— 这句回复里提到的**最后一个**段号，换成段 id。
//
// 「第 5 段」是 印记 对她唯一的坐标说法（prompt 里明令不许说 b1/b2）。一句话里
// 提到好几段时取最后一个：「第 2 段说了封锁，现在我们看第 5 段」—— 她要去的是
// 第 5 段。段号不存在（它数错了）就当没说。
func spokenParagraph(reply string, blocks []Block) string {
	re := regexp.MustCompile(`第\s*([0-9]{1,2}|[一二三四五六七八九十]{1,3})\s*段`)
	all := re.FindAllStringSubmatch(reply, -1)
	for i := len(all) - 1; i >= 0; i-- {
		n := parseChineseOrdinal(all[i][1])
		if n >= 1 && n <= len(blocks) {
			return blocks[n-1].ID
		}
	}
	return ""
}

// parseChineseOrdinal —— 「5」或者「五」变成 5。超出两位就不认了（段号不会那么大）。
func parseChineseOrdinal(s string) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	digits := map[rune]int{'一': 1, '二': 2, '三': 3, '四': 4, '五': 5,
		'六': 6, '七': 7, '八': 8, '九': 9}
	r := []rune(s)
	switch {
	case len(r) == 1 && r[0] == '十':
		return 10
	case len(r) == 1:
		return digits[r[0]]
	case len(r) == 2 && r[0] == '十': // 十一 … 十九
		return 10 + digits[r[1]]
	case len(r) == 2 && r[1] == '十': // 二十 … 九十
		return digits[r[0]] * 10
	case len(r) == 3 && r[1] == '十': // 二十一 …
		return digits[r[0]]*10 + digits[r[2]]
	}
	return 0
}

// lastOpenCard —— 她屏幕上现在摆着的那张卡片（最后一条 印记 的话带的那张），
// 她还没答的时候。答过了就不必再给模型看：那一轮的作答本来就在转写里。
func lastOpenCard(msgs []sqlc.AtomMessage) *coachCard {
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role == "student" {
			// 她在这张卡之后说过话 —— 那就是答过了（或者这一轮不是卡片轮）。
			if coachAnswerFromPayload(m.Payload) != nil {
				return nil
			}
			continue
		}
		if m.Role != "ai" || len(m.Payload) == 0 {
			continue
		}
		var p coachMessagePayload
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			return nil
		}
		return p.Card
	}
	return nil
}

// coachAnswerFromPayload —— 这条学生消息里有没有一次卡片作答。
func coachAnswerFromPayload(raw []byte) *coachCardAnswer {
	if len(raw) == 0 {
		return nil
	}
	var p coachMessagePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil
	}
	return p.Answer
}

// lastDroppedCard —— 最后一条 印记 说的话里，那张卡片是不是被丢掉了；是的话
// 给出理由。只看最后一条：再往前的那些它已经收到过反馈了。
func lastDroppedCard(msgs []sqlc.AtomMessage) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != "ai" {
			continue
		}
		if len(msgs[i].Payload) == 0 {
			return ""
		}
		var p coachMessagePayload
		if err := json.Unmarshal(msgs[i].Payload, &p); err != nil {
			return ""
		}
		return p.Dropped
	}
	return ""
}

// readingCoachToolMenu renders the paragraph tools for the article's language
// into the coach's own prompt, from the same table the explain endpoint
// validates against — so an id the coach names is always an id the endpoint
// will accept.
func readingCoachToolMenu(lang string) string {
	var b strings.Builder
	for _, t := range readingBlockToolsFor(lang) {
		b.WriteString("- tool=" + t.ID + " · " + t.Label + "\n")
	}
	return b.String()
}

// readingCoachLensMenu renders the reading deck — id + name + when to reach
// for it — from the registry-backed agent.ReadingDeck(), the same
// single-source-of-truth shape readingCoachToolMenu uses for the paragraph
// tools. If the deck can't be resolved, an empty string is returned: the
// coach then simply never names a lens, the safe direction to fail in.
func readingCoachLensMenu() string {
	deck, err := agent.ReadingDeck()
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, c := range deck {
		b.WriteString("- lens=" + c.CardID + " · " + c.Name + " · " + c.Trigger + "\n")
	}
	return b.String()
}

// buildReadingCoachSystem assembles the coach's system prompt for the
// article's language. Both placeholders are filled with strings.Replace at
// count 1 each — not fmt.Sprintf, and each call targets its own distinct
// token (%s for the paragraph-tool menu, %LENS% for the lens menu) so the
// second substitution never collides with or re-consumes the first.
func buildReadingCoachSystem(lang string) string {
	system := strings.Replace(readingCoachSystem, "%s", readingCoachToolMenu(lang), 1)
	system = strings.Replace(system, "%LENS%", readingCoachLensMenu(), 1)
	return system
}

// currentReadingTask is the first step not yet settled. Nil when everything is
// done or skipped — the state where the coach stops leading rather than
// inventing a step to fill the silence.
func currentReadingTask(tasks []sqlc.ReadingTask) *sqlc.ReadingTask {
	for i := range tasks {
		if tasks[i].Status == "pending" {
			return &tasks[i]
		}
	}
	return nil
}

type readingCoachReply struct {
	Reply      string `json:"reply"`
	Advance    string `json:"advance"`
	FocusBlock string `json:"focusBlock"`
	// cardWhy 是这一轮那张卡片为什么没发出去（发出去了就是空）。不是模型给的，
	// 是校验器填的 —— 所以没有 json tag，它不参与解析。跟着 reply 一起走出去，
	// 是为了让它能被存进这条消息的 payload，下一轮当面告诉模型。
	cardWhy cardReject
	// lensWhy 同理：这一轮那副透镜为什么没落到文章上。
	lensWhy string
	// lensRetry：透镜递出去了，但这一轮的话配不上它 —— 没有当着她的面做一遍，
	// 或者话里说的是另一件她此刻做不了的事（板）。
	// 和上面两个不一样：它不导致任何东西被丢掉，只让这一轮重来一次。
	lensRetry    bool
	lensRetryWhy string
	// The paragraph tool the coach chose to reach for this turn, if any. The
	// tools are its teaching instruments, not a menu she is left to browse.
	Tool string `json:"tool"`
	// Lens is the reading-deck card id the coach reaches for this turn, if
	// any. A paragraph tool explains a paragraph; a lens makes her perform an
	// analysis on a sentence she chooses herself. Empty on most turns.
	Lens string `json:"lens"`
	// Card is the tappable card the coach wrote into this reply, if any. It
	// rides on the message, not on atom_card: this is not a lens summon, it
	// is the step's own instruction shaped so she answers by pointing.
	// Nil on most turns, and nil whenever validateCoachCard refused it —
	// a refused card is not an error, she simply gets words instead.
	Card *coachCard `json:"card"`
}

// tailRunes returns the last n runes of s, for logging a reply we could not
// read. Rune-safe: cutting UTF-8 by bytes puts a broken character in the log.
func tailRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return "…" + string(r[len(r)-n:])
}

// headRunes returns the first n runes of s, for the same reason tailRunes
// returns the last ones.
//
// 🚨 两头都要。一份读不出来的回复，只看尾巴分不出「前几块是好的、坏在最后一
// 块」和「第一块就坏了」——而这两种的处置完全相反：前者该查救援那条路，
// 后者该查提示词。2026-09-11 线上那一条只有尾巴，两种猜都成立。
func headRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// salvageCoachReply reads a coach reply key by key and keeps every field that
// arrived whole, stopping at the first one that did not.
//
// # 🚨 为什么需要它
//
// 模型会**在正常收尾的同时**把 JSON 断在半路：provider 报 finish_reason
// "stop"，completion 只有一两百个 token（上限是 16000），而对象停在
//
//	…"advance":"","focusBlock":"","tool":"","lens":"","card":{"type":"choose_span",
//	"prompt":"作者站哪一边？","options":[{"blockId":"b10","quote":"It shows that the
//
// 实测（TestLiveEnglishCoachFirstTurnParses，dashscope/deepseek-v4-pro）：
// **英文文章的第一轮 6 次里断 3 次**，加上生产那一次重试仍有约四分之一的轮次
// 直接变成 502。中文文章上同一个毛病只有 1/6（lensdone 那个文件量到的），
// 差别在于第一轮总会带一张卡片，而卡片的 options 要**逐字引用原文** ——
// 英文原句是同义中文句的三到五倍 token，尾巴就长得多。
// 给 provider 加 response_format=json_object 没有用（实测 4/6，没有变好）。
//
// 关键的事实是：断点**永远在 reply 后面**。学生要的那句话每一次都是齐的，
// 被切掉的是那件可有可无的教具。整份丢掉，等于为了一张卡片扔掉一轮好回答。
//
// 这不是编一个回答（[[ai-errors-must-surface-never-fake]] 禁的是那件事）——
// 留下来的每个字都是模型真的发出来的；没到齐的字段当作它没给。reply 自己
// 断了就仍然算失败，调用点会再问一次，两次都断才报错给她看。
//
// 同 `salvagePlanets`（news/select.go）：一天的星图也曾因为同一件事整个丢掉。
func salvageCoachReply(s string) (readingCoachReply, bool) {
	dec := json.NewDecoder(strings.NewReader(s))
	tok, err := dec.Token()
	if err != nil {
		return readingCoachReply{}, false
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return readingCoachReply{}, false
	}
	var got readingCoachReply
	for {
		key, kerr := dec.Token()
		if kerr != nil {
			break
		}
		if d, isDelim := key.(json.Delim); isDelim && d == '}' {
			break
		}
		name, isStr := key.(string)
		if !isStr {
			break
		}
		var raw json.RawMessage
		if verr := dec.Decode(&raw); verr != nil {
			// 这个字段没写完 —— 后面不会再有完整的东西了。
			break
		}
		// 单个字段类型不对，只丢这一个字段，不牵连整轮。
		switch name {
		case "reply":
			_ = json.Unmarshal(raw, &got.Reply)
		case "advance":
			_ = json.Unmarshal(raw, &got.Advance)
		case "focusBlock":
			_ = json.Unmarshal(raw, &got.FocusBlock)
		case "tool":
			_ = json.Unmarshal(raw, &got.Tool)
		case "lens":
			_ = json.Unmarshal(raw, &got.Lens)
		case "card":
			var card coachCard
			if json.Unmarshal(raw, &card) == nil {
				got.Card = &card
			}
		}
	}
	if strings.TrimSpace(got.Reply) == "" {
		return readingCoachReply{}, false
	}
	return got, true
}

func parseReadingCoachReply(text string, blocks []Block, lang string, lensOK func(cardID string) bool) (readingCoachReply, bool) {
	valid := make(map[string]bool, len(blocks))
	for _, blk := range blocks {
		valid[blk.ID] = true
	}
	c := strings.TrimSpace(text)
	if strings.HasPrefix(c, "```json") {
		c = strings.TrimLeft(strings.TrimPrefix(c, "```json"), " \t\r\n")
	} else if strings.HasPrefix(c, "```") {
		c = strings.TrimLeft(c[3:], " \t\r\n")
	}
	if strings.HasSuffix(c, "```") {
		c = strings.TrimRight(c[:len(c)-3], " \t\r\n")
	}
	if i := strings.IndexByte(c, '{'); i > 0 {
		c = c[i:]
	}
	whole := strings.TrimSpace(c)
	if j := strings.LastIndexByte(c, '}'); j >= 0 && j < len(c)-1 {
		c = c[:j+1]
	}
	var got readingCoachReply
	if err := json.Unmarshal([]byte(strings.TrimSpace(c)), &got); err != nil {
		// 🚨 断在半路的回复，把已经到齐的那部分留下来。见 salvageCoachReply。
		var ok bool
		if got, ok = salvageCoachReply(whole); !ok {
			// 🚨 最后一种：它压根没在写 JSON，直接说了人话。
			//
			// 2026-09-10 的模拟学生走查抓到的，日志里逐字记着：256 个字符、
			// stop_reason "stop"、一句**完全能用**的话 ——
			// 「你的眼睛很准——第4段是整个报道里信息最密的一段。现在我要给你
			// 一张卡片……」。我们把它整份丢掉，然后给她看
			// 「后台错误：AI 响应错误（model_unavailable）」，她当场卡死。
			//
			// 把它当 reply 用，别的字段全空：她拿到 印记 真正说的那句话，
			// 没有卡片、不推进，下一轮照常。这不是编造 —— 这就是模型说的话，
			// 只是没穿那件 JSON 外套。
			//
			// 🚨 只在**一个左大括号都没有**的时候这么做。它要是在写 JSON 只是
			// 写坏了，原样端给她的会是一堆 {"reply":... —— 那比报错更糟。
			if prose := strings.TrimSpace(whole); prose != "" && !strings.Contains(prose, "{") {
				return readingCoachReply{Reply: prose}, true
			}
			return readingCoachReply{}, false
		}
	}
	got.Reply = strings.TrimSpace(got.Reply)
	if got.Reply == "" {
		return readingCoachReply{}, false
	}
	// Only the two advances the contract names. Anything else — including a
	// model trying to jump several steps by inventing a value — leaves her
	// exactly where she is, which is the safe direction to fail in.
	if got.Advance != "done" && got.Advance != "skipped" {
		got.Advance = ""
	}
	if got.FocusBlock != "" && !valid[got.FocusBlock] {
		got.FocusBlock = ""
	}
	// A tool the coach named must exist AND fit this article's language — a
	// 语法 breakdown of a Chinese paragraph is confidently useless. An
	// unusable id is dropped rather than passed on: the reply still stands,
	// she just doesn't get an instrument that would have opened onto nothing.
	if got.Tool != "" {
		tool, found := findReadingBlockTool(got.Tool)
		if !found || (tool.Lang != "" && tool.Lang != lang) {
			got.Tool = ""
		}
	}
	// A tool with no paragraph to open on is meaningless.
	if got.Tool != "" && got.FocusBlock == "" {
		got.Tool = ""
	}
	// A lens must be aimed. An un-aimed coach summon is exactly the 透镜库
	// she already has — what makes this the thing the product asked for is
	// that it lands on the paragraph the coach just talked about.
	if got.Lens != "" && (got.FocusBlock == "" || lensOK == nil || !lensOK(got.Lens)) {
		// 🚨 透镜和卡片一样，被丢掉是**静默**的。线上实测（OSIRIS-REx 那篇）：
		// 印记 连着八轮**一字不差**地说「用一副透镜重新看一遍第8段」——透镜每轮
		// 都因为没有落点被丢掉，她屏幕上什么都没变，于是下一轮的状态和上一轮
		// 完全一样，同样的输入自然产出同样的输出。
		//
		// 记下来，理由和卡片走同一条路：存进这条回复的 payload，下一轮当面说。
		got.Lens = ""
		if got.lensWhy == "" {
			if got.FocusBlock == "" {
				got.lensWhy = "the lens had no focusBlock to land on"
			} else {
				got.lensWhy = "the lens id is not in the deck"
			}
		}
	}
	cardType, cardPrompt := "", ""
	if got.Card != nil {
		cardType = got.Card.Type
		cardPrompt = tailRunes(got.Card.Prompt, 60)
	}
	// A card whose options are not literally in the article is the one failure
	// she could never detect herself — the whole reason to build the card is
	// that its answer doesn't exist outside the text. So it is checked, not
	// trusted, and a card that fails is dropped rather than repaired: the turn
	// still succeeds and she gets the coach's words with no card attached.
	got.Card, got.cardWhy = validateCoachCardWhy(got.Card, blocks)
	// 🚨 说出为什么。丢掉是对的，静默不是：走查里 印记 连着两轮在说「把这几句
	// 拖到格子里」而板从来没出现过，日志里一个字都没有，只能靠猜。
	// 「no card in the reply」不记 —— 大多数轮本来就没有卡片，那不是失败。
	// 🚨 说了「点这张卡」却没给卡：对她来说和「卡片被丢掉」长得一模一样 ——
	// 屏幕上一句指着空气的话。区别只在日志里干净得可怕（没有东西被丢掉，
	// 是根本没有东西），所以这一条必须自己抓。
	//
	// 🚨 只在**它压根没给卡**的时候判这一条。给了卡但卡被上面那道校验刷掉，
	// 真正的原因是那一条（句子不在原文里、选项只来自一段、问法被禁……），
	// 在这里改写成「你提了卡却没给」就把真原因盖掉了 —— 日志和喂回去的
	// 修正话术会一起说错，而喂错了它下一轮只会照着错的方向改。
	// 实测那一幕：日志写着「提了卡片却没附」，同一行里却印着那张卡的 type
	// 和 prompt；她那一步什么都没等到，屏幕上只有一句指着空气的话。
	if got.Card == nil && got.cardWhy == cardRejectNoCard && replyPromisesACard(got.Reply) {
		got.cardWhy = cardRejectPromised
	}
	// 🚨 断在半句上的回复也算这一轮坏了。她读到的是半截话，不知道该干嘛。
	// 只在没卡片也没透镜的时候判 —— 带着卡片时用冒号收尾是正常写法。
	//
	// 🚨 这里原来写的是 `got.cardWhy == cardOK`，而**没有卡片的那一轮 cardWhy 是
	// cardRejectNoCard**（见 validateCoachCardWhy）—— 于是这道闸只对「带着卡片的
	// 回复」生效，而它的条件里又要求 Card == nil。两个条件永远不会同时成立，
	// 这道闸从写下来那天起一次都没响过。
	//
	// 线上逐字证据（atom 609f3910，2026-09-11）：seq 3 是
	// 「对，调查数据是一个方向。**但」，payload 里 dropped 是空的 —— 没有任何
	// 东西被判失败，她只能自己打一个「?」去问。产品负责人报的第 1 条就是它。
	if (got.cardWhy == cardOK || got.cardWhy == cardRejectNoCard) &&
		got.Card == nil && got.Lens == "" && replyLooksCutOff(got.Reply) {
		got.cardWhy = cardRejectCutOff
	}
	// 🚨 递透镜的那一轮，话里必须当着她的面把这套看法做一遍 —— 拿原文的一句。
	//
	// prompt 里早就写着「先在 reply 里挑出这一段里的某一句，当着她的面把这种
	// 分析做一遍」，但没有任何东西验它。实测她逐字报的：
	//   「它一直让我用一副『透镜』去拆句子，但从来没给我看过这副透镜是什么、
	//     怎么用。前面说要先演示一遍给我看，结果什么都没有。」
	//   「它让我用传播学的角度分析……我真的不懂这些词是什么意思，我只是个高中生。」
	// 方法名照说（[[yinji-must-talk-like-a-teacher]]：要用真的方法名），
	// 但光有名字没有示范，那个名字对她就是一个生词。
	//
	// 判据是能验的那一个：**这一轮的话里有没有一段逐字来自落点段的原文**。
	// 示范一定引原句，空谈一定不引。
	//
	// 🚨 这一条**不丢透镜**。丢了她就只剩那几个生词而没有工具，比教得薄更糟。
	// 走的是重来一次那条路：第二次带上示范就用第二次，仍然没有就照常把透镜给她。
	if got.Lens != "" && got.lensWhy == "" && !replyQuotesBlock(got.Reply, blocks, got.FocusBlock) {
		got.lensRetry = true
		got.lensRetryWhy = "the lens turn never demonstrates the method on a real sentence"
	}
	// 🚨 一轮里递了透镜，话里却在说板 —— 她照着话去做，做不成。
	//
	// 铁律③ 一次只交给她一件事：透镜在的时候卡片会被丢掉（cardRejectLensWon），
	// 于是「把这句挪到证据那个格子里」这句话指向的东西根本不存在。实测她逐字
	// 报的：「它让我把句子挪到『证据』那个格子里，但我现在看不到任何可以拖拽的
	// 板子或卡片，只有文本框。」
	// 递哪件，话就只说哪件。这一轮重来一次。
	if got.Lens != "" && got.lensWhy == "" && replyPromisesACard(got.Reply) {
		got.lensRetry = true
		got.lensRetryWhy = "the turn gives a lens but the words describe a board"
	}
	// 🚨 递透镜的那一轮，话不要以一个问句收尾。
	//
	// 透镜自己就是那句「请她做什么」：她要去文章里点一句。话里再抛一个问题，
	// 屏幕上就有了两件事，而它们要的动作不一样 —— 一个要她点句子，一个要她
	// 打字。实测她逐字报的：
	//   「它让我用『经济学透镜』在第12段里找一句，看哪个成本被漏掉了。但它又说
	//     『这一步要在文章里做』，我不知道到底是要我从第12段 pick 一句英文，
	//     还是在下面那个框里用中文写答案。」
	// 示范照做（上面那条），收尾用陈述句把手交给她。
	if got.Lens != "" && got.lensWhy == "" && !got.lensRetry && replyEndsOnAQuestion(got.Reply) {
		got.lensRetry = true
		got.lensRetryWhy = "the lens turn ends on a question, which asks her to type instead of pick"
	}
	// 🚨 讲完就停、什么也没请她做的那一轮，也算这一轮坏了。
	// 她屏幕上只剩一句讲完的话和一个灰着的发送键，而她不知道该等还是该点。
	//
	// 🚨 推进了一步**不算**给了她事做。第一版在这里加了 Advance == ""，于是
	// 「你选得准，我们进到下一段」这种一句话的收尾照样溜过去 —— 而下一步要她
	// 先开口，她手上却没有任何东西可说。实测她逐字报的：「它说我选得准、推进到
	// 下一段了，但是下面没有任何新题目或者按钮让我继续，发送也按不动。」
	// 推进和交给她一件事，是这一轮要同时做的两件事。
	if (got.cardWhy == cardOK || got.cardWhy == cardRejectNoCard) &&
		got.Card == nil && got.Lens == "" &&
		!replyAsksForSomething(got.Reply) {
		got.cardWhy = cardRejectDeadTurn
	}
	if got.cardWhy != cardOK && got.cardWhy != cardRejectNoCard {
		slog.Info("reading coach: card dropped", "why", string(got.cardWhy),
			"type", cardType, "prompt", cardPrompt)
	}
	if got.lensWhy != "" {
		slog.Info("reading coach: lens dropped", "why", got.lensWhy)
	}
	// 铁律③「一次只问一个」：透镜和卡片都是把这一步交回她手上。两个一起弹到
	// 屏幕上，她第一件要做的事就变成了「先做哪个」—— 那是我们替她制造的分心。
	//
	// 丢卡片、留透镜：透镜是更重的教学器械（召唤 → 选句 → 评估 → 发现一整套
	// 流程），已经落进 atom_card 并受 atom_card_one_open_idx 约束；卡片是轻的，
	// 只在这条消息上，下一轮再给一张没有任何损失。
	//
	// 这一步**必须排在两个校验之后**：透镜自己没活下来（比如没给 focusBlock）
	// 的那一轮，她只剩卡片一件教具，「一次只问一个」本来就满足了，
	// 没有理由让卡片跟着陪葬。
	if got.Lens != "" && got.Card != nil {
		got.Card = nil
		// 🚨 第三个静默丢弃点，而且它在上面那两条日志**之后**，所以之前一次都
		// 没被记下来过。模拟学生走查（2026-09-10，第三轮）就死在这儿：
		// 印记 说「现在我给你一张卡片，把这三层的关系看清楚」，卡片被这一行拿掉，
		// 她在屏幕上找了三轮那张卡，最后 stuck。日志里干干净净。
		//
		// 丢卡片仍然是对的（铁律③：一次只问一个），但她那边少了一样它刚说过的
		// 东西，所以照样要说 —— 走的是和另外两种同一条路。
		got.cardWhy = cardRejectLensWon
		slog.Info("reading coach: card dropped", "why", string(got.cardWhy))
	}
	return got, true
}

// enforceLensDoneTurn is the code half of the system prompt's 「她刚做完一副
// 透镜的那一轮」 rule: she just handed something in, so this turn must not
// hand her something new.
//
// 🚨 Why this is code and not only prose. The prompt already says 「lens、card
// 都留空」. Running the real model three times through the real prompt
// (TestLiveLensDoneReplyParses, 2026-09-04) gave: 3/3 parsed, 3/3 advanced
// with "done", the words themselves good — and **2 of 3 attached a card
// anyway**. Which is not mysterious: the same prompt carries a STRONGER
// standing default (「带一步的默认方式就是给她一张卡片」), and when two rules
// collide a model follows the louder one.
//
// That is exactly what [[prompt-output-must-be-verifiable-2026-09-03]] is
// about — a 「必须」 in a prompt has to be checkable in code. So this rule
// lands here, structurally identical to the parser's own 「已给 lens 就丢卡片」
// (铁律③): drop the lighter instrument so the thing she just finished is what
// gets seen.
//
// Only `Card` and `Lens` are dropped. `Reply`, `Advance` and `FocusBlock` are
// precisely what this turn SHOULD carry: catch the sentence she picked, settle
// the step, walk her to the next one.
//
// A pure function rather than four lines inline, so the live test can run the
// same rule production runs instead of asserting on an approximation of it.
func enforceLensDoneTurn(got readingCoachReply, lensDone *readingLensDone) readingCoachReply {
	if lensDone == nil {
		return got
	}
	got.Card = nil
	got.Lens = ""
	return got
}

// postReadingCoachTurn is POST /api/v1/readings/{id}/coach.
//
// One guided turn. `text` empty means she pressed 开始 — the coach introduces
// the plan and leads her into step one. A plan is generated on demand if there
// isn't one yet, so 开始 is genuinely the only button she needs.
func (a *API) postReadingCoachTurn(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	entitled, eerr := HasEntitlement(r.Context(), u)
	if eerr != nil {
		httpx.WriteError(w, r, eerr)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var req struct {
		Text  string        `json:"text"`
		Picks []readingPick `json:"picks"`
		// CardAnswer is set when this turn IS a tap on the card 印记 wrote into
		// its last reply. It is not a substitute for `text` — she may tap and
		// type in the same turn — and it never arrives as free-floating words:
		// the choice is checked against the article before it is allowed near
		// the transcript.
		CardAnswer *coachCardAnswer `json:"cardAnswer"`
		// LensDone is set when the ROOM is reporting that she finished a lens,
		// rather than her saying something. Carries no authority of its own:
		// it only ever lands in the prompt (see readingLensDone), never in the
		// transcript, and never marks a step done by itself — 印记 still
		// decides `advance`.
		LensDone *readingLensDone `json:"lensDone"`
	}
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	studentText := strings.TrimSpace(req.Text)
	// Trimmed here so `clean()` and every prompt read below agree, and so a
	// client sending whitespace cannot buy a turn.
	lensDone := req.LensDone
	if !lensDone.clean() {
		lensDone = nil
	}

	src, err := a.d.Queries.GetReadingSource(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	blocks := SplitBlocks(src.Body)
	if len(blocks) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_article", "这篇还没有正文，先把文章贴进来。", nil))
		return
	}
	picks := validateReadingPicks(req.Picks, blocks)

	// A tapped answer becomes a REAL turn: one student message, written into
	// the same transcript everything else reads. Anything less and her answer
	// is invisible — buildReadingCoachPrompt would not see it next turn, and a
	// refresh would show 印记 asking a question she had already answered.
	//
	// studentContent is what actually gets stored and what the model is shown,
	// deliberately the same string: what the coach reads this turn is exactly
	// what the next turn will read back out of 【你们刚才聊的】.
	studentContent := studentText
	var studentPayload []byte
	// An answer with nothing in it is not a turn: composing on the prompt alone
	// would store a student message that is only 印记's own question.
	if ca := req.CardAnswer; ca != nil && (normalizeCardAnswerText(ca.Choice) != "" || studentText != "") {
		answer := &coachCardAnswer{
			Type: strings.TrimSpace(ca.Type),
			// The client's own copy of 印记's question, held to the same cap
			// validateCoachCard holds the card to — so what is stored is what
			// the room will re-render, and neither can be an essay.
			Prompt:  collapseCardPrompt(ca.Prompt),
			Choice:  normalizeCardAnswerText(ca.Choice),
			BlockID: strings.TrimSpace(ca.BlockID),
		}
		quote, ownFromChoice, pointed := cardAnswerChoiceParts(answer.Choice, answer.BlockID, blocks)
		picks = dedupeReadingPicks(append(picks, pointed...))
		own := studentText
		if ownFromChoice != "" {
			if own != "" {
				own = ownFromChoice + "\n\n" + own
			} else {
				own = ownFromChoice
			}
		}
		if content := composeCardAnswerMessage(answer.Prompt, quote, own, blocks...); content != "" {
			studentContent = content
			studentPayload = coachCardAnswerPayload(answer)
		}
	}

	turnCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancel()

	// A plan on demand: 开始 is the only button, so pressing it with no plan
	// yet must produce one rather than refusing. This is the ONLY caller that
	// plans implicitly — the explicit endpoint stays for 重排.
	tasks, err := a.d.Queries.ListReadingTasks(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(tasks) == 0 {
		if tasks, err = a.planReadingTasks(turnCtx, u.ID, at.ID, src, blocks); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}

	msgs, err := a.d.Queries.ListAtomMessages(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// §model-routing · dialogue.
	//
	// 🚨 This is the single biggest deliberate downgrade of the 2026-09-02 class
	// migration, and it is a HYPOTHESIS, not a settled decision. The previous
	// note here argued flagship: "leading someone through a text — deciding
	// whether what she just said actually counts as having done this step — is
	// judgement, not conversation." That reasoning is real. What it was weighed
	// against, when there were only three lanes, was a chaperone lane that also
	// served the ordinary chat turn — so "some judgement" could only be bought
	// by buying the never-downgrade reviewer, full reasoning and all.
	//
	// The cost of that: every turn a student waits through in the lite reading
	// room ran at flagship price with reasoning at max. dialogue is the class
	// that says "she is watching this land, so it has 4 seconds" — and the
	// judgement it must still make is what routebench's dialogue judge case
	// scores. If a chaperone-class model cannot tell "she did the step" from
	// "she did not", this call goes back up to review and the win is given back.
	resolved, okResolve := a.route(turnCtx, gateway.ClassDialogue)
	if !okResolve {
		slog.Warn("reading coach: no provider resolved",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	lang := readingLangOf(src.Body)
	system := buildReadingCoachSystem(lang)
	chatReq := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: system},
			{Role: gateway.RoleUser, Content: buildReadingCoachPrompt(src.Title, blocks, decodeOutline(src.Outline), tasks, msgs, picks, studentContent, lensDone)},
		},
	}
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, chatReq)
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "reading_coach", resolved, res.Usage)
	if cerr != nil {
		slog.Warn("reading coach: provider call failed", "err", cerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	cardRows, err := a.d.Queries.ListAtomCards(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	deck, deckErr := agent.ReadingDeck()
	ordering := readingOrderingGuard(cardRows)
	anyOpen := false
	for _, c := range cardRows {
		if c.Status == "proposed" || c.Status == "active" {
			anyOpen = true
			break
		}
	}
	// The coach may only reach for a lens the room could actually open right
	// now. Checking here rather than after the call means a refused summon
	// never reaches her as a card that silently failed to appear.
	lensOK := func(id string) bool {
		if deckErr != nil || anyOpen || !inReadingDeck(deck, id) {
			return false
		}
		if id == "sift" && !ordering.AllowSift {
			return false
		}
		if id == "craap" && !ordering.AllowCraap {
			return false
		}
		return true
	}
	parsed, okParse := parseReadingCoachReply(res.Text, blocks, lang, lensOK)
	// 🚨 让她把一张卡挪到它**已经在**的那一格，是一条她做不到的指令。
	//
	// 她摆完的结果原样在转写里（「限制：」加上那一句），所以这是它没读，不是
	// 我们没给。实测她逐字报的：「它让我把发电机那句拖到限制格，但那句已经在
	// 限制格里了……屏幕上显示的摆放和它文字描述的矛盾了，我没法确定该怎么挪。」
	//
	// 判据要同时满足三件事，才不会误伤一句正常的肯定
	// （「你把发电机那句放进限制，这个判断很准」是对的，不能拦）：
	// 话里有一个「挪」的动词 + 提到了那一句 + 点了那一格的名字。
	if okParse && replyAsksForANoOpMove(parsed.Reply, lastBoardPlacement(msgs)) {
		parsed.lensRetry = true
		parsed.lensRetryWhy = "the reply asks her to move a card into the bin it is already in"
	}
	// 🚨 「说了给卡片，却没给」也算这一轮坏了，和解析失败一样，也用同一条退路：
	// 再问一次。
	//
	// 这是模拟学生走查里唯一一个反复挡住整条链子的东西：印记 在话里说
	// 「现在给你一张卡片」「用一张卡片收」，JSON 里却没有 card，她屏幕上什么都
	// 没有 —— 于是她在那儿找卡片，找不到就 stuck。四条走查死在这上面。
	//
	// 把理由喂给下一轮是对的，但**救不了这一轮**：她这一轮看到的仍然是一句指着
	// 空气的话，而她往往就在这一轮放弃了。所以在把它交给她之前先重来一次。
	//
	// 只重来一次，而且失败了就照常往下走（她拿到那句话，没有卡片）——
	// 不编、不改写模型的话（[[ai-errors-must-surface-never-fake]]）。
	// 🚨 被校验刷掉的卡片也要立刻重来一次，不只是「说了卡却没给」那一种。
	//
	// 把理由留给下一轮，救不了这一轮：她这一轮看到的是 印记 说「我给你一张卡」
	// 而屏幕上什么都没有。线上逐字证据（atom 609f3910，2026-09-11）：seq 36
	// 的卡因为选项全来自同一段被丢掉，seq 42 的卡因为引文对不上被丢掉 ——
	// 中间 印记 连着两轮道歉「卡没送到你手里」，她连着两轮回「没有卡啊」。
	// 产品负责人报的第 5 条就是这两轮。
	if okParse && (parsed.cardWhy == cardRejectPromised || parsed.cardWhy == cardRejectCutOff ||
		parsed.cardWhy == cardRejectDeadTurn || parsed.lensRetry ||
		parsed.cardWhy == cardRejectOneBlock || parsed.cardWhy == cardRejectFewOptions ||
		parsed.cardWhy == cardRejectFewWords || parsed.cardWhy == cardRejectBannedForm) {
		why := string(parsed.cardWhy)
		if parsed.lensRetry {
			why = parsed.lensRetryWhy
		}
		slog.Warn("reading coach: reply looks broken, retrying once",
			"why", why,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		if retryRes, retryErr := gateway.Collect(turnCtx, a.d.Provider, resolved, chatReq); retryErr == nil {
			a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "reading_coach", resolved, retryRes.Usage)
			// 只在第二次**确实更好**的时候采用它：解析得动，而且不再是一句空话。
			// 否则留着第一次那份 —— 它至少是完整的一句话。
			// 第二次只在**它确实更好**的时候采用：解析得动，而且没有被判失败。
			if again, ok2 := parseReadingCoachReply(retryRes.Text, blocks, lang, lensOK); ok2 &&
				!again.lensRetry &&
				(again.cardWhy == cardOK || again.cardWhy == cardRejectNoCard) {
				res, parsed = retryRes, again
			}
		}
	}
	if !okParse {
		// 🚨 ASK ONCE MORE. Measured 2026-09-04 against the live model
		// (TestLiveLensDoneReplyParses, 6 samples): **1 in 6 replies arrives
		// truncated** — the model writes a perfectly good reply, gets as far
		// as the trailing optional key and simply stops:
		//
		//	…"advance":"done","focusBlock":"","tool":"","lens":"","card":
		//
		// and the provider reports `stop_reason: "stop"`, i.e. a NORMAL
		// finish. So there is nothing to detect it by other than the parse
		// failing, and no client cap to raise: max_tokens is 16000 and the
		// reply was ~100.
		//
		// One retry turns a ~17% chance of 「AI 响应错误」 into ~3%. It is not
		// a workaround for a prompt problem — the reply that came back was
		// GOOD, it was cut off mid-serialisation — and it is not a fake
		// answer either ([[ai-errors-must-surface-never-fake]] forbids
		// inventing a plausible sentence; asking the model again is the
		// opposite of that). A second failure still surfaces honestly.
		//
		// This is a pre-existing defect of every reading-coach turn, not of
		// the lens-completion turn — it was simply never measured until a
		// live walk of the lens refeed hit it.
		slog.Warn("reading coach: reply unparseable, retrying once",
			"atom_id", at.ID, "stop_reason", res.StopReason,
			"request_id", httpx.RequestIDFromContext(r.Context()))
		res, cerr = gateway.Collect(turnCtx, a.d.Provider, resolved, chatReq)
		a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "reading_coach", resolved, res.Usage)
		if cerr != nil {
			slog.Warn("reading coach: retry call failed", "err", cerr,
				"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
			httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
			return
		}
		parsed, okParse = parseReadingCoachReply(res.Text, blocks, lang, lensOK)
		if !okParse {
			// 🚨 把回复的尾巴带上。2026-09-08 排查这条 502 时，日志里有
			// atom_id、有 stop_reason，唯独没有**模型到底回了什么** —— 而那
			// 正是唯一能分辨「断在半路」和「答得不对」的东西，只能靠重跑一遍
			// 实测去猜。尾巴 200 字，够看清断点在哪个字段上。
			slog.Warn("reading coach: reply unparseable after retry",
				"atom_id", at.ID, "stop_reason", res.StopReason,
				"reply_len", len(res.Text), "reply_tail", tailRunes(res.Text, 200),
				"request_id", httpx.RequestIDFromContext(r.Context()))
			httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
			return
		}
	}

	parsed = enforceLensDoneTurn(parsed, lensDone)

	// The student turn, the coach turn, and the step advance land together.
	// Split, a crash between them leaves the transcript saying one thing and
	// the plan another — and the plan is what the next turn reads to decide
	// where she is.
	tx, err := a.d.Pool.Begin(turnCtx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(turnCtx) }()
	qtx := a.d.Queries.WithTx(tx)

	// 串行化这颗原子的 seq 分配。NextAtomMessageSeq 是先读后插，
	// 并发下两笔事务会读到同一个 MAX——唯一索引保住的是数据，代价是
	// 其中一轮直接失败，而那一轮的模型钱已经花掉了。见 queries/atom.sql。
	if _, err := qtx.LockAtom(turnCtx, at.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	seq, err := qtx.NextAtomMessageSeq(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 🚨 studentContent, not studentText: a PURE tap types nothing, and the old
	// `studentText != ""` gate wrote no student row at all. Her answer would
	// have been missing from the next turn's context and gone after a refresh.
	// The tap's payload rides along so the room can re-render the card she
	// already answered instead of one still waiting for her.
	if studentContent != "" {
		if _, err := qtx.AppendAtomMessage(turnCtx, sqlc.AppendAtomMessageParams{
			AtomID: at.ID, Seq: seq, Role: "student", Content: studentContent,
			Payload: studentPayload,
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		seq++
	}
	// 🚨 走到「标注论证」这一步而模型没给板 —— 服务端自己摆一块。
	//
	// 这一步的**全部内容**就是那块板。而实测下来模型一遍遍在话里说「把这三句
	// 拖到格子里」却不附 card：十条走查里这是唯一反复挡住整条链子的东西。
	// 检测、重试、把理由喂回去，都只是降低概率，她还是会撞上「屏幕上根本没有板」。
	//
	// 这一步不需要模型来决定「有没有板」：读法库已经规定了它是标注论证
	// （reading_routines.go：步骤由 routine 拥有）。挑哪几句仍然优先用它的判断，
	// 它没给才用确定性的规则兜底 —— 和排读法那条链子是同一个分工。
	//
	// 🚨 兜底出来的那块板照样送进 validateCoachCard：它不是一条绕过校验的后门，
	// 句子逐字来自正文，本来就过得了。
	// 🚨 两种情况都要摆板：走到标注论证那一步，**或者**它嘴上说了板却没附。
	//
	// 后一种是实测反复出现的那一幕：她刚把板交上去（板随即从屏幕上收走），
	// 印记 接着说「把这句挪到主张旁边」「再拖一张过去」，她照着做时屏幕上什么
	// 都没有。prompt 里写了「想让她再摆一次就重新发一块新的板」，它不照做。
	// 写了两版规矩都不管用之后，改成：它说了，我们就真的给她一块。
	if cur := currentReadingTask(tasks); cur != nil && parsed.Card == nil && parsed.Lens == "" &&
		(cur.Kind == string(taskLabel) || replyPromisesACard(parsed.Reply)) {
		focus := parsed.FocusBlock
		if focus == "" {
			focus = cur.BlockID
		}
		// 🚨 它嘴上说的那一段才是她正在看的那一段。
		//
		// focusBlock 和这一步自带的 BlockID 经常都是空的，兜底就从第 1 段取句子
		// —— 而 印记 那句话说的是「我们来摆第 5 段的这几句」。实测她逐字报的：
		// 「它说让我摆第5段的句子，但板上给的卡片全是第1段的。」
		//
		// 它对她只会说「第几段」（prompt 里明令不许说 b1/b2），所以从回复里把那个
		// 段号读回来，比任何字段都准。
		if spoken := spokenParagraph(parsed.Reply, blocks); spoken != "" {
			focus = spoken
		}
		if built := validateCoachCard(buildLabelBoardFromReply(blocks, focus, parsed.Reply), blocks); built != nil {
			slog.Info("reading coach: label step had no board, built one",
				"atom_id", at.ID, "options", len(built.Options))
			parsed.Card = built
			parsed.cardWhy = cardOK
		}
	}

	// The card rides on the AI message's payload (0106), inside the same
	// transaction as the words it came with — so a refresh can never show her
	// the reply without the card it was written around.
	if _, err := qtx.AppendAtomMessage(turnCtx, sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: seq, Role: "ai", Content: parsed.Reply,
		// 卡片没发出去的时候，理由也一起存 —— 下一轮当面告诉它。见
		// coachMessagePayload.Dropped。
		// 🚨 两次都断的时候，这条半句话仍然会交给她（不编、不改写它的话）——
		// 但要让界面说出「这条没说完」。产品负责人 2026-09-12：
		// 「sometimes the AI response interrupts mid-stream without any notice」。
		Payload: coachCardPayloadFull(parsed.Card, parsed.dropReason(),
			replyLooksCutOff(parsed.Reply)),
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if current := currentReadingTask(tasks); current != nil && parsed.Advance != "" {
		advance := parsed.Advance
		// F3: a hunt step settling on "done" must be backed by an actual point,
		// not an assertion the model was talked into accepting. "skipped" is
		// deliberately untouched — 铁律② means she can always decline a step by
		// saying so, and a guard that trapped her on the hunt would defeat the
		// whole point of that ruling.
		if current.Kind == string(taskHunt) && advance == "done" && !hasHuntPickEvidence(picks, msgs, blocks) {
			advance = ""
		}
		// 🚨 她把标注板摆完了，这一步就是做完了 —— 不由模型决定。
		//
		// 和上面那条 hunt 是同一件事的另一半：hunt 那条防的是「模型被说服了就
		// 推进」，这条防的是「她真的做完了，模型却不推进」。
		//
		// 实测（2026-09-11 模拟学生走查）：她把三张卡片全摆进格子、提交，印记
		// 回了一段很好的点评（「这三张贴得很干净……」），然后 advance 给了空 ——
		// 这一步永远停在那儿。130 步只走完 8 步里的 3 步，卡的就是这里。
		// prompt 里那一节明写着「这一步已经用这块板做完了，advance 给 done」，
		// 而它不照做，所以改由代码兜底
		// （[[prompt-output-must-be-verifiable-2026-09-03]]）。
		//
		// 这不是替她判对错：板上本来就没有对错，摆完这个动作本身就是这一步的
		// 产出，和 hunt 要求「真的点一句」是同一种判据。
		if current.Kind == string(taskLabel) && advance == "" && answeredBoard(req.CardAnswer) {
			advance = "done"
		}
		// 🚨 一步耗满六轮，我们替她往下走。
		//
		// 实测下来模型**极少主动推进**：一条 150 步的走查里，八步只走完两步，
		// 每一步都在「再找一句」「再说说看」之间来回。卡住提示（第 3 轮）给过
		// 之后再等三轮，还不动就是不会动了 —— 这正是通读那一步变成审问的那个
		// 形状，只是换了一步。
		//
		// 往下走**不等于**判她做完了：她在这一步里说过的每一句话都在转写里，
		// 过程评估读的是那个（铁律④）。把她钉在原地才是真的丢东西 ——
		// 她会直接关掉页面。
		//
		// hunt 那一步不在此列：它要的是「真的在文章里点一句」，而那个证据
		// 上面已经单独判过了，替她推进会把这一步唯一的保证也抹掉。
		if advance == "" && current.Kind != string(taskHunt) && coachStepStalled(tasks, msgs) {
			// 🚨 透镜那一步耗满了，先把敞开的透镜撤掉再往下走。
			//
			// 不撤的话她根本走不掉：透镜开着时这一栏是锁住的，而「往下走」只改了
			// 任务状态，屏幕上那副透镜还在，她还是只能对着它。
			//
			// 实测那一幕：印记 在一篇打仗救援的新闻上召了「科学方法论」的透镜，
			// 一遍遍要她找「样本不够、测量有偏差」的句子。她连着说了三遍
			// 「这篇根本没有做实验」，越说越烦 —— 而那样的句子确实不存在。
			// 一副套不上这篇文章的透镜，硬耗下去只会把她耗走。
			if current.Kind == string(taskLens) {
				for _, c := range cardRows {
					if c.Status == "proposed" || c.Status == "active" {
						if _, err := qtx.UpdateAtomCardStatus(turnCtx, sqlc.UpdateAtomCardStatusParams{ID: c.ID, Status: "skipped"}); err != nil {
							slog.Warn("reading coach: could not retire the stalled lens",
								"err", err, "atom_id", at.ID)
						}
					}
				}
			}
			slog.Info("reading coach: step stalled, advancing for her",
				"atom_id", at.ID, "kind", current.Kind)
			advance = "done"
		}
		if advance != "" {
			if _, err := qtx.SetReadingTaskStatus(turnCtx, sqlc.SetReadingTaskStatusParams{
				AtomID: at.ID, ID: current.ID, Status: advance,
			}); err != nil {
				httpx.WriteError(w, r, err)
				return
			}
		}
	}
	if err := tx.Commit(turnCtx); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	after, err := a.d.Queries.ListReadingTasks(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	next := currentReadingTask(after)
	currentID := ""
	if next != nil {
		currentID = next.ID.String()
	}

	var cardOut *cardDTO
	nudge := ""
	if parsed.Lens != "" {
		// 铁律④ — origin is 'router': SHE did not pick this lens, and the
		// autonomy signal on the row must say so. Failure is silent: the
		// coach's words still stand, she simply doesn't get the instrument.
		lres, serr := a.summonReadingLens(turnCtx, u.ID, at.ID, parsed.Lens, parsed.FocusBlock, cardOriginRouter)
		if serr != nil {
			slog.Info("lite coach: lens summon failed; turn stands without it",
				"atom_id", at.ID, "card_id", parsed.Lens,
				"request_id", httpx.RequestIDFromContext(r.Context()), "err", serr)
		} else if lres.Card != nil {
			cardOut, nudge = lres.Card, lres.Nudge
		}
	}

	// F4: the card and the scroll must agree. When a lens actually minted this
	// turn, it is aimed at parsed.FocusBlock (parseReadingCoachReply requires
	// that pairing), and the response's focusBlock must stay there — not jump
	// to the NEXT step's paragraph, which is a different one on any turn that
	// both advances into a focus_block step and summons a lens. Only absent a
	// minted lens does the plan's own paragraph win over the model's guess:
	// the plan already decided which paragraph the next step is about, and
	// letting a per-turn guess override it would scroll her somewhere the step
	// never meant.
	focus := parsed.FocusBlock
	if cardOut == nil && next != nil && next.BlockID != "" {
		focus = next.BlockID
	}

	resp := map[string]any{
		"reply":         parsed.Reply,
		"tasks":         readingTaskDTOs(after),
		"currentTaskId": currentID,
		"focusBlock":    focus,
		"tool":          parsed.Tool,
		"finished":      next == nil,
	}
	// The model's thinking for this turn, shown FOLDED next to the reply so she
	// can open it if she wants to see how the thing asking her questions got to
	// this one. Absent whenever the class this call routes to has thinking off,
	// and the client then renders no fold at all — an empty fold reads as "it
	// did not think" when the truth is "there was nothing to read".
	//
	// Never persisted: it is not written into the transcript this turn is saved
	// into, so a refresh loses it. The model's scratch work is not her record;
	// the process tree is.
	if t := strings.TrimSpace(res.Reasoning); t != "" {
		resp["thinking"] = t
	}
	if cardOut != nil {
		resp["card"] = cardOut
		resp["nudge"] = nudge
	}
	// 🚨 NOT "card". That key above is the lens card — a different thing with
	// a different lifetime (a row in atom_card, opened and closed) — and the
	// two would silently overwrite each other on any turn that produced both.
	if parsed.Card != nil {
		resp["coachCard"] = parsed.Card
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// replyQuotesBlock —— 这一轮的话里，有没有一段逐字来自那一段的原文。
//
// 用来判「它是真的做了一遍示范，还是只说了这套看法的名字」。示范一定引原句
// （prompt 要求挑出那一段里的某一句当着她的面分析），空谈一定不引。
//
// 窗口沿用 replyMentionsSentence 那一套：中文八个字、英文二十个字母。
func replyQuotesBlock(reply string, blocks []Block, blockID string) bool {
	if reply == "" || blockID == "" {
		return false
	}
	for _, b := range blocks {
		if b.ID != blockID {
			continue
		}
		for _, sent := range splitSentences(b.Text) {
			if replyMentionsSentence(reply, sent) {
				return true
			}
		}
		return false
	}
	return false
}

// replyEndsOnAQuestion —— 这句话是不是以一个问句收尾。
//
// 只看**最后一句**：中间出现问号是正常的（「这句在问什么？它在说成本」这种
// 自问自答是讲解的一部分），收尾那句才决定她接下来伸手去做什么。
func replyEndsOnAQuestion(reply string) bool {
	r := []rune(strings.TrimSpace(reply))
	if len(r) == 0 {
		return false
	}
	// 收尾的引号、括号不算数，往回找到真正的最后一个字。
	for len(r) > 0 && strings.ContainsRune("」』）)\"'“”", r[len(r)-1]) {
		r = r[:len(r)-1]
	}
	if len(r) == 0 {
		return false
	}
	return r[len(r)-1] == '？' || r[len(r)-1] == '?'
}

// lastBoardPlacement —— 她最近一次摆完的板：每一句现在在哪一格。
//
// 读的是她那条消息里那份原样的作答（composeBoardAnswer 写的格式）：
// 一行「格子名：」，下一行是那句原文。
func lastBoardPlacement(msgs []sqlc.AtomMessage) map[string]string {
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role != "student" {
			continue
		}
		ans := coachAnswerFromPayload(m.Payload)
		if ans == nil || ans.Type != coachCardLabelRoles {
			continue
		}
		out := map[string]string{}
		lines := strings.Split(ans.Choice, "\n")
		for j := 0; j+1 < len(lines); j++ {
			bin := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(lines[j]), "："))
			if !isRoleLabel(bin) {
				continue
			}
			if sent := strings.TrimSpace(lines[j+1]); sent != "" {
				out[sent] = bin
			}
		}
		return out
	}
	return nil
}

// coachMoveVerbs —— 「把它挪过去」的说法。
var coachMoveVerbs = []string{"拖到", "拖进", "挪到", "挪进", "移到", "移进", "放进", "放到", "改放", "换到"}

// replyAsksForANoOpMove —— 这句话是不是在让她把一张卡挪到它已经在的那一格。
func replyAsksForANoOpMove(reply string, placed map[string]string) bool {
	if reply == "" || len(placed) == 0 {
		return false
	}
	moves := false
	for _, v := range coachMoveVerbs {
		if strings.Contains(reply, v) {
			moves = true
			break
		}
	}
	if !moves {
		return false
	}
	for sent, bin := range placed {
		if strings.Contains(reply, bin) && replyMentionsSentence(reply, sent) {
			return true
		}
	}
	return false
}
