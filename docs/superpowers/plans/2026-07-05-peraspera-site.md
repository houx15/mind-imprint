# Per Aspera Site — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an independent, bilingual (zh/en), Astra-Nova-styled marketing site for Per Aspera at `apps/peraspera`, with real lead-capture to Supabase and Vercel deploy.

**Architecture:** Astro 5 static site + `@astrojs/vercel` adapter (hybrid). Marketing pages prerender to static HTML; only `src/pages/api/*` run as Vercel serverless functions. Bilingual mirrors `apps/site`: zh at `/`, en at `/en/`, `t(lang, zh, en)`, zh is source of truth. Content lives in typed `src/content/*.ts` modules (each string a `{zh,en}` pair). One design-system stylesheet.

**Tech Stack:** Astro 5, TypeScript, `@astrojs/vercel`, Supabase (server-side via service key), pnpm workspace, Google Fonts (grotesk + Noto Sans SC).

## Global Constraints

- Location: `apps/peraspera`; package `name: "peraspera"`; add to `pnpm-workspace.yaml` (already globs `apps/*`).
- Independent of `apps/web` / `apps/api` / `apps/site` — no cross-imports.
- Full bilingual: every page in zh (`/`) and en (`/en/`). zh source of truth, en high-quality idiomatic translation (not literal).
- **Copy rule: NO "不是…而是" / "not X but Y" antithesis. Write positive declaratives.** No lorem ipsum — use real copy from the source docs.
- Authoritative copy: `docs/astranova/PerAspera官网文案v2-多页版.md` (v2 5-page = authoritative) + `PerAspera官网文案v1.md` (founder letter full text §8, full FAQ §9). Research long-form sources: `docs/astranova/ad-astra-调研报告.html`, `astra-nova高中申请与真实案例.html`.
- **Apply page is lead-capture, NOT Astra's video+letter application.** Funnel = 留资 → 说明会 → 面谈. Drop all "2分钟思辨视频 + 一页家长信" copy; nav button = "预约说明会".
- Design: editorial minimalism; warm off-white paper, near-black ink, ONE deep ink-blue accent; strong grotesk headlines; Noto Sans SC for Chinese; generous whitespace; restrained reveal-on-scroll (`html.js` progressive enhancement, `prefers-reduced-motion` off).
- Secrets server-only: Supabase service key / DATABASE_URL never in client bundle, logs, or error responses. `.env.vercel` (repo root, gitignored) is the secret source; local `apps/peraspera/.env` (gitignored).
- Countdown target: 2026-10-15.
- Site-wide footer: bilingual non-affiliation disclaimer (verbatim from v2 §6).
- Verification per task: `pnpm --filter peraspera build` succeeds AND `pnpm --filter peraspera exec astro check` reports 0 errors, before commit.

---

## File Structure

```
apps/peraspera/
  package.json                 # name "peraspera", astro + @astrojs/vercel
  astro.config.mjs             # i18n(zh default, en), site, vercel adapter
  tsconfig.json                # extends astro strict
  .env                         # local secrets (gitignored)
  .gitignore                   # .env, dist, .vercel
  supabase/migrations/0001_leads.sql
  public/favicon.svg
  src/
    i18n/utils.ts              # locales, getLang, t, localePath, switchLocaleUrl (from apps/site)
    styles/global.css          # design system (tokens + component classes)
    layouts/Base.astro         # head, fonts, hreflang, reveal + accordion scripts
    lib/supabase.ts            # server-only admin client factory
    components/
      Icon.astro  IconSprite.astro
      Nav.astro  Footer.astro
      Countdown.astro          # client countdown to 2026-10-15
      Accordion.astro          # FAQ (details/summary + enhancement)
      Timeline.astro           # horizontal timeline
      TierCards.astro          # expandable tiers
      PullQuote.astro          # large position-statement
      LeadForm.astro           # lead form + fetch to /api/lead
      sections/                # per-home-section blocks (optional split)
      pages/                   # HomePage/ProgramsPage/InstitutePage/AboutPage/ApplyPage .astro
    content/
      home.ts programs.ts institute.ts about.ts apply.ts research.ts
    pages/
      index.astro  programs.astro  institute.astro  about.astro  apply.astro
      institute/ad-astra.astro  institute/schools.astro
      en/index.astro  en/programs.astro  en/institute.astro  en/about.astro  en/apply.astro
      en/institute/ad-astra.astro  en/institute/schools.astro
      api/lead.ts              # prerender=false, POST -> Supabase
```

