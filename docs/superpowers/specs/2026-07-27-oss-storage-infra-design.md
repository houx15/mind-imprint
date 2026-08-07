# OSS Storage Infrastructure — Design

**Status:** Approved (brainstorm) · 2026-07-27
**Owner:** houyuxin
**Type:** Backend platform infra (foundational)

## Goal

Give the platform a first-class way to store and serve binary files (documents,
images, media) on **Aliyun OSS with CDN acceleration**, without ever exposing
the OSS AccessKey to clients. The browser obtains a short-lived **presigned URL**
from the backend, then talks to OSS directly:

- **Upload** — client asks the API for a presigned `PUT` URL, then uploads the
  bytes straight to the OSS origin.
- **Read** — client asks the API for a signed `GET` URL, served through the CDN
  domain; every read is login-gated.

This is the shared substrate for three current upload categories (web resources,
course materials, user images) and any future ones — a new category is one
config-table row, not new code.

## Architecture

```
browser ──POST /api/v1/oss/upload-url──▶ Go API ──sign (server AccessKey)──▶ { putUrl }
browser ──PUT bytes───────────────────▶ OSS origin (mind-imprint.oss-cn-beijing.aliyuncs.com)

browser ──POST /api/v1/oss/resolve-url─▶ Go API ──sign (server AccessKey)──▶ { url }
browser ──GET (signed) ───────────────▶ CDN (mind-oss.uni-robot.cn) ──private origin──▶ OSS
```

- A new server-side package `apps/api/internal/oss` holds the single OSS
  AccessKey and does all signing. Nothing else in the app touches the key.
- Uploads are signed against the **OSS origin endpoint**; reads are signed
  against the **CDN domain** (SDK client configured with `UseCname(true)`) so
  reads are CDN-accelerated.
- The bucket stays **private**. A presigned read URL carries an OSS signature
  that the CDN forwards to the private origin, which validates it. Expired or
  tampered signature → `403`.
- If the OSS AccessKey is unset, the routes return **503** and the platform
  still boots — the same optional-feature pattern the `voice` package uses
  (`Deps.Voice` stays nil when unconfigured).

## Tech Stack

- **Signing:** Aliyun OSS Go SDK — `github.com/aliyun/aliyun-oss-go-sdk/oss`,
  `bucket.SignURL(...)`. No STS, no RAM role.
- Config via the existing `caarlos0/env` loader in `apps/api/internal/config`.
- Auth via the existing `protected` / `adminOnly` middleware and
  `UserFromContext`.
- Frontend: a thin typed client in `apps/web/src/api/oss.ts`. No UI.

## Global Constraints

- **The OSS AccessKey lives only in `apps/api` server env** — never in git,
  logs, thrown/rendered errors, data storage, or any evaluation payload. (铁律:
  密钥只在服务端。)
- **Client never holds the key** — it only ever receives short-lived presigned
  URLs.
- **The bucket is private.** No object gets public-read ACL. All reads are
  login-gated through `resolve-url` (uniform read model across all scopes).
- **Stateless** — no new Postgres table. The caller records the returned
  `objectKey` wherever it is used (a course row, a profile field) in a separate
  feature.
- **Server assigns the object key.** The client never chooses the path; keys are
  `{prefix}/{uuid}{ext}` so a user cannot write outside their allowed prefix.
- Fixed values: bucket `mind-imprint`, region/origin
  `mind-imprint.oss-cn-beijing.aliyuncs.com`, CDN domain `mind-oss.uni-robot.cn`.

---

## 1. Scope model

Every upload declares a **scope**. A scope is a single registry entry defining
four things — prefix, write gate, content-type allowlist, and size cap. Adding a
category later means adding one entry, not a new endpoint or handler (mirrors
"new card = new JSON, not new renderer code").

| scope | key prefix | write gate | content-type allowlist | max size |
|---|---|---|---|---|
| `web_resource` | `web/` | admin key | `image/png`, `image/jpeg`, `image/webp`, `image/svg+xml` | 10 MB |
| `course_material` | `courses/` | admin key | `image/png`, `image/jpeg`, `image/webp`, `application/pdf`, `video/mp4`, `video/webm`, `video/quicktime` | 500 MB |
| `user_image` | `users/{uid}/images/` | logged-in session (own uid only) | `image/png`, `image/jpeg`, `image/webp` | 10 MB |

