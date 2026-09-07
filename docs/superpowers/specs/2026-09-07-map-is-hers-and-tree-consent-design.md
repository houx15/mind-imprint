# 探索地图改成她自己的，兴趣树改成她点过头的

2026-09-07。产品负责人一次提了九件（八条 + 一条 bug 追报）。这份文档记的是每一件
背后的判断，不是操作步骤。

接着 `2026-09-05-exploration-map-and-recommendations-design.md`——那一轮加的推荐层，
这一轮撤掉了，理由见 §6。

---

## 1 · 星球上的 1–5 角标删掉

> we have numbers: 1, 2, 3, 4, 5, which is no.x -> but that looks like new
> message notification so leads to misunderstandings. delete that.

一个圆角框里的小数字挂在球的右上角，是十五年互联网教出来的一个固定读法：未读。
排序本身没有丢——它就是球的**大小和位置**（`PLANET_SLOTS`），一个视觉上已经说清
的事实不需要再写一个数字。面板头上的「NO.3」一起删掉，它是同一件事。

## 2 · 钩子问的是这条新闻自己，不是下一件事

她给了一组对照。原来那条：

> AI 能验证已有证明，那它能独立发现一个全新的数学定理吗？

她想要的：

> 为什么AI可以做到人类数学家做不到的事情？这件事代表着AI的智力超越了全人类了嘛？

差别不在语气，在**打向哪里**。前者把这条新闻当跳板，问的是它之后的事；后者打在
这条新闻自己的说法上——它凭什么这么说，这个说法能推到多远。前者看着像个问题，
其实让她跳过了眼前这条。

prompt 现在给四种问法，每种带一对好/差例句：结论撑不撑得住、这个词是什么意思、
这一步能推多远、是谁在说怎么算的。**并且把「必须是个问题」写成了代码里的判据**
（`isQuestion`，认中英文问号）——memory 的 `prompt-output-must-be-verifiable`：
一条只能在 prompt 里写、代码里验不了的规矩，实测下来迟早被悄悄破掉，而破掉的样子
是「它想问你」下面摆着一句陈述。

### 2b · 🚨 钩子写长了，整天的星图就没了

实测（`LIVE_LLM=1 ... TestLivePromptStarmap`）：新钩子的内容完全对，但模型写得长，
回复在第五颗星中间断掉。`sliceJSONObject` 对截断的输入报错，于是**当天一颗星都
没有**——星图一天只生成一次，代价是一整天的探索地图空着。第一次跑本地栈时那一屏
就是空的，正是这个原因。

两头都修：

- prompt 里把 hook 压到 45 字以内（她给的例句本来就是这个长度）；
- `salvagePlanets` 用流式解码逐颗读，**断在哪算到哪**。四颗真的星，好过零颗。

`stop="stop"` 的那次回复完整、五颗齐全，所以截断是模型的间歇行为，不是固定上限。
正因为它是间歇的，兜底必须存在。

## 3 · 「读原文」改成「现在读 / 稍后读」

> what I really hope is to read in our platform. but I understand that, on our
> platform, it is difficult to fetch the original content. then, maybe we can
> jump to the reading room, and also open a new page, and invite students to
> paste here?

两个动作都在阅读室建**同一篇**（服务端幂等，`news_saved.reading_id`，迁移 0138）：

- **现在读** —— 原文另开一页，同时进阅读室。我们先替她试一次服务端抓正文
  （`putReadingSource({url})`，那条路本来就有，失败时干净地报错、不存垃圾）。
  抓到了她就在我们这儿读；抓不到，阅读室摆出粘贴框，而她要复制的那一页已经开在
  旁边。
- **稍后读** —— 只建那一篇，她留在地图上。

幂等是必须的：先「稍后读」再「现在读」必须落在同一篇上，否则阅读室里会攒下两篇
同名的、其中一篇再也找不到。（同一个教训在 `ReadingsLanding.handleRecommendation`
上已经付过一次学费。）

