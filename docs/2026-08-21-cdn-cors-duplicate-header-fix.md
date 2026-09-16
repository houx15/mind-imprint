# CDN CORS: duplicate `Access-Control-Allow-Origin` on course assets

**Date:** 2026-08-21 · **Status:** ✅ FIXED — OSS bucket CORS rule deleted + CDN `/courses/` 目录刷新.
Verified from outside: the OSS origin preflight now answers `AccessForbidden / CORSResponse: CORS is
not enabled for this bucket` and its GET carries no `Access-Control-Allow-Origin`, while the CDN edge
still emits exactly one `Access-Control-Allow-Origin: *`. Kept as a runbook — the diagnosis technique
below generalises to any duplicated header on a CDN-fronted bucket.

## Symptom

Loading a 2.0 course with a video interaction fails in the browser:

```
Access to fetch at 'https://mind-oss.uni-robot.cn/courses/course-01/interactions/video/
rogelj-symmetry-video.json?auth_key=...' from origin 'https://mind-web.uni-robot.cn'
has been blocked by CORS policy: The 'Access-Control-Allow-Origin' header contains
multiple values 'https://mind-web.uni-robot.cn, *', but only one is allowed.
```

Only the **interaction JSON** trips this, because it is the one course asset read with
`fetch()` (`apps/web/src/course/interactionLoader.ts:41`). Video / images / PDFs load via
`<video>` / `<img>` / iframe, which are not CORS-checked, so the same duplicate header is
present on those responses but harmless.

## Root cause

**Two independent layers each emit `Access-Control-Allow-Origin`, and the CDN *appends*
rather than replaces.**

1. **OSS bucket `mind-imprint`** has a CORS rule for `https://mind-web.uni-robot.cn`.
   Proven — unsigned request straight to the OSS origin host:

   ```
   $ curl -sI -H 'Origin: https://mind-web.uni-robot.cn' \
       https://mind-imprint.oss-cn-beijing.aliyuncs.com/courses/course-01/interactions/video/rogelj-symmetry-video.json
   HTTP/1.1 403 Forbidden          # bucket-acl denial, but CORS headers still applied
   Access-Control-Allow-Origin: https://mind-web.uni-robot.cn
   Access-Control-Allow-Credentials: true
   Access-Control-Allow-Methods: GET, HEAD
   Access-Control-Max-Age: 600
   ```

2. **CDN domain `mind-oss.uni-robot.cn`** has a response-header rule adding
   `Access-Control-Allow-Origin: *`. Proven — the CDN's *own* URL-auth rejection, which
   never reaches the origin, still carries it:

   ```
   $ curl -sI -H 'Origin: https://mind-web.uni-robot.cn' \
       https://mind-oss.uni-robot.cn/courses/course-01/interactions/video/rogelj-symmetry-video.json
   HTTP/1.1 403 Forbidden
   Server: Tengine
   X-Tengine-Error: denied by req auth: no url arg auth_key   # edge-generated, no origin hop
   Access-Control-Allow-Origin: *
   ```

**Control** — the sibling CDN domain `mind-assets.uni-robot.cn` (site bucket) has no such
rule, and a response that *did* reach the origin carries no `Access-Control-Allow-Origin`
at all. So the `*` on `mind-oss` is deliberate per-domain config, not an Aliyun default:

```
$ curl -sI -H 'Origin: https://mind-web.uni-robot.cn' https://mind-assets.uni-robot.cn/home/course-player.png
HTTP/1.1 404 Not Found
x-oss-request-id: 6A8861E0BAB2A535315D799C     # reached origin
Timing-Allow-Origin: *                          # Aliyun default, present on both domains
(no Access-Control-Allow-Origin)
```

On a real signed 200 the CDN forwards the origin's header **and** appends its own →
`https://mind-web.uni-robot.cn, *` → the browser rejects it. Either value on its own
would work: `interactionLoader` uses a bare `fetch(url)`, so the request is not
credentialed and no preflight is involved.

## Fix — remove the header at the **origin** (OSS bucket), keep the CDN's `*`

Aliyun console → **对象存储 OSS → Bucket `mind-imprint` → 权限管理 → 跨域设置 (CORS)** →
delete the rule that allows `https://mind-web.uni-robot.cn`.

Why this side rather than the CDN side:

- **Cache-stable.** OSS only emits CORS headers when the request carries an `Origin`.
  If we instead kept the OSS rule and dropped the CDN's `*`, the CDN could cache an
  `Origin`-less copy of an object (`auth_key` is excluded from the cache key, so one
  cached copy serves everyone) with **no** `Access-Control-Allow-Origin` — and the next
  `fetch()` would break again, intermittently. The CDN's static `*` has no such
  dependency.
- **`*` is safe here.** Access control for these assets is the time-limited Type-A
  `auth_key`, not a cookie. This is *not* the API-with-cookie case that
  `docs/architecture/go-backend-best-practices.md` warns about — that rule still stands
  for `apps/api`.
- Removing the bucket rule also drops `Access-Control-Allow-Credentials: true`, which
  should never travel next to `*`.

**Blast radius:** the bucket CORS rule only matters for browser requests sent straight to
the OSS origin host — i.e. the presigned-GET fallback used when `OSS_CDN_AUTH_KEY` is
unset. Production has the key set (`de4eb414`), so every read is a CDN URL. Local course
preview resolves assets to local file/blob URLs and never touches OSS
(`docs/2026-08-17-course-authoring-api-handover.md` §6). Asset **uploads** are signed PUTs
run from scripts, not a browser — and the bucket rule allows only `GET, HEAD` anyway, so
it was never serving them.

### Alternative, if exact-origin CORS must be kept

Drop the CDN rule instead: 阿里云 CDN → 域名 `mind-oss.uni-robot.cn` → **缓存配置 → HTTP 头
(or 跨域配置)** → remove the `Access-Control-Allow-Origin: *` entry. Then also add
`Origin` to that domain's cache key, or the intermittent-missing-header failure above will
appear. Aliyun's own guidance for a duplicated header is to remove it at the origin, which
is why the OSS-side fix is preferred.

## Verify after the change

Ask the API for a signed URL (`POST /courses/{slug}/asset-urls`) and check that exactly
one value comes back:

```
$ curl -sI -H 'Origin: https://mind-web.uni-robot.cn' '<signed-cdn-url-with-auth_key>' \
    | grep -i access-control
Access-Control-Allow-Origin: *
```

Then hard-reload a course with a video interaction — the cue overlay should appear with no
CORS error in the console. If a stale copy is served, refresh the object on the CDN
(刷新预热 → URL 刷新) since the bad headers may be cached.

## Note on permissions

The deploy RAM sub-user in `.deploy-local/env.prod` cannot apply either fix — it is
implicitly denied `oss:GetBucketCors` (so almost certainly `PutBucketCors` too) and gets
`Forbidden.RAM` on every `cdn.aliyuncs.com` API. Grant those actions if this should be
automatable from a script rather than the console.
