# 2026-08-23（第二批）· 课程「卡住 / 显示不全」批量排查

九条用户报告，全部复现并定位。和上一批一样，**先判断每个问题来自课程数据还是运行时机制**再决定改哪一边。
这一批**九个全部是课程数据问题**——运行时没有新 bug，昨天那两个运行时修复已经在线上生效
（线上 CSS 已含 `align-self:stretch`，已核对）。

| # | 课程 | 位置 | 症状 | 根因 | 处理 |
|---|---|---|---|---|---|
| 1 | course-19 Tesla 漂绿 | slice 17 | 什么都不显示 | 交互自己缩放，内容跑出画面左上角 | 资产 A |
| 2 | course-25 逻辑侦探局 | slice 5 | 过不去 | 旁白门 + 55 秒隐藏计时 | 定义 T5 |
| 3 | course-26 语言塑造思维 | slice 4 | HTML 点不动；退出无进度；再进就能点 | 交互初始 `enabled: []`，旁白结束才启用 | 定义 T4 |
| 4 | course-26 | slice 5 / 进度 14% | 过不去；7 步走到第 5 步只有 14% | 同上 + 进度按「已完成」计数 | 定义 T5 |
| 5 | course-06 事实观点价值 | slice 4 | 右侧 HTML 不能滚动 | 交互自锁 4:3 + `overflow:hidden`，750px 只用 386px | 资产 B |
| 6 | course-11 哥伦布 | slice 5 | 太矮，占不满右半边 | 同 5 | 资产 B |
| 7 | course-23 影像侦探局 | slice 17 | 拖到最后、答了题仍过不去 | 80 分钟影片，必须真的触发 `ended` + 两个必答 cue | 定义 T6 |
| 8 | course-04 FLICC | slice 18 | 「提交」点不动 | 自由填空 `submit-correct`：答对才算完成 | 定义 T2 |
| 9 | course-30 提示词 | slice 6 | 下一步永远灰的 | **答题顺序陷阱**（已线上复现） | 定义 T3 |

course-17（数据分析）slice 9/10 的改版请求见最后一节，**未包含在本批**。

---

## 一、资产问题：交互自己重新规定了画框

宿主早就把**整个 slot** 交给 `interactiveHtml`，并允许沙箱文档自己滚动
（`packages/course-renderer/src/styles/course.css`：`aspectRatio` 只是作者侧建议）。
这些交互把它丢掉，自己又套了一个固定画框，于是出现两种失败：

### A · SCALE-CROP（course-19 五个交互）

```css
body{display:flex;align-items:center;justify-content:center}   /* 居中的是变换前的盒子 */
#stage{width:1024px;height:768px;transform-origin:top left}    /* 然后从左上角缩小 */
```

flex 居中的是**变换前**的 1024×768，缩放再把它从那个中心往左上拖走。
`s < 1` 时内容就跑出画面，而 body 被 JS 设成缩放后的尺寸、又是 `overflow-x:hidden`，
**没有任何东西可以滚回去**。实测（slice 17 的真实画框）：

| 窗口 | 画框 | 缩放 | 左上角被切掉 | 学生看到 |
|---|---|---|---|---|
| 1440×900 | 1078×750 | 0.98 | 12 / 9 px | 正常 |
| 1280×720 | 918×570 | 0.74 | 132 / 99 px | 标题没了，「可以直接说」整列被切掉 |
| 1200×520 | 918×370 | 0.48 | 245 / 184 px | **几乎全空**，三个按钮都不在 |

**改法**：居中**缩放后**的footprint。`html` 变成居中容器，`body` 就是缩放后的盒子
（尺寸本来就由交互自己的 `fit()` 设定），`#stage` 钉在原点，于是从左上角缩放就是从 0,0 开始。

### B · RATIO-BOX（其余 19 个）

`#stage` / `.canvas` 自锁 `aspect-ratio` 并 `overflow:hidden`。
slot 形状和这个比例不一样时，交互就把自己装进信箱框：**750px 的槽只用 386px**，
超出比例框的内容被裁掉且滚不到。course-08 更糟——比例框比画框还高，
被 flex 居中顶出上边缘，`overflow:hidden` 让那部分永远够不着。

