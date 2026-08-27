# Lite Edition · Atom Foundation and Reading / Writing — Design Spec

> 2026-08-26 · Status: not yet implemented.
>
> **This is a program-level design**, covering the overall shape of P1–P3. Consistent with the repo's existing practice, **each phase gets its own spec → plan → build**; the implementation plan that follows immediately covers only P1 from §10.

## 1. Background and Goals

The existing product serves IB / AP Seminar the student, built around a heavyweight **project lifecycle** (立题 (topic proposal) → management → reading → writing → review). The new customer segment is **ordinary schools**: they don't run research projects, and they don't need evidence maps, proposal tracks, or an essay-track state machine. What they want is **a single reading exercise** and **a single writing exercise** — each independent, repeatable, and completable within one class period.

Lite Edition is therefore an **independent frontend site** + **an independent set of backend tables**. What it shares with the existing product is **capability**, not **data structure**.

### Goals

- An independent frontend site whose left sidebar has only two items: **Writing** and **Reading**.
- **Reuse AI capability, frontend design, and design tokens**; **do not reuse** project's tables and handlers.
- Reading gets a new **comprehension check**: after finishing, the AI asks a few questions to verify whether the student actually understood it.
- A new **lite report**, replacing the project-level `EvaluationReport`.
- Writing supports both Chinese and English; **English writing provides an exemplar paragraph**.
- Accounts are routed by **school** edition.
- **Lay groundwork for future AI chat and AI projects**: adding a new form = adding one table + reusing the foundation, instead of rewriting messages, cards, annotations, and reports all over again.

### Non-goals (not in this round)

- Do not migrate the existing pro `project` onto the new foundation. The existing project's data structure is left untouched; if project is ever to become one form of atom, that gets its own spec.
- the teacher assigning reading tasks, small in-class chats, small-scale PBL — the foundation leaves room for these in shape, but they are not implemented.
- Lite Edition has no 图鉴 (compendium) / course / project tabs.
- No change to any behavior of the existing (pro) version.
- AI does not write body text on the student's behalf (see §2).

## 2. The Design 铁律 (Iron Rules) We Hold

