# Slice 3 · Read room / Library — SHARED SPEC

Room = `workspace/blocks/ReadingBlock.tsx`. Design: PRD §4.2 + `apps/web/src/proto/blocks/ReadingBlock.tsx`. Layer 1 = Zotero-style library (collections + reference table + preview + floating coach + add-source + empty state + annotated-bib export). Layer 2 = the **existing** Reading Room, entered from the preview's 进入阅读室 (do NOT rewrite it).

## Endpoint contracts (`/api/v1/projects/{id}`, session + ownership; 404 hides non-owned)

| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/library` | — | `{collections: Collection[], references: Reference[]}` (references carry projected `notes[]` + `materialId`) |
| POST | `/collections` | `{name, parentId?}` | `{collection}` |
| PATCH | `/collections/{cid}` | `{name?,parentId?,position?}` | `{collection}` |
| DELETE | `/collections/{cid}` | — | 204 (its references' `collection_id` → null via FK) |
| POST | `/references` | `{title?,url?,classification?,collectionId?,pending?,searchHints?}` | `{reference}` |
| PATCH | `/references/{rid}` | partial Reference (any of title/classification/author/credentials/year/url/tags/collectionId/credibility/evaluation/decision/pending/searchHints) | `{reference}` |
| DELETE | `/references/{rid}` | — | 204 |
| POST | `/references/{rid}/enter-reading` | — | `MaterialSource` (full DTO) or 422 `{error}` if no content source |

`Reference` / `Collection` = the contracts. `MaterialSource` = the existing `@mind-imprint/contracts` DTO (`packages/contracts/src/studioState.ts`).

## Backend (`apps/api`)
- Study: `internal/api/materials.go` `ingestMaterial` (URL fetch via injected `materialize.Fetcher`, SSRF-guarded, splits into blocks, creates `material` + `source_log_entry`), `internal/studio/projection.go` (how a material row → `MaterialSource` DTO, and how `card_instances.anchors` project). Reuse these.
- sqlc `workspace.sql` (extend): collections `ListCollections/CreateCollection/GetCollection/UpdateCollection/DeleteCollection`; references `ListReferences/CreateReference/GetReference/UpdateReference/DeleteReference`; plus a query to fetch submitted reading-card anchors for a material (or reuse an existing card_instance-by-material query) to project `notes`.
- Handlers in `internal/api/workspace_library.go`. All cheap CRUD (no LLM) except none here spend.
- `GET /library`: return all collections + references for the project. For each reference, project `notes: [{quote, finding}]` from **submitted** `card_instances` whose anchors target this reference's `material_id` (quote ← anchor.quote; finding ← anchor.answer or the card's finding field — best-effort; empty array if none/none-read). `materialId` from the reference row. Map DB snake↔JSON camel (`collection_id`→`collectionId`, `search_hints`→`searchHints`).
- `POST /references`: create a blank-metadata reference (pending default false unless passed). `PATCH`: merge partial over current row (validate enum fields → 400). `DELETE`.
- Collections CRUD similarly; a nestable `parent_id` (self-FK). Deleting a parent cascades children (FK) — fine.
- **`POST /references/{rid}/enter-reading`**: load owned reference. Ensure a `material`:
  - if `reference.material_id` set → load it;
  - else if `reference.url` non-empty → fetch via the Fetcher (same as ingestMaterial), create `material` (+ `source_log_entry`), set `reference.material_id`;
  - else → 422 `{error:"这条来源还没有可读内容——先补一个链接或粘贴正文"}`.
  Then build and return the full `MaterialSource` DTO for that material: real `blocks`; real `anchors`/`timeSpentS`/`lateralRead`/etc. if the material already has them (reuse projection), otherwise the zero/false defaults (locked=false, role/tier/takeaway="", anchors=[], timeSpentS=0, lateralRead/isLateralInstrument/siftSkipped=false, lateralRelation/lateralJudgment=""). Call `appendAutoLog(...,"打开来源《title》进入阅读室")`.
- Register routes in `internal/api/api.go`.
- Go tests: collections CRUD; references CRUD + PATCH metadata; library GET returns both; enter-reading with a URL (inject a fake Fetcher like existing tests) returns a MaterialSource with blocks + creates the material row + sets material_id; enter-reading with no url/material → 422; ownership 404.

## Frontend (`apps/web/src/workspace`)
- `api/workspace.ts` (extend): `getLibrary(id)`, `createCollection/patchCollection/deleteCollection`, `createReference/patchReference/deleteReference`, `enterReading(id, rid): Promise<MaterialSource>`.
- `WorkspaceContainer.tsx`: pass `projectId` + `setReadingSource` (already have the state) + `onOpenLogged?` down to `ReadingBlock`. `ReadingBlock`'s 进入阅读室 → `enterReading` → `setReadingSource(materialSource)` (container already renders `<ReadingRoom>` when `readingSource != null`). Handle the 422 with a gentle inline message.
- `blocks/ReadingBlock.tsx` — replace mock with real data:
  - `getLibrary` on mount; empty → the empty-library state + add-first-source.
  - Collections rail: fold/expand (local), nested render, **drag a reference row onto a collection** → `patchReference({collectionId})` (optimistic); `+` add collection → `createCollection`.
  - Reference table: rows from the library; multi-select; batch **export annotated-bib** (structured `.md`/`.csv` now, real `.xlsx` in slice 6) with columns aligned to `资源评估表.xlsx` (资源/分类/作者/作者资历/期刊·网站/相关性=notes/可信度评估/是否采用); drag rows (set dataTransfer ref id).
  - Preview: **all metadata edited in place → `patchReference`** (debounced for text fields; immediate for selectors) — title/author/classification/year/url/credentials/tags(add·remove)/decision(use·maybe·drop)/credibility(strong·mixed·weak)/evaluation. Pending refs show the search-hints view.
  - Add-source modal: link/DOI (POST /references with url) · upload (accepts a file → for now POST a reference with the filename as title, kind "上传文档"; real file→material parsing is a later concern) · manual (title) → `createReference`, land it in the chosen collection.
  - Floating coach → `coach(projectId, "find_sources", text)`.
  - 进入阅读室 → `enterReading`.
- Keep interactions identical to the prototype; the difference is persistence + real reading-room entry.

## Acceptance
- `go build ./...` + new Go tests green (Docker). `pnpm --filter web build` + `pnpm --filter web test` green.
- Data-flow: add a reference, edit its metadata, drag it into a collection, reload → persists. Enter a URL reference → the real Reading Room opens on its fetched content, back returns to the library. Reading outcomes on a material show up as that reference's `notes` in the annotated-bib export.
