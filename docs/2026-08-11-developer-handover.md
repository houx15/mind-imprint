# Developer Handover — Mind Imprint (思维印记)

> **Start here** if you're joining or taking over part of this codebase. This is the orientation map: the technical requirements to run it, how the code is laid out, the design principles you must not break, and an index to every authoritative document. It complements the root **`README.md`** (the fuller, Chinese product+architecture overview) — read that next for depth.

---

## 1. What this is (in two sentences)

Mind Imprint is an **AI learning platform for IB / international-track students** that occupies the **thinking** layer, not the "homework gets done and submitted" layer. Students bring a real task; the AI guides them through structured thinking via interactive **thinking-tool cards (思维工具卡)**, and the whole process is recorded and turned into a **process evaluation (过程评估)** — the product's moat.

It is shaped like an agent tool (Cowork / Codex style) but wraps two capabilities found nowhere else: **(1)** schema-driven cards the AI *hands back to the student to fill in* (it never thinks for them), and **(2)** a dual-axis process evaluation over the full interaction trail. Live at **https://mind-web.uni-robot.cn**.

---

## 2. Product laws & engineering constraints (must not break)

> These are *product* laws, not UI design — for the visual/design architecture (design system, components, styles) see §5.

These are the product's spine — **the AGENTS.md calls them 铁律 (iron laws); violating them is self-destruction.**

| # | Law | What it means for your code |
|---|---|---|
| 1 | **AI restraint — never conclude for the student (AI 克制)** | The AI's job is to hand *thinking* back at the right moment, not give answers. This lives in the system prompt's "restraint ladder." |
| 2 | **No manipulation (不操纵)** | No slot-machine mechanics (streaks, leaderboards, addictive push). Cards **trigger automatically, but the student always confirms "open."** 印记 *proposes*, the student *confirms* (e.g. question-graph edges, source placement). |
| 3 | **One question at a time (一次只问一个)** | Coaching turns are short — saves tokens, protects the student's thinking rhythm. |
| 4 | **Process is data (过程即数据)** | A student skipping a card or letting the AI answer directly is *also* recorded. Friction becomes signal, not something to eliminate. |

**Critical scope boundary (do not over-apply the laws):** "AI never does it for the student / student confirms" applies **only to the student's body text (正文)** — the AI never ghostwrites prose. It does **not** apply to deterministic system steps: generating a plan, deriving an outline from a stated research question or a standard framework template are things the system *should* auto-do — that is not manipulation and needs no confirmation gate. Don't use law #2 to challenge those.

**Writing-room stance (2026-07-30):** the writing room is a **plain writing surface** — the student writes the body themselves; the AI only helps them think and checks argument/structure. Three hard constraints: **① AI never ghostwrites; ② no fancy formatting / document editor** (not a Google Docs replacement); **③ we never submit on the student's behalf.** The process tree on the right is **read-only**.

### Hard engineering constraints
- **The client never talks to a model directly.** All LLM calls go through the Go gateway, which records tier + tokens + cost. API keys live only server-side (`apps/api` env).
- **Card spec single source of truth is the contracts registry.** The decision-layer catalog is *derived* from it — never hand-write a second copy (avoids `trigger_condition` drift).
- **New card = new JSON config, no renderer code change.** This is the test of whether schema-driven holds. Compose from existing field primitives unless a genuinely new interaction is needed.
- **The standard envelope shape is a shared foundation** for the process tree, usage counts, and evaluation — don't change it casually. Go validates only the outer envelope; the deep interior shape is owned by the Zod contracts in `packages/contracts`.
- **Org invariant:** every account belongs to a school; students/teachers also belong to ≥1 class. No "org-less account." Signup requires a valid class join code (teachers require an admin-issued invite); account creation and `enrollments` happen in one transaction.
- **Secrets only in `apps/api` server env** (LLM key / DB DSN / session secret / SMTP) — never in git, logs, thrown/rendered errors, data storage, or evaluation payloads.
- **Every implementation plan must be checked against `docs/2026-08-09-all-statuses.md`** before building — it is the single source of truth for the writing flow's per-status behavior.

