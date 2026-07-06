# Slice 2 · Material Substrate — Design Spec

> 2026-07-06 · Part of `docs/2026-07-06-refactor-roadmap.md` (Slice 2 of 5). Stacked on branch `slice-1-nav-ia-shell`.
> Binding design: `思维印记 工作区.dc.html` — `TASKS: WORKSPACE` right sidebar + `material-annotation model` section.
> Authority: for anything backend/DB/API, `docs/architecture/*` governs (Go + Postgres). The `.dc.html` is binding for UI.

## Goal

Introduce the `material` entity (a task's text + span index) end-to-end — Zod contract + Go/Postgres + API — and turn the workspace's right panel into a **材料 / 过程树** tabbed, draggable sidebar whose 材料 pane displays a task's materials. Material text arrives by **fetching the task's seed URL server-side, falling back to student paste** on failure. This is the substrate the Slice 3 card engine anchors onto; it adds **no** highlights, anchors, or AI questions.

## Global constraints

- **Backend is Go + Postgres.** Follow existing `apps/api` patterns: goose migration → sqlc query set → `internal/api` handler + DTO + route; ownership scoped so a task not owned by the authed user returns **404** (never 403). Client never calls the model; this slice makes no LLM call at all.
- **Contract parity is manual.** The Go DTO struct field names/JSON tags must match the Zod `Material` shape (there is no automated parity test — mirror by hand, as `dto.go` already does for tasks/cards).
- **SSRF guard is required** on the server-side URL fetch (details in §3). Non-negotiable.
- **UI copy is Chinese, verbatim from the binding design.** Code identifiers/comments English.
- **Docker is available**; Go tests use testcontainers (`postgres:16-alpine`), self-skipping under `-short`.
- **Scope cuts:** PDF parsing deferred — material `kind` is text-only (`article` | `draft`). The chat/card mechanism is untouched (that is Slice 3). No teacher surface.

## 1. Data model

### 1.1 Zod contract — `packages/contracts/src/material.ts` (exported from `index.ts`)

```
MaterialKind   = 'article' | 'draft'
MaterialSource = 'fetched' | 'pasted'
MaterialBlock  = { id: string, text: string }            // a paragraph; id stable within the material
Material = {
  id: string,
  task_id: string,
  kind: MaterialKind,
  source: MaterialSource,
  title: string,
  source_url: string | null,
  blocks: MaterialBlock[],       // ordered paragraphs — the span index
  scratch: string,               // 随手记, default ""
  created_at: string,            // RFC3339
}
```

The **span model:** a future anchor (Slice 3) addresses a range as `{ block_id, start, end }` (character offsets within that block's `text`). Slice 2 only stores blocks with stable ids; it defines no anchor type yet.

### 1.2 Go / Postgres

- Migration `internal/store/migrations/0009_material.sql`:
  - `CREATE TABLE material (id uuid PK default gen_random_uuid(), task_id uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE, kind text NOT NULL CHECK (kind IN ('article','draft')), source text NOT NULL CHECK (source IN ('fetched','pasted')), title text NOT NULL, source_url text, blocks jsonb NOT NULL DEFAULT '[]', scratch text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now())`
  - Index `material_task_created_idx (task_id, created_at)`.
  - `-- +goose Down`: `DROP TABLE material;`
- Queries `internal/store/queries/material.sql`: `CreateMaterial :one` (task_id,kind,source,title,source_url,blocks), `ListMaterialsByTask :many` (ORDER BY created_at), `GetMaterial :one`, `UpdateMaterialScratch :one` (`SET scratch=$3 WHERE id=$1 AND task_id=$2 RETURNING *`). Regenerate with `make sqlc`.
- DTO `materialDTO` + `toMaterialDTO` in `internal/api/dto.go`: `blocks` jsonb `[]byte` → `json.RawMessage`; timestamps `time.RFC3339Nano`; `source_url` `*string` → `null`.

## 2. Read + scratch endpoints (`internal/api/material.go`)

All wrapped by `RequireUser`; each resolves ownership via the existing `loadOwnedTask` helper (404 on non-owner). Routes registered in `internal/api/api.go`.

- `GET /api/v1/tasks/{id}/materials` → `200 { "materials": [Material...] }`.
- `POST /api/v1/tasks/{id}/materials` (paste) — body `{ kind: MaterialKind, title: string, text: string }`. Boundary validation: `kind` in enum; `title` trimmed non-empty (fallback `"未命名材料"` if empty); `text` non-empty and ≤ 500 KB. Segment `text` into blocks (§4). Create `source:'pasted'`. → `201 { "material": Material }`.
- `PUT /api/v1/tasks/{id}/materials/{mid}/scratch` — body `{ scratch: string }` (≤ 20 KB). `UpdateMaterialScratch` scoped by task → `200 { "material": Material }`; unknown/other-owner `mid` → 404.

## 3. Fetch-from-seed endpoint (`internal/api/material.go` + `internal/materialize/`)

`POST /api/v1/tasks/{id}/materials/from-seed` — no body. Loads the owned task; if `task.seed` is null/empty → `422 { error:{ code:"material_fetch_failed", reason:"no_seed" } }`. Otherwise fetch + extract + segment, create `source:'fetched', kind:'article', source_url:seed`, → `201 { "material": Material }`. On any fetch/extract failure → `422 { error:{ code:"material_fetch_failed", reason:<machine reason> } }` and **no** material persisted. The client treats any `material_fetch_failed` as "show the paste fallback".

Reasons (machine strings): `no_seed`, `blocked` (SSRF-denied host), `unreachable` (DNS/dial/timeout), `bad_status` (non-2xx), `unsupported_content` (not html/plain), `too_large`, `empty` (extraction produced no paragraphs).

### 3.1 Fetch + SSRF guard (`internal/materialize/fetch.go`)

A dedicated package, unit-testable without a task/DB. The material handler depends on a small `Fetcher` interface (`FetchReadable(ctx, rawURL) (title, text string, err error)`) held on `API`/`Deps`, defaulting to the real `materialize` implementation; handler tests inject a fake `Fetcher` so the `from-seed` success path is deterministic (no real network). The concrete `materialize.FetchReadable` carries a machine reason on `err`.

- URL must parse and use scheme `http`/`https`; else `blocked`.
- `http.Client` with `Timeout: 8s`; `CheckRedirect` caps redirects at 3 and re-runs the host/IP guard on each hop.
- Custom `Transport.DialContext`: after DNS resolution, reject if **any** resolved IP is loopback (`127.0.0.0/8`, `::1`), private (`10/8`, `172.16/12`, `192.168/16`, `fc00::/7`), link-local (`169.254.0.0/16`, `fe80::/10`) — this covers the `169.254.169.254` cloud-metadata address — unspecified (`0.0.0.0`, `::`), or not global-unicast → reason `blocked`. Guard at **dial time** (post-DNS) to defeat DNS-rebinding.
- Response: require `Content-Type` `text/html` or `text/plain` (else `unsupported_content`); read through `io.LimitReader` at 2 MB (exceed → `too_large`); set a `User-Agent`.
- Extract with a **minimal internal extractor on `golang.org/x/net/html`** (already in the module graph as a transitive dep — no new external download, build-robust): parse the HTML, take `<title>`, and collect text from block elements (`p`, `h1`–`h3`, `li`, `blockquote`) while skipping `script`/`style`/`nav`/`header`/`footer`/`aside`/`noscript`. `text/plain` responses skip parsing and use the body verbatim. (Chosen over `go-shiori/go-readability` to avoid a new external dependency whose download couldn't be verified in this environment; the spec's sanctioned fallback.)

### 3.2 Segmentation (`internal/materialize/segment.go`)

`Segment(text string) []Block` where `Block{ID, Text}`: split on blank-line / newline paragraph boundaries, trim each, drop empties, collapse internal runs of whitespace minimally (preserve sentence text). Assign ids `b0, b1, …` in order. Shared by both paste and fetch paths (paste calls the same segmenter). If zero blocks result → the fetch path reports `empty`; the paste path rejects with 400 (text was non-empty but unsegmentable is treated as invalid).

## 4. Frontend (`apps/web`)

- **API client** `src/api/materials.ts`: `listMaterials(taskId)`, `fetchMaterialFromSeed(taskId)` (resolves to `Material`; on a `material_fetch_failed` response throws a typed `MaterialFetchError` carrying `reason`), `createMaterial(taskId, {kind,title,text})`, `saveScratch(taskId, materialId, scratch)`. Wire into `src/api/index.ts` (`ApiClient` + `api`).
- **RightPanel** `src/workspace/RightPanel.tsx`: the sidebar shell — a 材料/过程树 segmented control (design `matTabStyle`/`treeTabStyle`), a draggable width divider (min 320 / max 640 px, held in local state), and a collapse toggle. Renders the existing `TreePanel` unchanged under 过程树, and `MaterialPane` under 材料. `WorkspaceView` swaps its current direct `TreePanel` render for `RightPanel` (passing the tree props through plus `taskId` and `seedUrl`).
- **MaterialPane** `src/workspace/MaterialPane.tsx`: self-contained (owns its material state via the API client; does not touch the store). On mount: `listMaterials`; if empty **and** `seedUrl` present, call `fetchMaterialFromSeed` once — on `MaterialFetchError` render the paste fallback (a title + textarea + "加材料" submit → `createMaterial`), on success show the material. Renders material tabs (design `materialTabs` chips) + active material's `blocks` as plain `<p>` paragraphs (no highlighting — Slice 3 adds it) + a debounced 随手记 textarea (`saveScratch`, ~600 ms). "加材料" is always available to paste an additional material (kind `draft` for own writing, `article` for pasted web text).
- No store-schema change; `MaterialPane` isolation keeps the substrate from entangling the workspace store.

## 5. Data flow

Open workspace → `RightPanel` (default tab 过程树, unchanged) → user switches to 材料 → `MaterialPane` mounts → `listMaterials`. Empty + seed → `from-seed` (server fetches, SSRF-guarded, extracts, segments, persists) → material renders. Fetch fails → paste fallback → `createMaterial` (server segments) → material renders. Scratch edits debounce-save. Everything task-ownership scoped server-side.

## 6. Error handling

- Server: typed `material_fetch_failed{reason}` (422) is the only non-standard result; all other errors use the existing `httpx` envelopes (400 validation, 404 ownership, 500 internal). Fetch never persists a partial material.
- Client: `MaterialFetchError` → paste fallback (not an error toast). `listMaterials`/`createMaterial`/`saveScratch` failures → inline non-blocking message; scratch save failure keeps local text and retries on next edit.
- SSRF-blocked and oversized fetches fail closed (no material, `blocked`/`too_large` reason).

## 7. Testing

- **Contracts:** `packages/contracts/src/material.test.ts` — `Material`/`MaterialBlock` parse + reject bad `kind`/`source`.
- **Go — `internal/materialize`:** `segment_test.go` (paragraph splitting, trimming, empty→none, id ordering); `fetch_test.go` using `httptest.Server` for success (html→title+text), non-2xx (`bad_status`), non-html (`unsupported_content`), oversized body (`too_large`); SSRF unit tests that the IP guard rejects loopback/private/link-local/metadata and accepts a public IP (test the guard function directly, no real network).
- **Go — `internal/api/material_test.go`** (testcontainers + `signInSeed`): paste create + list + scratch update happy paths; paste validation 400 (bad kind, empty text, oversized); cross-task-ownership 404 on list/scratch; `from-seed` with a null-seed task → 422 `no_seed`; `from-seed` success by injecting a fake `Fetcher` (returns a fixed title/text) → 201 with a `source:'fetched'` material whose blocks come from the segmenter.
- **Web:** `MaterialPane.test.tsx` — renders blocks from a mocked `listMaterials`; tab switch; scratch debounced save calls `saveScratch`; empty+seed triggers `fetchMaterialFromSeed`; `MaterialFetchError` renders the paste fallback and submitting calls `createMaterial`. `RightPanel.test.tsx` — 材料/过程树 flip renders the right child. Update any workspace test that asserted the old direct `TreePanel` render.

## 8. Out of scope (explicit)

Anchors / highlights / AI-generated questions / inline card branches (Slice 3); PDF materials; URL fetch of anything but the task seed; editing/deleting materials; multi-select or reordering; store-schema changes; teacher surface; the chat redesign.

## 9. Risks / notes

- Extraction uses a minimal internal `x/net/html` extractor (see §3.1) rather than `go-readability`, to avoid a new external dependency whose proxy availability couldn't be confirmed. Quality is lower than a full readability library but adequate for the substrate; revisitable later.
- Server-side fetch is the one security-sensitive addition — the SSRF guard and size/time caps are load-bearing and must be tested directly.
- Slice 2 is larger than Slice 1 (backend + frontend). If the plan's task list runs long, the natural split is **2a backend (contract + Go + fetch)** then **2b frontend (sidebar + pane)** — but it remains one spec.
