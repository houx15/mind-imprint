# 课程语音讲解 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 打开课程逐块展开时，用预生成的 TTS 音频朗读当前 teaching 块；空格暂停/继续（无音频可控时兜底前进），点击展开下一块（停当前音频、播下一块），可一键静音。

**Architecture:** 发布期（seed / admin 上传）**逐 teaching 段**用现有 `voice.Client.Synthesize` 生成 mp3，服务端直传 **OSS**（`course-audio/<slug>/<stepId>#<segIdx>-<hash8>.mp3`），把 `pieceId→objectKey` 存进 `course.audio_manifest`（迁移 0052）。`GET /courses/{slug}` 用 `oss.SignDownload` 把命中块签成 `audioUrls` 平行 map 返给前端。前端逐块播放 + 空格 + 点击继续 + 静音。设计 spec：`docs/superpowers/specs/2026-08-03-course-voice-narration-design.md`。

**Tech Stack:** Go(net/http + pgx + 手改 sqlc + goose) · Aliyun OSS SDK · 火山 TTS(已存在) · React+Vite+TS · Zod contracts.

## Global Constraints

- **密钥只在服务端**：OSS key / TTS key 绝不进 git、日志、错误、返回体。签名(`SignDownload`)是本地 HMAC，不网络调用 OSS。
- **render_cache 逐字节 verbatim**：不解析、不注入 audioUrl；audioUrls 是 payload 顶层 `pieceId→url` map。
- **优雅降级**：`Deps.Voice == nil` 或 `oss.Service == nil` → 跳过音频生成/签名，课程照常可用（静默）。gen 某块失败 → warn+跳过，不阻塞其余块与发布。**幂等**：`oss.Exists(key)` 命中则跳过合成。
- **pieceId 格式**：`"<stepId>#<segIdx>"`（JSON map key），segIdx = 该 teaching 段在 `content.segments` 的原始下标（非 teaching 段一起计）。
- **object key 格式**：`courses/audio/<slug>/<stepId>_<segIdx>_<hash8>.mp3`，`hash8 = hex(sha256(voiceName+"\n"+text))[:8]`。**URL 安全（无 `#`）**，在既有 `courses/` 前缀下。pieceId（含 `#`）只作 map key，不进 object 路径。
- **读路径 = resolve-on-demand**（OSS 惯例：读 URL 5min、按需解析、不存 URL）：payload 带 object **keys**（`audioKeys`），前端用已有 `api.resolveUrl(key)` 按需换读 URL + 预取下一块。`getCourse` **不签名、不碰 oss**。object key 非机密（读取由 `/oss/resolve-url` 的 session 门控）。
- **sqlc 手改**：改 `.sql` + `.sql.go` 两处；新列放 SELECT 末尾，Scan 顺序 == SELECT 顺序。若跑 sqlc 生成，先 pin `@v1.27.0`。
- **v1 只朗读 teaching 段**（不含小测题干 / 板书 / subtitle）。
- **不回归**：现有 706 web 测试 + 全部 Go 课程/迁移测试保持绿。

---

### Task 1: OSS 服务端直传与存在性检查

**Files:**
- Modify: `apps/api/internal/oss/oss.go`
- Test: `apps/api/internal/oss/oss_test.go`（若无则新建）

**Interfaces:**
- Produces:
  - `func (s *Service) PutObject(ctx context.Context, key, contentType string, data []byte) error`
  - `func (s *Service) Exists(ctx context.Context, key string) (bool, error)`

**说明：** `Service.origin` 是 `*alioss.Bucket`。`PutObject` 用 `s.origin.PutObject(key, bytes.NewReader(data), alioss.ContentType(contentType))`。`Exists` 用 `s.origin.IsObjectExist(key)`。两者都在 Service 非 nil 时可用。

- [ ] **Step 1: 写实现**（PutObject / Exists）。
- [ ] **Step 2: 写单测**：因为 SDK bucket 需真实 OSS，测试**不打网络**——只断言方法存在、签名正确、参数拼装（可对 `contentType==""` 走默认）。若无法免网络地测 SDK，退化为编译期存在性断言 + 一个 `TestPutObjectContentTypeDefault` 之类纯逻辑测试；**不要** mock 整个 SDK。实测由后续任务的 stub OSS 覆盖。
- [ ] **Step 3: 编译 + vet**：`cd apps/api && CGO_ENABLED=0 go build ./... && go vet ./internal/oss/`。
- [ ] **Step 4: Commit**：`git add apps/api/internal/oss/oss.go apps/api/internal/oss/oss_test.go && git commit -m "feat(oss): server-side PutObject + Exists"`。