**改法**：让 wrapper 填满拿到的 slot，去掉比例钳制，超出就滚动而不是裁掉。

两种补丁都是**只追加**：在 `</body>` 前插入一个带标记的 `<style>`，靠源码顺序取胜，
`--revert` 能精确移除。作者写的结构和逻辑一个字都没动。

### 验证方式（客观，不靠肉眼）

`.tmp-measure.mjs` 用 Playwright 把每个交互放进宿主真正会给的画框里量：

- **OFFSCREEN**：有元素在画框原点的上方/左方。滚动偏移不可能为负，那些像素就是够不着。
- **LETTERBOX**：最外层 wrapper 高度远小于画框，而内部又有区域在溢出。

补丁前：**79 个交互里 35 个有够不着 / 信箱框的内容**（OFFSCREEN 10、LETTERBOX 25）。
补丁后：本目录 24 个文件在它们**实际被渲染的 slot 尺寸**下全部干净。
（course-19 那 5 个在假想的窄 slot 下仍会信箱框，但它们全部用 `full` 布局，线上不存在那种情形。）

25 个 LETTERBOX 里只有 **14 个真的落在窄 slot**（`split-*` / `grid`），其余在 `full` slot 里
比例和画框基本一致，**没有动**——避免为了一个正则就改 54 个文件。

---

## 二、定义问题：会把学生永久卡死的门

`fix_gates.py` 六个变换。全部只改**什么东西挡路**，不改教学内容：
对错照常判、照常反馈、照常记进 payload（铁律④：摩擦转成信号，不是路障）。

- **T1 `answer.correct` → `block.completed`**
  只在「答对」时才走的 transition，会把答错的学生留在原地：block 已锁定或已用尽次数，
  `answer.correct` 永远不来，而这一屏没有别的出口。
- **T2 `submit-correct` → `submit-correct-or-exhausted(2)` / `submit-any`**
  `submit-correct` 是「无限次尝试，但答对之前永不完成」（见渲染器 `evaluateSubmission`）。
  **落在自由填空上基本无解**——course-04 slice 18 就是这个。
- **T3 顺序陷阱 → 与顺序无关的门**
  `wait-A -> wait-B -> finish`，但 A、B 从一开始就都能答。**先答 B**：B 完成并**锁定**
  （`submit-any` 完成即锁），工作流这时还在等 A；等它走到 wait-B，B 的事件再也不会来了。
  **已在线上复现**（course-30 slice 6：两题都答完、两个「提交」都变灰、下一步仍然是灰的）。
  改成对这些 block 的**积集自动机**，任何作答顺序都到同一个 finish。
- **T4 `enabled: []` + 旁白门 → 进来就能点**
  交互在屏幕上但是死的，直到 ~45 秒旁白放完。学生点了没反应就走了。
  旁白照放，只是不再卡着 block。
- **T5 「只能等」→ 进入即完成**
  `narration.ended -> timer.elapsed(30–80s) -> completeSlice`：一屏纯阅读，配一个看不见的倒计时，
  下一步一直是灰的、也不说为什么。改成进入即完成的纯阅读页
  （运行时支持初始步骤即终止步骤——昨天那个修复），下一步立刻可用，学生想读多久读多久。
- **T6 影片必须看到底 → 增加一个明确的学生出口**
  `video-ended-and-interactions-completed` 要求真的触发 `ended` 且清掉每个必答 cue。
  course-23 那部 **80 分钟**影片，cue 在 6:47 / 7:59，拖过去就再也够不着。
  原有的两条路都保留，另外加一个 `student.continue`。

### 影响范围是刻意收窄的

**改定义会改变 content hash，从而重置该课程所有进行中的会话。** 所以：

