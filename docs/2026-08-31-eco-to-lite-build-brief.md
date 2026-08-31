# /eco prototype — what exists, where it is, and the principles behind it

> Handoff for the next session. The `/eco` prototype is a **UI and
> interaction-rules** prototype: entirely front-end, `sessionStorage` for state,
> no API, no model calls, mocked data, scripted AI replies.
>
> It lives on the **`worktree-reading-cards`** branch, under
> `apps/lite-web/src/eco/`. It is not on main, and work continues on this branch.

---

## 1 · What exists, and where

### Documents

| Path | What it holds |
|---|---|
| `docs/2026-08-30-ecosystem-prototype-spec.md` | The spec, revisions v1–v9. Each round records what changed and **why the previous version was rejected** — the rejections are the useful part |
| `docs/2026-08-31-pbl-tool-model.md` | When a tool runs inside the chat vs. when it opens the right-hand panel |
| `AGENTS.md` | Standing project rules; still fully in force |

### The app shell

| Path | What it is |
|---|---|
| `eco/EcoApp.tsx` | Left rail, one surface in the middle, 印记 as a drawer. Three tabs: 首页 / 项目 / 我的主页 |
| `eco/route.ts` | Root-relative paths + History API, no router library |
| `eco/store.tsx` | All state and every action. `sessionStorage`, key `mk-eco-proto-v6` |
| `eco/ui.tsx` | The prototype's own small UI kit (`Btn`, `Panel`, `Field`, `Sys`, `Empty`, `cx`) |

### 世界 — news as planets

| Path | What it is |
|---|---|
| `eco/world/WorldView.tsx` | One locked screen; date axis top-left; a single ⓘ for the selection/politics/prototype notes |
| `eco/world/Planet.tsx` | A floating bubble with the headline inside it |
| `eco/world/NewsSheet.tsx` | The sheet behind a planet: 导读, headline, hook question, exits |
| `eco/data/news.ts` | The invented news (816 lines). Bilingual, dated, five per day |

### 我的树 — her interests, grown

| Path | What it is |
|---|---|
| `eco/home/TreeView.tsx` | Six branches, keywords as leaves, a growth axis that replays 起点 → 现在 |
| `eco/home/KeywordDrawer.tsx` | One keyword: where it came from, 印记's read of it, where to dig |
| `eco/home/ViewSwitch.tsx` | 世界 ⇄ 我的树 |
| `eco/data/tree.ts` | Fields, branch geometry, growth stops |
| `eco/data/library.ts` | The student (林知遥) and her readings and writings |
| `eco/data/dig.ts` | What a keyword offers next |

### 项目 — the PBL workbench (the core)

| Path | What it is |
|---|---|
| `eco/projects/ProjectsHub.tsx` | First run (one button) → switcher: 开始新项目 composer with category bubbles, and 我的项目 shelf |
| `eco/projects/Workbench.tsx` | The room: chat left, panel right, publish at the end |
| `eco/projects/CardSurface.tsx` | Renders a 工具卡 from its spec — schema-driven, no per-card code |
| `eco/projects/CoverPicker.tsx` | Project covers: 8 grounds × 16 glyphs |
| `eco/projects/NewProject.tsx` | The other door: pick a track, or talk it through |
| `eco/projects/panels/Planner.tsx` | The plan: number + name + one line, then 准备好了吗 · 开始 |
| `eco/projects/panels/StepIntro.tsx` | A step's goal, its 分工方案, and the decision it asks of her |
| `eco/projects/panels/Roads.tsx` | Two or more approaches, each a hook |
| `eco/projects/panels/BranchTalk.tsx` | Digging into one approach, then bringing back a reason |
| `eco/projects/panels/Make.tsx` | 印记's outputs: options / draft / build, each gated on a written reason |
| `eco/projects/panels/FormSurface.tsx` | The questionnaire: 印记 flags its own bad questions; she rewrites the invitation |
| `eco/projects/panels/Overview.tsx` | 计划 / 成果 / 方法 tabs |
| `eco/data/cards.ts` | 20 tool specs, in 7 categories |
| `eco/data/artifacts.ts` | What 印记 produces, with its guesses and its own admitted faults |
| `eco/data/plan.ts` | Tool categories, the three plans, approaches, branch scripts |
| `eco/data/projects.ts` | Tracks, seed projects, covers |
| `eco/data/types.ts` | Every shape in the prototype, heavily commented |

### 我的主页 — the site she builds

| Path | What it is |
|---|---|
| `eco/page/MyPage.tsx` | Her own view, in the shell: public link, 电脑/手机 toggle |
| `eco/page/PersonalPage.tsx` | `/eco/p/:handle` — the visitor's view, no navigation |
| `eco/site/BuiltSite.tsx` | Picks the layout from the 方案 she chose |
| `eco/site/Essay.tsx` | 方案 A — no nav, one enormous serif sentence, a mono metadata table |
| `eco/site/Ledger.tsx` | 方案 B — dark monospace, one date-sorted index |
| `eco/site/Magazine.tsx` | 方案 C — banner, nav, post cards, sidebar (the classic blog shape) |
| `eco/site/parts.tsx` | Shared pieces, including `Banner`, the site's own drawn artwork |
| `eco/data/site.ts` | The site's content model and where each field comes from |

