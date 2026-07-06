# Per Aspera Site — Product-Centric Refactor (2026-07-06)

> Executes the 2026-07-06 strategy update in `docs/astranova/website_refactor_0706.md`.
> Branch: `feat/peraspera-site`. Builds on the dark redesign already live.

## The one big shift

Before, the site presented **three parallel services** (coaching / academy / partnership) and
buried 思维印记 (Mind Imprint) inside the partnership page as a school-facing *evaluation* tool.

Now the whole story is **product-first**:

- **思维印记 (Mind Imprint) is the ONE core product** — an integrated AI education product with
  **three components**:
  1. **AI-assisted interactive courses** — live, small-group, AI-assisted. **No recorded video
     lectures** (passive video is a pre-AI format). Two threads: **critical thinking (思辨)** and
     **product thinking (产品思维)**. The critical-thinking thread is grown from 30+ courses proven
     in real international (IB) classrooms.
  2. **Two AI-guided workbenches (工作台)** — interactive task-completion platforms where AI guides,
     probes and challenges but never concludes for the child:
     - **批判性思维工作台 / Critical Thinking Workbench** — questioning, source verification,
       argument construction & revision.
     - **产品思维工作台 / Product Thinking Workbench** — real, age-adapted problems worked from
       problem statement to a shipped prototype, with AI as a colleague whose output must be verified.
  3. **过程评估 / Process assessment** — turns the reasoning *process* into a report card. Every
     interaction inside courses and workbenches is assessment data. (Already faithfully rendered on
     the current partnership page — reuse it.)
- **Three services are built ON the product** (services layer on top, never the other way around):
  - **学校合作 (Schools, 2B)** → `/partnership`
  - **课程 / 家庭 (Families, 2C — courses + winter/summer camps)** → `/academy`
  - **申请辅导 (Admissions coaching, 2C — highly selective, capped)** → `/coaching`

The two abilities we focus on right now: **思辨能力 (critical thinking)** and **产品思维 (product thinking)**.

## Public vs. confidential boundary (IMPORTANT)

`website_refactor_0706.md` is a **confidential BP**. The website is **public**, parent- & school-facing.
Translate the *strategy* into warm, plain copy. **Do NOT publish**: dated roadmap (Aug 2026 demo …),
"assessment is the moat" / "antifragile" investor framing, Palantir fellowship stats, competitor
tuitions ($75k), the κ≥0.7 validation study, "2B/2C" labels, "free pilot" as a dated promise. These
stay internal. Soft, true public versions are fine ("our courses grew in real IB classrooms";
"we're beginning to pilot with schools").

## Faithful product facts (do not distort)

- Process assessment cognitive model = **two faces** (生成式驾驭 🚀 / 批判式防护 🛡️) → **10 dimensions
  (D1–D10)** → **SOLO L1–L4** (萌芽 / 发展中 / 熟练 / 卓越). The current site is accurate; KEEP 10 dims.
  (The BP says "9" — that is a BP simplification; the real product is 10. Flagged to user.)
- Real product truth we MAY add: an **initiative gradient** — the process records whether the student
  reached for a thinking step on their own, only when prompted, or not at all (主动 / 被动 / 缺失).
  This is genuine ("过程即数据" — skipping a tool-card is also recorded). Fine to surface lightly.
- Assessment principles unchanged: **private to the student**, **always the strongest model**,
  **offered, never pushed** (no badges / leaderboards / auto-open).

## Global copy rules (bind every task — unchanged from prior rounds)

- **Plain wording, NO jargon.** Write the way a normal parent talks. Banned: 留资 / 漏斗 / 触点 /
  转化 / 赋能 / 抓手 / 闭环 / 心智 / 打法 / 对齐 / 颗粒度 / 生态位 + English buzzwords. Also avoid
  productizing jargon that a parent wouldn't parse ("工作台" is OK if introduced plainly as
  "一个让孩子和 AI 一起完成任务的互动界面").
- **No antithesis** in any form: `不是…而是` / `不…而…` / `而非` / `而不是` / "not X but Y" /
  contrastive "rather than". Positive declaratives only.
- **Single language per locale.** zh at `/`, en at `/en/`. On `/en/` **no Chinese may render**
  (except the 中/EN toggle glyph). zh is the source of truth; en is idiomatic, not literal.