- **T1/T2/T3/T6（会永久卡死的）**：在扫描发现的每一门已发布课程上都修。
  真被卡住的人本来也没有进度可丢，而且他们的会话还引用着被删掉的 step id——重置是必需的，不是代价。
- **T4/T5（判断题，最终会自己解开）**：**只**改这两门被报告的课（course-25 / course-26），
  而不是同样有这个形态的全部 31 门。

最终：**12 门课、77 处**（不设策略时是 31 门、144 处）。

### 验证

`packages/course-contract/test/tmp-patched-definitions.test.ts` 用项目自己的
`validateCourseDefinition`（structural + referential + quality + workflow-graph）
逐份校验改写后的定义，并把改写前的原件作为对照跑一遍。**25 个用例全过。**

---

## 发布

```bash
# 1. 资产（沿用原 object key：不动 definition、不改 hash、不重置进度）
python3 deploy/course-layout-gate-fixes-2026-08-23/upload_assets.py --apply

# 2. CDN 目录刷新 —— 上传不等于发布，边缘缓存 30 天
python3 deploy/course-gate-fixes-2026-08-23/cdn_refresh.py \
  https://mind-oss.uni-robot.cn/courses/course-03/interactions/html/ \
  https://mind-oss.uni-robot.cn/courses/course-06/interactions/html/ \
  https://mind-oss.uni-robot.cn/courses/course-08/interactions/html/ \
  https://mind-oss.uni-robot.cn/courses/course-11/interactions/html/ \
  https://mind-oss.uni-robot.cn/courses/course-19/interactions/html/ \
  https://mind-oss.uni-robot.cn/courses/course-24/interactions/html/ \
  https://mind-oss.uni-robot.cn/courses/follow-the-money-fossil-fuel/interactions/html/

# 3. 定义（⚠️ 会重置这 12 门课的进行中会话）
python3 deploy/course-layout-gate-fixes-2026-08-23/fix_gates.py --apply
```

回滚：

```bash
python3 deploy/course-layout-gate-fixes-2026-08-23/patch_layout.py --revert   # 再跑一次 upload_assets.py --apply
# 定义回滚：definitions/<slug>.before.json 就是改写前的原件，PUT 回去即可
```

---

## 给课程生成器的提醒（接昨天那三条）

4. **交互不要自己实现视口缩放，也不要自己重新规定画框。**
   宿主已经把整个 slot 给它并允许滚动；`transform: scale()` 从左上角缩放配上外层居中，
   或者自锁 `aspect-ratio` + `overflow:hidden`，只会和宿主打架——本批 79 个交互里 35 个中招。
5. **不要用「答对」当放行条件**，尤其是自由填空（`submit-correct`）。用 `submit-any` 或
   `submit-correct-or-exhausted`，把对错留给反馈和评估信号。
6. **同一屏有多个可答 block 时，不要写成串行的 `block.completed` 链。**
   学生不会按作者想的顺序作答，而 block 完成即锁定——先答后面那个就是死局。
7. **不要把一屏的唯一出口挂在旁白或隐藏计时器上。** 纯阅读页就让它进入即完成。
8. **不要让学生被一段视频扣住。** 必须看完才能走的设计，至少留一个看得见的出口。

---

## 未包含：course-17（数据分析）slice 9 / 10 改版

现状：slice 9 和 slice 10 都是一个 `grid` 布局里竖着堆三块——
richText 卡片、**两张** PPT 尺寸大图（`slide-14/15`、`slide-16/17`）、一道三选一。

用户要求：拆成三屏——第一屏一张图，第二屏另一张图，第三屏左 richText 右选项；
并且因为图是 PPT 尺寸，选项应当**以模态弹出**。

阻塞点：**运行时目前没有「把某个 block 渲染成模态」这个作者层能力。**
`InteractionModal` / `PdfModal` / 图片 lightbox 都存在，但都是渲染器内部行为，
`BlockDefinition` 上没有 `presentation: "modal"` 之类的字段。所以这一项需要
**契约 + 渲染器 + CSS + 测试**的改动，不是纯数据改动——已单独提出，等确认范围后再做。
