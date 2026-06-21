# Slice 6 — App Shell (B7) · Design Spec

> Status: approved design, ready for plan. Date: 2026-06-21. Branch: `slice-6-app-shell`.
> Binding UI source: `docs/design/思维印记_工作区.dc.html` (lift styles verbatim). PRD governs non-visual.
> This is the final slice to reach an **end-to-end runnable platform**: 新建任务 → 粘链接 → 陪练对话 → AI 调卡 → 填卡 → 过程树实时长 → 评估那一刀 → 记录页.

## Goal

Replace the dev `DevApp` with the real product shell: mock auth, left-rail navigation, a task directory/home, directory↔workspace routing, a records page (active calendar + card usage + 9-dim ability), and a settings page that owns real LLM configuration behind a non-dismissable first-run key-gate. Fold in the S5 carry-forwards (canonical 9-dim rubric, eval error UX, re-run guard, setState-swap audit).

## Global Constraints (apply to every task)

- **Stack:** pnpm monorepo. `packages/contracts` (Zod, single source of truth) + `apps/web` (React 18 + Vite + TS + Tailwind). Verification gate: `pnpm -r typecheck` (tsc `--noEmit`) **and** `pnpm -r test` must be green. CRITICAL: vite/vitest use esbuild and do NOT typecheck — run `tsc` per task.
- **Key-safety (hard):** frontend-direct BYO-key. The API key lives ONLY in `mk.llmConfig` (localStorage) and in request headers. It must NEVER appear in any log, thrown error, rendered output, the data store, or the `mk.session` blob. Re-verify at the end of the slice.
- **Reactive store pattern:** `useSyncExternalStore` requires a STABLE `getSnapshot` reference — state ref swaps and `notify()` MUST be gated on an actual change (the bug fixed in `createEvaluator`). Any new reactive controller/store follows this.
- **Design fidelity:** lift inline styles verbatim from the binding HTML; mockups use real Phoebe content, never lorem ipsum. One documented copy deviation: the card-usage intro says "五大分支" but the library has 8 real branches — group by the 8 registry categories and soften the copy to "课程库分支".
- **Restraint:** no addictive mechanics (streaks/leaderboards/badges/push). Records is a quiet read-only surface.

## Architecture

The design HTML is already a screen state machine (`screen / tab / taskView / recordTab`). Mirror it with a top-level `AppShell` holding ephemeral nav state in React `useState` — **no react-router** (single-window demo). Two concerns persist separately from the data store, keeping the store pure (`task/message/card/evaluation` only):

- **`mk.session`** (new) — `{ authed: boolean; aiAvatar: string }`, zod-parsed, owned by the shell. Drives the login gate (refresh stays in-app) and the AI-avatar color preference.
- **`mk.llmConfig`** (existing) — gains `verified: boolean` so the key-gate knows a connection test passed.

```
AppShell  (screen/tab/taskView/recordTab/activeTaskId in useState)
├─ screen 'login'|'register'|'bind'  → AuthScreen        (click-through mock)
└─ screen 'app'
   ├─ LeftRail  (任务 / 记录 / 设置 + avatar→settings)
   ├─ KeyGateModal  (non-dismissable; mounts when llmConfig missing OR !verified)
   └─ content by tab:
      ├─ tasks + taskView 'directory'  → DirectoryView       (store tasks)
      ├─ tasks + taskView 'workspace'  → WorkspaceContainer  (per-activeTaskId controllers)
      ├─ records                       → RecordsView         (learning/cards/ability)
      └─ settings                      → SettingsView
```

`main.tsx` mounts `AppShell`. `DevApp`/`Harness`/`StorePanel`/`SettingsPanel`/`WorkspaceDev` stay in-repo as a dev-only harness (no longer the default entry); not deleted.

## Components & files