---

### Task 2: 迁移 0052 + audio_manifest 存取管线

**Files:**
- Create: `apps/api/internal/store/migrations/0052_course_audio_manifest.sql`
- Modify: `apps/api/internal/store/queries/course.sql`, `apps/api/internal/store/sqlc/course.sql.go`
- Modify: `apps/api/internal/agent/coursestore.go`（`CoursePlayerPayload` / `UpsertCourseInput` / `UpsertCourse` / `GetCoursePayload`）
- Test: `apps/api/internal/store/` 迁移测试已有（跑一次确认 0052 应用）；`apps/api/internal/agent/coursestore_test.go`（round-trip manifest）

**Interfaces:**
- Produces：`CoursePlayerPayload.AudioManifest map[string]string`；`UpsertCourseInput.AudioManifest map[string]string`。
- Consumes：Task 4 调 UpsertCourse 存 manifest；Task 5 从 payload 读 manifest。

**迁移：**
```sql
-- +goose Up
ALTER TABLE course ADD COLUMN audio_manifest jsonb NOT NULL DEFAULT '{}';
-- +goose Down
ALTER TABLE course DROP COLUMN audio_manifest;
```

**sqlc 手改（.sql + .sql.go 同步）：**
- `GetCourseBySlug`：SELECT 末尾加 `audio_manifest`；`GetCourseBySlugRow` 加 `AudioManifest []byte`（jsonb 以 `[]byte` 收，和 structure/render_cache 一致）；Scan 末尾加 `&i.AudioManifest`。
- `UpsertCourse`：INSERT 列 + VALUES 加 `audio_manifest`（新参数 `$10` 放 `now()` 之前对应位；注意当前 VALUES 末尾是 `now()`，新增列插在其前）；ON CONFLICT DO UPDATE 加 `audio_manifest = EXCLUDED.audio_manifest`；`UpsertCourseParams` 加 `AudioManifest []byte`；QueryRow 传参补上。
- `coursestore.go`：
  - `CoursePlayerPayload` 加 `AudioManifest map[string]string`；`GetCoursePayload` 把 `row.AudioManifest`（[]byte）`json.Unmarshal` 进去（空/nil → 空 map）。
  - `UpsertCourseInput` 加 `AudioManifest map[string]string`；`UpsertCourse` 把它 `json.Marshal` 成 []byte 传给 sqlc（nil → `{}`）。

- [ ] **Step 1: 写迁移 0052。**
- [ ] **Step 2: 改 course.sql（两个 query）。**
- [ ] **Step 3: 手改 course.sql.go（const SQL + Params/Row struct + Scan/QueryRow 顺序）。**
- [ ] **Step 4: 改 coursestore.go**（payload/upsert 字段 + marshal/unmarshal）。
- [ ] **Step 5: 加/改测试**：`coursestore_test.go` 里 UpsertCourse 带一个 `AudioManifest{"s0#0":"k0"}`，GetCoursePayload round-trip 断言相等。
- [ ] **Step 6: 跑**：`CGO_ENABLED=0 go test ./internal/store/ -run TestMigrate -count=1`（0052 应用）+ `go test ./internal/agent/ -run TestSqlcAgentStore -count=1`（需 Docker）。
- [ ] **Step 7: Commit。**

---

### Task 3: GenerateCourseAudio（逐块生成）

**Files:**
- Create: `apps/api/internal/agent/course_audio.go`
- Test: `apps/api/internal/agent/course_audio_test.go`

**Interfaces:**
- Produces：
  ```go
  // Synth 与 ObjectStore 是窄接口，便于测试 stub（生产传 api.VoiceService 适配器 + *oss.Service）。
  type CourseAudioSynth interface { Synthesize(ctx context.Context, text string, speed float64) ([]byte, error); Voice() string }
  type CourseAudioStore interface { Exists(ctx context.Context, key string) (bool, error); PutObject(ctx context.Context, key, contentType string, data []byte) error }
  // renderCache 是 course.render_cache 原始 []byte。返回 pieceId->objectKey。
  func GenerateCourseAudio(ctx context.Context, synth CourseAudioSynth, store CourseAudioStore, slug string, renderCache []byte) (map[string]string, error)
  ```
- Consumes：Task 4 用它；Task 5 无关。

