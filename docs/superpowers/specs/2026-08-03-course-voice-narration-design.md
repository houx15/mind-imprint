# 课程语音讲解（Course Voice Narration）· 设计 Spec

**状态：设计已与 Owner 对齐，待 Owner 复核 → 然后 plan → build。**
日期：2026-08-03。

## Goal（一句话）

打开课程时，用真实的 AI 语音朗读当前这一步的文字；按 **空格键**（及可见的 ▶/⏸ 按钮）
暂停 / 继续。音频在**发布期预生成**、存 **OSS**、经 **CDN** 播放。

## 背景与决策（已在设计对话中拍板）

- 复用已存在的 TTS：`voice.Client.Synthesize`（火山 v3，返回 mp3）。**不**新建 TTS 引擎。
- **存 OSS，不存 DB。** 现有 `POST /voice/tts` 把 mp3 存进 Postgres `voice_tts_cache.audio bytea`
  ——对**动态**文本可以，但课程正文是**静态**的，应存对象存储 + CDN（与课程图片同款）。
- **不打 zip 包整门下载。** mp3 已压缩，zip 省不了体积；整门预下载会在开课时阻塞、浪费带宽、
  还要引入 JS 解压库并失去 CDN 的流式/范围请求/逐文件缓存与失效。→ **每步一个 mp3 对象**，
  播放器按需取当前步、后台预取下一步；浏览器 HTTP 缓存 + CDN 边缘自动缓存。
- **消除「两次调用」的担忧：** 签名是本地 HMAC（不访问 OSS 网络）。在 `GET /courses/{slug}`
  这一次已有的请求里，服务端**顺带把每步的已签名音频 URL** 一并返回。客户端因此**零额外解析调用**，
  只对 CDN 做**一次**（且可预取、可缓存、可流式）音频 fetch。
- 生成/上传发生在**发布期、服务端直传**（API 自己持有 OSS 凭证，直接 `PutObject`，不走客户端
  presigned 两步）——完全不在学生开课的热路径上。

## 非目标（本期明确不做）

- ❌ 逐词高亮 / 幻灯片模型 / 视频（那是 `docs/2026-08-03-narrated-slide-course-exploration.md`
  的大改，本期只做「朗读当前步正文 + 空格暂停」）。
- ❌ 给**动态**文本（陪练 Q&A 回复）配音。
- ❌ 离线打包 / zip。
- ❌ 逐词时间戳（虽然 `enable_timestamp` 已存在——留给幻灯片期）。

## 架构

**粒度：音频是「逐块（per piece）」的，不是「逐步（per step）」。** 一个 piece = 一条
`kind=="teaching"` 段。内容仍**逐块点击展开**（沿用已上线的 reveal），每露出一块就播它自己的音频。

```
发布期（seed / admin 上传）:
  for each course, for each step, for each teaching segment seg (按 content.segments 顺序):
    text = seg.text
    if text 非空 且 Voice 可用:
      pieceId = "<stepId>#<segIdx>"                       # JSON map key（segIdx=该段在 content.segments 的下标）
      key = "courses/audio/<slug>/<stepId>_<segIdx>_<hash8>.mp3"   # URL 安全（无 #），在既有 courses/ 前缀下
      if !oss.Exists(key): oss.PutObject(key, "audio/mpeg", Voice.Synthesize(text, 1.0))  # 服务端直传
      manifest[pieceId] = key
  course.audio_manifest = manifest                        # 存 course 行的新 jsonb 列: pieceId→objectKey

播放期（按 OSS 惯例：读 URL 短命(5min)、按需解析、不存 URL）:
  GET /courses/{slug}:
    payload.audioKeys = manifest                          # pieceId→objectKey（原样透传；对象 key 非机密，
                                                          # 读取由 /oss/resolve-url 的 session 门控）
    # render_cache 仍逐字节 verbatim 返回；audioKeys 是 payload 顶层平行 map；getCourse 不碰 oss。
  前端 CoursePlayer:
    reveal 逻辑不变（逐块，点击/空格-兜底 展开）。每露出一块 teaching 段:
      pieceId = `${stepId}#${segIdx}`；若 audioKeys[pieceId] 且未静音:
        url = await api.resolveUrl(audioKeys[pieceId])    # 现有客户端（图片同款），按需解析
        <audio src=url> 播放（首块受 autoplay 策略限制，见下）。
    空格: 当前块音频 播放→暂停 / 暂停→继续；无音频可控时 → 兜底=前进（同点击）。
    点击内容区任意处: 展开下一块 → 停当前音频、resolve+播下一块；本步块出完且小测答完 → 进入下一步。
    静音开关（🔊/🔇）: 关掉全部音频（"stop audio mode"）——只剩点击展开体验。
    预取: 展开某块时后台 resolve + `new Audio(url).load()` 预热**下一块**，把「解析+取音频」两跳藏在播放背后。
    换块/换步/卸载: 停并释放当前音频。
