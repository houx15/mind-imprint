# OSS CDN URL 鉴权 — Design

> **Status:** design, ready for plan.
> **Supersedes:** the deferred cover-caching fix ([[oss-cover-cdn-bypass-2026-08-13]]) and the placeholder course `AssetResolver` from Course Runtime Slice 8.
> **Related:** `docs/2026-07-27-oss-storage-infra-design.md` (the OSS/CDN infra this builds on), `docs/2026-08-15-student-course-runtime-data-and-renderer-design.md` (the course runtime that consumes the asset URLs).

## Problem

Every read URL on the platform is minted by a single seam — `oss.Service.SignDownload(objectKey, ttl)` — which returns an **Aliyun OSS presigned GET** (`https://mind-oss.uni-robot.cn/<key>?OSSAccessKeyId=…&Signature=…&Expires=…`). Four callers use it:

| caller | serves | scope |
|---|---|---|
| `apps/api/internal/api/project_covers.go:86` (`resolveCoverURL`) | project covers | `web/` |
| `apps/api/internal/api/cards_catalog.go:134` | card covers | `web/` |
| `apps/api/internal/api/oss.go:267` (`ossResolveURL`, `POST /oss/resolve-url`) | user images, uploaded docs, course media | `users/`, `courses/` |
| `apps/api/internal/api/course_scene.go:197` | course narration audio | `courses/` |

`mind-oss.uni-robot.cn` **is** the Aliyun (Kunlun) CDN. But the CDN caches on the **full URL including query string**, and the OSS signature (`Signature`/`Expires`) rotates on every request — so the edge sees a fresh URL each time, gets a 100% cache miss, and proxies to the OSS origin on every read. Net effect: **nothing on the platform is actually CDN-accelerated**, despite fronting a CDN.

The bucket is **private**, so we cannot simply serve unsigned public URLs for the login-gated content under `courses/` and `users/`.

## Goal

Make CDN caching actually work for **all** read URLs — covers, card art, user images, course media — while keeping the bucket private and reads access-controlled. Close out the placeholder `makeCdnAssetResolver` from Course Runtime Slice 8 so a 2.0 course's referenced assets resolve to real, cacheable, access-controlled URLs.

## Mechanism: Aliyun CDN URL 鉴权 (Type A)

Change the **inside** of `SignDownload` only. Instead of an OSS presigned URL, emit a CDN **URL 鉴权 Type A** URL:

```
https://mind-oss.uni-robot.cn/<key>?auth_key=<ts>-<rand>-<uid>-<md5>
```

where, per Aliyun Type A:

- `ts`   = unix seconds at URL generation time
- `rand` = `"0"` (Aliyun reserves it; a fixed value is accepted)
- `uid`  = `"0"` (Aliyun reserves it; unused)
- `md5`  = `md5( "<uri>-<ts>-<rand>-<uid>-<privateKey>" )`, lowercase hex
- `uri`  = the object path with a single leading slash, url-**decoded** (e.g. `/courses/<slug>/assets/videos/case.mp4`)
- `privateKey` = the console 主KEY (server env `OSS_CDN_AUTH_KEY`)

The CDN edge validates the URL by recomputing the md5 and checking `now - ts <= 验证时长` (the validity window configured in the console). **`auth_key` is auto-excluded from the CDN cache key**, so two students requesting the same object share the edge cache while carrying different tokens.

**Key property — caching is window-independent.** Because `auth_key` (including `ts`) is excluded from the cache key, cache hit rate does not depend on the validity window. We therefore keep the window modest for security **and** get full caching — no tradeoff. The only floor on the window is "longer than one continuous consumption of a single asset" (a long video is fetched with range requests against one URL whose `auth_key` was minted once at resolve time; the window must outlast playback+idle for that file). With a 500 MB video ceiling we set the console 验证时长 to **7200s (2h)**.

### Why the private bucket still works (critical console step)

Switching off OSS presigning means the client URL no longer carries an OSS signature for the CDN to forward to the origin. So the **CDN itself** must authenticate to the private bucket on origin-fetch. This is a one-time console setting: enable **OSS 私有 Bucket 回源** (authorize the CDN domain to read the private bucket) on `mind-oss.uni-robot.cn`. Without it, requests 403 at origin **even with a valid `auth_key`**.

## Architecture

### Server: the seam (`apps/api/internal/oss`)

`oss.Service` gains three fields resolved from config in `New`: `cdnAuthKey string`, `cdnDomain string`, `cdnAuthWindow time.Duration`.

`SignDownload` **branches on configuration**:

