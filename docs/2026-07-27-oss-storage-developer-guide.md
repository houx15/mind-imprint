# OSS Storage — Developer Guide

How to upload and read files (images, course media, documents) on the platform.
Storage is **Aliyun OSS** behind a **CDN** (`mind-oss.uni-robot.cn`), accelerated
and private.

> **Design & rationale:** `docs/superpowers/specs/2026-07-27-oss-storage-infra-design.md`.
> This guide is the how-to; the spec is the why.

---

## Mental model (read this first)

- **The client never holds the OSS AccessKey.** It asks our API for a
  short-lived **presigned URL**, then talks to OSS directly.
- **Upload** = ask for a signed `PUT` URL → `PUT` the bytes to OSS.
- **Read** = ask for a signed `GET` URL → use it as an `<img src>` / download it.
- **The bucket is private.** Every read is gated: an unsigned or expired URL
  gets `403`. Reads are served through the CDN for speed.
- **Server assigns the object key.** You never choose the path; you get a
  `objectKey` back and store it wherever you reference the file.
- **Stateless.** We don't track uploads in a table. If you upload a file, record
  its `objectKey` in whatever owns it (a course row, a profile field).

---

## The three endpoints

All live under `/api/v1/oss`. Errors use the standard envelope
`{ "error": { "code", "message" } }`.

### 1. `POST /oss/upload-url` — user image upload (session)

For a **logged-in user** uploading **their own image**. Requires a valid session
cookie. The object key is automatically scoped to the caller's user id.

```jsonc
// request
{ "contentType": "image/png", "size": 34567, "filename": "avatar.png" }
// response 200
{
  "putUrl": "https://mind-imprint.oss-cn-beijing.aliyuncs.com/users/<uid>/images/<uuid>.png?...",
  "objectKey": "users/<uid>/images/<uuid>.png",
  "requiredContentType": "image/png",
  "maxBytes": 10485760,
  "expiresAt": "2026-07-27T12:34:56Z"
}
```

### 2. `POST /oss/admin/upload-url` — admin upload (admin key)