```

## 数据模型

- 迁移 **0052**：`ALTER TABLE course ADD COLUMN audio_manifest jsonb NOT NULL DEFAULT '{}'`。
  **pieceId → objectKey**，形如
  `{"step_01#0":"courses/audio/a-mid/step_01_0_ab12cd34.mp3", "step_01#1":"courses/audio/a-mid/step_01_1_....mp3", ...}`。
  - 放在 course 行的**独立列**（不塞进 `render_cache`，保持外部授权内容逐字段 verbatim 的信封原则）。
  - object key 在既有 `courses/` 前缀下、无 `#`（URL 安全）；pieceId（含 `#`）只作 JSON map key。
- sqlc 手改：`GetCourseBySlug` / `UpsertCourse` / seed upsert 带上 `audio_manifest`
  （新列放最后，Scan 顺序 == SELECT 顺序，pin sqlc@v1.27.0）。
- `key` 含 narration 内容 hash：正文改动 → 新 key → CDN 不会发旧音频（旧对象成孤儿，量小，
  未来加 OSS lifecycle 清理；本期不做）。

## narration 文本（逐块、确定性）

**一个 teaching 段 = 一块 = 一段音频**，narration 文本就是该段的 `seg.text`（不跨段拼接）——
这样点击展开某块就精确对应播放该块的音频。
- v1 只朗读 `kind=="teaching"` 段。**不**朗读 `kind=="structure"`（板书是学生填的脚手架）
  与 interaction（小测题干）——这两类块无音频。
- 空 `text` 的 teaching 段 → 无音频（manifest 无该 pieceId；前端无 URL → 该块静默，正常）。
- `content.subtitle` 是否也作为一块朗读？v1 **不**单独朗读 subtitle（它是副标题，不在
  timeline 的 reveal 块里）。

> 待 Owner 确认：v1 只逐块读 teaching 正文（不含小测题干 / 板书 / 副标题）是否 OK。

## 生成的接线点

- `SeedCourses`（`apps/api/internal/store/seed_courses.go`）：为 a-mid / b-mid 生成音频并写 manifest。
- `postAdminUploadCourse`（`apps/api/internal/api/course_admin.go`）：上传新课后生成音频。
- **优雅降级**：`Deps.Voice == nil`（无 TTS 凭证，如本地/测试）或 `oss.Service == nil`
  → 跳过生成，manifest 留空，课程照常上线（只是没有语音）。gen 失败某一步 → 记 warn、跳过该步，
  不阻塞其余步与整体发布（幂等：重跑覆盖）。
- 成本：预生成一次。key 由 narration 内容 hash 确定 → **gen 前 `oss.Exists(key)`（HeadObject）
  命中则跳过 Synthesize**，所以每次 migrate-up 重跑 SeedCourses **不会**重复合成（内容没变就零 TTS 调用）。
  无按播放成本。

## 后端改动清单

1. `oss.Service.PutObject(ctx, key, contentType string, data []byte) error` —— 包 `s.origin.PutObject`
   （SDK bucket 已在 Service 里）。`oss.Service.Exists(ctx, key) (bool, error)` —— 包 `IsObjectExist`，
   供 gen 跳过已存在对象。
2. **读路径按 OSS 惯例（resolve-on-demand，不在 getCourse 里签）**：payload 只带 object keys；前端用
   已有 `api.resolveUrl(key)`（`/oss/resolve-url`, session 门控, 5min）按需换读 URL。getCourse **不碰 oss**。
3. `voice.Client.Synthesize` —— 已存在，直接用（不改；`enable_timestamp` 仍 false）。
4. 新 `GenerateCourseAudio(ctx, voice, oss, slug, renderCache) (manifest map[string]string, err)`
   —— 遍历每步每个 teaching 段：算 pieceId + key；`oss.Exists(key)` 未命中才 Synthesize + PutObject；
   返 pieceId→key manifest。voice/oss 为 nil → 返空 manifest（不报错）。
5. 迁移 0052 + sqlc 手改（audio_manifest 读写）。
6. 课程 DTO（`course_dto.go` `toCoursePayloadDTO`）：新增顶层 `audioKeys map[string]string`
   —— **原样透传** `payload.AudioManifest`（pieceId→objectKey）。render_cache 仍逐字节 verbatim 返回。
   - **无需签名、无需 oss**：object key 非机密，读取由 `/oss/resolve-url` 的 session 门控。
   - `GetCoursePayload` 需把 `course.audio_manifest` 一并带出（store 层加字段，Task 2）。
7. `SeedCourses` / `postAdminUploadCourse` 调 `GenerateCourseAudio` 并存 manifest。

## 前端改动清单

- `packages/contracts/src/course.ts`：`CoursePlayerPayload` 顶层加可选
  `audioKeys?: Record<string, string>`（pieceId → objectKey）。**不**改 render_cache 内的 step 结构（verbatim）。
  前端用 `payload.audioKeys?.[`${stepId}#${segIdx}`]` 拿 key，再 `await api.resolveUrl(key)` 换读 URL。