- No lorem. No deadlines / 说明会 / 冲刺营 / 亚洲时区. Warm, concrete, parent-legible.
- Git hygiene: explicit `git add` of only peraspera files; never root `package.json` /
  `packages/contracts/src/evaluation.test.ts` / any `.env`. Trailer exactly:
  `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.
- Build gate: `pnpm --filter peraspera build` + `astro check` 0 errors.

## Routes & nav

- **NEW page `/mind-imprint`** (思维印记 / Mind Imprint) — the core product page. Three components
  (courses + 2 workbenches + assessment). Absorbs & expands the deep assessment mockup currently on
  `/partnership`. zh + `/en/` twin.
- Nav (sitewide): **首页 · 思维印记 · 项目 ▾ (申请辅导 `/coaching`, 课程 `/academy`) · 合作 `/partnership`
  · 关于我们 `/about`** + [lang] + button 联系我们 `/contact`. (Add the 思维印记 top link; rename
  academy nav label 学院→课程 for parent clarity.)
- Existing service pages kept; realigned to reference the product.

## Tasks

### T0 — Nav + site.ts (spine)
Add `mindImprint` nav entry (思维印记 / Mind Imprint); rename `academy` label to 课程 / Courses.
Add the top-level 思维印记 link in `Nav.astro` (desktop + mobile-subnav untouched; it's a top link,
not under the dropdown). Verify active-state logic covers `/mind-imprint`.

### T1 — NEW product page `/mind-imprint`
`src/content/mind-imprint.ts` + `src/components/pages/MindImprintPage.astro` + `src/pages/mind-imprint.astro`
+ `src/pages/en/mind-imprint.astro`. Sections:
1. **Hero** — 思维印记 is our core product: one AI education product that carries the whole way of
   learning — courses, workbenches, and an assessment that sees the thinking process.
2. **Why a product** — in the AI era, ability has to be *built and seen*; we productized this so every
   child gets strong AI guidance plus a real record of how their thinking grows. (No BP moat language.)
3. **Component 1 · AI-assisted interactive courses** — live, small-group, AI-assisted; no recorded
   video; two threads (思辨 + 产品思维); grown from 30+ real-classroom courses. Link → `/academy`.
4. **Component 2 · Two workbenches** — introduce plainly: an interactive space where a child works a
   real task WITH AI, and the AI keeps asking them to think for themselves. Two of them: Critical
   Thinking Workbench + Product Thinking Workbench. Use the real product's 思维工具卡 idea (AI pauses
   and hands the thinking back at the right moment).
5. **Component 3 · Process assessment** — MOVE the faithful dark interface mockup + two-faces/10-dim/
   SOLO/principles content here from partnership (read the current `PartnershipPage.astro` + the
   `mi-` scoped classes in `global.css`; reuse verbatim where possible). Add the initiative-gradient
   note. Keep private / strongest-model / offered-not-pushed.
6. **Closing** — this one product powers everything we do → three short links (coaching / courses /
   schools) + 联系我们 → `/contact`.

### T2 — Home reframe (`home.ts` + `HomePage.astro`)
Keep the 6-screen full-screen structure. Reframe so the PRODUCT is the spine:
- Hero: sharpen to the two abilities (raise young people who can think critically and turn problems
  into products, alongside AI). Keep two buttons.
- Mission (ANL): keep, tighten.
- **Replace/rework "What we offer"** into a **product-first** telling: first a screen introducing
  **思维印记, our core product** (one line each on courses / workbenches / assessment, link →
  `/mind-imprint`), then the three services as "built on it" (coaching / courses / schools). This may
  become two screens or one screen with the product feature + three service cards below — implementer's
  call, keep it clean and full-screen-friendly.
- Beliefs: keep (single language). Optionally align one belief to "growth has to be visible."
- Team: update the intro/bios to match the new team framing (see T5).
- FAQ: keep; ensure the Astra-Nova-relationship + no-guarantee items remain.

### T3 — Academy realign (`academy.ts` + `AcademyPage.astro`)
Frame courses as **AI-assisted interactive courses, no recorded video**, on **two threads (思辨 +
产品思维)** — the existing 6 modules stay (they already cover critical-thinking + a building module);
name the two threads explicitly and fold the modules under them, or keep modules and add a short
"two threads" intro. Add that **families can also join winter/summer camps** (concentrated, immersive)
— light mention, no dates. Note courses run on 思维印记 (link → `/mind-imprint`). Keep personalized-plan
+ showcase. No time zones, no deadlines.

### T4 — Partnership realign (`partnership.ts` + `PartnershipPage.astro`)
Keep the schools story (理念 → 方案 → details). Two solutions stay (build the AI teaching system +
AI camps). Since the deep 思维印记 mockup MOVES to `/mind-imprint`, replace the long in-page product
deep-dive with a **tighter product summary + a clear link → `/mind-imprint`** ("完整了解思维印记").
Keep course-system + teacher-training detail (those are school-specific). Soft public "we're beginning
to pilot with schools" is OK; no dated free-pilot promise.

### T5 — About realign (`about.ts` + `AboutPage.astro`)
Update founder bios to the new framing (keep plain, no BP buzzwords):
- 陈玉洁 — CEO · 联合创始人 · 教育研究院负责人. UN ESG & climate programs + IB coaching; creator of the
  思维印记 approach: 30+ critical-thinking courses + the process-assessment rubric, all grown in real
  classrooms.
- 侯煜欣 — 联合创始人 · 产品负责人. Serial founder; educator in AI, product & computational thinking;
  leads 思维印记 productization (workbench interaction, assessment engineering, AI tuning).
Keep letter / name-origin / FAQ; tighten letter to mention the product lightly. No dates, no moat talk.

### T6 — Build + Chrome check + Deploy
- `pnpm --filter peraspera build` + `astro check` = 0 errors; grep changed files for forbidden terms.
- Chrome: load every page zh + `/en/`; verify (1) looks good, (2) en = English only, zh = Chinese,
  (3) no jargon / unintelligible words, (4) internal links resolve.
- Deploy to Vercel production; smoke-test all routes 200 + /api/lead.

## Verification per task
Build + astro check 0; single-language-per-locale (no Chinese on `/en/`); no antithesis / jargon /
deadlines; internal links resolve; dark theme legible; mobile OK.
