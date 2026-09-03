# The website project, rebuilt as a real PBL project

Design spec, 2026-09-03. Satisfies
`docs/2026-09-03-website-project-owner-requirements.md`, which is the source of
truth for what the owner asked for. Where this spec and that document disagree,
that document wins.

Amends `docs/superpowers/specs/2026-09-01-pbl-project-room-design.md` §4 and
§15. §15's three-layout menu is retired; see §7 below.

---

## 1 · What is wrong today

`projects/ProjectSurface.tsx:42` returns `<SiteStudio>` whenever
`kind === "website"`, so the project room never renders for the one project every
student is forced to do first. The website project creates a `pbl_project` row
and then bypasses every piece of machinery attached to it: the 印记 coach, the
dynamic plan, the ten tools, 审核助手, 分工建议, 复盘, 长期迭代.

What it replaced them with is a nine-field labelled form — 首屏那句话 / 你是谁 /
开场一段 / 关于 / 现在 / 现在在做的事 / 标签 / 页头几个词 / 联系方式 — plus one
textarea per reading, writing and project.

## 2 · The shape of the fix

**The website project is a normal project in the normal room.** No branch in
`ProjectSurface`. What makes it different is three pieces of data, not a
different application:

1. A **defined topic and driving question**, seeded at creation instead of
   elicited.
2. A **suggested routine** — a version-1 plan of five steps, seeded at creation,
   `decided_by = "ai"`, every step `tentative`, so she meets a real task list
   and still has to approve it.
3. A **website routine block in the coach prompt**, selected by
   `kind == "website"`, that tells 印记 which tool belongs to which stage and
   what it is allowed to say it can do.

Everything else — the coach turn, plan check, tool offers, refeed, 复盘 — is the
machinery that already exists.

### The driving question

Topic: her own personal website. Driving question:

> 「我想让谁，看见我的什么？」

This is what makes it a question-defined project rather than a build order.
Stage 1 answers it, stage 2 turns the answer into a structure, and stage 5
checks the built page against it. The question is stored as the project's
`idea`, replacing today's flat description.

## 3 · The five stages, as plan steps

Each step is a `pbl_plan_step` row: `title`, `blurb`, `goal`, `you_bring`,
`i_bring`, `decide`, `then_bring`. `decide` is mandatory server-side — a step
where she judges nothing is refused — which is exactly the property that keeps
this routine from becoming a form wizard.

| # | Step | Tool | 她判断什么 (`decide`) |
|---|---|---|---|
| 1 | 想清楚给谁看 | `persona` (new) | 哪一个受众是真的，哪些关键词留下 |
| 2 | 去看真的个人网站 | `sites` (new) → `structure` | 哪些结构值得学，她的结构是什么样 |
| 3 | 给网站定调子 | `look` (new) | 配色、风格、要不要头图 |
| 4 | 我来生成，你来分工 | `split` → artifact | 哪些活归 AI，哪些归她 |
| 5 | 逐处审改，然后上线 | `review` | 这一页哪里还不对 |

After publish the project moves to `keeping`, where `lookback` (项目复盘) and
`keep` (长期迭代) apply — that is requirement 13, "modification any time",
already built.

## 4 · Three new tools

The toolbox is open: a summoned tool with no surface degrades to a plain card, so
adding one is one row in `internal/pbl/tools.go`, one row in
`projects/tools/registry.tsx`, and one surface file. No renderer changes.

### `persona` · 受众画像 (stage 1)

**No `Needs`** — corrected during the build. The pairing rule exists because
`decide` / `structure` / `split` / `review` render something 印记 made in the same
turn, and are blank without it. `persona` generates its own first screen when she
opens it, so requiring a paired produce would add a failure mode (a tool offered
without its payload) for no gain. Same for `sites` and `look`.

印记 produces **two or three candidate audiences** from what it already knows
about her (her real readings, writings, projects): who they are, why they would
know her, what they would want to see, what feeling the page should give them.
Each candidate carries an **AI-generated photo** and **tagged keywords**.

Her actions: pick one, reject all (印记 re-proposes), drag keywords between
keep and discard, add a keyword. **Typing: none required.**

Output: the kept audience plus 3–6 keywords, written back to the thread. Those
keywords are the contract stage 2 verifies against and stage 3 derives the
palette from.

### `sites` · 站点采集 (stage 2)

印记 opens with why real sites beat templates, and **six real personal websites**
as starting points — the ones named in `2026-09-01-pbl-project-room-design.md` §4.

