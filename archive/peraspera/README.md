# Per Aspera — archived marketing site

> 已归档：此目录是历史源码，不属于当前官网或 Lite 产品，也不参与 pnpm workspace 和部署。以下内容仅供历史参考；只有用户明确要求处理 Per Aspera 时才修改或发布。

A bilingual (zh / en) marketing site for **Per Aspera**, an education startup that prepares
students for the AI era. It is an **independent** app: it shares the pnpm workspace but has no code
dependency on `apps/web` (the mind-imprint product), `apps/api`, or `apps/site`. Pure presentation +
one lead-capture endpoint. Deploys to Vercel.

- **Live:** https://peraspera-liart.vercel.app
- **Design:** dark editorial, styled after astranova.org's *look* (not its content).

> This README is the "continue-here" doc. Read the two sections **The story this site tells** and
> **Copy voice — hard rules** before changing any copy, or edits will drift off-message.

---

## The story this site tells (keep every page coherent with this)

The site is **product-first**. There is **one core product — 思维印记 (Mind Imprint)** — and three
services built *on top of it*. Never present the services as three peers with the product hidden; the
product is the spine.

**思维印记 = one AI education product with three components:**
1. **AI-assisted interactive courses** — live, small-group, AI-assisted. **No recorded video lectures.**
   Two threads: **思辨 (critical thinking)** + **产品思维 (product thinking)**. The critical-thinking
   thread grew from 30+ courses proven in real international (IB) classrooms.
2. **Two AI-guided workbenches (工作台)** — a child works a real task *with* AI, and the AI keeps
   handing the thinking back: **批判性思维工作台** (questioning / source-checking / argument building)
   and **产品思维工作台** (a real problem → a working prototype).
3. **过程评估 (process assessment)** — turns the reasoning *process* into a private growth record.
   Cognitive model = **two faces** (生成式驾驭 🚀 / 批判式防护 🛡️) → **10 dimensions (D1–D10)** →
   **SOLO L1–L4** (萌芽 / 发展中 / 熟练 / 卓越), plus an **initiative gradient** (self-initiated /
   nudged / skipped). Principles: private to the student, always the strongest model, offered-never-pushed.