## 4 · 抽屉压在切换器下面 —— 根因不是 z-index

> after clicking one node, the sidebar appears, but its layer is under the
> switcher, looks a little strange.

抽屉是 `z-50`，切换器是 `z-30`，看起来该赢。但 `.exp-sky` 上有
`isolation: isolate`（星层要靠它才不漏出去），它**建立了一个层叠上下文**，于是
抽屉的 z-50 只在地图内部有效，整棵子树按父级的层级去画——排在切换器后面。

所以修的不是数字，是位置：`Drawer` 用 portal 挂到 `document.body`。以后谁在哪一屏
加 `isolation` / `transform` / `filter` 都不会再把它压下去。

### 4b · 🚨 portal 带来的回归：七个颜色变量丢了

抽屉一挂到 body 就**离开了 `.exp-sky` / `.tree-grove` 的作用域**，而七根主枝的颜色
是定义在那两个选择器上的（有意为之：别的界面没有「学科主枝」这个概念）。失效不
报错，只是一声不响地少颜色——主枝色的点没了、「它想问你」的描边没了、「现在读」
变成深色底上的深色块。**截图里一清二楚，代码里毫无痕迹。**

`branchHues.css` 一份定义管三个作用域（`.tree-grove` / `.exp-sky` /
`.mk-branch-hues`），抽屉自己带上第三个类。原来 tree.css 和 explore.css 各抄一份，
现在一份。

## 5 · 地图上不再种词；词长在读完之后

> we don't ask students to 收进我的树 here. we only invite students to read now,
> or read later. after reading finished, we would get a report, on that we can
> propose several keywords, that students can agree to add to their tree
> (similar in writing, do you think this would be better?)

**是更好，做了。** 理由不止「顺序更对」：这棵树的整个说法是「这就是你的模型」，
而一个她没点过头的模型只是我们对她的记录。树上那个「你凭什么这么说我」的问题，
从这一步开始有了她自己给的答案。

采集因此分两路（`landHarvest`，迁移 0140）：

- **她树上已经有的词** —— 照旧直接种。那是给一个已经认过的词再添一条来源，
  不是一个新说法。为一个三个月前就认过的词再问一次同意，是把同意变成打卡。
- **新词** —— 进 `interest_proposal`，摆在报告上等她认。

只有阅读和写作走这条路：它们有报告，而报告是候选词唯一有地方待的位置。项目和兴趣
测试照旧直接种——**兴趣测试本身就是她在挑词**，那一步已经是同意了。

拒绝也落库（`accepted=false`），不是丢掉：铁律④，她拒了什么和她认了什么一样是过程
数据；而且重跑采集不会把同一个词再问一遍。每一条候选都带着**她自己写的那一句**
（evidence）——一个只写「电池」的候选，她没有办法判断该不该认。

## 6 · 外圈换成她自己的词

> I have a point under the five big balls, one is 手机 … is it a recommended
> keyword? then we don't need to recommend keyword here. we just show how the
> five dots connected with students' already existed nodes. click node can show
> that node's learning history? when added? etc.

她读不出那一圈是什么，而且理由比「读不懂」更硬：**一颗推荐星是我们对她的一个猜测，
它长得和她自己的词一模一样、摆在同一圈上**。这张图于是同时在说两件事（这是你的 /
这是我们猜的），却没有任何东西把两者分开。

现在外圈只有一种东西：她的。点开一颗，打开的是**树上那一屏的同一个抽屉**
（`KeywordDrawer`）——它是什么、第一次出现是哪天、由哪几件事长出来、接下来能挖
什么。在地图上另写一份「这个词是什么」的面板，等于同一个东西有两个说法，而其中
一个迟早会说错。

`recommend.ts` / `RecSheet.tsx` / 迁移 0137 的 `interest_dismissal` 全部删掉。
那一轮学到的东西（大学科要折价，不然「电池 + 太阳能」第一名是咖啡）记在
memory 里，代码不留。