- `CoursePlayer.tsx` / `SegmentTimeline.tsx`：`NarrationController`（逐块）
  - reveal 不变（逐块，点击内容区任意处展开下一块）。`SegmentBlock` 渲染 teaching 段时需知道其 `segIdx`。
  - **每露出一块 teaching 段**：若 `audioKeys[`${stepId}#${segIdx}`]` 存在且未静音 →
    `await api.resolveUrl(key)` 换读 URL → 建/复用 `<audio>` 播它，并**停掉上一块**的音频。▶/⏸ 按钮显式控当前块音频。
  - **空格**（window `keydown`，`preventDefault()` 防滚动；焦点在 Q&A 输入框时不劫持；不重复激活聚焦按钮）：
    1. 当前块音频**正在播** → 暂停；
    2. **已暂停未播完** → 继续；
    3. **无音频可控**（已播完 / 静音 / 该块无音频）→ **兜底=前进**（同「点击继续」）。
  - **点击内容区任意处 = 继续**：展开下一块 → 停当前音频、播下一块音频；本步块出完且小测答完 → 进入下一步。
    （排除 button / a / input / textarea / 小测选项 / 问印记面板 的点击。）
  - **静音开关（🔊/🔇）**：关 = "stop audio mode"，展开时不播音频；偏好存 `localStorage`。
  - **autoplay 策略**：浏览器在首个用户手势前禁止有声自动播放；开课**首块**通常需一次 空格/点击/▶ 启动，
    此后逐块可自动播。
  - **预取**：露出某块时后台 `resolveUrl(下一块 key)` + `new Audio(url).load()` 预热**下一块**。
  - 换块 / 换步 / 卸载：停止并释放当前音频，移除监听。

## 与「逐步门槛 + 主动时长」的关系（复用上一期成果，基本不变）

- reveal 保持**逐块点击展开**（不 auto-reveal）——音频只是给「当前露出的块」配音。
- 门槛不变：**看完（reveal 全部）+ 答完本步小测** 才能进入下一步（`stepDone`）。
  「点击/空格继续」在本步块未出完时 = 展开下一块；出完且小测答完时 = 进入下一步；被未答小测挡住 = 提示。
- 主动时长（active_seconds）继续照常累计。

## 测试

- Go：`GenerateCourseAudio`（stub Voice 返固定 bytes、stub/fake oss 记录 PutObject key + payload）；
  voice/oss 为 nil → 空 manifest 不报错；空 narration 步跳过。
- Go：`getCourse` 对 manifest 命中的步返回 `audioUrl`（fake oss SignDownload），未命中步不返回。
- Go：迁移 0052 应用；GetCourseBySlug/UpsertCourse round-trip manifest。
- Web：NarrationController —— 有 audioUrl 时渲染 ▶；空格 toggle（mock HTMLAudioElement play/pause）；
  焦点在输入框时空格不劫持；换步停止旧音频。
- 现有 706 web + Go 课程测试保持绿。

## 部署

- 有迁移（0052）→ full deploy（migrate-up 会重跑 SeedCourses → 为 a-mid/b-mid 生成音频到 prod OSS）。
- 预检：prod OSS 已配置、TTS voice 已配置（seed-tts-2.0，[[voice-tts-asr-feature]] 已验证）。
- 部署后 live 验证：开课按空格 → 有声；换步预取；报告/门槛/时长不回归。

## 决策（Owner 2026-08-03 已定，采纳全部推荐）

1. ✅ v1 逐块**只朗读 teaching 正文**（不含小测题干 / 板书 / 副标题）。
2. ✅ 空格保留「**无音频可控时兜底=前进**」（主用途 暂停/继续；此为键盘便利兜底）。
3. ✅ 静音开关**默认开声**，偏好记 `localStorage`。
4. ✅ key 用「**内容 hash 版本化**」（正文改自动换新音频、旧对象成孤儿，量小，本期不清理）。

## 代码锚点

- TTS：`apps/api/internal/voice/tts.go`（`Synthesize`）· 现有 HTTP：`apps/api/internal/api/voice.go`
- OSS：`apps/api/internal/oss/oss.go`（`SignUpload`/`SignDownload`/`origin` bucket；加 `PutObject`）
- 课程：`course_dto.go`（`toCoursePayloadDTO`）· `course.go`（`getCourse`）·
  `seed_courses.go`（`SeedCourses`）· `course_admin.go`（`postAdminUploadCourse`）
- 播放器：`apps/web/src/shell/courses/CoursePlayer.tsx` · 契约 `packages/contracts/src/course.ts`
- 关联：[[course-v2-showlogic-2026-08-03]]（现课程）· [[oss-storage-infra]] · [[voice-tts-asr-feature]] ·
  未来幻灯片 `docs/2026-08-03-narrated-slide-course-exploration.md`