---

## 3. Technical requirements & running it

**Prerequisites — exact versions, pinned to match the Docker build (do not loosen to "≥ x"):**

| Tool | Version | Pinned by |
|---|---|---|
| Go | **1.26** | `apps/api/go.mod` + `golang:1.26` build image |
| Node | **22** | `apps/web/Dockerfile` → `node:22-bookworm-slim` (+ `deploy/pull-base-images.sh`) |
| pnpm | **10.29.3** | root `package.json` `packageManager` field + `pnpm-lock.yaml` + web image `corepack prepare pnpm@10.29.3` |
| PostgreSQL | **16** | `deploy/docker-compose.prod.yml` → `postgres:16-alpine` |

Plus: one LLM API key (DeepSeek or Anthropic) · Docker (for testcontainers-based Go integration tests). pnpm is activated via corepack — do **not** install it manually.

```bash
corepack enable                                # auto-activates pnpm 10.29.3 from the packageManager field
pnpm install --frozen-lockfile                 # frontend + contracts deps (matches CI/Docker)
cp apps/api/.env.example apps/api/.env.local   # fill DATABASE_URL, CORS_ORIGINS, DEEPSEEK_API_KEY / ANTHROPIC_API_KEY
cd apps/api && make migrate-up && cd ../..     # tables + seed (school + class + Phoebe + wu.teacher + admin)
cd apps/api && make run                        # backend gateway → http://localhost:8080
# in another terminal:
cp apps/web/.env.example apps/web/.env.local   # point at http://localhost:8080
pnpm --filter web dev                          # frontend → http://localhost:5173
```

Seed logins: `phoebe@demo.mindimprint.local` / `wu.teacher@demo.mindimprint.local` (password `phoebe-dev-pass`); admin `admin@demo.mindimprint.local` / `admin-dev-pass`. Or visit `?trial=1` to auto-login as Phoebe.

**Quality gates (all must be green before commit):**
```bash
pnpm -r typecheck                 # frontend + contracts (tsc --noEmit)
pnpm -r test                      # frontend + contracts (Vitest)
cd apps/api && make test          # Go unit + testcontainers integration
```
- Regenerate sqlc after editing `.sql` queries: `cd apps/api && make sqlc` (on macOS uses `CGO_ENABLED=0` WASM parser; pin sqlc `@v1.27.0`).
- A separate **live E2E suite** hits the real backend + real DeepSeek key to catch LLM output-shape drift the mock suite can't.
- **Reasoning models** spend "thinking" tokens before visible output — if `maxTokens` is too small, `content` comes back empty / JSON truncated. The gateway budgets each call path; evaluation runs flagship, never downgraded.

**Tech stack:** React 18 + Vite + TS + Tailwind (web) · Go 1.26 `net/http` + `pgx/v5` + `sqlc` + `goose` (api) · PostgreSQL 16 · Zod contracts · DeepSeek V4 (flagship for eval, flash for coaching) · Volcano Engine TTS/ASR · OpenAlex + Crossref for literature · argon2id + session cookies · Docker Compose + host nginx + certbot on Aliyun ECS.

---

## 4. Code structure

pnpm monorepo (frontend + contracts + marketing sites) + a standalone Go module (backend):