### Persistence / session
- **Create `apps/web/src/shell/session.ts`** — `SESSION_KEY = "mk.session"`. `Session = z.object({ authed: z.boolean().default(false), aiAvatar: z.string().default("#2A3B7A") })`. `readSession(storage)` (fail-soft: missing/corrupt → defaults), `writeSession(storage, s)` (validate-then-persist), and a `createSession({storage})` reactive holder (stable `getSnapshot`) + `useSession()` hook. Behind the existing injectable `RawStorage` (memory storage in tests).
- **Modify `apps/web/src/llm/config.ts`** — add `verified?: boolean` to the stored config; replace `JSON.parse(stored) as LlmConfig` with a zod `safeParse` fallback to env (S5 carry-forward: config hardening). Saving a fresh key resets `verified` to false; a passing 测试连接 sets it true.

### Auth (design lines 31–98)
- **Create `apps/web/src/shell/auth/AuthScreen.tsx`** — renders login / register / bind by an internal `authStep` state. All inputs are cosmetic (pre-filled Phoebe values verbatim). `登录` and `完成，进入` and `暂时跳过` → `onEnterApp` (sets `session.authed = true`, screen → 'app'). `注册`↔`登录`↔`绑定` navigate steps. No validation, no backend.

### Left rail (design lines 104–141)
- **Create `apps/web/src/shell/LeftRail.tsx`** — logo + 任务/记录/设置 nav items + avatar dot → settings. Active state styling computed from current `tab` (the `nav*.box / .icon / .label` style objects). Add `role="tab"` / `aria-selected` (a11y carry-forward).

### Directory / home (design lines 147–196)
- **Create `apps/web/src/shell/directory/DirectoryView.tsx`** — greeting (下午好，Phoebe / 你想搞懂什么？), the new-task entry (single input + 开始 button), and the in-progress task grid (`taskCount` + 2-col cards).
- **Create `apps/web/src/shell/directory/taskCardView.ts`** (pure) — maps a `Task` + its `card_instances` → `{ status, statusStyle, last, title, barStyle, cardsLabel }`. Status pill from `task.status` (active→进行中 green, evaluated→已评估). Progress bar + `cardsLabel` from completed card count. `last` from `last_active_at` (relative time).
- **New task flow:** 开始 → `createTask` in store (title = trimmed input; if the input contains a URL, set `seed` = that URL) → seed the first **student message** with the input text (and, if a URL is present, the workspace renders the existing link-chip via `m.hasLink`) → set `activeTaskId`, `taskView = 'workspace'`. Empty input → button no-op/disabled.

### Workspace container (per-task lifecycle — the main integration risk)
- **Create `apps/web/src/shell/WorkspaceContainer.tsx`** — given `activeTaskId`, lazily creates `createConversation` + `createEvaluator` for that task (held in a ref keyed by id; recreate when id changes, so switching tasks yields a fresh tree/chat/eval with no stale subscription). Renders `WorkspaceView` with those controllers + the breadcrumb `onBackToDir` → `taskView = 'directory'`. `WorkspaceView` stays a pure view (no lifecycle ownership).
- This replaces `WorkspaceDev`'s module-load single-task wiring.