**Page-route pattern (keeps zh/en in one component):** each `src/pages/<x>.astro` and `src/pages/en/<x>.astro` is a 3-line file that imports the shared `pages/<X>Page.astro` and renders it inside `Base` — the page component reads `getLang(Astro.currentLocale)`. Mirror `apps/site/src/pages/*` exactly.

---

## Phase A — Scaffold + design system + Home

### Task A1: Scaffold app, workspace wiring, config

**Files:**
- Create: `apps/peraspera/package.json`, `astro.config.mjs`, `tsconfig.json`, `.gitignore`, `public/favicon.svg`
- Verify: `pnpm-workspace.yaml` already contains `apps/*` (check; add if missing)

**Produces:** a buildable empty Astro app named `peraspera` with vercel adapter + i18n config.

- [ ] **Step 1: Add dependencies.** `apps/peraspera/package.json`:
```json
{
  "name": "peraspera",
  "type": "module",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev": "astro dev",
    "build": "astro build",
    "preview": "astro preview",
    "check": "astro check"
  },
  "dependencies": {
    "astro": "^5.18.2",
    "@astrojs/vercel": "^8.2.7",
    "@supabase/supabase-js": "^2.58.0"
  },
  "devDependencies": {
    "@astrojs/check": "^0.9.9",
    "typescript": "^5.9.3"
  }
}
```
- [ ] **Step 2: `astro.config.mjs`:**
```js
import { defineConfig } from "astro/config";
import vercel from "@astrojs/vercel";

export default defineConfig({
  site: "https://peraspera.example.com", // update to real domain at deploy
  adapter: vercel(),
  i18n: {
    locales: ["zh", "en"],
    defaultLocale: "zh",
    routing: { prefixDefaultLocale: false },
  },
});
```
- [ ] **Step 3: `tsconfig.json`:** `{ "extends": "astro/tsconfigs/strict" }`
- [ ] **Step 4: `.gitignore`:** lines `dist/`, `.env`, `.env.*`, `.vercel/`, `node_modules/`, `.astro/`
- [ ] **Step 5: `public/favicon.svg`:** a minimal deep-ink-blue "★" / "PA" mark SVG.
- [ ] **Step 6: Install + build.** Run: `pnpm install` then `pnpm --filter peraspera build`. Expected: build succeeds (empty site OK — Astro allows no pages? if it errors on no pages, defer build to A2). Commit.

```bash
git add apps/peraspera pnpm-workspace.yaml pnpm-lock.yaml
git commit -m "feat(peraspera): scaffold astro app + vercel adapter + i18n config"
```

### Task A2: i18n utils + Base layout + Nav + Footer + Icon

**Files:**
- Create: `src/i18n/utils.ts`, `src/layouts/Base.astro`, `src/components/{Nav,Footer,Icon,IconSprite}.astro`, `src/content/site.ts`
- Test: temporary `src/pages/index.astro` + `src/pages/en/index.astro` stubs to render.

**Interfaces:**
- Produces: `t(lang, zh, en): string`, `getLang(locale)`, `localePath(lang, path)`, `switchLocaleUrl(pathname, lang)` — copy verbatim from `apps/site/src/i18n/utils.ts`.
- Produces: `<Base title description>` layout wrapping `<Nav/><main><slot/></main><Footer/>` with hreflang + fonts + reveal/accordion scripts.