🚨 Corrected during the build: this section originally said the three retired
renderers would be shown as style examples. They cannot be — a rendered example
needs content, and the only legitimate content in this product is *hers*. The
prototype put example content in (林知遥) and thereby made every student's page
someone else's. Six real links cost nothing and invent no second fake student.
The three layouts still serve as the typographic bases in stage 3 (§7).

Her action: **paste URLs** of sites she likes. Each paste is fetched server-side
by the existing `materialize.FetchReadable` and digested (`digest` class) into a
card: what this site does, its structure (identity block, post list, site info),
and the one thing it does best. Three cards unlock the next step — a real gate,
not a greyed button.

印记 then proposes what her three have **in common** and what could be **hers
alone**; she keeps or crosses out each line. That hands off to the existing
`structure` tool, whose surface already renders a mindmap, where she composes her
own structure and checks it against her stage-1 keywords.

**Typing: pasted URLs only.**

### `look` · 视觉基调 (stage 3)

印记 first shows her **what it already has**: her real readings, writings and
projects, assembled by the existing `internal/pbl/site.go`. That is requirement
"scan your history — we have got these". She adds by **telling the story**, in
the thread, to 印记 — not by filling in fields.

Then, derived from her keywords: three **palettes**, each explained in one line;
a **style**; and a **hero image** — three drafts, generated, with regenerate and
a one-line nudge. "Your website is becoming real" lands here.

Her actions: pick a palette, pick a style, pick or regenerate an image.
**Typing: one optional nudge for the image.**

## 5 · Image generation

`models.json` already lists `qwen-image-3.0-pro`, `qwen-image-3.0` and
`qwen-image-edit-max` with `capabilities: ["image"]`, but they were catalog
metadata that nothing could call. Needed in two places (persona photos, hero
image), so it became real infrastructure:

- A `Drawer` seam in the gateway alongside `Provider` — generating an image is
  not a stream, so it is a second path rather than a method on the chat one.
- A new capability class, `draw`, validated at boot like every other class. It
  accepts only models whose `capabilities` include `image`; a wrong binding
  fails startup rather than at the student's screen.
- Output fetched immediately and stored in our own OSS (`internal/api/pbl_draw.go`).
  Cost recorded in `llm_call` like every other call.

**🚨 The endpoint was measured, not inferred — and the catalog's own comment was
wrong.** This spec originally said the image models "answer on
`/images/generations`", repeating `models.json`'s `_capabilityComment`. The
first `LIVE_LLM=1` run proved otherwise (2026-09-04):

| Request | Result |
|---|---|
| `POST {maas}/compatible-mode/v1/images/generations` | 404, empty body |
| `POST dashscope.aliyuncs.com/compatible-mode/v1/images/generations` | 404, empty body |
| `POST {maas}/compatible-mode/v1/chat/completions` with an image model | 400 InvalidParameter |
| `POST dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation` | **200, returns an image** |

So the chat route's aggregated endpoint cannot draw at all, even though its own
`GET /models` lists `qwen-image-3.0`. Image generation needs a separate provider
(`dashscope_image`: different host, different wire shape), an `input.messages[]
.content[]` **array** body, and a response read from
`output.choices[].message.content[].image`. That URL is signed and **expires**,
so it is downloaded and re-stored on the spot; storing it would put a broken
image on her homepage days later. One live call takes ~69 s, which is why
persona portraits are one request each rather than three in one.

Had the stub tests been trusted, every generation in production would have been
a 404 with a green suite above it. This is the rule in `AGENTS.md` doing its job.

**Persona photos are labelled as generated, in the UI, always.** They are
stand-ins for an imagined reader, and a student should never be unclear about
which faces on her screen are real.

## 6 · Stage 4 — generation, and the honest split

Stage 4 is where 印记 builds the page, and it is the one place where requirement
"AI's turn — generating" meets 铁律① "AI 不替学生撰写".

The resolution is the same one the writing surface already uses, and it is why
`split` 分工建议 belongs in this step: **印记 produces the split itself and she
judges it.** Concretely — 印记 renders structure, layout, palette and image; she
owns every sentence that speaks as her. The generated page therefore ships with
her own words in it and 印记's rendering around them, and the split card says so
in writing. That also satisfies 铁律①'s closing clause: 诚实介绍 AI 和人的分工.

The generated page is handed over as an `artifact`, which is what makes stage 5
the existing `review` tool — 审核助手, already reshaped to 「先自己找，再对答案」 —
with `marks` quoting real lines from the page and `dimensions` naming what to
check. That is the review card the owner asked for, doing its real job.

## 7 · The three layouts are retired as a menu

`2026-09-01-pbl-project-room-design.md` §15 required three genuinely different
layouts with her justified choice visible. Stage 2 now ends with her composing
her own structure, so a menu of three would make stages 2 and 4 decorative.