`course_material` carries course **video** (`mp4`/`webm`/`mov`) alongside
images/PDF; its 500 MB cap is the scope ceiling (not per-type). Video is
admin-key-only and never permitted in `web_resource`/`user_image`. The presign
request body itself is capped at 4 KB (`http.MaxBytesReader`), independent of
the soft `size` check on the eventual `PUT`.

**Write gates are two distinct mechanisms:**
- **admin key** — a static bearer secret (`OSS_ADMIN_KEY`) sent as
  `Authorization: Bearer <key>`. Used by **backend scripts** (course/asset
  management) — no user session required. This is why admin uploads are *not*
  gated by user role.
- **logged-in session** — the existing cookie session; `user_image` keys are
  scoped to the caller's uid so a user can never write under another user's
  prefix.

**Read access is uniform:** all reads are gated — a valid **session or the admin
key** can `resolve-url` any object key. (Per the approved decision, web
resources are *not* public; the marketing site would need auth to display them,
which is acceptable for now.)

The registry is a Go map keyed by scope string. Each entry:

```go
type Scope struct {
    Prefix       string          // "courses/" or "users/" (user_image expands with uid)
    WriteGate    WriteGate       // GateAdminKey | GateSelf
    AllowedTypes map[string]bool // content-type allowlist
    MaxBytes     int64
}
```

`GateSelf` scopes interpolate the caller's uid into the prefix
(`users/{uid}/images/`) and require a valid session. `GateAdminKey` scopes
require a valid `OSS_ADMIN_KEY` bearer token and are meant for scripts.

---

## 2. API surface

Three routes. The two admin routes are gated by the admin-key seam; the user and
resolve routes accept a session (resolve also accepts the admin key).

### `POST /api/v1/oss/admin/upload-url` (admin key)

For backend scripts. Requires `Authorization: Bearer <OSS_ADMIN_KEY>`; no session.

Request:

```json
{ "scope": "course_material", "contentType": "application/pdf", "size": 240311, "filename": "unit3.pdf" }
```

- `scope` — must be an **admin-key scope** (`web_resource` or `course_material`);
  a `user_image` scope here → `400 unknown_scope`.
- validation of `contentType` / `size` / `filename` as below.
- Missing/invalid bearer key → `401 unauthorized`.

### `POST /api/v1/oss/upload-url` (session)

For logged-in users uploading their own images. Registered under `protected`.

Request:

```json
{ "contentType": "image/png", "size": 34567, "filename": "cat.png" }
```

- Scope is implicitly `user_image`; the key is scoped to the caller's uid.
- validation of `contentType` / `size` / `filename` as below.

### Common upload validation & response

- `contentType` — must be in the scope's allowlist; else `400 unsupported_type`.
- `size` — must be `> 0` and `<= scope.MaxBytes`; else `400 file_too_large`.
- `filename` — optional; only its extension is used (sanitized) to build the key.
  The filename itself never appears in the object key.

Response `200`:

```json
{
  "putUrl": "https://mind-imprint.oss-cn-beijing.aliyuncs.com/users/<uid>/images/<uuid>.png?OSSAccessKeyId=...&Expires=...&Signature=...",
  "objectKey": "users/<uid>/images/<uuid>.png",
  "requiredContentType": "image/png",
  "maxBytes": 10485760,
  "expiresAt": "2026-07-27T12:34:56Z"
}
```

The client must send the `Content-Type` header **exactly** matching
`requiredContentType` on its `PUT` (it is baked into the signature). The URL
expires in **10 minutes** (upload TTL).

### `POST /api/v1/oss/resolve-url`

Request:

```json
{ "objectKey": "courses/<uuid>.pdf" }
```

Authorized by a valid **session or** `Authorization: Bearer <OSS_ADMIN_KEY>`;
neither → `401 unauthorized`.

- `objectKey` — must be non-empty, contain no `..` path traversal, and start
  with one of the known scope prefixes (`web/`, `courses/`, `users/`); else
  `400 invalid_key`. No per-object ownership check (uniform gated read).

Response `200`:

```json
{
  "url": "https://mind-oss.uni-robot.cn/courses/<uuid>.pdf?OSSAccessKeyId=...&Expires=...&Signature=...",
  "expiresAt": "2026-07-27T12:39:56Z"
}
```