- [ ] **Step 1:** Copy `apps/site/src/i18n/utils.ts` verbatim into `apps/peraspera/src/i18n/utils.ts`.
- [ ] **Step 2:** Create `Base.astro` modeled on `apps/site/src/layouts/Base.astro` (head, viewport, `html.js` inline script, hreflang alternates, OG tags), but load fonts: a grotesk (e.g. `Space+Grotesk:wght@500;600;700` or `Archivo:wght@600;700;800`) + `Noto+Sans+SC:wght@400;500;700;900`. Include the reveal IntersectionObserver script and a `<details>`-based accordion (native, so no JS needed) — keep the reveal + `[data-tabs]` scripts from apps/site.
- [ ] **Step 3:** `src/content/site.ts` — nav labels, footer disclaimer (verbatim v2 §6 zh+en), contact placeholder, brand name "Per Aspera", each as `{zh,en}`.
- [ ] **Step 4:** `Nav.astro` — sticky minimal nav: brand left; links 课程/研究院/关于我们; right = language toggle (`switchLocaleUrl`) + primary button 预约说明会 → `localePath(lang,'/apply')`. Use `t()` for labels. `aria-current` on active.
- [ ] **Step 5:** `Footer.astro` — bilingual disclaimer, contact, © 2026 Per Aspera, social slots.
- [ ] **Step 6:** `Icon.astro` + `IconSprite.astro` — inline SVG sprite pattern from apps/site; include icons needed site-wide (arrow, check, clock, compass, shield, book, lock, star, chevron, mail, quote).
- [ ] **Step 7:** Temp `index.astro` + `en/index.astro` rendering `<Base>` with a placeholder `<h1>`. Run `pnpm --filter peraspera build` + `astro check`. Expected: 0 errors, both routes emit. Commit.

```bash
git commit -am "feat(peraspera): i18n utils, Base layout, Nav, Footer, Icon sprite"
```

### Task A3: Design system stylesheet + fonts

**Files:**
- Create: `src/styles/global.css` (imported by `Base.astro`)

**Produces:** design tokens (`--paper`, `--ink`, `--primary` deep ink-blue, neutrals, `--line`, radii, `--maxw`), base resets, typography scale (grotesk headings via `clamp()`, Noto SC), and component classes: `.container`, `.section`, `.btn` variants, `nav`, `.hero`, `.countdown`, `.beliefs`, `.pullquote`, `.timeline`, `.tiers`, `.faq`, `.table`, `.cta`, `footer`, `.reveal` (progressive), responsive breakpoints (≤900, ≤760). Learn structure from `apps/site/src/styles/global.css` but produce an entirely new deep-ink-blue/grotesk visual identity.

- [ ] **Step 1:** Define `:root` tokens. Palette: `--paper:#FAF8F4`, `--surface:#FFFFFF`, `--ink:#15171C`, `--muted:#6B7280`, `--line:#E7E3DB`, `--primary:#1E3A5F` (deep ink-blue; pick a final value with good contrast on paper), `--primary-hover`, `--primary-tint`. Radii + `--maxw:1140px`.
- [ ] **Step 2:** Base resets, body font stack (grotesk display for headings via a `.display`/`h1,h2` rule + Noto SC fallback), `::selection`, links, `.container`, `.section` rhythm (~clamp 88–128px vertical).
- [ ] **Step 3:** Buttons (`.btn`, `.btn-primary` filled ink-blue, `.btn-ghost`, `.btn-lg`), eyebrow/section-label, big `clamp()` headline scale.
- [ ] **Step 4:** Component classes for hero, countdown bar, belief cards, pull-quote, timeline, tier cards, FAQ (`details`/`summary`), tables, CTA band, footer. Keep it clean and minimal (Astra Nova restraint).
- [ ] **Step 5:** `.reveal` progressive-enhancement rules (only hide under `html.js`; reduced-motion disables) — copy the pattern from apps/site.
- [ ] **Step 6:** Responsive: collapse grids to 1col ≤900; nav links hide + smaller headings ≤760.
- [ ] **Step 7:** Build + check. Visually confirm the temp home renders with new type/paper/accent. Commit.