### 印记

| Path | What it is |
|---|---|
| `eco/coach/CoachDrawer.tsx` | The drawer any surface can call |
| `eco/data/coach.ts` | Its voice and its openers |
| `eco/data/method.ts` | What it will and will not do, stated to the student |

### Dead weight

`eco/projects/PageStudio.tsx` — the old six-step homepage wizard. No entry point
any more; the 我的主页 tab goes straight to the site. Delete when you like.

---

## 2 · Design principles

Settled across nine rounds. Where an implementation detail collides with one of
these, the principle wins.

**On the conversation**

- **One question at a time.** Five textareas in a panel is the same five
  questions asked at once, which is what this rule exists to prevent.
- **One task at a time.** A question that is just prose is asked **in the chat**.
  A panel has to be earned — only front-end tools, documents, web pages and
  designs open it. The test: does the finished thing become an object she points
  at later, or just answers she gave once?
- **The toolset is open.** A tool is a spec, so 印记 can mint a new one mid-project.
  A fixed list of steps the AI must follow is the thing being avoided.

**On who decides**

- **印记 never decides.** It offers two or more approaches. Each is a hook that
  opens its own branch of conversation; she digs, comes back, and **decides with
  a reason**.
- **Nothing settles without a written reason.** Every 收下 / 定稿 button stays
  disabled until she writes why. The moment 「就用这个」 works on its own, this is
  a machine that generates and a student who approves.
- **印记 may do the work; the judgement stays hers.** It writes the code, lays
  out the page, generates the images. 铁律's 「不代写」 governs **the student's own
  body text only** — do not re-apply it to the artifact layer, which *is* the
  design.
- **印记 names what it guessed**, and admits what is still wrong with its own
  work. A handover that hides its assumptions can only be accepted, never
  reviewed.

**On the plan**

- Show the steps, then 准备好了吗 · 开始. Do not front-load every detail.
- **No per-step scheduling.** Asking a student to put a date on each step was
  tried and cut.
- Each step states its **goal**, its **分工方案** (what 印记 does, what she does,
  what she brings back), and the **decision** it asks of her.

**On the site she builds**

- **It is hers, not ours.** Nothing in `site/` may import the app's UI kit
  (`Btn`, `Panel`, any `mk-*` token). The first version did, and it read as a
  product screen instead of a person's website.
- **Her style choice must change the page**, not just the palette. Three options
  described three different pages; rendering one layout in three colourways made
  the decision she justified invisible.
- **Real personal-site furniture**: an identity block, section names people
  actually use (文章 / 项目 / 关于), a post list with a real meta row, 站点信息,
  and a footer that is just © name · built with X. No rubric labels over her own
  paragraphs.
- **The banner is the site's own artwork**, never a crop of one of her projects —
  those do different jobs.

**On copy**

- Plain words. No invented jargon (「你排的时间：本周」, 「印记做的 · 你来判断」
  were all cut); academic register where a term is needed.
- **No `不是…而是` antithesis**, Chinese or English. Positive declaratives.
- **No false state claims.** Never name a room she cannot reach or a state that
  did not happen.
- **No manipulation**: no streaks, leaderboards, badges or push. Tools trigger
  automatically, but she confirms opening them.

---

## 3 · Work principles

- **Look at the UI in a real browser.** Take a Playwright screenshot and actually
  look at the image. In 2026-08-30 lite had 344 green tests sitting on top of an
  exported report PNG that was completely blank.
- **Logic tests only** — pure functions, reducers, normalizers, permission
  checks. No assertion per rendered element; they break on every honest redesign
  and catch nothing.
- **AI failures surface.** Never a plausible canned sentence when a model call or
  a parse fails.
- **Do not port the mock plumbing** — the `sessionStorage` store, the timer
  theatre in `Make.tsx`, the scripted replies in `data/plan.ts`, the invented
  news. Those exist so the design could be settled without a backend.
- **Documents are written in English**, filenames `YYYY-MM-DD-<kebab>.md`.
  Chinese stays for product and UI strings.
- **The client never calls a model directly**; keys are server-side only.

---

## 4 · First message for the new session

> I want to build the `/eco` prototype into lite. It is on the
> `worktree-reading-cards` branch under `apps/lite-web/src/eco/` (not on main),
> and we build on that branch. It is a UI and interaction-rules prototype —
> front-end only, mocked data, scripted AI.
>
> Read `docs/2026-08-31-eco-to-lite-build-brief.md`, then the two documents it
> points to. Then tell me how you would bring the 项目 room into lite for real,
> and what you would build first. Flag anything that needs my decision instead of
> choosing it yourself.
>
> Do not write code yet.
