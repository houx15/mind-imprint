# 2026-08-23 · 九个「卡住的 Slice」排查

用 Phoebe 账号在线上逐个复现，先判断**每个问题来自课程数据还是运行时机制**，再决定改哪一边。
结论：**四个是运行时机制问题（改代码），五个是课程数据问题（改数据，不给渲染器打补丁去迁就它）。**

| # | 课程 | Slice | 症状 | 归属 | 处理 |
|---|---|---|---|---|---|
| 1 | course-10 可比性检验 | 1 | 做完任务仍不能下一步 | **数据** | `fix_course10_slice1.py` |
| 2 | course-05 一张图四次翻案 | 3 | 不能下一步 | **数据** | `patch_opcvl_lab.py` |
| 3 | course-33 谁为 AI 付了账 | 5 | 过早显示「总结要求已满足」 | **数据** | `source-chain-sort.html` |
| 4 | course-13 威尼斯三选一 | 1 | 不能下一步 | 机制 | 代码（见下） |
| 5 | course-20 多模态信息甄别 | 8 | 不能下一步 | **数据** | 2026-08-21 已备好，从未上传 |
| 6 | course-23 影像侦探局 | 5 | 不能下一步 | 机制 | 代码 |
| 7 | course-04 FLICC | 1 | 不能下一步 | 机制 | 代码 |
| 8 | course-19 Tesla 漂绿 | 1 | 不能下一步 | 机制 | 代码 |
| 9 | course-25 逻辑侦探局 | 4 | 不能纵向滚动、不能占满宽度 | 机制 | 代码 |

---

## 机制问题（已在代码里修掉，随前端发版）

### A. 只读 Slice 永远无法完成 —— #4 #6 #7 #8

`SlicePlayer` 原本在**工作流初始步骤本身就是终止步骤**时，跳过整个 slice 的 start effects：

```js
const startEffects = runtime.start();
if (!runtime.isTerminal) applyEffectsRef.current(startEffects);   // ← 吞掉了唯一的 completeSlice
```

纯阅读页（「看这张图、读这段话」）正是被生成器写成**单步 + `completeSlice`** 的，于是它唯一一次完成信号被吞掉，
`status` 永远不是 `completed`；而这些 slice 的 `manualNext` 是 `after-completion`，**下一步永远是灰的，没有任何出口**。

那个守卫本来是为 §revisit 服务的（回到已完成的 slice 不该重新完成 / 重新跳转），所以判据应该是
**「是不是在恢复」而不是「是不是终止步骤」**：

```js
if (restoreStepId === undefined || !runtime.isTerminal) applyEffectsRef.current(startEffects);
```

静态扫描这四门课，符合该形态的 slice 共 **49 个**（course-04 有 32 个），
首个卡住的位置与报告完全吻合：course-04 #1、course-13 #1、course-19 #1、course-23 **#5**
（course-23 前四个 slice 不是这种形态，所以它是从第 5 个开始卡——这条对得上，是判断正确的关键证据）。

回归测试：`packages/course-renderer/test/readOnlySlice.test.tsx`。

### B. interactive-HTML 被 aspectRatio 卡死 —— #9

`aspectRatio`（`1:1` / `4:3`）原本被当成**硬性尺寸约束**下发：

```css
.course-block--interactive-html { height:100%; width:auto; /* 由比例反推 */ align-self:center; }
```

结果宽版交互在宽 slot 里被压成一根窄柱：1440px 视口下实测**只占 slot 宽度的 81%**（`1:1` 更窄），
而被压出可视区的部分——通常正是交互自己的「完成任务」页脚——**slot 滚不到**（block 已经「装得下」了），
**iframe 也滚不到**。在 `after-completion` 的 slice 上这同样是死局。

现在 `aspectRatio` 只是**作者侧的建议**：它仍作为 `data-aspect-ratio` 保留供样式使用，但宿主对所有取值
一律把整个 slot 交给 iframe（`width:100%; height:100%`），内容超出时由沙箱文档自己纵向滚动。
契约新增 `fill` 作为「没有偏好形状」的显式取值。实测：81% → **99%**。

---

## 数据问题（本目录负责）

### #1 course-10 slice 1 —— 把参考资料面板当成了完成门槛

这一屏并排两个 interactive-HTML：

- 左 `p111-paper-homepages`：A/B 论文截图，**参考资料**。它自己的状态行写着
  「A/B 原文截图和提示已并排显示；**右侧继续作答**」——明确告诉学生答题在右边。
- 右 `p114-opening`：真正的三页任务。

但工作流要求**两个都** `interaction.completed` 才能到 `completeSlice`。学生做完右边点了「完成任务」，
下一步仍然是灰的——因为左边那个毫无提示的「完成」按钮没点。线上实测确认：
只完成右边 → 下一步禁用；两个都完成 → 下一步可用。

改法：完成条件只看真正的任务 `p114-opening`。面板照常可见可用、事件照常上报（过程数据不丢），**只是不再是门槛**。