| 铁律 | How it lands in Lite Edition |
|---|---|
| ① AI never writes body text on the student's behalf | Outline generation and 「合成全文」("compose the full draft") are both **deterministic system steps** (explicitly allowed by AGENTS.md): composing only concatenates fragments the student wrote themselves, never inventing a single new character. The English exemplar paragraph is **explicitly labeled as an exemplar**, separated from the draft both structurally and visually, and is never auto-inserted. |
| ② No manipulation | No streaks, no leaderboards, no badges, no push notifications. 工具卡 (tool card) trigger automatically but are opened only on the student's confirmation, same as today. |
| ③ Ask one thing at a time | Both the 陪练 (coach) dialogue and the guiding-question blocks follow the existing pacing (`ApplyReadingGate`'s pacing is reused as-is). |
| ④ Process is data | Skipping a 工具卡 and letting the AI answer directly is likewise recorded, and flows into the deterministic-facts section of the lite report. |

## 3. Reuse Boundary (the core judgment call of this design)

**Reuse capability, not data structure.** This boundary holds because the reading AI layer was already storage-agnostic to begin with:

```go
// apps/api/internal/agent/reading_router.go
type ReadingRouteInput struct {
    StudentText    string
    Article        string        // 文章正文，纯字符串
    FocusedSpans   []FocusSpan
    RecentTurns    []string
    Catalog        []ReadingCard
    ScaffoldLevels map[string]int
    Pacing         PacingState
    Brief          ReadingBrief
}
func RouteReading(ctx, p gateway.Provider, resolver gateway.KeyResolver, in ReadingRouteInput) (ReadingDecision, ...)
```

**`ReadingRouteInput` contains no project reference at all**, and `RouteReading` / `ApplyReadingGate` / `ResolveExampleAnchor` / `ReadingDeck` are all pure functions. The handler just needs to assemble these values from its own tables — this AI layer doesn't need a single line changed.

| Reuse | Do not reuse |
|---|---|
| `internal/agent`'s reading brain: `RouteReading`, `ApplyReadingGate`, `ResolveExampleAnchor`, `ReadingDeck`, and related prompts | pro's handlers (not one of `/projects/*` is touched) |
| `internal/gateway`: model routing, degradation, token and cost metering | project lifecycle (立题 (topic proposal) / planning / evidence map / exploration / essay-track / review) |
| `internal/docextract`: real PDF / DOCX body-text extraction | project's `reference` / `material` / `card_instances` / `outline_node` / `snippet` / `draft_snapshot`, and other project sub-tables |
| `internal/cards` + `packages/contracts`: card specs and the standard envelope contract | the project-level `EvaluationReport` |
| frontend room components, interaction design, **design tokens (`mk-*`) and CSS** | pro's shell, navigation, 图鉴, courses |

## 4. Domain Model: Atom Foundation

### 4.1 Why a Foundation

After reading comes writing, and after that, **AI chat** and **AI projects**. What differs between these four is their own dedicated fields; **what's the same across all four is four things**: one conversation message stream, a batch of tool-card instances, one annotation layer, one lite report. If a separate set were written for each form, these four things would need to be written four times and fixed four times.

Hence: **a thin shared-identity + shared-mechanism layer, with dedicated fields each in their own table.**

```
atom                身份：id / kind / user_id / created_at
├── reading         阅读专属字段                （writing / chat / project 之后并列加入）
├── atom_message    对话消息流        ← 四种形态共用
├── atom_card       工具卡实例（标准信封）  ← 四种形态共用
├── atom_annotation 批注层            ← 四种形态共用
└── atom_report     简版报告          ← 四种形态共用
```

Adding a new form = **adding one dedicated table + reusing the four shared tables**. The shared tables use **real foreign keys** pointing at `atom`, not polymorphic columns like `(owner_kind, owner_id)` — that would lose referential integrity.

### 4.2 Identity and Form

```sql
CREATE TABLE atom (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind       text NOT NULL CHECK (kind IN ('reading','writing')),
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX atom_user_kind_idx ON atom (user_id, kind, created_at DESC);

CREATE TABLE reading (
  atom_id     uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  title       text NOT NULL DEFAULT '',
  lang        text NOT NULL CHECK (lang IN ('zh','en')),
  status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active','finished')),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz
);
```

> The `kind` CHECK relaxes as more forms are added (`'chat'`, `'project'`) — a one-line migration. The `writing` table has the same shape as `reading`, landing with P3.

### 4.3 Shared Mechanisms

```sql
-- 对话消息流。seq 由服务端分配，(atom_id, seq) 唯一，保证顺序可靠。
CREATE TABLE atom_message (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  seq        integer NOT NULL,
  role       text NOT NULL CHECK (role IN ('student','ai','system')),
  content    text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX atom_message_seq_idx ON atom_message (atom_id, seq);

-- 工具卡实例。标准信封结构与 pro 一致（这是过程树与评估的共同地基，不得随意改）；
-- Go 只做边界校验（status 枚举 / id / field_values 为对象 / event_trace 为数组），
-- 内层深结构的真相仍归 packages/contracts 的 Zod 契约。
CREATE TABLE atom_card (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id      uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  card_id      text NOT NULL,
  block_id     text,                    -- 悬挂在正文哪一段（阅读）
  status       text NOT NULL CHECK (status IN ('proposed','active','submitted','skipped')),
  field_values jsonb NOT NULL DEFAULT '{}'::jsonb,
  event_trace  jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at   timestamptz NOT NULL DEFAULT now(),
  submitted_at timestamptz
);
CREATE INDEX atom_card_atom_idx ON atom_card (atom_id, created_at);

-- 批注层：学生划的线与写的话。
CREATE TABLE atom_annotation (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  block_id   text NOT NULL,
  span       jsonb NOT NULL,            -- {start,end}，与前端 Annotate 的 span 同形
  quote      text NOT NULL DEFAULT '',
  note       text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX atom_annotation_atom_idx ON atom_annotation (atom_id, created_at);

-- 简版报告（P2 起用）。沿用既有 evaluation 的原子 ON CONFLICT 单飞 + 三态 + 轮询。
CREATE TABLE atom_report (
  atom_id      uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  status       text NOT NULL CHECK (status IN ('pending','ready','failed')),
  payload      jsonb,
  error        text,
  created_at   timestamptz NOT NULL DEFAULT now(),
  generated_at timestamptz
);
```

### 4.4 Reading-Specific

```sql
-- 文章正文：一次阅读只有一篇。body 是抽取后的可读正文（复用 internal/docextract）。
CREATE TABLE reading_source (
  atom_id     uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  title       text NOT NULL DEFAULT '',
  body        text NOT NULL,
  source_url  text,
  bib         jsonb,
  ingested_at timestamptz NOT NULL DEFAULT now()
);

-- 读前定位：为什么读、读什么。直接喂给 ReadingRouteInput.Brief。
CREATE TABLE reading_brief (
  atom_id        uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  phase_tag      text,
  reading_reason text NOT NULL DEFAULT '',
  reading_focus  text NOT NULL DEFAULT '',
  updated_at     timestamptz NOT NULL DEFAULT now()
);

-- 我的收获：学生自己的话。
CREATE TABLE reading_takeaway (
  atom_id    uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  text       text NOT NULL DEFAULT '',
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- 理解检测（P2）。answerKey 只留服务端，不下发；判分在服务端做。
CREATE TABLE reading_check (
  atom_id      uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  questions    jsonb NOT NULL,
  answers      jsonb,
  score        integer,
  total        integer,
  generated_at timestamptz NOT NULL DEFAULT now(),
  answered_at  timestamptz
);
```

### 4.5 Cost Attribution

`llm_call` already has `project_id` (nullable; course/chat calls are NULL) and a `surface` discriminator column. Lite Edition's calls record `project_id = NULL`, `surface = 'lite'`, and add one new column pointing back to the atom:

```sql
ALTER TABLE llm_call ADD COLUMN atom_id uuid REFERENCES atom(id) ON DELETE SET NULL;
CREATE INDEX llm_call_atom_created_idx ON llm_call (atom_id, created_at DESC);
```

The `llm_usage` view aggregates by `user_id` / tier / tokens / cost, and **does not read `project_id`**, so school-level cost roll-ups pick up Lite Edition automatically with no changes needed.

### 4.6 Site Routing: Edition Belongs to the **School**

**Whether a school has bought Lite Edition or the existing edition determines where accounts within it land.** When a student registers with a join code, they enter a class → the class belongs to a school → that school's edition determines which site the account lands on. So **no column is added to `users`**.

```sql
ALTER TABLE schools ADD COLUMN edition text NOT NULL DEFAULT 'pro'
  CHECK (edition IN ('pro','lite'));
```

The organizational invariant is unchanged: every account still must belong to a school + ≥1 class, and registration still requires a join code.

**Server-side determination:** the session principal (`api.User` in `apps/api/internal/api/auth.go`) already carries `SchoolID`; the auth gate reads the school's edition from it — no new account field is needed.

**`/auth/me` does not return the edition.** The frontend doesn't need to know it: in the future each school will use its own subdomain, and the domain itself already determines which site to enter; a request that hits the wrong site is uniformly 404'd by the server (the same semantics as an ownership failure, so the existence of the other side is never leaked).

> If, in the future, "a few accounts at the same school need to go to the other side" becomes a requirement, add one nullable `users.edition` override layer — per YAGNI, this round doesn't do that.

## 5. API

`{id}` is always the **atom id**. All handlers belong to Lite Edition itself; **none of pro's handlers are passed through or reused**.

```
GET    /api/v1/readings                     我的阅读列表
POST   /api/v1/readings                     {title?, lang} → 建 atom + reading（同一事务）
GET    /api/v1/readings/{id}                原子 + source 概要 + 进度
PATCH  /api/v1/readings/{id}                改标题
PUT    /api/v1/readings/{id}/source         贴正文 / 上传文件 / 贴链接
GET    /api/v1/readings/{id}/source         正文与分段（blocks）
GET    /api/v1/readings/{id}/brief
PUT    /api/v1/readings/{id}/brief
GET    /api/v1/readings/{id}/messages
POST   /api/v1/readings/{id}/turn           陪练一轮：装配 ReadingRouteInput → RouteReading
                                            → ApplyReadingGate；可能返回 summon 决策
GET    /api/v1/readings/{id}/annotations
POST   /api/v1/readings/{id}/annotations
POST   /api/v1/readings/{id}/cards/{cid}/activate
POST   /api/v1/readings/{id}/cards/{cid}/skip
POST   /api/v1/readings/{id}/cards/{cid}/submit
GET    /api/v1/readings/{id}/takeaway
PUT    /api/v1/readings/{id}/takeaway
POST   /api/v1/readings/{id}/finish

-- P2
GET    /api/v1/readings/{id}/check
POST   /api/v1/readings/{id}/check/generate
POST   /api/v1/readings/{id}/check/answer
GET    /api/v1/readings/{id}/report
POST   /api/v1/readings/{id}/report/generate
```

### 5.1 Authorization

- Atom endpoints are readable/writable only by their owner; an ownership failure is always **404**, and existence is never leaked (following the existing convention).
- An account whose school has `edition = 'lite'` accessing `/projects/*` → 404; an account with `edition = 'pro'` accessing `/readings|writings/*` → 404. Same semantics as an ownership failure; no new error code is added.
- Endpoints that consume tokens still gate on `HasEntitlement(ctx, user)` as before.

## 6. The Two Main Flows

### 6.1 Reading

```
新建 → 放入文本（粘贴 / 上传 / 链接）
     → 精读：透镜库、段落上悬挂工具卡、锚点选取、批注、AI 引导
     → 我的收获（takeaway，学生自己的话）
     → 理解检测（P2）
     → 简版报告（P2）
```

The interaction is **the same as** pro's 阅读室 (reading room) (same lens, 工具卡, annotation, and one-question-at-a-time pacing), because the frontend components and the AI brain are the exact same set. The only two differences: **there's no 立题**, so the wrap-up doesn't write "new leads / impact on the topic" but instead "我的收获 (my takeaway) + comprehension check"; and **there's no evidence map or exploration**, so the related panels don't appear.

### 6.2 Writing

Writing has the same shape as reading: **one box goes in, one focused task comes out.**

#### 6.2.1 Entry

The landing page shares the same skeleton as reading's (see §7.2b): a centered greeting + one box + recommendations + a "我的写作" (My Writing) entry in the top right + a hint bar.

**the student types "what do you want to write" directly into the box** — it's not a form, and there's no picking a genre first. the student types the thought in (one sentence is enough), and lands on the writing page.

The landing page likewise offers: **suggested topics** (for when you don't know what to write), **history** (unfinished items on top — click to continue; finished items — click to view the report), and **the teacher-assigned writing tasks** (wired in P4; the slot is reserved now).

#### 6.2.2 Once Inside: Talk First, Then Write

Once on the writing page, **the AI discusses the idea with the student first** — not immediately handing over an outline, and not immediately telling them to write. One question at a time (铁律③).

Then the AI breaks the whole thing into **four stages**, and shows the student where they currently are:

```
构思 → 大纲 → 段落 → 成稿
```

**构思 (ideate)**: just **talk simply** about the topic — try to connect it to **evidence, ideas, or a story**. This stage also has one thing to settle: **how long this piece should be** (an order-of-magnitude word count). Length determines the granularity of the later outline and the number of paragraphs; without it, every step after this is guesswork.

**Who "must" here binds (resolved 2026-08-27 — the obligation is the coach's, never the server's).** An earlier draft of this section read as if length *had* to be set before 构思 could be left, which would make it a gate and collide with 铁律② (stages are a map, not a gate). The resolution:

- The **coach must raise it** while the student is in 构思. That is what "settle it here" means — an obligation on the AI to ask, subject to 铁律③ (one question at a time), not a precondition on the student to answer.
- The **server must not block** anything on `target_words`. No stage transition, no outline generation, no compose step may 403 or refuse because it is NULL. A student who ignores the question keeps moving.
- **Downstream steps degrade honestly rather than guess.** With no target, the outline is generated without length guidance and the paragraph stage does not imply a paragraph count. Nothing invents a number on her behalf.
- **Leaving it unset is recorded, not corrected** (铁律④). `target_words` stays NULL and the report's deterministic-facts section simply omits it — an omitted field, never a zero and never an estimate.

So plan Task 3 is right to keep `targetWords` optional at every stage, and this section is not asking for a gate.

**大纲 (outline)**: **derived** from what the student has already said out loud, and the student can edit it. Generating the outline is a deterministic system step, explicitly allowed by AGENTS.md — **it is not writing on the student's behalf**.

**段落 (paragraphs)**: **guided fragment writing** — one paragraph at a time, with guiding questions appearing beside it as blocks. **English writing comes with an exemplar paragraph**: explicitly labeled as an exemplar, separated from the draft both structurally and visually; the interface offers no one-click insert, and the exemplar text is never written into any draft table (铁律①).

**成稿 (compose)**: **stitch** the fragments the student wrote themselves **into a full text** — only concatenation, never inventing a single new character. Then the AI gives feedback on the whole piece (structure, argument, clarity) — still not writing on the student's behalf.

Finally: finish → lite report (§8).

#### 6.2.3 The Stages Are Shown, Not Gates

The four stages are **a map for the student to see**, so they know where they are and what's next. **No stage is a mandatory gate**: if the student wants to write a paragraph first and go back to fill in the outline afterward, that's allowed; wanting to skip 构思 and go straight to writing is allowed too — but **the skip is recorded** (铁律④), and flows into the deterministic-facts section of the lite report.

> The difference from pro's essay-track: pro is a state machine for research-paper stages, complete with an evidence map and a proposal track; here there are only four steps, with no 立题, no evidence map, and no essay-track state machine. What's shared is **the foundation** (`atom` / `atom_message` / `atom_card` / `atom_annotation`) and **AI capability**, not the process.

#### 6.2.4 Data

`writing`, like `reading`, is one form of `atom` (§4.2), with its own dedicated tables:

```sql
CREATE TABLE writing (
  atom_id     uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  title       text NOT NULL DEFAULT '',
  lang        text NOT NULL CHECK (lang IN ('zh','en')),
  stage       text NOT NULL DEFAULT 'ideate'
              CHECK (stage IN ('ideate','outline','snippets','draft','finished')),
  target_words integer,                    -- 构思阶段敲定的篇幅，NULL 表示还没定
  status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active','finished')),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz
);

CREATE TABLE writing_outline (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id    uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  text       text NOT NULL,
  depth      integer NOT NULL DEFAULT 0,
  position   integer NOT NULL
);
CREATE INDEX writing_outline_atom_idx ON writing_outline (atom_id, position);

CREATE TABLE writing_snippet (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  atom_id     uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  outline_id  uuid REFERENCES writing_outline(id) ON DELETE SET NULL,
  position    integer NOT NULL,
  text        text NOT NULL DEFAULT '',
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX writing_snippet_atom_idx ON writing_snippet (atom_id, position);

-- 成稿：由片段拼成，可再编辑。只有学生的字。
CREATE TABLE writing_draft (
  atom_id    uuid PRIMARY KEY REFERENCES atom(id) ON DELETE CASCADE,
  body       text NOT NULL DEFAULT '',
  updated_at timestamptz NOT NULL DEFAULT now()
);
```

`atom_message` carries the "talk first" and per-stage 陪练 dialogue; `atom_card` carries the 工具卡 used in writing; `atom_report` carries the lite report — **not one of the four shared tables needs to be newly created**, and that's exactly the payoff of the §4.1 foundation.

#### 6.2.5 API (Same Shape as Reading)

```
GET/POST  /api/v1/writings                  列表 / 新建（body: {idea, lang}）
GET/PATCH /api/v1/writings/{id}
POST      /api/v1/writings/{id}/turn        陪练一轮（复用 AI 层）
GET       /api/v1/writings/{id}/messages
POST      /api/v1/writings/{id}/stage       推进 / 回退阶段（记录，不设关卡）
PUT       /api/v1/writings/{id}/target-words
GET/PUT   /api/v1/writings/{id}/outline
GET/PUT   /api/v1/writings/{id}/snippets
POST      /api/v1/writings/{id}/compose     把片段拼成成稿（确定性，不调模型）
GET/PUT   /api/v1/writings/{id}/draft
POST      /api/v1/writings/{id}/review      AI 给整篇反馈
POST      /api/v1/writings/{id}/finish
GET/POST  /api/v1/writings/{id}/report[/generate]
```

`{id}` is always the atom id. Authorization is the same as reading: ownership failure → 404, cross-edition → 404.


## 7. Frontend

### 7.1 `apps/lite-web`

An independent directory, with its own `vite.config.ts` / `index.html` / `tailwind.config` / Dockerfile / nginx / deploy scripts, deployed to its own domain.

Depends on `@mind-imprint/contracts` and `@mind-imprint/web` (`apps/web` takes on a package name and exposes source via `exports`); **room components and design tokens are imported from source, not copied.**

Two build traps that must be handled in P1:

- **Tailwind purge:** lite's `content` glob must include `../web/src/**`, or room styles get purged out.
- **Path aliasing:** lite's vite / tsconfig must replicate web's `@/` → `apps/web/src` alias, or every internal `@/…` import inside the room fails to resolve.
- Also note that `mk-*` tokens are **bare CSS variables**: every Tailwind alpha syntax (`bg-mk-x/NN`) **produces no CSS at all**; use `linear-gradient` / `color-mix` instead, and verify in a real browser.

### 7.2 Shell

`LiteApp.tsx`, **a left sidebar + tab switching, which can auto-collapse to icons only** (same shape as the existing frontend), URL routing follows the existing "no router library" approach:

- **Writing** `/writings` — lands on one big dialog box (an AI-app shape): the student types the idea directly into that one box and writing begins; history is below it.
- **Reading** `/readings` — lands on a submission surface (paste / upload an article) + a history list. `/readings/:id` enters the 阅读室.

No 图鉴, no courses, no projects.

### 7.2b Reading Landing Page — the Shape of the Front Door (settled 2026-08-26 after the user reviewed it on a real device)

The landing page should feel like an **AI product**, not a form:

1. **A centered greeting**: the AI asks 「Hi，今天要读点什么」 ("Hi, what do you want to read today"), with **the character 读 ("read") specially designed** (a hand-drawn book / circled word / handwritten typeface) — the page's single visual anchor.
2. **A paste box**: the border has a **subtle glow** (a breathing feel). It accepts a pasted link, a pasted full body of text, and **DOCX / PDF upload** as well.
3. **Don't know what to read?**: today's recommendations. Seed data for this round; a sample library and the teacher-assigned tasks will connect to it later.
4. **Top-right entry → 我的阅读 (My Reading)**: **unfinished items on top**, click to continue; finished items below, click to view the report. History **does not go into the left sidebar** — the left sidebar is 「工作的种类」("categories of work") (reading / writing); stuffing sub-views into it would break that semantics.
5. **A hint bar**: when there are unfinished items, it shows 「你有 N 篇还没读完」 ("you have N pieces you haven't finished reading"); a slot is reserved for the teacher tasks (wired in P4).

> There are two paths to discovering history, so the entry point can stay tucked away: the hint bar is an **active reminder**, and the top-right icon is an **archive entry point**. This also keeps the landing page single-purpose — start reading a piece.

**The reading journey itself is unchanged** (lens, 工具卡, annotation, one-question-at-a-time). **In the future**, per-paragraph AI assistance can be added: analyzing craft, narrative, keywords, structure, or **read-aloud** (`/voice/tts` already exists and can be reused).

**Not adopting Cowork's "top switcher + session list below" layout.** That kind of layout serves **parallel work** — many threads alive at once, where the list is the workspace; whereas reading one article or writing one piece is a **focused task** — only one thing is in hand at any given moment, and the sidebar is just navigation. The tab shape + auto-collapse-to-icons keeps navigation clear while giving the horizontal space back to the 阅读室 (the body text, the 陪练, and the hanging cards all compete for width).

### 7.3 Room Capability Object

Room components currently have scattered direct reads of project-lifecycle facts; `demoMode` is already a precedent for this pattern. Generalize it into an object that rooms only read from:

```ts
export type RoomCapabilities = {
  mode: 'pro' | 'lite' | 'demo'
  plan, evidenceMap, explorationLeads, proposalImpact, essayTrack: boolean  // lite: false
  comprehensionCheck, exemplars: boolean                                    // lite: true
}
```

Every data in/out for the room goes through an injected `api` object — lite passes its own client, pro passes the existing one, and the component itself doesn't know which set of tables is behind it.

## 8. Lite Report (P2/P3)

`packages/contracts` gets a new `AtomReport` contract (v1): `overview` (MODEL) + `facts` (FACT, deterministic) + `strengths` (2 items) + `nextTime` (1 item).

Every fact comes from existing records, with no new collection added: word count (`agent.CountWords`), which cards were used (submitted `atom_card` rows), number of question turns (`atom_message` rows with `role='student'`), comprehension-check score (`reading_check`). Missing fields are **omitted, not zero-filled**, and never estimated.

The overview and recommendations = **one flagship-tier call** (versus four for the project report). No tiers, no rankings (铁律②). The frontend must render through a markdown renderer; the contract uses `.nullish().transform(v => v ?? [])` for nullable arrays (an existing lesson: a strict `z.array` hitting a legacy `null` in stored JSON would fail parsing for the whole report).

## 9. Risks and Verification

1. **Envelope drift**: `atom_card` and pro's `card_instances` are two separate tables, but the envelope structure must stay identical — it's the shared foundation for the 过程树 (process tree) and evaluation. Defense: both sides share the Zod contract in `packages/contracts`, the Go side only does boundary validation, and there's a contract test asserting the two envelopes have the same shape.
2. **`ReadingRouteInput` assembly drift**: no matter how cleanly the AI brain is reused, if `Article` segmentation, `FocusedSpans`, and `RecentTurns` are assembled with different semantics than pro's, AI behavior degrades. Defense: the assembly logic is its own function with its own unit tests, using the same segmentation rules as pro.
3. **Shared components regressing pro**: lite and pro share room components. Defense: the capability object, plus a test under both the lite and pro capability sets for every component touched.
4. **铁律① drift**: the English exemplar is the only place that could slide toward writing on the student's behalf. Defense: the exemplar text never enters any draft table, the UI has no insertion entry point, and both must have test assertions.
5. **Test duration**: every Go integration test in this repo spins up its own Postgres container, and a full-package run takes 10+ minutes, **exceeding the foreground command limit**. Full-package verification must run in the background, coordinated by the controller; implementers only run targeted `-run` tests.

**Test surface:** Go unit tests cover atom CRUD, assembly functions, authorization (self / cross-edition), envelope boundary validation, and the report's three-state single-flight; contract tests cover `AtomReport` and envelope shape parity; frontend components are tested under both capability sets; e2e covers one complete reading flow (paste → close reading → takeaway) and one complete writing flow. Go tests uniformly use `-timeout 1800s` + `CGO_ENABLED=0`.

## 10. Phasing

| Phase | Content |
|---|---|
| **P1 · Foundation + Reading** | Create tables `atom` / `reading` / `atom_message` / `atom_card` / `atom_annotation` / `reading_source` / `reading_brief` / `reading_takeaway`; `schools.edition` + the routing gate; `llm_call.atom_id`; all reading endpoints (including `/turn` assembling and calling `RouteReading`, lens summoning, quote-selection feedback, anchor persistence); `apps/lite-web` + capability object + shell; **DOCX/PDF upload**; **reading landing page redesign** (greeting + glow box + recommendations + My Reading panel + hint bar); end-to-end walkthrough |
| **P2 · Reading Wrap-up** | `reading_check` comprehension check (questions generated at completion to verify real understanding); `atom_report` + lite report (stats + reading notes + interaction summary) |
| **P3 · Writing** | `writing` / `writing_outline` / `writing_snippet` / `writing_draft`; one box in → AI discusses the idea first → four stages (构思 / 大纲 / 段落 / 成稿); length settled during 构思; English exemplar; writing landing page (suggested topics + history + unfinished + the teacher-task slot); writing report |
| **P4 · Later** | the teacher assigning reading and writing tasks (with deadlines and landing-page reminders); per-paragraph AI assistance (craft / narrative / keywords / structure / read-aloud, reusing `/voice/tts`); sample library; `chat` atom; AI project atom |

Each phase gets its own independent spec → plan → build.

## 11. Decisions Already Made

| Question | Conclusion |
|---|---|
| Relationship between Lite Edition and pro | Same backend service, same Postgres, same auth and organization; **independent frontend site, independent backend tables** |
| Whether to borrow the `project` row as a storage container | **No** (explicitly vetoed by the user on 2026-08-26). Atoms hold their own storage; the `project.kind` approach was rolled back. |
| What gets reused | AI functions (the reading brain was already storage-agnostic), frontend design and design tokens, card specs and the envelope contract, docextract, gateway metering |
| Why an `atom` foundation | AI chat and AI projects are on the way; the four things — message stream / tool card / annotation / report — shouldn't be written four separate times |
| Whether to migrate the existing project onto the foundation | **Not this round**; if it happens later, it gets its own spec |
| Whether 工具卡 are kept | Kept inside the room, same as pro; Lite Edition's left sidebar has no 图鉴 / course tab |
| Process evaluation | The project-level `EvaluationReport` is not used for Lite Edition; a new `AtomReport` is defined |
| Who writes the body text | the student writes every character; the AI gives outline suggestions and guiding questions; English writing provides an **exemplar** (never entering the draft) |
| Where the edition lives | **The school** (`schools.edition`), settled at registration via the join code's class → school; no column added to `users`, not returned in `/auth/me` |