For **backend scripts** uploading course materials / web assets. Requires the
admin bearer key (see [Keys](#keys)); **no user session**. Declare a `scope`
(`web_resource` or `course_material`).

```jsonc
// request  (header: Authorization: Bearer <OSS_ADMIN_KEY>)
{ "scope": "course_material", "contentType": "application/pdf", "size": 240311, "filename": "unit3.pdf" }
// response 200  → same shape as above; objectKey under courses/ or web/
```

### 3. `POST /oss/resolve-url` — get a read URL (session **or** admin key)

Turns a stored `objectKey` into a short-lived signed `GET` URL on the CDN.
Authorized by a session **or** the admin key.

```jsonc
// request
{ "objectKey": "courses/<uuid>.pdf" }
// response 200
{ "url": "https://mind-oss.uni-robot.cn/courses/<uuid>.pdf?...", "expiresAt": "..." }
```

---

## Scopes

A scope decides the key prefix, who may upload, the allowed content types, and
the size cap. Adding a new category = one entry in `ossScopes`
(`apps/api/internal/api/oss.go`) — no new endpoint.

| scope | prefix | who uploads | allowed types | max |
|---|---|---|---|---|
| `web_resource` | `web/` | admin key | png, jpeg, webp, svg | 10 MB |
| `course_material` | `courses/` | admin key | png, jpeg, webp, pdf | 50 MB |
| `user_image` | `users/{uid}/images/` | logged-in user (own uid) | png, jpeg, webp | 10 MB |

**All reads are login-gated** (a session or the admin key), regardless of scope.

---

## Using it from the frontend (TypeScript)

The web client is on the shared `api` object (`apps/web/src/api/oss.ts`):

```ts
import { api } from "@/api";

// Upload a user's image → returns the objectKey to persist.
const objectKey = await api.uploadUserImage(file); // file: File (png/jpeg/webp)

// Later, turn a stored key into a displayable URL (expires ~5 min — resolve on
// demand right before use, don't cache the URL).
const url = await api.resolveUrl(objectKey);
imgEl.src = url;
```

`uploadUserImage` calls `/oss/upload-url`, then `PUT`s the bytes to OSS with the
exact `Content-Type` the server signed. There is **no browser client for admin
uploads** — those are for scripts holding the admin key.

## Using it from a backend script (admin uploads)

Use the admin key. Two steps: get a signed URL, then `PUT` the file.

```bash
ADMIN_KEY="$OSS_ADMIN_KEY"   # from the maintainer / server env — never hard-code
API="https://mind-api.uni-robot.cn"

# 1) ask for an upload URL
resp=$(curl -sS -X POST "$API/api/v1/oss/admin/upload-url" \
  -H "Authorization: Bearer $ADMIN_KEY" -H "Content-Type: application/json" \
  -d '{"scope":"course_material","contentType":"application/pdf","size":'"$(wc -c < unit3.pdf)"',"filename":"unit3.pdf"}')
put_url=$(echo "$resp" | jq -r .putUrl)
object_key=$(echo "$resp" | jq -r .objectKey)

# 2) upload the bytes (Content-Type MUST match what was signed)
curl -sS -X PUT "$put_url" -H "Content-Type: application/pdf" --data-binary @unit3.pdf -w "PUT %{http_code}\n"

echo "stored as: $object_key"   # save this key wherever the course references it
```

To read it back (e.g. verify), resolve it:

```bash
curl -sS -X POST "$API/api/v1/oss/resolve-url" \
  -H "Authorization: Bearer $ADMIN_KEY" -H "Content-Type: application/json" \
  -d "{\"objectKey\":\"$object_key\"}" | jq -r .url
```

---

## Keys

There are **two different secrets**, both server-side only, **never committed to
git**:

| secret | what it is | where it lives |
|---|---|---|
| `OSS_ACCESS_KEY_ID` / `OSS_ACCESS_KEY_SECRET` | the Aliyun OSS AccessKey — signs every presigned URL | `apps/api/.env.local` (dev), `deploy/.env.prod` (server), `.deploy-local/env.prod` (local backup) |
| `OSS_ADMIN_KEY` | static bearer token authorizing admin uploads from scripts | same files |

- **Dev:** values are in `apps/api/.env.local` (git-ignored). Ask the maintainer
  if you need them.
- **Prod:** values are in `deploy/.env.prod` on the ECS (git-ignored), passed
  into the api container by `deploy/docker-compose.prod.yml`.
- If any `OSS_*` access var is unset, the `/oss/*` routes return `503`
  (`oss_disabled`) — the platform still boots. If `OSS_ADMIN_KEY` is empty, the
  admin upload route specifically returns `503`.
- **Rotating a key:** update the value in `deploy/.env.prod` (and
  `.deploy-local/env.prod` + `apps/api/.env.local`), then
  `docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod.yml up -d --build api`.
  Rotating `OSS_ADMIN_KEY` invalidates all scripts using the old value.

Full config var list: `OSS_ENDPOINT`, `OSS_BUCKET`, `OSS_CDN_DOMAIN`,
`OSS_ACCESS_KEY_ID`, `OSS_ACCESS_KEY_SECRET`, `OSS_ADMIN_KEY` (see
`apps/api/.env.example`).

---

## Gotchas

- **Content-Type must match.** The `PUT` must send exactly the
  `requiredContentType` the server returned — it's bound into the signature.
  Send the wrong type and OSS rejects it.
- **URLs are short-lived.** Upload URLs expire in **10 min**, read URLs in
  **5 min**. Resolve read URLs on demand; don't store them.
- **Size cap is a soft check.** `size` is validated before signing, but a plain
  presigned `PUT` can't hard-enforce byte count. Don't rely on it for untrusted
  clients beyond the login gate.
- **`%2F` in signed URLs is normal.** The signer percent-encodes `/` in keys;
  OSS decodes it. Round-trips fine — don't "fix" it.
- **Reads aren't owned.** Any logged-in user can resolve any key (uniform
  login-gated read). If you need per-object ownership, that's a future addition.

---

## Where course JSON goes (not OSS)

Course definition JSON (~100–200 lines) lives in **Postgres**, not OSS — it's
small, queryable, and permissioned. OSS holds the **media a course references**
(images/diagrams/PDF) under `courses/…`; the course JSON carries the object keys,
resolved via `/oss/resolve-url`.
