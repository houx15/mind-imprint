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
| `web_resource` | `web/` | admin | `image/png`, `image/jpeg`, `image/webp`, `image/svg+xml` | 10 MB |
| `course_material` | `courses/` | admin | `image/png`, `image/jpeg`, `image/webp`, `application/pdf` | 50 MB |
| `user_image` | `users/{uid}/images/` | any logged-in user (own uid only) | `image/png`, `image/jpeg`, `image/webp` | 10 MB |

**Read access is uniform:** all three are login-gated — any authenticated user
can `resolve-url` any object key. (Per the approved decision, web resources are
*not* public; the marketing site would need auth to display them, which is
acceptable for now.)

The registry is a Go map keyed by scope string. Each entry:

```go
type Scope struct {
    Prefix       string          // "courses/" or "users/" (user_image expands with uid)
    WriteGate    WriteGate       // GateAdmin | GateSelf
    AllowedTypes map[string]bool // content-type allowlist
    MaxBytes     int64
}
```

`GateSelf` scopes interpolate the caller's uid into the prefix
(`users/{uid}/images/`). `GateAdmin` scopes require the caller's role to be
`admin` — the same predicate behind the existing `RequireRole("admin")`
middleware, checked inline (`u.Role == "admin"` via `UserFromContext`) because
the gate is conditional on scope within a single endpoint.

---

## 2. API surface

Both routes are registered under `protected` (require a valid session).

### `POST /api/v1/oss/upload-url`

Request:

```json
{ "scope": "user_image", "contentType": "image/png", "size": 34567, "filename": "cat.png" }
```

- `scope` — must be a known scope; else `400 unknown_scope`.
- `contentType` — must be in the scope's allowlist; else `400 unsupported_type`.
- `size` — must be `> 0` and `<= scope.MaxBytes`; else `400 file_too_large`.
- `filename` — optional; only its extension is used (sanitized) to build the key.
  The filename itself never appears in the object key.

Authorization:
- `web_resource` / `course_material` → caller's role must be `admin`
  (inline `u.Role == "admin"`); else `403` (`httpx.ErrForbidden("权限不足")`).
- `user_image` → any logged-in user; key is scoped to the caller's uid.

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

- `objectKey` — must be non-empty, contain no `..` path traversal, and start
  with one of the known scope prefixes (`web/`, `courses/`, `users/`); else
  `400 invalid_key`. No per-object ownership check (uniform login-gated read).

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
```

`oss.New` treats an empty `OSSAccessKeyID` as "disabled" and returns nil.

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
// Uploads a File and returns its object key (to store wherever it's used).
export async function uploadFile(scope: OssScope, file: File): Promise<string>;

// Resolves an object key to a short-lived signed GET URL for display/download.
export async function resolveUrl(objectKey: string): Promise<string>;
```

`uploadFile` calls `POST /oss/upload-url`, then `PUT`s the bytes to `putUrl` with
the required `Content-Type` header, and returns `objectKey`. Exported from
`apps/web/src/api/index.ts`. **No course-upload or profile-image UI** — those
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
- `course_material` / `web_resource` upload → `403` for a non-admin session,
  `200` for an admin;
- `user_image` upload → key is under `users/{callerUid}/images/`;
- content-type outside the allowlist → `400 unsupported_type`;
- `size > MaxBytes` → `400 file_too_large`;
- `resolve-url` with a traversal key (`../`) → `400 invalid_key`;
- both routes → `503 oss_disabled` when `Deps.OSS` is nil.

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
