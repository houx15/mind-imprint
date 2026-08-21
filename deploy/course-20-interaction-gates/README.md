# course-20 · 两个写作交互的「完成」门槛修复

**症状（2026-08-21，Phoebe 账号实测）：** 课程《生成式 AI 时代的多模态信息甄别与可信判断》
第 8 屏（`emotional-image`）——选择题答完后消失、写作交互出现，**认真写完两栏文字之后
「完成任务」仍然是灰的，「下一步」也点不动，学生被卡死在这一屏**。

## 根因

这一屏的 `interactiveHtml` 交互 `emotion-pause-writer.html` 把「完成任务」按钮的可用性
挂在了两条**作者预设的关键词正则**上：

```js
const emotionCue  = /同情|担心|担忧|难过|震惊|触动|害怕|感动|情绪|转发冲动|想转发/;
const evidenceCue = /来源|原新闻|原始发布|发布者|反向搜图|Google\s*Images|时间|地点|EXIF|…/;
complete.disabled = ts.some((x) => !x[1]);   // 命不中 → 永远灰
```

学生写「我第一反应是**心疼**这个小女孩，很想马上转出去」——真实、具体、完全切题——
但 `心疼` 不在词表里，于是按钮永远不亮。该 Slice 的 `navigation.manualNext` 是
`after-completion`，Slice 未完成 → 「下一步」也是禁用的，**没有任何出口**。

同一门课第 13 屏（`image-forensics-summary`）的 `image-evidence-writer.html` 有**完全相同**
的陷阱：`外部核查动作` 一栏必须命中 `externalCues`，「去 Google 上找找这张图最早出现在哪」
一个词都不匹配。

这不是渲染器的问题——宿主对作者写的 HTML 无能为力（这是**唯一**运行课程作者代码的地方，
沙箱之外宿主不介入它的按钮状态）。修复必须落在资源本身。

全库扫描：44 个 `interactiveHtml` 资源里，只有 course-20 的这两个用关键词正则做**门槛**。

## 改了什么

门槛只看**有没有认真写**，不看**有没有猜中词**：

- 长度 + 非退化（不允许同一个字连刷）+ 两栏不能是同一句话；每条 check 直接写出
  「至少 N 字，已写 M 字」，学生随时知道还差什么。
- 原来的词表**降级为提示**（`提示（不影响提交）：…`），永远不阻断提交。
- 命中与否仍然记录：`completed` payload 里多了 `namedAnEmotion` /
  `namedACheckableAction` / `namedAnExternalEntryPoint` 布尔量——摩擦转成信号，
  而不是转成路障（铁律④），学生也不再被「猜词」俘获（铁律②）。

payload 形状不变（`{resultId, value}`），已对 `HtmlCompletedPayload`（`.strict()`）
校验通过；`ready`/`completed` 握手在浏览器里实测走通。

## 部署

```bash
bash deploy/course-20-interaction-gates/upload.sh            # 上传修复版
bash deploy/course-20-interaction-gates/upload.sh --rollback # 一键回滚到 original/
```

两个文件**沿用原有 object key**，所以 CourseDefinition 不动、definition hash 不变、
学生的课程进度不会被重置。代价是 CDN 可能还存着旧副本——上传后到阿里云 CDN 控制台
「刷新预热 → 目录刷新」刷一次：

```
https://mind-oss.uni-robot.cn/courses/course-20/interactions/html/
```

`original/` 里是逐字节的线上原件，回滚就是把它们再传一次。

## 给课程生成器的提醒

生成 `interactiveHtml` 时，**完成门槛不要依赖关键词匹配**。作者永远列不全学生的说法，
而一个卡住的门槛在 `manualNext: after-completion` 的 Slice 上等于让学生退无可退。
门槛用「写了没有」，词表用来提示、用来记录评估信号。