### #2 course-05 slice 3 —— 用「答对」卡「能不能往下走」

`stepDone` 要求两道选择题**必须选对**，`allDone()` 又是「完成任务」按钮的开关，
而该按钮是这个 block 唯一的 `interaction.completed` 来源：

```js
const choiceDone = !step.choice || ans.choice === step.choice.correct;   // 选错 → 永远不算完成
```

改成**「答了没有」**。正确与否照常判定、照常在每步反馈里告诉学生，也照常写进 `completed` payload
（`isCorrect` / `correctAnswer` 不变），**摩擦转成信号，而不是路障**（铁律④）——
与 2026-08-21 course-20 的修法一致。

> 只提交打好补丁的文件（它内嵌 2.5 MB 图像，再存一份原件不划算）。
> 补丁是单处精确字符串替换，`--reverse` 能从提交的文件**逐字节**还原线上原件（已验证）。

### #3 course-33 slice 5 —— 提示语报了一个「已经满足」的条件

`updateSummaryGuidance()` 只看总结文本，不看分类：

```js
function updateSummaryGuidance(){
  if(!state.checked)return;
  if(summaryState().ok){ feedback.innerHTML="<strong>总结要求已满足。</strong> 现在可以点击“完成任务”。"; return; }
  ...
}
```

而 `updateFinish()` 真正的门槛是 `state.checked && 全部分类 && 关键卡全对 && summaryOk()`。
于是关键卡还分错的学生，只要总结写够字数和关键词，就会看到
**「总结要求已满足。现在可以点击『完成任务』」——而『完成任务』其实是灰的**：
提示语报的是**已经满足的那个条件**，真正挡着他的那个（分类）屏幕上根本没人说。

改成和 `updateFinish()` 同一个条件，逐条说明还差什么；「检查分类」自己那句
「关键卡 X/Y 正确」保留（`updateSummaryGuidance(true)`），状态行仍然列出待办。

### #5 course-20 slice 8 —— 关键词正则门槛

2026-08-21 已完整诊断并写好修复（`deploy/course-20-interaction-gates/`），
但**从未上传**：线上文件与该目录下的 `original/` **逐字节相同**。这次一并发出去。

---

## 发布

```bash
bash deploy/course-gate-fixes-2026-08-23/upload.sh            # 发布
bash deploy/course-gate-fixes-2026-08-23/upload.sh --rollback # 回滚（course-10 需手动，见下）
```

course-33 / course-05 / course-20 **沿用原 object key**，所以 CourseDefinition 不动、definition hash 不变、
学生进度不会被重置。

> ⚠️ **上传本身不会让学生看到新版本，必须刷 CDN。**
> `mind-oss.uni-robot.cn` 实测 `X-Swift-CacheTime: 2592000`（**30 天**）。
> 2026-08-23 上传后立刻回查，边缘仍是 `X-Cache: HIT`、`Content-Length: 14506`（旧版；新版 15636）。
> 也就是说：**不刷新，改动最长一个月都到不了学生手里。**

```
https://mind-oss.uni-robot.cn/courses/course-33/interactions/html/
https://mind-oss.uni-robot.cn/courses/course-05/interactions/html/
https://mind-oss.uni-robot.cn/courses/course-20/interactions/html/
```

`cdn_refresh.py` 封装了 CDN `RefreshObjectCaches`（签名已验证可用），但
**`OSS_ACCESS_KEY_ID` 对应的 RAM 用户没有 CDN 权限**，实测返回：

```
Forbidden.RAM — User not authorized to operate on the specified resource
```

所以目前只能走控制台「CDN → 刷新预热 → 目录刷新」。
若之后给这个 RAM 用户加上 `AliyunCDNFullAccess`（或仅 `cdn:RefreshObjectCaches`），
`upload.sh` 之后直接 `python3 deploy/course-gate-fixes-2026-08-23/cdn_refresh.py` 即可自动刷新。

course-10 改的是 definition（走 API，不经 CDN），**不需要刷新**。

**course-10 是例外**：它改的是 definition 本身，**hash 会变，course-10 的进行中会话会被重置**。
`--rollback` 不会自动回滚它——需要手动把原 workflow 再 PUT 回去
（原始 workflow 见本文件 git 历史与 `fix_course10_slice1.py` 的 `--dry-run` 输出）。

## 给课程生成器的提醒

三条反复出现的坑，都在于**把「学得对不对」当成了「能不能往下走」**：

1. **完成门槛不要依赖关键词匹配或选对答案。** 作者永远列不全学生的说法。门槛用「做了没有」，
   正确与否用来提示、用来记录评估信号。
2. **参考资料不要当门槛。** 一个没有提示的「完成」按钮，学生不会知道要点。
   若某个 block 只是资料，就不要把它写进完成条件。
3. **提示语必须和真正的门槛同源。** 报一个已经满足的条件，比不报更糟——
   学生会以为自己做完了，却发现按钮是灰的。
