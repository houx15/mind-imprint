# 课程体验重构探索 · 语音讲解 + 幻灯片式 (Narrated Slide Course)

> **状态：探索中，未定稿、未开工。** 这是一次设计对话的记录，供以后 resume。
> 触发：Owner 认为当前「竖向滚动、点击展开」的课程体验不够好，希望改成
> **真·AI 语音讲解 + 幻灯片式**（slide-feeling）的体验：图片/视频素材与讲解对齐、
> 文字随讲解高亮等。下一步不是马上做，而是先记录，择时再正式 brainstorm → spec → build。

---

## 1. 愿景（Owner 的想法）

把课程从「一页竖着滑、点一下出一段」变成一段**被讲出来的课**：

- **真实的 AI 语音讲解**（TTS 朗读课程内容），一张张幻灯片推进；
- 素材（图/视频）与讲解**时间对齐**——讲到哪，画面/图就到哪；
- 文字随讲解**高亮**（karaoke 式或分点浮现）；
- 整体更像一段有节奏的「讲解片」，而不是一篇长文档。

---

## 2. 核心判断：这是「内容模型」的更替，不是加字段

- **当前是「文档模型」**：render-cache 描述*有哪些文字块*（`segments[]` + 内联
  `asset_ids`），前端竖向 timeline、点击 reveal。
- **目标是「时序场景模型」**：一串**幻灯片（slide/scene）**，每张 = 一个视觉构图 +
  一条**讲解旁白（narration）**轨道，配合**时间线上的 cue**触发视觉变化（浮现某个
  bullet、高亮某段短语、平移/放大某张图、播放某段视频）。

> 结论：当前 JSON「不够」不是缺某个字段，而是缺了**一整个内容类型**。要新增一个
> 「lesson / deck」发布物，与 render-cache 并列（或最终取代它），而不是给 render-cache
> 打补丁。

---

## 3. 需要先达成的共识：与「四条铁律」的张力（重要）

产品铁律：**AI 克制、不啰嗦、一次只问一个、绝不替学生定论。** 一段全自动播放的
「AI 讲课」会把课程拉向**教学视频**那一极——正好是「陪练克制」的反面。

**我的立场（待 Owner 确认）：** 这在**课程这一层是可接受的**——课程本就是
「系统地学会一种思考方式 / AI 带着你走、有讲解」的教学层，和写作工作室（铁律最神圣的
地方）不同。**前提**是：

1. 学生始终**掌控**：暂停 / 重播 / 跳过 / 变速；
2. 学生保持**主动**而非被动：复用我们刚做的**逐步完成门槛 + 小测**（看完+答题才能下一步）；
3. 旁白**保持精炼**，不做「老虎机式」留人 / 自动连播俘获。

> 需要有意识地为「课程层允许 AI 讲解」这一收窄背书，而不是任其漂移。

---

## 4. 技术地基（已核实，降低了风险）

- **TTS 已支持时间戳。** `apps/api/internal/voice/tts.go`：请求体
  `ttsAudioParams.EnableTimestamp`（第 45 行，当前 `false`，第 82 行）。火山 v3
  单向 TTS 在开启后会在 `FullServerResponse` 帧里回传逐字/逐词时间。**我们目前把这些
  元数据帧丢弃了**（`Synthesize` 只拼接 `AudioOnlyServer` 的音频字节，返回 `[]byte`）。
  → **「文字随讲解高亮」不需要额外的 forced-alignment**，翻开开关 + 解析我们本就收到、
  却丢掉的时间帧即可。这是最省事的关键点。
- **音频是 mp3、可流式**（`format:"mp3"`, 24kHz）。适合**发布期预生成 + 缓存到 OSS**，
  而不是每次播放现调 TTS（省成本/延迟/避免时间漂移）。
- **已有「预发布产物」范式**：course.render_cache 就是外部授权、逐字段存 jsonb、边界校验、
  播放期直接用。**「预生成音频 + 时间线」完全套用同一范式**（一个「published lesson」产物 =
  slides + 每张的音频 objectKey + 时间数组）。
- **已有 admin-key 上传通道**：`apps/api/internal/api/course_admin.go` 的
  `postAdminUploadCourse` + `ossAdminAuthed`（常量时间比较）。新「lesson」发布/生成走同款门。
- **已有 OSS 基建**：presigned 上传/读取、3 scope、`OSS_ADMIN_KEY` 门；课程图片经
  `/oss/resolve-url` 解析（见 `AssetView.objectKeyFrom`）。
- **已有语音输入**：AskPanel 的「按住说话」ASR（`apps/web/src/api/voice` + `audio/capture`）。
  与 TTS 同一套火山客户端 `apps/api/internal/voice`。
- **已有 LLM 计费接缝**：`RecordCourseLLMCall`（若走 AI 生成旁白/切片，用它记档位+token+成本）。

---

## 5. 需要的组件（"what steps"）

1. **幻灯片 / lesson schema**（schema 驱动，像卡片一样：Zod + Go `go:embed` 单一真相源）。
   一小组**幻灯片模板**：标题页 / 图文左右 / 全幅图或视频 / 引用 / 板书 / 小测。每张携带：
   - `elements`：屏上元素（标题、bullets、素材引用、可高亮目标）；
   - `narration`：**朗读脚本**（与屏上文字*分离*——屏上常是精炼要点，旁白是完整口语）；
   - `cues`：旁白时刻 → 元素动作 的映射（浮现 bullet 2 / 高亮「NASA」/ 放大图 / 播放片段）。
     可以是**授权的 marker**，或从 **TTS 逐词时间戳**推导。