```bash
git commit -am "feat(peraspera): deep-ink-blue editorial design system"
```

### Task A4: Countdown component

**Files:**
- Create: `src/components/Countdown.astro`

**Interfaces:**
- Produces: `<Countdown lang={lang} />` — renders a bar "距 Astra Nova 高中申请截止（2026-10-15）还有 N 天" (en: "N days until the Astra Nova high-school application deadline (Oct 15, 2026)"). After the deadline, switches to a neutral "申请窗口已截止 · 冲刺营持续招募" style message.

- [ ] **Step 1:** Markup: a slim full-width bar with `data-countdown="2026-10-15T23:59:59-07:00"` and a `<span data-days>` slot; server-render a sensible default (compute days at build is fine but must also update client-side).
- [ ] **Step 2:** Inline `<script>`: on load, parse target, compute whole days remaining, write into `[data-days]`; if past, swap text to the closed-window message. Guard for missing element (no-op on pages without countdown). Bilingual strings passed via `data-` attributes or two prewritten spans toggled by a `lang` attr.
- [ ] **Step 3:** Build + check + render both langs. Confirm number shows and is plausible (e.g. days between build date and 2026-10-15). Commit.

```bash
git commit -am "feat(peraspera): countdown-to-deadline bar"
```

### Task A5: Home page (zh + en)

**Files:**
- Create: `src/content/home.ts`, `src/components/pages/HomePage.astro`, real `src/pages/index.astro` + `src/pages/en/index.astro`
- Optional: `src/components/sections/*` split if HomePage grows large

**Content source:** v2 §一 (Hero / 三支柱概览 / 我们相信什么 / 精选 FAQ / 底部 CTA). **Reframe** the bottom CTA per Global Constraints (预约说明会, drop video+letter).

**Interfaces:**
- Consumes: `Base`, `Nav`, `Footer`, `Countdown`, `Accordion`(if built else native details), belief/pullquote classes, `t()`.

