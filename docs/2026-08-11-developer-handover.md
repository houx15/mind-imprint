# Developer Handover — Mind Imprint (思维印记)

> **Start here** if you're joining or taking over part of this codebase. This is the orientation map: the technical requirements to run it, how the code is laid out, the design principles you must not break, and an index to every authoritative document. It complements the root **`README.md`** (the fuller, Chinese product+architecture overview) — read that next for depth.

---

## 1. What this is (in two sentences)

Mind Imprint is an **AI learning platform for IB / international-track students** that occupies the **thinking** layer, not the "homework gets done and submitted" layer. Students bring a real task; the AI guides them through structured thinking via interactive **thinking-tool cards (思维工具卡)**, and the whole process is recorded and turned into a **process evaluation (过程评估)** — the product's moat.

It is shaped like an agent tool (Cowork / Codex style) but wraps two capabilities found nowhere else: **(1)** schema-driven cards the AI *hands back to the student to fill in* (it never thinks for them), and **(2)** a dual-axis process evaluation over the full interaction trail. Live at **https://mind-web.uni-robot.cn**.

---

## 2. Design principles you must not break

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

**Prerequisites:** Go ≥ 1.26 · Node ≥ 20 · pnpm ≥ 8 · a PostgreSQL instance · one LLM API key (DeepSeek or Anthropic) · Docker (for testcontainers-based Go integration tests).

```bash
pnpm install                                   # frontend + contracts deps
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

## 5. Documentation map

> Convention: new docs are English-named, date-prefixed `YYYY-MM-DD-<kebab>.md`. Chinese-named docs predate that convention. Many dated `docs/2026-07-*` and `*-tracker.md` files are historical build logs — useful for archaeology, not current truth.

### Start here / cross-cutting
| Doc | What it is |
|---|---|
| **`README.md`** (root) | Fullest product + architecture overview (Chinese). Read after this handover. |
| **`AGENTS.md`** (root) | Hard-constraint summary for AI collaborators + the iron laws. `CLAUDE.md`/`GEMINI.md` only import it. **Shared memory is written here only.** |
| **This doc** | English onboarding / handover orientation. |

### Product & principles (authoritative)
| Doc | Authority |
|---|---|
| **`docs/思维印记_Demo_PRD.md`** | Product / invariant skeleton. **On product-philosophy conflicts, the PRD wins.** |
| **`docs/工具包库/`** | Full thinking-tool-card methodology library. |
| **`docs/design/`** | Design handoffs — the `.dc.html` files are **binding** for UI. |

### Architecture & backend (authoritative)
| Doc | Authority |
|---|---|
| **`docs/superpowers/specs/2026-06-24-backend-platform-architecture-design.md`** | Backend platform **north-star**. On anything backend / DB / API / auth / org / async-eval, **the architecture docs win** over the PRD's older "thin Node backend / SQLite" text. |
| **`docs/architecture/database-schema.md`** | DB schema reference. |
| **`docs/architecture/api-design.md`** | API design reference. |
| **`docs/architecture/go-backend-best-practices.md`** | Go backend conventions. |
| **`docs/architecture/chat-history-storage.md`** | Chat/thread storage design. |

### Behavior spec (single source of truth for the writing flow)
| Doc | Authority |
|---|---|
| **`docs/2026-08-09-all-statuses.md`** | **The behavior truth source** for the writing flow — every status's page/cards/AI-role/transition. **Check every plan against it before building.** |
| **`docs/2026-08-10-user-walk-and-model-routing-report.md`** | Model-tier routing (flash coaching vs flagship review) + a full user-walk report. |

### Evaluation & teacher-end (the current handover focus)
| Doc | What it is |
|---|---|
| **`docs/2026-08-11-evaluation-data-storage-guide.md`** | Where a student's whole journey lands — table by table, column by column, tied to the code that reads/writes it. For building process-evaluation logic. |
| **`docs/2026-08-11-evaluation-and-teacher-code-map.md`** | Where the evaluation code and the teacher/admin code live — every endpoint, agent, sqlc query, migration, contract, and view. Plus build-completeness and a quick-start reading order for each takeover. |

### Deploy & ops
| Doc | What it is |
|---|---|
| **`docs/deploy/README.md`** (+ `api` / `web` / `tls`) | Production runbooks (Docker Compose + host nginx + certbot on Aliyun ECS). |
| **`docs/2026-07-27-oss-storage-developer-guide.md`** | Aliyun OSS + CDN presigned upload/read developer guide. |

### Backlog / traceability
| Doc | What it is |
|---|---|
| **`docs/遗留项追踪_Carryforward.md`** | Item-by-item carry-forward tracker (ensures nothing gets dropped end-to-end). |
| **`docs/superpowers/specs/`** + **`plans/`** | Finalized design specs and their bite-sized implementation plans (the build history). |

---

## 6. Contributing checklist

1. Before changing anything, read `AGENTS.md`'s hard constraints and the four iron laws (§2 above).
2. Card spec's single source of truth is the `packages/contracts` registry — derive, don't duplicate.
3. **The client never talks to a model directly** — keys stay in `apps/api` server-side.
4. Before writing an implementation plan, check it against `docs/2026-08-09-all-statuses.md`.
5. Before committing, ensure `pnpm -r typecheck`, `pnpm -r test`, and `cd apps/api && make test` are all green.
6. **Shared memory / new rules go into `AGENTS.md` only** (`CLAUDE.md` / `GEMINI.md` just import it — never edit them directly).