- `cdnAuthKey != ""` → return a Type A URL鉴权 link (the new path).
- `cdnAuthKey == ""` → return the existing OSS presigned URL (`s.cdn.SignURL(...)`) — unchanged behavior, so the code can ship before the console is cut over.

The Type A computation is a **pure function** `signTypeA(cdnDomain, privateKey, objectKey string, ts int64) string` (no clock, no I/O) so it is deterministic and unit-testable against a known vector. `SignDownload` supplies `time.Now().Unix()`.

`SignDownload`'s signature drops its `ttl` parameter (Type A has no per-URL TTL — the window is the console's). The four callers stop passing a TTL. Callers that report an `expiresAt` to the client compute it from `cdnAuthWindow` (via a new `Service.DownloadWindow() time.Duration` accessor) rather than a local constant, so what the client is told matches the real window.

`SignUpload`, `PutObject`, `GetObject`, `Exists` are unchanged — uploads stay OSS-presigned against the origin; server-side object I/O stays on the origin client.

### Server: config (`apps/api/internal/config/config.go`)

Two new env vars (server-side only, both optional so the platform boots without them):

- `OSS_CDN_AUTH_KEY` — the URL鉴权 主KEY. Empty ⇒ presigned fallback (above).
- `OSS_CDN_AUTH_WINDOW` — validity window in **seconds**, `envDefault:"7200"`. Must mirror the console 验证时长. Used only to compute `expiresAt`/refresh timing; never sent to a client as a secret.

### Server: course asset-URL endpoint (`apps/api/internal/api/course_asset_urls.go`)

`POST /api/v1/courses/{slug}/asset-urls`, session-gated (same auth as the other course-runtime endpoints).

```jsonc
// request
{ "paths": ["assets/videos/case.mp4", "assets/images/case-poster.jpg", "assets/audio/intro.mp3"] }
// response 200
{
  "assetUrls": {
    "assets/videos/case.mp4":       "https://mind-oss.uni-robot.cn/courses/<slug>/assets/videos/case.mp4?auth_key=…",
    "assets/images/case-poster.jpg":"https://…?auth_key=…",
    "assets/audio/intro.mp3":       "https://…?auth_key=…"
  },
  "expiresAt": "2026-08-16T12:00:00Z"
}
```

Behavior:

- For each `path`, the server builds the object key `courses/<slug>/<path>` and signs it via `SignDownload`. The client sends **relative paths, never keys** — it can only obtain URLs within its own course's asset namespace.
- Each path is validated to be a safe relative asset path: non-empty, no leading `/`, no `..` segment, no scheme (rejects `http://`, `data:`). Invalid ⇒ `400 invalid_asset_path`. (This mirrors `packages/course-contract` `relativeAssetPathSchema`; the Go check is a small border validator, not a schema port.)
- The request body is capped (`http.MaxBytesReader`) and the path list is capped at a sane maximum (e.g. 256 paths) to bound work.
- If OSS is unconfigured (`a.d.OSS == nil`) ⇒ `503` like the other `/oss/*` routes.
- **This same endpoint is the refresh endpoint** — the client re-POSTs the same paths when the session outlasts the window.