**逻辑：**
- `synth==nil || store==nil` → 返回 `map[string]string{}, nil`（降级）。
- 解析 renderCache 为窄结构：`{steps:[{stepId, content:{segments:[{kind,text}]}}]}`。
- 对每 step 每个 `segments[i]`，若 `kind=="teaching"` 且 `strings.TrimSpace(text)!=""`：
  - `pieceId := stepId + "#" + strconv.Itoa(i)`  // JSON map key
  - `hash8 := hex(sha256(synth.Voice() + "\n" + text))[:8]`
  - `key := "courses/audio/" + slug + "/" + stepId + "_" + strconv.Itoa(i) + "_" + hash8 + ".mp3"`  // URL-safe, no '#'
  - `if ok,_ := store.Exists(ctx,key); !ok { audio,err := synth.Synthesize(ctx,text,1.0); if err!=nil { log warn; continue }; if err:=store.PutObject(ctx,key,"audio/mpeg",audio); err!=nil { log warn; continue } }`
  - `manifest[pieceId] = key`
- 返回 manifest（累积成功的块；个别失败跳过不返错）。

- [ ] **Step 1: 写失败测试**：stub synth 返回固定 bytes + 记录调用；stub store 记录 Exists/PutObject。喂一个含 2 teaching + 1 structure + 1 空 teaching 的 renderCache。断言：manifest 只含 2 个非空 teaching 的 pieceId；PutObject 被调 2 次且 key 前缀/ContentType 正确；structure/空段无 key。
- [ ] **Step 2: 跑测试见 fail**（函数未实现）。
- [ ] **Step 3: 实现。**
- [ ] **Step 4: 加测试**：`Exists` 返回 true 时**跳过** Synthesize（断言 synth 未被调用该块）；`synth==nil` → 空 manifest 无错。
- [ ] **Step 5: 跑绿**：`go test ./internal/agent/ -run TestGenerateCourseAudio -count=1`（纯逻辑，免 Docker）。
- [ ] **Step 6: Commit。**

---

### Task 4: 接线 seed + admin 上传

**Files:**
- Modify: `apps/api/internal/store/seed_courses.go`（`SeedCourses`）
- Modify: `apps/api/internal/api/course_admin.go`（`postAdminUploadCourse`）
- Modify: 相关装配处让 `SeedCourses` 能拿到 voice + oss（看现有签名；`main.go` 若需传入则改）
- Test: 现有 seed / admin 测试保持绿；新增一条「manifest 被写入」的断言（admin 上传后 GetCoursePayload 的 AudioManifest 非空——用 stub voice+oss 的测试装配，或在无 voice/oss 时断言为空且不报错）

**说明：**
- `SeedCourses`：对 a-mid/b-mid，在 upsert course 前调 `GenerateCourseAudio(ctx, voiceAdapter, ossSvc, slug, renderCache)`，把结果放进 `UpsertCourseInput.AudioManifest`。若 voice/oss 不可用 → 空 manifest（种子照常）。
- `postAdminUploadCourse`：上传解析后、upsert 前，同样生成并存 manifest。
- voice 适配器：`api.VoiceService` 已有 `Synthesize` + `Voice()`，直接满足 `CourseAudioSynth`。`*oss.Service` 满足 `CourseAudioStore`（Task 1 之后）。注意 `Deps` 是否已持有 Voice / OSS；seed 若在 store 层跑不到 Deps，则把 voice/oss 作为参数传进 `SeedCourses`（改其签名 + 调用点）。

- [ ] **Step 1: 看现状**：`SeedCourses` 当前签名与调用点（`main.go` 的 `--migrate-up` 路径）；admin handler 拿 Deps 的方式。
- [ ] **Step 2: 接 admin 上传**（能直接拿 `a.d.Voice` / oss）。
- [ ] **Step 3: 接 seed**（按需扩签名传 voice/oss；不可用则空 manifest）。
- [ ] **Step 4: 测试**：admin 测试用已有 stub voice；断言上传后 payload.AudioManifest 命中；无 voice 装配下为空、不报错。
- [ ] **Step 5: 跑**：`go test ./internal/api/ -run 'TestAdmin|TestCourse' -count=1` + `go test ./internal/store/ -run TestSeed -count=1`（需 Docker）。
- [ ] **Step 6: Commit。**

---

### Task 5: 播放期 audioKeys（DTO 透传 + 契约）

**Files:**
- Modify: `packages/contracts/src/course.ts`（`CoursePlayerPayload` 加 `audioKeys`）
- Modify: `apps/api/internal/api/course_dto.go`（`coursePayloadDTO` + `toCoursePayloadDTO`）
- Test: `apps/api/internal/api/course_test.go`