2. **发布期「音频 + 时间线」流水线**（新）：发布时对每张 narration 调 TTS（`enable_timestamp:true`），
   音频存 OSS，逐词时间存在该 slide 旁（一个「narration manifest」= 音频 objectKey + 时间数组）。
   后端小改：扩展 `Synthesize` 让它**同时返回时间帧**（今天丢弃的那部分）；加一个
   「narrate lesson」job，挂在 admin-key 通道后。
3. **新播放器 runtime**（前端）：用**幻灯片 deck + `<audio>` 调度器**替换竖向 `SegmentTimeline`。
   `timeupdate` 时按 cue 列表推进（浮现 bullet / 移动高亮 / 触发素材动画），到音频结束
   自动/点击进入下一张。控件：播放/暂停、重播本页、上/下页、变速、静音、字幕（由时间线生成）、
   尊重 `prefers-reduced-motion`。**保留顶部进度条 + 侧边 问印记**。小测幻灯片暂停播放直到作答
   ——**复用刚做好的逐步门槛**，接口很干净。
4. **`video` 素材类型**（+ 静图可选 Ken-Burns 运动，营造「幻灯片感」），扩展现有
   image/link/text 的 `AssetView`。
5. **授权 / 生成**：谁产出幻灯片 + 旁白脚本 + 音频。**最左右路线图的分叉**（见下）。

**分期路线：**
① 定内容模型 + 授权归属（本次对话）→ ② 写 lesson schema spec → ③ TTS 时间戳 + 发布流水线
→ ④ 幻灯片播放器（先用一门转换后的样例 a-mid 落地）→ ⑤ 提升保真度（逐词高亮、图片运动、视频）
→ ⑥ 迁移两门样例，退役滚动播放器。

---

## 6. 待决分叉（下次 resume 先答这些）

### A. 授权归属（最影响路线图）
新幻灯片 + 旁白脚本 + 音频，谁来产？
- **我们建 AI 转换**：平台把现有课程内容 AI 生成幻灯片 + 旁白，再 TTS→音频+时间。自助、可自由迭代；
  我们工作量大，但不依赖同事的工具。
- **class-agent 授权**：同事工具产出新格式并发布（与「docx→json + 素材」已在那边一致）；
  我们只建播放器 + 音频生成接缝。我们轻，但被其工具卡住。
- **混合（我倾向）**：作者（class-agent 或人工）产出幻灯片版式 + 旁白**文本**；**我们**拥有
  音频+时间生成（需我们的 TTS key + OSS）与播放器。接缝清晰，两边不长期互相阻塞。

### B. 同步保真度（最影响播放器复杂度）
- **逐词高亮（karaoke）**：讲到哪个短语哪个亮，bullet/素材按词 cue 浮现。最「活」，直接吃 TTS 逐词时间；
  播放器 + 授权更复杂（要标高亮 span）。
- **分点/场景级（我倾向作为起点）**：bullet 与素材在授权 cue（或句子边界）浮现，不追每个词。播放器简单得多，
  仍是「有讲解的幻灯片」；日后可无缝升级到逐词，不用重做模型。

### C. 迁移
- **替换**：幻灯片/语音播放器成为*唯一*课程体验；两门都迁；达到 parity 后退役滚动播放器。
- **并存**：新/转换课程走幻灯片模式，未配音内容仍用滚动播放器作兜底。过渡更稳，但一段时间维护两套。

### 其他 Owner 提到、值得一并想清楚的维度
- **AI 语音的「角色」**：是**旁白式**（朗读幻灯片）还是更**对话式的「老师」声音**？
- **动画/视频的野心**：多少转场、图片运动、真视频片段？
- **体验基调**：更像**讲座**还是**交互式走查**（walkthrough）？
- **成本**：发布期 TTS + 可能的 AI 生成旁白的一次性成本（因预生成+缓存而非按播放计，可控）。

---

## 7. 代码锚点（resume 时直接看这些）

| 关注点 | 位置 |
|---|---|
| 课程内容契约（Zod） | `packages/contracts/src/course.ts` |
| 课程数据模型 / 迁移 | `course`(structure/render_cache jsonb, card_ids, step_count) · `course_progress`(+active_seconds) · 迁移 0050/0051 |
| 当前播放器（要被替换） | `apps/web/src/shell/courses/CoursePlayer.tsx` · `SegmentTimeline.tsx` · `AssetView.tsx` |
| 逐步门槛 + 主动学习时长（刚做） | CoursePlayer（answered set / stepDone gate / active_seconds flush） |
| TTS（翻 timestamp 开关 + 解析丢弃的时间帧） | `apps/api/internal/voice/tts.go`（`EnableTimestamp` 45/82；`Synthesize` 只返回音频） |
| ASR 语音输入（已用） | `apps/web/src/shell/courses/AskPanel.tsx` · `apps/web/src/api/voice` · `audio/capture` |
| OSS 上传/解析 + admin 门 | `apps/api/internal/api/course_admin.go`（`ossAdminAuthed`）· `/oss/resolve-url` |
| 课程种子 | `apps/api/internal/store/seed/courses/*.json` · `SeedCourses` |
| LLM 计费接缝 | `RecordCourseLLMCall` |
| 参考（同事工具） | `docs/reference/class-agent/`（docx→json + 素材，出学生端 scope） |

---

## 8. Resume 提示

- 本次对话**没有做任何决定**，也没写代码。恢复时：先在第 6 节的 A/B/C（+ 其他维度）上和 Owner
  拍板，再走正式 `brainstorming → spec（docs/superpowers/specs/…）→ writing-plans → build`。
- 现有课程体验（滚动播放器 + 逐步门槛 + 主动时长 + 富工具卡报告）已上线 prod（`c73953b`, db v51），
  在做决定前**保持可用**——这次是「换车」，成品文档/体验都还在。
