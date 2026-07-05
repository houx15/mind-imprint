# Per Aspera Site — Refactor Plan (dark redesign + restructure)

> Executes user's 2026-07-05 refactor feedback. Branch: `feat/peraspera-site`.
> REQUIRED SUB-SKILL: subagent-driven-development. Checkbox steps.

**Goal:** Rebuild the Per Aspera site with a dark (near-monochrome + faint-blue on black) astranova-*style* design, a new nav/structure, and rewritten realistic content — centered on the mission of **AI-Native Learning (ANL)**.

## Global constraints (bind every task)
- **Design:** near-black background (~#0B0B0C), off-white text, hairline borders, ONE faint cool-blue accent used sparingly. Astra-Nova *style* only (full-screen hero, one-section-per-screen, big restrained type) — NOT their content. Space Grotesk headlines, Noto Sans SC.
- **Page `<title>`s must be 写实/plain**: 首页→"Per Aspera"; 申请辅导; 学院 / 课程项目; 合作; 关于我们; 联系我们. NEVER a long sentence title.
- **Remove sitewide:** 说明会 (replace with "留下联系方式，我们会联系你" / "leave your contact and we'll reach out"); ALL deadlines + the Countdown component/usages; the "冲刺营" name.
- **Bilingual, single-language per locale.** zh at `/`, en at `/en/`. On `/en/` NO Chinese may render (fixes the belief-cards bug where zh showed on the EN page). `t(lang,zh,en)`.
- **Copy voice:** NO antithesis of any form (`不是…而是`/`而非`/`而不是`/"not X but Y"/"rather than"). Positive declaratives. No lorem ipsum. Realistic, parent-legible copy — a parent must immediately understand what we do.
- **PLAIN WORDING — no jargon (hard rule).** Write the way a normal parent talks. BANNED words/marketing-CRM-jargon: 留资 (use "留下联系方式"), 漏斗, 触点, 转化, 赋能, 抓手, 闭环, 心智, 打法, 对齐, 颗粒度, 抓手, 生态位, and any consultant/startup buzzword. Prefer everyday verbs and concrete nouns. If unsure how to phrase something naturally, look at how real education/school/tutoring websites word it and match that register. English copy: same — plain, warm, concrete; no buzzwords.
- **Contact = the conversion.** Dedicated `/contact` (联系我们) page holds the Supabase lead form (reframed, no 说明会). Every "联系我们 / leave contact" CTA links to `/contact`.
- Secrets server-only; git hygiene (explicit `git add`, never root package.json / orphan test / .env); trailer `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`. Build gate: `pnpm --filter peraspera build` + `astro check` 0 errors.

## Founders (correct facts)
- **陈玉洁** — CEO · 联合创始人 · 教育研究院负责人 (CEO, co-founder, head of the education research institute).
- **侯煜欣** — 联合创始人 · 产品负责人 (co-founder, head of product).

## Routes (final)
- `/` 首页 (home, full-screen sections)
- `/coaching` 申请辅导  (replaces old /programs sprint content; drop "冲刺营")
- `/academy` 学院 / 课程项目  (our courses)
- `/partnership` 合作  (NEW — school AI-transformation + mind-imprint)
- `/about` 关于我们  (rewrite)
- `/contact` 联系我们  (NEW — lead form)
- `/institute/ad-astra`, `/institute/schools` — research long-reads KEPT, recolored dark, relinked from `/coaching`
- DELETE old pages: `/programs`, `/institute` (index), `/apply`. Each has an `/en/` twin.
- **Nav (sitewide):** 首页 · 项目 ▾ (申请辅导 `/coaching`, 学院 `/academy`) · 合作 `/partnership` · 关于我们 `/about` · [lang] · button 联系我们 `/contact`.

## Phases / tasks

### R1 — Dark design system + full-screen section utilities
Rewrite `src/styles/global.css` to the dark palette. Tokens: `--bg` near-black, `--surface` slightly lifted, `--ink` off-white, `--muted` grey, `--line` rgba(255,255,255,.10), `--accent` faint cool-blue (low saturation, e.g. ~#8AA0C8) used only for small marks/links; primary button = high-contrast (white bg/black text) with a ghost/outline secondary. Recolor ALL existing component classes (hero/beliefs/pullquote/timeline/tiers/faq/table/cta/nav/footer/cards/forms) for dark. Add full-screen section utilities: `.screen{min-height:100svh; display:grid; align-items:center}` and a scroll-snap container for the homepage (snap on desktop pointer; on mobile/short-viewport fall back to natural flow so content never clips). Keep `.reveal` (js-gated) + reduced-motion. Responsive ≤900/≤760. Review with a reviewer (foundational).

### R2 — Nav + Footer (dark) + remove Countdown component
Update `Nav.astro` to the new structure (首页/项目▾[申请辅导,学院]/合作/关于我们 + lang + 联系我们 button). The 项目 dropdown: accessible disclosure (hover on desktop, click/focus works; keyboard-navigable; degrades to plain links). Update `Footer.astro` for dark (keep the non-affiliation disclaimer). Delete `Countdown.astro` and all its imports/usages. Update `site.ts` nav labels.

### R3 — /contact page (联系我们) + reframed LeadForm
Reframe `LeadForm.astro` copy: remove all 说明会 wording; success = "我们收到啦，会尽快与你联系。"/"Got it — we'll be in touch soon." Fields unchanged (contact_name/contact/child_age/interest/message + honeypot + locale). `interest` options updated to: 申请辅导 (coaching) / 课程项目 (academy) / 学校合作 (partnership) / 先了解一下 (explore). Create `src/content/contact.ts`, `ContactPage.astro`, `/contact` + `/en/contact`. Delete `/apply` + `/en/apply` + apply.ts/ApplyPage. `/api/lead` unchanged (still works). Review (form logic).

### R4 — Homepage (dark, full-screen sections)
`src/content/home.ts` + `HomePage.astro` + index wrappers. Sections, each a full `.screen`:
1. **Hero** — one slogan (craft around ANL: education is entering the AI-native age; we raise people who solve real problems with AI) + two buttons: 了解更多 (scrolls to mission) + 联系我们 (→/contact). Full-screen, dark, big type. No countdown.
2. **Mission** — polish this into a paragraph: education is undergoing a transformation toward **ANL (AI-Native Learning)**. More students are starting to learn and work inside companies, and to build/entrepreneur earlier — developing the ability to solve real-world problems with AI. State it as our conviction.
3. **What we offer** — 3 cards: (a) 申请辅导 — prepare students & parents for top AI-era schools, e.g. Elon Musk's **Astra Nova** → /coaching; (b) 课程项目 — for families who share the philosophy but haven't applied/enrolled in those schools: part-time online courses → /academy; (c) 学校合作 — for schools that share the philosophy: AI-transformation solutions (AI usage & the mind-imprint evaluation product, teacher training, courses) → /partnership. Polish each into clear copy.
4. **What we believe** — belief cards (SINGLE language per locale — fixes the bug).
5. **Who we are** — short founders/team intro (correct roles), link → /about.
6. **Questions** — a few FAQ (native details), link → /about#faq or /contact.
Beliefs: reuse/refresh the 4 convictions but render only the current language.

### R5 — 申请辅导 `/coaching`
`src/content/coaching.ts` + `CoachingPage.astro` + `/coaching` + `/en/coaching`. Sections:
1. **What Astra Nova is** — introduce the school plainly (online, Musk-founded, admits on reasoning not test scores). Link to the kept research long-read `/institute/ad-astra` ("深入了解 Astra Nova") and `/institute/schools`.
2. **Why it's valuable, and why it's hard** — the profile it selects for; the live reversal-questioning interviews; why coaching-to-a-script fails.
3. **What we offer — a 2-month, 3-stage journey** (be detailed + parent-reassuring):
   - Stage 1 (ability): 信息素养与思辨 · 问题解决 · AI 使用 · 自信表达 · 项目能力 — aligned to the profile Astra Nova favors.
   - Stage 2: mock interviews + training (reversal questioning, no-leader group discussion, English expression).
   - Stage 3: application preparation.
   - **Parents:** month 1 = a weekly one-on-one chat; month 2 = help preparing the letter.
4. **How to apply** — plain steps + encourage leaving contact → /contact. NO deadline, NO 说明会, NO "冲刺营" name.

### R6 — 学院 / 课程项目 `/academy`
`src/content/academy.ts` + `AcademyPage.astro` + routes. Introduce our course idea (look at how Astra Nova frames its course program — problem-driven, discussion-based). **Part-time, online.** Present the course library as **MODULES, not a long list** — from `docs/03_课程库_单课设计`: 信息素养 · 知识工具与思辨 · 五大知识领域(TOK) · AI协作与伦理 · 元认知与反身 — PLUS a new **AI 与产品 / 建造** module (computational thinking, collaborating with AI while keeping judgment, from problem to prototype/product). State clearly: **we design a personalized plan for each child** (different characteristics + interests). Detailed for both parents and students. Do NOT mention 亚洲时区. No deadlines/说明会.

### R7 — 合作 `/partnership` (NEW)
`src/content/partnership.ts` + `PartnershipPage.astro` + routes. Intro: **AI-Native Transformation Solutions for Schools.** Two solution types:
1. Help the school build its AI system — AI course design + the **mind-imprint** AI-evaluation tool + teacher training.
2. AI **summer/winter camps** we run for the school.
Then **introduce the mind-imprint product** — READ `apps/site/src/components/pages/EvaluationPage.astro` and `apps/site/src/components/pages/HomePage.astro` for how the product describes itself (two faces 生成式驾驭/批判式防护 → 10 dims, SOLO L1–L4, process-not-product, private, offered-not-pushed, the interactive 思维工具卡 + 过程评估). Present it accurately and attractively. End with **contact us for more** → /contact.

### R8 — 关于我们 `/about`
Rewrite `about.ts`/`AboutPage.astro`: 写实 hero title (e.g. "关于我们" / "About us" — NOT "两个人，一个执念…"). Founders with CORRECTED roles (陈玉洁 CEO·联合创始人·教育研究院负责人; 侯煜欣 联合创始人·产品负责人). Keep a trimmed founder note + name origin + FAQ (`id="faq"`), all dark, single-language per locale, no 说明会/deadline/antithesis.

### R9 — Recolor research pages + cleanup + build
Recolor `/institute/ad-astra` + `/institute/schools` for the dark theme (they inherit global.css; verify readability, tables, links on black). Confirm all old routes removed and no dangling links (grep for /programs, /apply, /institute (index), Countdown, 说明会, 冲刺australia, deadline). Full build + astro check. Then redeploy to Vercel production.

## Verification per phase
Build + astro check 0 errors; load zh + en; **grep the changed files for forbidden terms: `说明会`, `冲刺营`, `Countdown`, deadline/截止, and the antithesis family**; confirm `/en/` pages render NO Chinese; internal links resolve; dark theme legible; mobile OK. Final: redeploy prod, smoke-test all pages + /api/lead.