**Interfaces:**
- Produces（契约）：`CoursePlayerPayload.audioKeys?: Record<string,string>`（pieceId→objectKey）。前端 Task 6/7 用。

**说明（读路径 = resolve-on-demand，getCourse 不碰 oss、不签名）：**
- 契约：`packages/contracts/src/course.ts` 的 `CoursePlayerPayload` 加 `audioKeys: z.record(z.string()).optional().default({})`。
- DTO：`coursePayloadDTO` 加 `AudioKeys map[string]string \`json:"audioKeys"\``。`toCoursePayloadDTO`
  把 `p.AudioManifest`（Task 2 已带出，pieceId→objectKey）**原样透传**（nil → 空 map `{}`）。
- `getCourse` **不改逻辑、不引入 oss**——object key 非机密，读取由 `/oss/resolve-url` 的 session 门控。
- 空 manifest → `audioKeys` 为空对象。

- [ ] **Step 1: 契约加 audioKeys + 跑 contracts 测试。**
- [ ] **Step 2: DTO 加 AudioKeys 字段并在 toCoursePayloadDTO 透传 p.AudioManifest。**
- [ ] **Step 3: 测试**：测试里 UpsertCourse 带 `AudioManifest{"s0#0":"courses/audio/x/s0_0_ab12cd34.mp3"}`，
  `GET /courses/{slug}` 断言 `audioKeys["s0#0"]` == 该 key。无 manifest → `audioKeys` 为空对象 `{}`。
- [ ] **Step 4: 跑**：`go test ./internal/api/ -run TestCourse -count=1`（Docker）+ contracts 测试（`packages/contracts` 下 `npx vitest run`）。
- [ ] **Step 5: Commit。**

---

### Task 6: 前端逐块音频播放控制器

**Files:**
- Modify: `apps/web/src/shell/courses/CoursePlayer.tsx`
- Modify: `apps/web/src/shell/courses/SegmentTimeline.tsx`（`SegmentBlock` 暴露 `segIdx`；把当前展示到的 teaching 块的 segIdx 上报给 CoursePlayer，或由 CoursePlayer 从 revealed 计算「当前块」）
- Test: `apps/web/test/shell/courses/CoursePlayer.test.tsx`

**Interfaces:**
- Consumes：`payload.audioKeys`（Task 5）+ 已有 `api.resolveUrl(objectKey): Promise<string>`（`apps/web/src/api/oss.ts`，AssetView 同款）。
- Produces：一个「当前播放块」+ `<audio>` 控制；静音状态。供 Task 7 的空格/点击接。

**说明（resolve-on-demand + 预取）：**
- 「当前块」判定：timeline 逐块 reveal（`revealed`）。当前块 = 最新露出的 item；若是 teaching 段，其 `segIdx` = 它在 `content.segments` 的原始下标（`buildTimeline` 的 `segment-<index>` key 已含该 index；把 index 透出到 item 上，或在 CoursePlayer 重算）。`pieceIdFor(stepId, segIdx) = `${stepId}#${segIdx}``。
- 播放：`revealed` 增加时，若新块是 teaching 且 `audioKeys[pieceId]` 存在且未静音 →
  `const url = await api.resolveUrl(audioKeys[pieceId])` → 停旧 `<audio>`、建/复用 `audioRef` 播 url。
  注意 resolve 是 async：入块时旧的 in-flight resolve 要能被后续块取消/忽略（用一个递增 token/ref 防竞态）。
- 首块 autoplay：受浏览器策略限制——首次播放放在用户手势里（首个 空格/点击/▶）。`hasGesture` ref，首手势后允许自动播新块。
- 静音：`muted` state，默认 `false`，`localStorage['course-audio-muted']` 持久化。静音时不 resolve/不播；▶/⏸ 与 🔊/🔇 按钮放 header 或底栏。
- 预取：露出某块后，若下一个 teaching 块有 key，后台 `api.resolveUrl(nextKey)` + `new Audio(url).load()` 预热。
- 卸载/换步：停并释放 audio，作废 in-flight resolve token。