The URL is signed against the **CDN domain** and expires in **5 minutes** (read
TTL). It can be used directly as an `<img src>` / download link.

### Errors & disabled state

- All error bodies use the existing `httpx` error contract (`code` + Chinese
  `message`).
- When OSS is unconfigured (no AccessKey in env), both routes return
  `503 oss_disabled` with message "文件存储暂未开启。".
- Entitlement: uploads do **not** spend LLM tokens, so they are **not** gated by
  `HasEntitlement`; the session + scope gate is sufficient.

---

## 3. Object-key scheme

- `web_resource` → `web/{uuid}{ext}`
- `course_material` → `courses/{uuid}{ext}`
- `user_image` → `users/{uid}/images/{uuid}{ext}`

`{uuid}` is a fresh UUIDv4. `{ext}` is derived from the request `filename`
extension when present and safe (alphanumeric, `.`-prefixed, length ≤ 8), else
from a content-type→extension map, else empty. The client-supplied filename is
never used as a path segment.

---

## 4. The `oss` service package

`apps/api/internal/oss/oss.go`:

```go
// Service signs presigned OSS URLs. Nil when OSS is unconfigured.
type Service struct {
    origin *oss.Bucket // client bound to the OSS origin endpoint (uploads)
    cdn    *oss.Bucket // client bound to the CDN domain, UseCname(true) (reads)
    bucket string
}

// New returns nil (not an error) when cfg has no AccessKey — OSS is optional.
func New(cfg config.Config) (*Service, error)

// SignUpload returns a presigned PUT URL that requires the given content type.
func (s *Service) SignUpload(objectKey, contentType string, ttl time.Duration) (string, error)

// SignDownload returns a presigned GET URL on the CDN domain.
func (s *Service) SignDownload(objectKey string, ttl time.Duration) (string, error)
```

- Two `oss.Bucket` handles: the origin one for `HTTPPut` signing, the CDN one
  (`oss.New(cdnDomain, ak, sk, oss.UseCname(true))`) for `HTTPGet` signing.
- `SignUpload` passes `oss.ContentType(contentType)` so the signature binds the
  content type. **Known limitation:** OSS V1 presigned `PUT` cannot bind
  `Content-Length`; `size` is validated server-side before signing but a client
  could upload more bytes than declared. Acceptable for a login-gated internal
  tool; documented here and revisited only if abuse appears.
- The scope registry + key builder live in
  `apps/api/internal/api/oss.go` (handler layer), calling the `oss.Service` for
  signing. `Deps.OSS *oss.Service` is wired in `main`, nil when unconfigured.

---

## 5. Config & secrets

Added to `config.Config`:

```go
OSSEndpoint     string `env:"OSS_ENDPOINT"`      // mind-imprint.oss-cn-beijing.aliyuncs.com
OSSBucket       string `env:"OSS_BUCKET"`        // mind-imprint
OSSCDNDomain    string `env:"OSS_CDN_DOMAIN"`    // mind-oss.uni-robot.cn
OSSAccessKeyID  string `env:"OSS_ACCESS_KEY_ID"`
OSSAccessSecret string `env:"OSS_ACCESS_KEY_SECRET"`
OSSAdminKey     string `env:"OSS_ADMIN_KEY"`     // static bearer secret for scripts
```

`oss.New` treats an empty `OSSAccessKeyID` as "disabled" and returns nil. An
empty `OSSAdminKey` disables the admin routes (they return `503 oss_disabled`),
so no request can ever authenticate against a blank admin key.

Placeholders (names only, no values) added to:
- `apps/api/.env.example`
- `deploy/env.prod.example`

Real values go only in `apps/api/.env.local` (dev) and `deploy/.env.prod`
(server), both git-ignored. Never committed.

---

## 6. Aliyun console setup (ops runbook)

Documented as deployment steps (not code):

1. **Bucket** `mind-imprint`, region 华北2（北京 / cn-beijing), ACL **私有
   (private)**.
2. **AccessKey** — one key pair with read+write on the bucket. Store in server
   env only. (Per the approved decision: one key, no separate RAM sub-account;
   backend/CLI course management uses this same key via scripts that read server
   env.)