**Three services built on the product:**
- **申请辅导 (admissions coaching)** → `/coaching` — help a family apply to AI-era schools (e.g.
  Elon Musk's Astra Nova). Highly selective / small number of families per round.
- **课程 (family courses)** → `/academy` — part-time, fully online; plus winter/summer camps.
- **学校合作 (schools)** → `/partnership` — help a school bring AI into teaching; the product +
  teacher training + course system.

**Public vs. confidential.** The strategy source is a *confidential BP*
(`docs/astranova/website_refactor_0706.md`). **Keep BP framing off the public site**: no roadmap
dates, no "moat"/"antifragile" investor language, no market stats, no validation-study numbers, no
"2B/2C" labels. Translate strategy into warm, parent/school-facing copy. (The real product uses
**10** assessment dimensions; the BP's "9" is a simplification — the site keeps 10.)

---

## Tech stack

| | |
|---|---|
| Framework | **Astro 5** (mostly static prerender + one serverless function) |
| Adapter | `@astrojs/vercel` (Build Output API v3) |
| Styling | one hand-written CSS file (`src/styles/global.css`), no framework |
| Fonts | Space Grotesk (headlines) + Noto Sans SC (Chinese), from Google Fonts |
| i18n | tiny in-repo helpers (`src/i18n/utils.ts`) — no i18n library |
| Lead form | `@supabase/supabase-js` → Supabase `leads` table, via a server-only endpoint |
| Deploy | Vercel (project `peraspera`) |

**Node:** `package.json` pins `engines.node = 22.x`. Vercel builds remotely on Node 22. **Node 20
runs `dev`/`build`/`check` locally fine** — the only thing that truly needs Node 22 is the *deployed*
`/api/lead` function at runtime (Supabase realtime needs a global `WebSocket`). So don't `--prebuilt`
a local Node-20 build; let Vercel build.

---

## Project structure

```
src/
  content/          ← ★ ALL copy lives here as typed { zh, en } data. Single source of truth.
    site.ts           brand + nav labels + footer (+ the Bilingual type)
    home.ts           / (7 full-screen sections)
    mind-imprint.ts   /mind-imprint  ← the core product page
    coaching.ts       /coaching
    academy.ts        /academy  (nav label: 课程 / Courses)
    partnership.ts    /partnership
    about.ts          /about
    contact.ts        /contact  (also the lead-form field labels + interest options)
    research.ts       /institute/ad-astra + /institute/schools (research long-reads)
  components/
    Nav.astro Footer.astro LeadForm.astro  + small UI bits (Accordion, PullQuote, Timeline…)
    pages/            one *Page.astro renderer per route (HomePage, MindImprintPage, …)
  pages/            ← thin route wrappers. zh at the root, en under /en/.
    <route>.astro       imports Base + the matching *Page.astro, sets <title>/description
    en/<route>.astro    the /en/ twin (identical, just points Base title to the en string)
    api/lead.ts         POST-only serverless endpoint (prerender = false)
  layouts/Base.astro  <head>, fonts, hreflang, Nav/Footer, the reveal-on-scroll script
  i18n/utils.ts       getLang / t / localePath / switchLocaleUrl
  styles/global.css   the whole design system + component styles
  lib/supabase.ts     server-only Supabase admin client (service key)
supabase/migrations/  leads table SQL
DEPLOY.md             full deploy runbook
```

**The pattern:** `content/*.ts` (data) → `components/pages/*Page.astro` (markup, reads the data via
`t()`) → `pages/*.astro` + `pages/en/*.astro` (thin wrappers that set the `<title>`). Copy edits
almost always happen in **`content/`** only.

---

## Bilingual model

- **zh is the source of truth; en mirrors it.** Every string is a `{ zh, en }` pair (`Bilingual` in
  `content/site.ts`).
- Each page renders **one language server-side** (SEO-correct) — nothing toggles in the DOM. `t(lang,
  zhStr, enStr)` returns the right string; `lang = getLang(Astro.currentLocale)`.
- **Routes:** zh at `/foo`, en at `/en/foo`. `localePath(lang, "/foo")` builds the correct href;
  `switchLocaleUrl` powers the 中/EN toggle.
- **HARD RULE: the `/en/` page must render NO Chinese** (the `中` language-toggle glyph is the only
  exception; proper nouns cited in the research long-reads are the one tolerated case). A common bug
  is glossing an English term with Chinese in parens, e.g. `critical thinking (思辨)` — don't. Verify
  with the CJK sweep below.

---

## Copy voice — hard rules

Write the way a normal parent talks. A parent must understand what we do on first read.

1. **No jargon / buzzwords.** Banned: 留资, 漏斗, 触点, 转化, 赋能, 抓手, 闭环, 心智, 打法, 对齐,
   颗粒度, 生态位 + English equivalents (leverage, synergy, empower, funnel…). Use plain words
   (e.g. 留下联系方式, not 留资). When you introduce a product term like 工作台, explain it in plain
   words right there.
2. **No antithesis.** Avoid the whole family: `不是…而是`, `不…而…`, `而非`, `而不是`, English
   "not X but Y", contrastive "rather than". Write positive declaratives. (Bare negations like
   "不排名" are fine; the idiom 似是而非 is fine — it's one word.)
3. **No deadlines, no 说明会 (info sessions), no 冲刺营, no 亚洲时区** in our marketing copy. (The
   `research.ts` long-reads *do* cite Astra Nova's real admission deadline/process — that's factual
   reporting about the school, not our offer, and is allowed there.)
4. No lorem. Concrete, warm, specific.

---

## Design system (in `global.css`)

- **Dark tokens:** `--bg` #0B0B0C · `--surface`/`--surface-2` lifted greys · `--ink`/`--ink-2`
  off-white · `--muted` grey · `--line` hairline · `--primary` faint cool-blue #8AA0C8 (used sparingly).
- **Full-screen sections:** `.screen { min-height: 100svh }`. The homepage document-scroll-snaps
  (`.snap`), gated to desktop + fine pointer + no-reduced-motion; on phones it flows naturally.
- **`.reveal`** = fade-in-on-scroll. Elements start at `opacity: 0` and get `.in` when scrolled into
  view (IntersectionObserver in `Base.astro`). **Gotcha:** a static full-page screenshot shows later
  sections blank because they never scrolled into view — that's not a bug. To screenshot the true
  layout, run `document.querySelectorAll('.reveal').forEach(el => el.classList.add('in'))` first.
- Primary button = high-contrast (light bg / dark text); secondary = outline/ghost.

---

## Common tasks

**Edit copy on an existing page** → edit the matching `content/*.ts` file only. Keep zh + en in sync
and obey the copy rules above.

**Add a new page `/foo`:**
1. `content/foo.ts` — export typed `{ zh, en }` content.
2. `components/pages/FooPage.astro` — render it (`const lang = getLang(Astro.currentLocale)`, use
   `t()`, `localePath()`; copy the section/`.reveal` idiom from an existing page).
3. `pages/foo.astro` **and** `pages/en/foo.astro` — thin wrappers:
   ```astro
   ---
   import Base from "../layouts/Base.astro";        // en/ version: "../../layouts/Base.astro"
   import FooPage from "../components/pages/FooPage.astro";
   ---
   <Base title="标题 · Per Aspera" description="…">   <!-- en/: English title + description -->
     <FooPage />
   </Base>
   ```
   Keep `<title>` **plain/写实** (e.g. `课程 · Per Aspera`), never a long sentence.
4. Add it to the nav if needed (see below).

**Add / rename a nav item** → `content/site.ts` `nav` map (label) + `components/Nav.astro` (the link,
its `active()` state, and the mobile-subnav if it belongs under the 项目 dropdown).

**Change the lead form** → fields + labels + interest options live in `content/contact.ts`; markup in
`components/LeadForm.astro`; server validation in `pages/api/lead.ts`.

---

## Lead form, Supabase & secrets

- `POST /api/lead` (server-only, `prerender = false`) validates + writes to Supabase `leads` via
  `lib/supabase.ts` (service key). Has a honeypot (`hp`), an `interest` allowlist, and a body-size cap.
- **Secrets are server-only and never committed.** They live in the repo-root **`.env.vercel`**
  (gitignored) and in Vercel project env vars. This site's `.env.vercel` points at a **test** Supabase
  project. Never print, log, or commit secret values. Env var names are documented in `DEPLOY.md`.

---

## Run locally

```bash
pnpm --filter peraspera dev      # http://localhost:4321  (Node 20 is fine for dev)
pnpm --filter peraspera build    # static build + the /api/lead function bundle
pnpm --filter peraspera check    # astro check — must be 0 errors before deploy
```

Submitting the lead form locally hits the test Supabase; page rendering does not touch Supabase.

## Deploy

Remote build on Vercel (Node 22 via the engines pin). From the repo root:

```bash
export VERCEL_TOKEN="$(grep '^VERCEL_TOKEN=' .env.vercel | cut -d= -f2-)"
pnpm dlx vercel --cwd apps/peraspera --prod --yes --token "$VERCEL_TOKEN"
```

Then verify (the alias auto-updates to the new deploy):

```bash
base="https://peraspera-liart.vercel.app"
for p in / /en/ /mind-imprint /en/mind-imprint /coaching /academy /partnership /about /contact; do
  echo "$(curl -s -m 25 -I "$base$p" | head -1 | tr -d '\r')  $p"; done
# /api/lead: POST {} → 400 validation (function is live); GET → 404 (POST-only, expected)
```

**Check `/en/` pages leak no Chinese** (run against a local `pnpm dev` or the live base):

```bash
python3 - <<'PY'
import urllib.request, re
base="http://localhost:4321"
for p in ["/en/","/en/mind-imprint","/en/academy","/en/partnership","/en/about","/en/coaching","/en/contact"]:
    html=urllib.request.urlopen(base+p,timeout=10).read().decode("utf-8","ignore")
    html=re.sub(r"<script.*?</script>|<style.*?</style>","",html,flags=re.S)
    text=re.sub(r"<[^>]+>"," ",html)
    hits=[m for m in re.findall(r"[^\s>]*[一-鿿][^<]{0,30}",text) if any(c!="中" and "一"<=c<="鿿" for c in m)]
    print(p, "clean" if not hits else hits[:6])
PY
```

Full runbook (one-time project setup, env var names, Node-22 rationale): **`DEPLOY.md`**.

---

## Open items / known TODOs

- `astro.config.mjs` `site:` is a **placeholder** (`peraspera.example.com`) — it feeds the hreflang
  alternates in `Base.astro`. Set it to the real production domain when there is one.
- Footer social links are `href="#"`; founder photos are placeholders; contact email/WeChat on
  `/contact` + `/about` are placeholders — swap in real values when available.
- Confirm Vercel deployment-protection stays **OFF** so the site is public.

## Reference docs

- **`DEPLOY.md`** — deployment runbook.
- `docs/superpowers/specs/2026-07-05-peraspera-site-design.md` — original design spec.
- `docs/superpowers/plans/2026-07-05-peraspera-site.md`, `2026-07-05-peraspera-refactor.md`,
  `2026-07-06-peraspera-product-refactor.md` — build/refactor plans (the last one = the product-first refactor).
- `docs/astranova/website_refactor_0706.md` — the confidential strategy BP (keep its framing off the public site).