The slug→prefix convention is **`courses/<slug>/<relativePath>`**. (The out-of-scope generator uploads a course's assets under that prefix; the golden seed course uses placeholder paths with no real OSS objects, so its video/pdf 404 at the edge — accepted, exactly as today.)

### Frontend: collect paths (`packages/course-contract`)

New exported pure function `collectAssetPaths(document: CourseDefinitionDocument): string[]` — walks the document and returns the **deduped** set of relative asset paths it references. It must cover **every** field typed `relativeAssetPathSchema` in the contract (so a new asset field can't silently drop): image item `source`, pdf `source`, video `source`/`poster`/`captions`/`interaction.source`, `interactiveHtml.source`, narration `audio`, and `opening.fallback.audio`/`closing.fallback.audio`. This is a dedicated walk (the referential validator does not touch assets). Absolute (`http(s)://`) and `data:` references are **excluded** (they need no signing).

### Frontend: the resolver becomes a lookup (`apps/web/src/course/assetResolver.ts`)

`makeCdnAssetResolver({ assetUrls })` returns a synchronous `AssetResolver` whose `resolve(relativePath)`:

1. passes through `http(s)://` and `data:` untouched;
2. returns `assetUrls[relativePath]` when present;
3. falls back to returning `relativePath` unchanged when absent (a missing asset renders as a broken/again-relative ref rather than throwing — the renderer already tolerates a failed asset load).

The old slug-base-join behavior is removed.

### Frontend: client + host wiring

- `apps/web/src/api/courseAssetUrls.ts` — `fetchCourseAssetUrls(slug, paths): Promise<{ assetUrls: Record<string,string>; expiresAt: string }>` calling the new endpoint.
- `apps/web/src/shell/courses/RuntimeCoursePlayer.tsx` — the host boundary. Restructure the load sequence:
  1. `getCourseDefinition(slug)` → document.
  2. `collectAssetPaths(document)` → paths.
  3. `fetchCourseAssetUrls(slug, paths)` → `assetUrls` (skip the call when `paths` is empty).
  4. Build adapters with `makeCdnAssetResolver({ assetUrls })`, then render `CoursePlayer`.
  - A **refresh timer** re-fetches `assetUrls` shortly before `expiresAt` (e.g. at 90% of the window) while the player is mounted, and swaps the resolver's map, so a long session's later asset loads keep a valid `auth_key`. Simplest correct implementation: store `assetUrls` in state, rebuild the resolver from it, and schedule the refetch from `expiresAt`.

The generic `POST /oss/resolve-url`, covers, and card art need **no** frontend change — they already consume whatever URL the server returns; the seam swap is transparent to them.

## Testing

- **Go `signTypeA`** — deterministic unit test: fixed `cdnDomain`, `privateKey`, `objectKey`, `ts` → assert the exact `auth_key` string against an independently computed md5 vector; assert the URL shape (`?auth_key=<ts>-0-0-<32 hex>`) and that the object path is preserved.
- **Go `SignDownload` branch** — with `cdnAuthKey` set, the URL carries `auth_key` and no `Signature`; with it empty, it falls back to the presigned path. (Construct `Service` directly in-package; no network.)
- **Go `course_asset_urls` handler** — table test over the pure request→key mapping and path validation (valid path → `courses/<slug>/<path>`; `..`, leading slash, scheme, empty → 400; over-cap list → 400). The signing itself is covered by the signer test; assert the handler builds the right keys and envelope. Auth gating mirrors the existing course-endpoint tests.
- **`collectAssetPaths`** — unit test against the golden coverage-course fixture: returns exactly its referenced relative paths, deduped, excluding absolute/`data:`.
- **`makeCdnAssetResolver`** — pure unit test: hit (returns mapped URL), miss (returns path unchanged), pass-through (`http(s)`/`data:`).
- **`RuntimeCoursePlayer`** — component test with the api mocked: loads definition → collects paths → fetches asset-urls → renders `CoursePlayer` with a resolver that resolves a known asset; empty-paths course skips the fetch.

## Rollout / Ops

Ordering matters — enabling URL鉴权 in the console immediately 403s any URL lacking `auth_key`:

1. **Console, non-breaking:** enable **OSS 私有 Bucket 回源** on `mind-oss.uni-robot.cn` (presigned reads keep working).
2. **Deploy, non-breaking:** ship this code with `OSS_CDN_AUTH_KEY` **unset** → identical to today (presigned fallback).
3. **Coordinated cutover:** in the console enable **URL鉴权 → Type A**, paste 主KEY + 备KEY, set 验证时长 = **7200s**; set `OSS_CDN_AUTH_KEY` = 主KEY and `OSS_CDN_AUTH_WINDOW=7200` in `deploy/.env.prod` (+ `.deploy-local/env.prod`, `apps/api/.env.local`); redeploy the api container. The brief window where an already-minted presigned URL is stale is harmless — read URLs are resolved on demand right before use, so a reload re-mints.

**Key rotation:** the 备KEY lets you rotate without downtime — Aliyun validates against both 主KEY and 备KEY. To rotate: set the new secret as 备KEY in the console, wait for propagation, move it to 主KEY in the console and `OSS_CDN_AUTH_KEY`, redeploy.

The 主KEY/备KEY for the cutover are **secrets and are delivered out-of-band (chat), never committed to git** — this doc must not contain their values. The server holds only the 主KEY (env `OSS_CDN_AUTH_KEY`); the 备KEY lives only in the CDN console. (An earlier draft of this doc committed a key pair; that pair is **burned** — do not use it — and was replaced with freshly generated keys delivered separately.)

## Out of scope

- The course **generator** that uploads real course assets under `courses/<slug>/…` and emits the relative paths (a separate skill/program). This slice makes the *serving* side correct; the golden seed course keeps placeholder asset paths.
- Per-object read ownership (any logged-in user can still resolve any key — the uniform login-gated read model is unchanged; URL鉴权 is a time-bounded bearer token per object, not an ownership check).
- Signed-cookie / whole-session auth (Aliyun has no CloudFront-style signed-cookie equivalent; Type A per-URL token is the mechanism).
```