```
mind-imprint/
├── packages/contracts/     # SINGLE SOURCE OF TRUTH: Zod schemas + card library (shared FE/BE)
│   ├── src/                # primitives · cardSpec · registry · envelope/event/graph ·
│   │                       #   dualaxis.json + dualAxisReport · exploration/reference ·
│   │                       #   chat/summonCard/refeed · rubric · studioState · parentReport …
│   └── cards/              # 34 cards, one JSON each (new card = new JSON)
├── apps/api/               # Go backend: intelligent gateway · data services · authz · evaluation
│   ├── cmd/api/            # entrypoint: migrate / serve
│   └── internal/
│       ├── gateway/        # LLM gateway: deepseek/anthropic providers, SSE, keyResolver, pricing
│       ├── agent/          # prompt / refeed / turn loop / evaluation orchestration
│       ├── studio/ course/ chat/   # the three collaboration surfaces' domain logic
│       ├── api/            # HTTP handlers: auth / 3 surfaces / evaluation / org / teacher / RBAC
│       ├── rubric/ ability/         # dual-axis rubric · ability model
│       ├── teacher/ parent/         # class weekly · student reports · parent reports
│       ├── auth/ org/               # argon2id · session · school/class/roster · join codes
│       ├── cards/ skills/ onboarding/ materialize/  # go:embed cards · skills · OpenAlex+Crossref
│       ├── voice/          # Volcano TTS / ASR
│       ├── store/          # pgx pool + sqlc + goose migrations
│       └── config/ httpx/  # 12-factor env · server · error envelope
├── apps/web/               # React SPA (pure render + API client; ONE bundle, role-routed)
│   └── src/
│       ├── api/            # backend API clients
│       ├── workspace/      # Studio's four rooms: project mgmt / reading / writing / review
│       │   └── blocks/exploration/   # rabbit-hole map (React Flow + dagre)
│       ├── studio/         # shared studio pieces: chat / process tree / card sheet / reading room
│       ├── cards/ primitives/        # schema-driven card renderer + field components + reducers
│       ├── console/        # teacher / admin console + class weekly + parent reports
│       ├── shell/          # app shell: role routing / auth / home / growth report / settings
│       └── dev/            # dev-only harnesses (not mounted in the product shell)
├── apps/site/              # Mind Imprint marketing site (Astro, zh/en)
├── apps/peraspera/         # Per Aspera parent-brand marketing site (separate Astro app → Vercel)
├── deploy/                 # prod orchestration: docker-compose + nginx sites (NO secrets)
└── docs/                   # authoritative product specs + architecture north-star + runbooks
```

**Core mental model:** a card = "a tool in a tool-use loop that a human executes." Three decoupled layers, each evolving independently:
**Decision layer (server)** decides use/which-card and wires via a single function `summon_card(card_id, reason, nudge_text)` → **Card Runtime (frontend)** renders from schema, three visual states, collects events, submits the standard envelope → **Sedimentation** grows all envelopes into a read-only process tree + feeds the flagship dual-axis evaluation.

**Three surfaces, one evaluation model:** Studio (project research) · Course (guided single lesson) · Chat (lightweight thread) — each produces isomorphic envelopes and is evaluated by the same dual-axis model.

---

## 5. Design architecture (design system · components · styles)

The UI is a **token-driven design system** — every color, radius, shadow, and type step is a CSS variable, and Tailwind classes map to those variables. The look is "Toddle warm-editorial + Apple narrative": warm paper neutrals, a single swappable accent, macaron category colors, restraint over decoration.

### 5.1 Design tokens (the single source for styling)
- **Definitions:** `apps/web/src/index.css` declares every `--mk-*` variable on `:root` (neutrals, accent 50→800, macarons, semantic, radius, shadow, motion). `apps/web/src/ui/tokens.ts` holds the same values as TS constants for JS-side use (e.g. inline SVG fills). `apps/web/tailwind.config.ts` maps `mk-*` Tailwind classes to those vars.
- **Neutrals:** warm paper — `--mk-paper #FBF8F4`, `--mk-surface #FFFFFF`, `--mk-border #EFE7DD`, ink `#33302E`, plus muted/secondary/faint.
- **Radius / shadow / motion:** `--mk-radius-{xs..full}`, `--mk-shadow-{xs..lg}` (warm-tinted), `--mk-ease cubic-bezier(.2,0,0,1)` with fast/base/slow durations.
- **Typography scale:** `mk-display / mk-h1 / mk-h2 / mk-h3 / mk-body-lg / mk-body / mk-small / mk-caption / mk-label` (font sizes in `tailwind.config.ts`). System font stack (PingFang SC / -apple-system).