- `Essay`, `Ledger`, `Magazine` survive as **style examples in `sites`** and as
  **typographic bases** for generation.
- `putSiteLayout` and the 选它的理由 textarea are deleted. Her justification does
  not vanish: it moves to the structure she builds and the keywords she checks it
  against, which is a stronger record than one paragraph defending a menu pick.
- §15's other rulings **stand unchanged**: `site/` may not import the app UI kit;
  the banner is the site's own artwork; content comes from her real work;
  visibility is an unlisted, revocable, `noindex` link.

## 8 · What is deleted

- `apps/lite-web/src/projects/SiteStudio.tsx` — the whole three-step form.
- The `kind === "website"` branch in `ProjectSurface.tsx`.
- `PUT /api/v1/pbl/site/layout` and `layout_why` handling.
- `apps/lite-web/src/eco/` — the dead prototype that seeded this defect
  (`STUDENT = { name: "林知遥" }`), still present alongside the real thing.

`pbl_site` the table, `internal/pbl/site.go` the assembly, the three renderers,
publish / revoke / `/p/:token` and the `409` gate on `POST /pbl/projects` all
stay. They were not the problem.

## 9 · Anti-form review checklist

Every stage is checked against this before it ships. From the requirements
appendix:

- AI moves first — the first thing on screen is material 印记 brought, never an
  empty box.
- Her action is a judgment — pick, reject, drag, cross out, reorder.
- The page grows visibly right after, and 印记 replies proving it read her.
- Max one input on screen at a time. Two labelled boxes side by side fails.
- No noun-slot labels.
- **She types in at most 3 places across the whole journey**: the story she
  tells 印记 in stage 3, the one line she wants remembered, and any sentence she
  rewrites in stage 5.
- Every unlock condition is real.

## 10 · Testing

Logic only, per `AGENTS.md`:

- Seeded routine: five steps, every one with a non-empty `decide`, version 1,
  `decided_by = "ai"`, and idempotent — creating the website project twice does
  not seed twice.
- Prompt selection: `kind == "website"` gets the website routine block and the
  generic 「七个不同的时刻」 list is absent; every other kind is unchanged.
- The website routine must not claim abilities 印记 lacks; the generic prompt's
  「不要说你能查资料、做图、写网站」 becomes conditional, and a test pins that the
  website block grants exactly draw + fetch + render and nothing else.
- `sites`: URL normalisation, duplicate rejection, the three-card gate.
- `draw`: boot fails when `MODEL_DRAW` names a model without the `image`
  capability.
- Browser walk (`apps/lite-web/e2e/`), because a green suite over a blank PNG is
  the lesson this repo already paid for: create the project, see the seeded task
  list, walk stages 1–5, publish, screenshot each stage.


---

## 11 · What was built, and what the browser walk caught

Built and verified (2026-09-04):

- The website project opens the normal project room. `SiteStudio`,
  `ProjectSurface` and the dead `src/eco/` prototype are deleted.
- Defined topic + driving question, and the five-step routine seeded at creation
  as plan v1, `decided_by = "ai"`, every step `tentative`.
- A website routine block in the coach prompt, selected by `kind`; the generic
  moments list and the generic "you cannot draw" prohibition are replaced only
  for this project kind, and tests pin that other projects are untouched.
- `site_content` produce + `GroundSiteDraft`: 印记 places her own words on the
  page, and a line it cannot find verbatim in what she said never lands.
- Stage 1 `persona`, stage 2 `sites`, stage 3 `look`, plus `ship` for publishing.
  All four refeed into 印记.
- The `draw` class, the image adapter, and OSS persistence.

**Two defects the browser walk caught that no unit test would have:**

1. The publish gate still required `layout_why` — a field that no longer exists
   in any interface. Every page would have been complete and unpublishable, with
   an error pointing at a deleted input box.
2. `getPublicSite` hand-builds its site row and did not copy the new `palette` /
   `hero_key` columns, so **visitors saw the layout's default colours instead of
   the palette she chose.** Her own preview was correct, which is what made it
   invisible.

The walk now measures the computed `--st-accent` on the visitor's page rather
than relying on a screenshot: `#9C3B26` and `#2F5D8A` are indistinguishable at
thumbnail size, and the difference between them is the whole of "she chose a
palette".

Not built: stage 2's mindmap hand-off to `structure` and stage 4's `split` card
are wired in the prompt but have no dedicated verification walk yet; the
model-driven half of the journey (stages 1–4 end to end with real calls) is
covered only by the `LIVE_LLM=1` path.