### Records (design lines 624–747) — pure derivations + views
All derivations are pure functions with unit tests; each view shows an empty-state when there is no data.
- **Create `apps/web/src/shell/records/activityCalendar.ts`** — `deriveActivityCalendar(events: {created_at}[], now)` buckets all message/card/eval timestamps into the last 17 weeks (119 days, 7 rows × 17 cols, column-major). Each cell → intensity 0–4 (count thresholds) → one of the 5 design swatches `#EDEFF4/#C9D0E8/#97A3D2/#5C6CB0/#2A3B7A`. Also returns `{ activeDays, weeks }` for the "过去 17 周 · 共 N 天有思考" header.
- **Create `apps/web/src/shell/records/growthReviews.ts`** — `deriveGrowthReviews(evaluations)` → `{ period, text }[]` from eval narratives, newest first; `period` from `created_at`.
- **Create `apps/web/src/shell/records/cardUsage.ts`** — `deriveCardUsage(cards, registry)` → groups by the 8 registry categories: `{ name, dotStyle, countLabel, cards: [{ name, badge, badgeStyle, purpose, usage, boxStyle }] }`. Badge from usage count (used vs 未用). `usage` = "用过 N 次".
- **Create `apps/web/src/shell/records/ability.ts`** — `deriveAbility(evaluations, FULL_RUBRIC)` → for each of the 9 dims, aggregate the SOLO level across evaluations (latest evaluation's score per dim; dims never scored → L1/未评估). Returns `abilityList` (`{ dim, levelLabel, segs, dotStyle }`) reusing the S5 seg logic.
- **Create `apps/web/src/shell/records/radarGeometry.ts`** — pure trig: from the 9 levels (1–4) compute `radarRings` (4), `radarAxes` (9), `radarPoints` (filled polygon), `radarDots` (9), `radarLabels` (9, with anchor) over the design's 280×270 viewBox.
- **Create `apps/web/src/shell/records/RecordsView.tsx`** — header + 3-tab switch (学习记录/工具卡/能力素养) rendering `LearningTab` (calendar + reviews), `CardsTab` (usage groups), `AbilityTab` (radar svg + ability list). Tab state from `recordTab`.

### Settings (design lines 749–828) + LLM config
- **Create `apps/web/src/shell/settings/LlmConfigForm.tsx`** (shared) — provider (openai/anthropic) · API key (password) · base URL · model · evalModel · **测试连接** button that runs a real minimal `chat()` ping (1 short message). Success → save config with `verified: true` + a ✓ state; failure → ✗ + error text (no key in the error). Reused by both the settings section and the key-gate.
- **Create `apps/web/src/shell/settings/SettingsView.tsx`** — design verbatim: 个人 (profile, cosmetic inputs), AI 形象 (avatar color picker → persists `session.aiAvatar`, with `avatarOptions` highlight), **模型 / API** (NEW card wrapping `LlmConfigForm`), 其他 (cosmetic toggles, local state) + 退出登录 (`onLogout` → `session.authed = false`, screen → 'login').
- **Create `apps/web/src/shell/KeyGateModal.tsx`** — non-dismissable overlay (no scrim-close, no X) mounting when `llmConfig` is absent OR `!verified`. Wraps `LlmConfigForm`; only a passing 测试连接 (→ saved + verified) lets it close and the app become usable.

### AppShell + entry
- **Create `apps/web/src/shell/AppShell.tsx`** — owns `screen / tab / taskView / recordTab / activeTaskId`; renders auth vs app; mounts `LeftRail`, `KeyGateModal`, and the content by tab. Nav handlers (`onTabTasks/Records/Settings`, `onOpenTask`, `onBackToDir`, `onEnterApp`, `onLogout`).
- **Modify `apps/web/src/main.tsx`** — mount `AppShell` instead of `DevApp`.

## Contracts — canonical 9-dim rubric (retires the carry-forward)

- **Modify `packages/contracts/src/rubric.ts`:**
  - **Delete** legacy `RUBRIC_TAGS` / `RubricTag` (the misaligned D1_来源意识/D2_交叉验证/D5_论证结构/D7_对立观点处理 set).
  - **Replace** `DEMO_RUBRIC` (5 dims) with **`FULL_RUBRIC`** — 9 dims D1–D9. Keep D2–D6 anchors verbatim from S5. Author D1/D7/D8/D9 anchors from each dim's authoritative definition + the SOLO scale:
    - **D1 提问清晰度** (ATL 思维 · QUEST-Q · 输入) — 能否给 AI 充分上下文。
    - **D7 论证质量** (QUEST-S · ATL 沟通 · 输出) — 产出是否符合结构标准。
    - **D8 信息再生产** (ATL 媒介伦理 · 学术诚信 · 输出) — 是否区分 AI 与个人贡献。
    - **D9 AI 边界与伦理** (TOK 知识与技术 · 伦理使用 · 输出) — 能否识别 AI 的局限。
  - Keep `SoloLevel`, `SOLO_LABELS`, `RubricDimension` unchanged.
- **Update the 2 demo card JSONs** (`sift_craap`, `concession`) `rubric_dims` to reference canonical D-ids (drop legacy tag strings).

## Eval — 9-dim (minimal change, mostly data)

`buildEvalPrompt` and `evalView` are already rubric-arg-driven.
- **Modify `apps/web/src/agent/evalPrompt.ts` callers** to pass `FULL_RUBRIC`; extend the Phoebe few-shot to score all 9 dims (real reasoning per dim + 9-entry JSON).
- **`runEvaluation`** parses N `DimScore`s (already generic) — pass `FULL_RUBRIC`.
- Tests assert structure: prompt names all 9 dims, parser accepts 9 scores, `evalView` renders 9. **Live re-validation is manual** (needs the user's key).

## S5 carry-forwards folded in

- **Eval error UX:** `EvalModal`/host handles `phase === "error"` → show a message + 重试 button (currently shows nothing).
- **Eval re-run guard:** 生成思维印记 when an evaluation already exists → confirm/relabel as 重新评估 before overwriting `getLatestEvaluation`.
- **setState-swap audit:** verify `createConversation` and `createStore` gate the state-ref swap + `notify()` on an actual change (the pattern fixed in `createEvaluator`); fix + add a stability test if not.

## Data flow

- Auth: AuthScreen → `session.authed = true` → AppShell renders 'app' → KeyGateModal blocks until `llmConfig.verified`.
- New task: DirectoryView input → `createTask` + seed student message → `activeTaskId` → WorkspaceContainer builds controllers → existing S3b/S4/S5 flow (chat → summon_card → tree → 生成思维印记 → eval).
- Records: read the store snapshot via `useStore`; pure derivations recompute in render → real-time. No new storage.
- Settings: avatar → `session`; LLM config → `mk.llmConfig`. Both via reactive holders.

## Error handling

- Store load already fail-soft (missing→empty, corrupt→empty+warn). `session` and `config` follow the same fail-soft pattern.
- 测试连接 failures surface a user-visible ✗ + message, never the key.
- Key-gate cannot be bypassed while `!verified`.
- Empty records (no tasks/cards/evals) → each tab shows its own empty-state, never a crash or fake numbers.

## Testing

- **Pure (unit):** `activityCalendar` bucket/intensity, `growthReviews` ordering, `cardUsage` grouping/badges, `ability` aggregation (incl. unscored→L1), `radarGeometry` point math, `taskCardView` mapping. `FULL_RUBRIC` shape (9 dims, anchors present), eval prompt names all 9, parser accepts 9.
- **Reactive:** `session` get/set + stable snapshot; `config` hardening fallback.
- **Components (render):** AuthScreen step nav + onEnterApp; LeftRail active state + tab switch; DirectoryView new-task creates + routes to workspace; WorkspaceContainer recreates controllers on activeTaskId change + back-to-directory; RecordsView tab switch + empty-states; SettingsView avatar persist + logout; KeyGateModal blocks until verified; eval error/​re-run UX.
- **Key-safety:** assert key never in store/session/logs/errors/render; 测试连接 path verified.
- Gate every task: `pnpm -r typecheck` + `pnpm -r test`.

## Out of scope (unchanged carry-forwards)

Real auth/backend/server-side key/metering; full skill tree; 3c rich card interactions; semantic tree enrichment + nodeView depth-walk; Marcus/Ethan/Eliza few-shot expansion; eval benchmark/fine-tune; streaming; teacher side. Toggles in settings are cosmetic (no behavior). Profile inputs are cosmetic.

## Conclusion

Shipping S6 makes the platform end-to-end runnable end-to-end (PRD §2 all four success criteria reachable from a cold start): a student logs in, enters their key, creates a real task, is coached through tool cards, watches the process tree grow, generates their 思维印记, and sees it accumulate in 记录.