> ⚠️ **Token gotcha (bit us repeatedly):** `bg-mk-<token>/<opacity>` renders **transparent** — the `mk-*` colors are CSS vars holding hex, and Tailwind's opacity modifier can't compose them into a valid rgb. Use a **solid** token, or `bg-black/NN` for translucency. Also `@keyframes` are global — prefix custom ones (`mk-pebble-*`, `mk-think-*`, `mk-shim`).

### 5.2 Accent "随人" (per-user accent) — `apps/web/src/ui/accent.tsx`
The accent color follows the user. **8 presets**, each a full 50→800 scale keyed to a fixed base at 500 (hard-coded literals, no runtime color math):
`vermilion 朱砂红 #EA5140` (default) · `clay 陶土红` · `tangerine 蜜柑橙` · `bamboo 竹青绿` · `teal 松石青` · `indigo 靛蓝` · `violet 紫棠` · `rose 玫紫`.
- `AccentProvider` seeds from the server-known user accent (`MeUser` color / `users.card_theme`) → falls back to localStorage → `vermilion`. `applyAccent(el, id)` writes `--mk-accent-*` onto `documentElement`; `setAccent` persists locally + fire-and-forget to the server. Everything themed through `mk-accent*` re-colors instantly.

### 5.3 Category & status colors
- **Macarons (7)** — `peach / butter / matcha / lake / mist / taro / berry`, each a `{base, bg, fg}` triple (`tokens.ts` / `--mk-<name>{,-bg,-fg}`). Used for categorical coding (card categories, tags), **not** as the accent.
- **Semantic (4)** — `success / warning / danger / info`, each `{base, bg}`. Semantic color is separate from the accent — don't reuse one for the other.