### 6b · 🚨 只认「同一门学科」时，实际一条线都没有

第一版的连线判据是「共用一门学科」或「同一个领域」。真数据下打开地图：**零条线**。
42 门学科铺得很开，那天五条新闻挂 logic-proof / anthropology / genetics /
public-health，她那六个词扎在 energy-systems / media-studies / statistics 上，
一处都不重合。而顶上还写着「连线是它们和今天这五条的关系」。

加第三档：**同一根主枝**。最弱，但仍然是真的，而且它就是两屏共用的那套颜色——
一条连到同色词的线不需要解释。三档按强度排，每颗星球最多三条（一门大学科会让
一颗星球连上她半棵树），同强度取近的。

顺带把那句说明改成条件的：只在真的有线时才说有线。

## 7 · 成长轴按她真实待了多久长出来

> we don't need to always show a 起点-半年前-近2个月-现在 timeline. the timeline
> should not be fixed, it should be a relative one. for a new student, we don't
> have that line. but when they came after some time, the line gradually becomes
> longer. until this four-dot version.

上一版**永远是四格**，标签写死成「起点 · 半年前 · 近两个月 · 现在」，只有位置是
相对的。一个上周才开始的学生因此看到一条写着「半年前」的轴——那六个月不存在。
（这一版本身就是上一次修「绝对月份」时留下的半步：位置改对了，标签没改。）

现在刻度数是跨度的函数：三周以内 1 格（界面据此**不画这条轴**）→ 三个月内 2 格
→ 八个月内 3 格 → 再往上 4 格。中间那几格的字由真实时间算（「3 个月前」），
副标题是**那一格对应的日期**。

原来的副标题是「读得多起来」「开始写」——我们并不知道她那两个月在做什么，那两句
是替她写的故事（界面文案 §10）。

抽屉里的「出现于」也从那个会变的格名换成了**第一次出现的日期**：格名随她继续用
还会变（同一个词今天写「4 个月前」，下个月写「5 个月前」），而「什么时候加进来的」
只有一个答案。

## 8 · 深挖的四颗种子要能让她的看法变

> similar for recommending tasks in interest tree -> leads to deep thinking or
> critical thinking.

四颗种子原来只要求「具体、扣住她的原话」。具体是够了，但一颗具体的种子照样可以
只让她把已经知道的再说一遍——「电池还有哪些应用」是具体的，也是无用的。

判据改成一条：**做完这一颗，她对这个词的看法有可能变。** 每一种给一个方向：
想一想指着她原话里的一个前提；去读换一个可能不同意她的立场；去写要求一个她得
辩护的判断；去做要产出一份能拿去检验她想法的证据。

和 §2 是同一条：问题要打在这个说法上，不是把它当跳板。

## 9 · 课程页出不去（她追报的 bug）

> I click 我的主页 but jumps to courses page … oh no, it is when I clicked course
> page, I clicked any tab, I will go to course page again.

`CoursesContainer` 在**卸载时**调 `onActiveCourseChange(null)`，而 lite 把它接到了
「回到课程目录」上。于是她在课程页点任意别的 tab：导航先把 URL 推到 `/site`，
课程页随即卸载、把 `/courses` 又推回来。从她那边看是「课程页出不去了」。

判据必须读**地址栏当下的路径**，不是 render 闭包里的 `route`：卸载时跑的是上一次
提交留下的那个回调，它闭包里的 `route` 还停在 courses，判不出来。

---

## 这一轮学到的

三个 bug 是**只有真浏览器才看得见**的（AGENTS.md：UI 用真浏览器看）：

1. 空星图（钩子写长 → 回复截断 → 当天没有地图）；
2. 一条连线都没有（学科对不上，而文案在承诺有线）；
3. 抽屉丢了七个颜色变量（portal 出了作用域）。

三个都不会让任何一个测试变红，代码上也看不出来。而**只有第 3 个是新引入的**——
另外两个是「看起来应该能用」的设计在真数据上的第一次曝光。