3. **CDN domain** `mind-oss.uni-robot.cn`, origin = the OSS bucket
   (`mind-imprint.oss-cn-beijing.aliyuncs.com`):
   - **保留参数回源 (forward query string to origin): ON** — so the client's OSS
     signature reaches the private origin.
   - Do **not** enable CDN's own "私有 Bucket 回源" identity — that would make
     the CDN fetch with its own credentials and bypass the per-request OSS
     signature (turning reads public). We want the OSS signature to be the
     authorizer.
4. **DNS** `mind-oss.uni-robot.cn` → the CDN CNAME
   `mind-oss.uni-robot.cn.w.kunlunaq.com` (already provisioned).

**Verified live 2026-07-27** (real bucket + CDN): presigned `PUT`→origin `200`;
signed `GET`→CDN `200` byte-exact; unsigned `GET`→CDN `403`; tampered signature
→CDN `403` with `SignatureDoesNotMatch` *from OSS* (confirming the CDN forwards
the signature to the private origin rather than masking it with its own
identity). Model A (OSS-signature-through-CDN) holds; no CDN URL-auth needed.

**Verification** (run after setup):

```bash
# valid signed URL → 200
curl -sI "$(curl -s -X POST https://mind-api.uni-robot.cn/api/v1/oss/resolve-url \
  -H 'Content-Type: application/json' -b cookies.txt \
  -d '{"objectKey":"courses/<known>.pdf"}' | jq -r .url)" | head -1   # HTTP/2 200

# tampered signature → 403
curl -sI "https://mind-oss.uni-robot.cn/courses/<known>.pdf?Signature=bogus" | head -1  # HTTP/2 403
```

---

## 7. Frontend client (thin, no UI)

`apps/web/src/api/oss.ts`:

```ts
// Uploads a user image and returns its object key (to store wherever it's used).
export async function uploadUserImage(file: File): Promise<string>;

// Resolves an object key to a short-lived signed GET URL for display/download.
export async function resolveUrl(objectKey: string): Promise<string>;
```

`uploadUserImage` calls `POST /oss/upload-url` (session), then `PUT`s the bytes
to `putUrl` with the required `Content-Type` header, and returns `objectKey`.
Both are added to the `ApiClient` interface + `api` object in
`apps/web/src/api/index.ts`. There is deliberately **no browser client for the
admin upload route** — that route is driven by backend scripts holding the
`OSS_ADMIN_KEY`, not the app. **No course-upload or profile-image UI** — those
are separate future features; this ships the consumable API only.

---

## 8. Testing

**Go unit tests (`oss` package)** — construct a `Service` with a fixed fake
AccessKey and assert on `SignUpload` / `SignDownload` output:
- upload URL host = OSS origin endpoint; download URL host = CDN domain;
- object key present in path; `OSSAccessKeyId`, `Expires`, `Signature` query
  params present;
- `nil` service returned when AccessKey empty.

**Go handler tests (`api` package, testcontainers)**:
- admin upload → `401` with no/wrong bearer key, `200` with the correct key;
- admin upload with `scope:"user_image"` → `400 unknown_scope`;
- user upload (session) → key is under `users/{callerUid}/images/`;
- content-type outside the allowlist → `400 unsupported_type`;
- `size > MaxBytes` → `400 file_too_large`;
- `resolve-url` → `200` with a session, `200` with the admin key, `401` with
  neither; traversal key (`../`) → `400 invalid_key`;
- all routes → `503 oss_disabled` when `Deps.OSS` is nil (and admin routes also
  `503` when `OSS_ADMIN_KEY` is empty).

Run the full `internal/oss` and `internal/api` packages (not `-run` subsets).

## 9. Out of scope (explicitly not this spec)

- The `course` table / new course-design (course JSON lives in Postgres `jsonb`,
  ~100–200 lines; separate spec). This spec only makes OSS ready to serve the
  media a course references.
- Any upload UI (course authoring screen, profile-image picker).
- An `oss_object` metadata table, listing, GC, or per-object ownership on read.
- STS / RAM roles, multipart / resumable uploads, public-read assets.

## Security invariants (recap)

- AccessKey server-only; never logged, rendered, stored, or committed.
- Client receives only short-lived presigned URLs (10 min upload / 5 min read).
- Bucket private; every read login-gated; server assigns keys so writes can't
  escape a scope's prefix (and `user_image` can't escape the caller's uid).