### 5.4 Iconography & illustration
- **Icons:** [`lucide-react`](https://lucide.dev/) via the `apps/web/src/ui/Icon.tsx` wrapper (note: `apps/web/src/workspace/Icon.tsx` is a workspace-local variant — **its `Icon` takes no `className`**, a known gotcha). No emoji as UI chrome.
- **Illustrations:** hand-picked [unDraw](https://undraw.co/) SVGs in `apps/web/src/assets/illustrations/` for empty states (never leave an empty panel blank).

### 5.5 AI presence & card visual states (the interaction signature)
- **豆豆 Pebble** (`apps/web/src/ui/Pebble.tsx`) — the AI avatar, an accent-colored pebble with **four+ animated states**: `idle · thinking · generating · processing · done`. Geometry and keyframes are copied verbatim from `docs/design/design-system-mockups/pebble-states.html` — **do not re-derive**; it never hardcodes a color (always the accent).
- **Card three visual states** — every thinking-tool card renders in one of three states, the product's core interaction rhythm: `建议工具卡` (student confirms opening) → `现在轮到你想` (active, collecting events) → `已完成 · 已钉到过程树` (serialized to the standard envelope). This is schema-driven — the renderer is `apps/web/src/cards/` + field components in `apps/web/src/primitives/`; a new card is JSON only (§2 hard constraints).

### 5.6 Component architecture (where UI lives)
The web `src` is organized by responsibility, not by technical layer:
| Dir | Responsibility |
|---|---|
| `ui/` | The design-system primitives — `tokens`, `accent`, `Icon`, `Pebble`, form controls, loaders. Start here for any shared visual element. |
| `primitives/` | Schema-driven **field** components (text / textarea / single_choice / rating / repeatable_group / link_check …) + the envelope reducer. |
| `cards/` | The schema-driven **card renderer** (three visual states) that composes primitives from a card's JSON. |
| `workspace/` | The Studio's four rooms; `workspace/blocks/` is the component library for them, `workspace/blocks/exploration/` the React-Flow rabbit-hole map. |
| `studio/` | Shared studio pieces — chat, process tree, card sheet, reading room, evaluation reveal, the 豆豆 `Bean`. |
| `console/` | The teacher/admin console views. |
| `shell/` | App shell — role routing, auth, home, growth report, settings (where `AccentProvider` is mounted). |
| `dev/` | Dev-only harnesses — **`DesignSystemGallery.tsx`**, reachable at `?ds`, is the living catalog of tokens/components. Not mounted in the product shell. |

**Styling conventions:** style through Tailwind `mk-*` classes (→ tokens), never raw hex in components; reach for `tokens.ts` constants only when JS needs a value (e.g. an inline SVG fill). Keep decoration restrained — the design language is quiet by intent (law #2). The writing room deliberately does **not** ship a rich formatting/document editor (§2 writing-room stance).

### 5.7 Design source of truth
- **`docs/design/思维印记_工作区.dc.html`** — the design handoff; **the `.dc.html` is binding for UI**. When code and this file disagree on layout/spacing/visual, the `.dc.html` wins (or return to the user).
- **`docs/design/design-system-mockups/`** — reference mockups (e.g. `pebble-states.html`) whose values are copied verbatim into code.
- **`docs/design/teacher end/`** — teacher-console design references.

---

## 6. Documentation map

> **Deliberately short.** This lists only docs **verified current as of 2026-08-11**. The repo holds many older specs, trackers, and an `docs/architecture/` set — but those are frozen at the June backend refactor, before the current schema, exploration graph, dual-axis evaluation, and status machine. They are **history, not truth**; don't onboard from them. When they disagree with the docs below or with the code, the docs below and the code win.

### The two you most need (this handover's focus)
| Doc | What it is |
|---|---|
| [Evaluation data-storage guide](./2026-08-11-evaluation-data-storage-guide.md) | Where a student's whole journey lands — table by table, column by column, tied to the code that reads/writes it. Grounded in the live prod schema. For building process-evaluation logic. |
| [Evaluation & teacher-end code map](./2026-08-11-evaluation-and-teacher-code-map.md) | Where the evaluation code and the teacher/admin code live — every endpoint, agent, sqlc query, migration, contract, and view. Plus build-completeness and a quick-start reading order for each takeover. |

### Living / cross-cutting (kept current by active use)
| Doc | What it is |
|---|---|
| [README.md](../README.md) | Fullest product + architecture overview (Chinese). Read after this handover. |
| [AGENTS.md](../AGENTS.md) | Hard-constraint summary for AI collaborators + the iron laws. `CLAUDE.md`/`GEMINI.md` only import it. **Shared memory is written here only.** |

### Behavior spec — single source of truth for the writing flow
| Doc | Authority |
|---|---|
| [all-statuses](./2026-08-09-all-statuses.md) | Every writing status's page / cards / AI-role / transition. **AGENTS.md mandates checking every implementation plan against it before building** (on conflict, it wins). |

### Deploy & ops
| Doc | What it is |
|---|---|
| [Deployment runbooks](./deploy/README.md) (+ `api` / `web` / `tls` alongside it) | Production runbooks — Docker Compose + host nginx + certbot on Aliyun ECS. |

---

## 7. Contributing checklist

1. Before changing anything, read `AGENTS.md`'s hard constraints and the four iron laws (§2 above).
2. Card spec's single source of truth is the `packages/contracts` registry — derive, don't duplicate.
3. **The client never talks to a model directly** — keys stay in `apps/api` server-side.
4. Before writing an implementation plan, check it against `docs/2026-08-09-all-statuses.md`.
5. Before committing, ensure `pnpm -r typecheck`, `pnpm -r test`, and `cd apps/api && make test` are all green.
6. **Shared memory / new rules go into `AGENTS.md` only** (`CLAUDE.md` / `GEMINI.md` just import it — never edit them directly).