- [ ] **Step 1:** `content/home.ts` — export objects with all Home copy as `{zh,en}`: hero headline (v2 lines 26–34, both langs already provided), CTA labels, 3 pillar cards (课程/研究院/理念), 4 belief cards (中英, verbatim v2 §我们相信什么), 3 featured FAQ, bottom CTA (reframed). Scan for and remove any 不是…而是.
- [ ] **Step 2:** `HomePage.astro` — compose: Hero (big grotesk headline, bilingual per current lang, dual CTA, `<Countdown/>`) → three pillar cards → belief cards (each shows zh + en together as in the copy doc's bilingual belief design) → featured FAQ (native `<details>`) → bottom CTA band linking `/apply`. Use `.reveal` on sections.
- [ ] **Step 3:** `index.astro` (zh) + `en/index.astro` — 3-line wrappers rendering `HomePage` inside `Base` with proper title/description per lang.
- [ ] **Step 4:** Build + `astro check`. Run `pnpm --filter peraspera dev`; load `/` and `/en/`. Verify: headline, countdown, language toggle round-trips `/`↔`/en/`, FAQ expands, mobile ≤760 looks right, no antithesis copy, no lorem. Commit.

```bash
git commit -am "feat(peraspera): bilingual home page"
```

### Task A6: Phase A E2E verification

- [ ] Start dev server; use the `verify` skill / browser to load `/` and `/en/`, screenshot desktop + mobile widths.
- [ ] Confirm: nav sticky, lang toggle keeps you on the same page in the other language, countdown renders a live day count, reveal animations fire, footer disclaimer present in both languages.
- [ ] Fix any visual/regression issues, then proceed to Phase B.

---

## Phase B — Programs + About

### Task B1: Timeline, TierCards, PullQuote, Accordion components

**Files:**
- Create: `src/components/{Timeline,TierCards,PullQuote,Accordion}.astro`

**Interfaces (used by B2/B3/C):**
- `<Timeline lang steps={[{label,date}]} />` — horizontal on desktop, vertical stack on mobile.
- `<TierCards lang tiers={[{name,hours,price,features:[]}]} />` — 3 cards (轻量/标准/完整) with expandable detail (`<details>`), like Astra Nova Level A/B/C.
- `<PullQuote lang zh en />` — large position-statement block.
- `<Accordion lang items={[{q,a}]} />` — native `<details>` list, styled.

- [ ] **Step 1:** Build each as a small focused component with props typed via an `interface Props`. No inline copy (data passed in).
- [ ] **Step 2:** Build + check with a throwaway usage. Commit.

```bash
git commit -am "feat(peraspera): timeline, tier cards, pull-quote, accordion components"
```

### Task B2: Programs page (zh + en)

**Files:**
- Create: `src/content/programs.ts`, `src/components/pages/ProgramsPage.astro`, `src/pages/programs.astro` + `src/pages/en/programs.astro`

**Content source:** v2 §二 (页首 / 冲刺营#sprint: 立场声明, 倒计时, 双轨设计, 时间线, 收费, 筛选声明 / 长线学院#academy: 为什么存在, 四条课程线, 三档, 本学期课程单). Keep `¥____` price blanks as "待定/TBD" styled cells (source intentionally blank).

- [ ] **Step 1:** `programs.ts` — all copy as `{zh,en}` (translate the zh-only sections to idiomatic en). Structured: sprint {position(pullquote), tracks[2], timeline[6], fees[], screening}, academy {why, lines[4], tiers[3], catalog[]}.
- [ ] **Step 2:** `ProgramsPage.astro` — 页首 → 冲刺营 section (PullQuote for 立场声明, `<Countdown/>`, two track cards, `<Timeline/>`, fee table, screening note) → 长线学院 section (why, four course-line cards, `<TierCards/>`, catalog table). Anchor ids `#sprint` `#academy`. `.reveal`.
- [ ] **Step 3:** zh + en route wrappers.
- [ ] **Step 4:** Build + check + dev render both langs; verify anchors, timeline, tier expansion, tables, mobile. No antithesis. Commit.

```bash
git commit -am "feat(peraspera): bilingual programs page"
```

### Task B3: About page (zh + en)

**Files:**
- Create: `src/content/about.ts`, `src/components/pages/AboutPage.astro`, `src/pages/about.astro` + `src/pages/en/about.astro`

**Content source:** v2 §四 (页首 / 创始人两卡 / 创始人信 / 名字的由来 / 完整 FAQ). Founder letter full text + full 8-item FAQ from v1 §8 and §9.

- [ ] **Step 1:** `about.ts` — founders[2] (name, role, bio, photo slot), founder letter (long, zh from v1 §8 + v2 末段; en idiomatic translation), name-origin, FAQ[8] (verbatim v1 §9, translated). Remove any 不是…而是 in the letter/FAQ when translating/editing.
- [ ] **Step 2:** `AboutPage.astro` — 页首 → founder cards → letter (editorial long-form, signature) → name origin → `<Accordion items={faq}/>`. `.reveal`.
- [ ] **Step 3:** zh + en wrappers.
- [ ] **Step 4:** Build + check + render; verify letter readability, FAQ 8 items expand, both langs. Commit.

```bash
git commit -am "feat(peraspera): bilingual about page"
```

---

## Phase C — Institute + Apply + real submission

### Task C1: Supabase leads table + server client

**Files:**
- Create: `apps/peraspera/supabase/migrations/0001_leads.sql`, `src/lib/supabase.ts`, `apps/peraspera/.env` (gitignored, seeded from `.env.vercel`)

**Interfaces:**
- Produces: `getAdminClient()` in `src/lib/supabase.ts` — returns a `@supabase/supabase-js` client built from `SUPABASE_URL` + `SUPABASE_SERVICE_KEY` (read via `import.meta.env`/`process.env`), server-only. Throws a generic error if env missing (never echo values).

- [ ] **Step 1:** Migration SQL:
```sql
create table if not exists public.leads (
  id uuid primary key default gen_random_uuid(),
  created_at timestamptz not null default now(),
  contact_name text,
  contact text not null,
  child_age text,
  interest text,
  message text,
  locale text,
  source text,
  user_agent text
);
alter table public.leads enable row level security;
-- No public policies: inserts happen server-side with the service key (bypasses RLS).
```
- [ ] **Step 2:** Apply migration to the Supabase project (via `DATABASE_URL` from `.env.vercel`: `psql "$DATABASE_URL" -f apps/peraspera/supabase/migrations/0001_leads.sql`). Confirm table exists.
- [ ] **Step 3:** `src/lib/supabase.ts`:
```ts
import { createClient } from "@supabase/supabase-js";
export function getAdminClient() {
  const url = import.meta.env.SUPABASE_URL ?? import.meta.env.NEXT_PUBLIC_SUPABASE_URL ?? process.env.SUPABASE_URL ?? process.env.NEXT_PUBLIC_SUPABASE_URL;
  const key = import.meta.env.SUPABASE_SERVICE_KEY ?? process.env.SUPABASE_SERVICE_KEY;
  if (!url || !key) throw new Error("Supabase server env not configured");
  return createClient(url, key, { auth: { persistSession: false } });
}
```
- [ ] **Step 4:** Create `apps/peraspera/.env` with `SUPABASE_URL=`, `SUPABASE_SERVICE_KEY=` (values from `.env.vercel`). Confirm gitignored (`git status` must NOT list it). Commit migration + lib only.

```bash
git add apps/peraspera/supabase apps/peraspera/src/lib/supabase.ts
git commit -m "feat(peraspera): supabase leads schema + server admin client"
```

### Task C2: /api/lead route

**Files:**
- Create: `src/pages/api/lead.ts`

**Interfaces:**
- Consumes: `getAdminClient()`.
- Produces: `POST /api/lead` accepting JSON `{contact_name?, contact, child_age?, interest?, message?, locale?, hp?}`. Returns `200 {ok:true}` on insert; `400 {ok:false,error}` on validation fail; `500 {ok:false}` on server error (no secret leakage). Honeypot field `hp` — if non-empty, return `200 {ok:true}` without inserting (silent bot drop).

- [ ] **Step 1:** Write route with `export const prerender = false;`. Validate: `contact` required, non-empty, ≤500 chars; other fields ≤2000 chars; reject if `hp` present → pretend success. Insert via `getAdminClient().from("leads").insert({...})`. Capture `user_agent` from request headers. Never include env/secret in any response or log.
- [ ] **Step 2 (test):** With dev server running and `.env` set, `curl -sX POST localhost:4321/api/lead -H 'content-type: application/json' -d '{"contact":"test@x.com","interest":"sprint","message":"plan test — delete me"}'` → expect `{"ok":true}`. Verify a row appears in Supabase (`psql "$DATABASE_URL" -c "select contact,interest from public.leads order by created_at desc limit 1;"`), then delete it. Also test missing `contact` → `400`; honeypot set → `{ok:true}` with no new row.
- [ ] **Step 3:** Commit.

```bash
git commit -am "feat(peraspera): /api/lead serverless endpoint -> supabase"
```

### Task C3: Apply page + LeadForm (zh + en)

**Files:**
- Create: `src/content/apply.ts`, `src/components/LeadForm.astro`, `src/components/pages/ApplyPage.astro`, `src/pages/apply.astro` + `src/pages/en/apply.astro`

**Content:** funnel framing (留资 → 说明会 → 面谈). Form fields per spec §3. NO video/letter.

- [ ] **Step 1:** `apply.ts` — headline, funnel explanation (3 steps), field labels + `interest` options (冲刺营 / 长线学院 / 先了解一下), submit label ("预约说明会 / Book an info session"), success + error messages — all `{zh,en}`.
- [ ] **Step 2:** `LeadForm.astro` — semantic form (labels, required on contact), hidden honeypot `hp`, hidden `locale`. Inline `<script>`: intercept submit, `fetch('/api/lead', {POST json})`, show success state (hide form, show thank-you), show inline error on failure. Progressive: works as a labelled form; JS enhances.
- [ ] **Step 3:** `ApplyPage.astro` — headline + funnel steps + `<LeadForm/>`. zh + en wrappers.
- [ ] **Step 4:** Build + check + dev; submit the form in-browser (both langs), confirm success state and a real row lands in Supabase, then delete test rows. Verify validation + honeypot. Commit.

```bash
git commit -am "feat(peraspera): bilingual apply (lead-capture) page + form"
```

### Task C4: Institute page (zh + en)

**Files:**
- Create: `src/content/institute.ts`, `src/components/pages/InstitutePage.astro`, `src/pages/institute.astro` + `src/pages/en/institute.astro`

**Content source:** v2 §三. **Mind-imprint section must match the real product** (see `apps/site/src/components/pages/EvaluationPage.astro`): two faces 生成式驾驭 / 批判式防护 → four categories → ten dimensions (D1–D10); SOLO L1–L4; private, student-only; strongest model for evaluation; offered-never-pushed (no badges/streaks). Research section previews link to `/institute/ad-astra` and `/institute/schools`.

- [ ] **Step 1:** `institute.ts` — 页首; 思维印记 {one-liner, four-step how-it-works, three design red-lines (AI never concludes for the student; rewards thinking not time-on-site; transparent to parents with real answer anchors), demo CTA ("预约演示 / Book a demo"), origin}; 公开研究 {intro, research1 preview→ad-astra, research2 preview→schools, research3 立场 short inline}. All `{zh,en}`. Keep consistent with mind-imprint's real model wording; no antithesis.
- [ ] **Step 2:** `InstitutePage.astro` — 页首 → 思维印记 block (one-liner big, 4-step diagram, 3 red-line cards, demo CTA, origin) → 公开研究 block (intro + two preview cards linking out + 立场 short text). `.reveal`.
- [ ] **Step 3:** zh + en wrappers. Demo CTA target: `/apply?interest=demo` (or a config seam) — no live embed this phase.
- [ ] **Step 4:** Build + check + render both langs; verify links to research pages resolve (create stub `ad-astra`/`schools` pages returning a "coming in Phase D" if not yet built, OR sequence C4 after D — here link to routes that will exist; if building C before D, add minimal placeholder routes so links don't 404, replaced in D). Commit.

```bash
git commit -am "feat(peraspera): bilingual institute page (mind-imprint + research preview)"
```

---

## Phase D — Research long-form articles

### Task D1: Ad Astra / Astra Nova research article (zh + en)

**Files:**
- Create: `src/content/research.ts` (ad-astra section), `src/components/pages/ResearchAdAstra.astro`, `src/pages/institute/ad-astra.astro` + `src/pages/en/institute/ad-astra.astro`

**Content source:** `docs/astranova/ad-astra-调研报告.html` + `astra-nova高中申请与真实案例.html`. Reformat into a clean editorial long-read. **Preserve all source links** (credibility core). Sections per v2 §三 研究1: 三实体谱系, 招生 official-vs-actual, 分年龄段, 高中三档+Corporate Collaborative+时间线+学费, 真实录取/被拒案例, 打假区.

- [ ] **Step 1:** Read the two HTML reports; extract structured content + every source URL into `research.ts` (ad-astra part) as `{zh,en}` (translate to idiomatic en). Do not fabricate; keep citations.
- [ ] **Step 2:** `ResearchAdAstra.astro` — long-read layout: title, lede, section headers, an interactive timeline (`<Timeline/>`) for the 3-entity genealogy, a comparison table for the 3 high-school tiers, a case-library block, a "打假/常见夸大" block, and a source-links list. `.reveal`.
- [ ] **Step 3:** zh + en wrappers; back-link to `/institute`.
- [ ] **Step 4:** Build + check + render both langs; verify links open, tables/timeline render, mobile readable. Commit.

```bash
git commit -am "feat(peraspera): ad-astra research long-read (bilingual)"
```

### Task D2: Global peer-schools research article (zh + en)

**Files:**
- Create: `research.ts` (schools section), `src/components/pages/ResearchSchools.astro`, `src/pages/institute/schools.astro` + `src/pages/en/institute/schools.astro`

**Content source:** report §3 comparison. Schools: Alpha, Synthesis, Khan Lab, Khan World, Minerva, Sora, Nueva, Acton. Four columns each: 招什么人 / 学费 / 怎么教 / 争议与风险. Keep critical content + sources.

- [ ] **Step 1:** Extract each school's 4-column data + sources into `research.ts` (schools part) as `{zh,en}`.
- [ ] **Step 2:** `ResearchSchools.astro` — intro + a responsive comparison table/cards (8 schools × 4 columns) + 立场 short + sources. `.reveal`.
- [ ] **Step 3:** zh + en wrappers; back-link to `/institute`; ensure institute preview cards now link here.
- [ ] **Step 4:** Build + check + render; verify table scrolls horizontally on mobile (`overflow-x:auto`). Commit.

```bash
git commit -am "feat(peraspera): peer-schools research long-read (bilingual)"
```

---

## Phase E — Deploy to Vercel

### Task E1: Vercel project config + env

**Files:**
- Modify: `astro.config.mjs` (`site` → real domain if known), add `apps/peraspera/vercel.json` if needed for monorepo root.

- [ ] **Step 1:** Confirm `@astrojs/vercel` adapter builds an SSR/hybrid output (`.vercel/output`). Run `pnpm --filter peraspera build`; confirm API route is in the functions manifest and pages are static.
- [ ] **Step 2:** Set Vercel project Root Directory = `apps/peraspera`. Add env vars in Vercel (from `.env.vercel`): `SUPABASE_URL`/`NEXT_PUBLIC_SUPABASE_URL`, `SUPABASE_SERVICE_KEY`, `DATABASE_URL`. Do NOT set `VERCEL_TOKEN` as a project env (CLI-only).
- [ ] **Step 3:** Deploy a preview (`vercel` with `VERCEL_TOKEN`, or via dashboard). Note preview URL.

### Task E2: Production verification

- [ ] **Step 1:** On the preview URL: load all pages in zh + en; toggle language; check countdown; submit the lead form → confirm a row in Supabase → delete test row.
- [ ] **Step 2:** Confirm no secrets in client bundle (search built JS for service key fragment → must be absent).
- [ ] **Step 3:** Fix issues; redeploy; then hand back to user with the preview URL + a summary of what to verify (real domain, real prices to fill `¥____`, founder photos, contact/social, demo embed decision).

---

## Self-Review (author check)

**Spec coverage:** §1 goals → all phases. §2 decisions → A1 (location/config/deploy), A2 (i18n), C1–C2 (Supabase/API), A3 (design). §3 apply funnel → C3. §4 IA → A5/B2/B3/C4/D1/D2 + A2 nav/footer. §5 architecture → A1–A3, C1–C2, content modules per page. §6 copy rules → Global Constraints + each content step. §7 deploy → E1/E2. §8 phasing → phase structure. §9 verification → per-task build/check + A6/E2 E2E. §10 risks → C1 env note, D scope isolation. ✅ all covered.

**Placeholder scan:** `¥____` and color final-value are intentional source blanks (flagged as "待定/TBD" styled). Font family named with concrete options. No "implement later" left. ✅

**Type consistency:** `getAdminClient()` (C1) used in C2. `t(lang,zh,en)` consistent. Component prop names (`Timeline steps`, `TierCards tiers`, `Accordion items`, `LeadForm`) consistent between B1 and consumers. `/api/lead` field names match C1 columns and C3 form. ✅