- [ ] **Step 1: 让「当前 teaching 块 + 其 segIdx」在 CoursePlayer 可得**（透出 buildTimeline item 的 segment index，或重算）。加 `pieceIdFor` helper。
- [ ] **Step 2: 写测试**（先失败）：spy `HTMLMediaElement.prototype.play/pause`（jsdom 未实现，需 `vi.spyOn`/stub）；mock `api.resolveUrl` 返回固定 url。payload 带 `audioKeys{"s0#0":"courses/audio/x/s0_0_h.mp3"}`；render → 首个手势后露出块0 → 断言 `resolveUrl` 被调该 key、`play` 被调、audio.src 含返回 url。点静音开关 → 断言不再 resolve/play + `localStorage` 写入。
- [ ] **Step 3: 实现播放控制器 + resolve 竞态防护 + 静音开关。**
- [ ] **Step 4: 跑**：`npx vitest run test/shell/courses/CoursePlayer.test.tsx`。
- [ ] **Step 5: Commit。**

---

### Task 7: 空格状态机 + 点击继续（跨块/跨步）

**Files:**
- Modify: `apps/web/src/shell/courses/CoursePlayer.tsx`
- Test: `apps/web/test/shell/courses/CoursePlayer.test.tsx`

**Interfaces:**
- Consumes：Task 6 的音频控制器 + `stepDone`（已存在的门槛）。

**说明（沿用 spec 的语义）：**
- **点击内容区任意处 = 继续**：现有 reveal handler 已在「本步还有块」时展开下一块（保留）。扩展：当本步块**已出完**（`!hasMore`）且 `stepDone`（小测答完）时，点击继续 → **进入下一步**（末步 → onFinish）。被未答小测挡住 → 现有提示。展开下一块时：停当前音频、播下一块音频（复用 Task 6）。排除 button/a/input/textarea/小测选项/问印记面板。
- **空格**（window keydown, `preventDefault()`, 焦点在 input/textarea 时跳过, 不重复激活聚焦按钮）：
  1. 当前 audio 正在播 → `pause()`；
  2. audio 存在且已暂停未 ended → `play()`；
  3. 否则（无 audio / 已 ended / 静音）→ **兜底=继续**（等价上面的点击继续：有下一块则展开+播；否则若 stepDone 则进入下一步）。
- 空格与「点击继续」共用一个 `continue()` 函数（reveal 下一块 or 跨步）。

- [ ] **Step 1: 写测试**（先失败）：
  - 空格暂停/继续：块0 播放中按空格 → `pause`；再按 → `play`。
  - 空格兜底前进：audio ended（触发 `ended` 或无 audio）后按空格 → 展开下一块 / 或 stepDone 时进入下一步（`onFinish` 末步）。
  - 点击继续跨步：单块无小测的步，块出完后点击内容区 → 进入下一步（saveCourseProgress 被调，objectContaining current_ordinal）。
  - 焦点在 问印记 输入框时按空格**不**前进/不劫持。
- [ ] **Step 2: 跑见 fail。**
- [ ] **Step 3: 实现 `continue()` + 空格状态机 + 点击跨步。**
- [ ] **Step 4: 跑绿** + 回归 `npx vitest run test/shell/courses/`（全绿，含既有门槛/时长测试）。
- [ ] **Step 5: Commit。**

---

### Task 8: 端到端与部署校验（收尾）

**Files:** 无代码（或按前序 review 的小修）。

- [ ] **Step 1: 全量测试**：`apps/web` `npx vitest run`（≈706+ 绿）；`apps/api` `CGO_ENABLED=0 go test ./... -count=1`（需 Docker；课程/迁移/agent 全绿）。
- [ ] **Step 2: tsc + build**：`apps/web` `npx tsc --noEmit -p tsconfig.json` + `npm run build`；`apps/api` `go build ./...`。
- [ ] **Step 3: 本地 live 校验**（vite 代理 prod API 的方式见记忆；此功能需 prod 已部署新 API 才能验签名/音频，故 live 验证放在部署后）：确认无音频时页面正常（降级）。
- [ ] **Step 4: 部署**（有迁移 0052 → full deploy：备份 DB → `run --rm --build api -migrate-up`（会重跑 SeedCourses 生成 a-mid/b-mid 音频到 prod OSS）→ `up -d --build api web`；**部署后核对容器 CREATED 时间**，不只看 db 版本）。
- [ ] **Step 5: prod live 校验**（Playwright，真 origin）：开课首块按空格→有声；点击展开下一块→换音频；🔇 静音→无声；空格兜底前进；门槛/时长/报告不回归。
- [ ] **Step 6: 更新记忆**（course-v2 memory 追加 FOLLOW-UP + 提交/合并）。

---

## 备注

- 执行时先 `cat .superpowers/sdd/progress.md`（若存在）恢复进度。
- 每个 Task 独立 commit；末尾 whole-branch review（opus）后走 finishing-a-development-branch 合并 main + 部署。
- 现有分支：`course-voice-narration`（从 main）。
