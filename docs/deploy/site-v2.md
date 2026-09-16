# Active marketing deployment: V2

Published 2026-09-11 to https://mind.uni-robot.cn/ (English) and `/zh/` (Chinese).

The active container is `mindimprint-site-site-1`, compose project
`mindimprint-site`, bound to `127.0.0.1:8092`. Host nginx and TLS are unchanged.
The initial image is `mindimprint-site:v2-20260911T101000`. The former site's
image was deleted after verification, at the owner's request. Old source stays
in `apps/site`; the active source is `apps/site-v2`.

## Publish subsequent changes

From the repository root, run `bash deploy/deploy-site-v2-artifact.sh`.
This checks and builds the current V2 working tree locally, uploads the static
artifact and builds a small nginx runtime on the existing host. It uses the
existing ignored `.deploy-local/id_deploy` credential. It does not push git or
reset a server checkout. Commit/source-control management remains separate.

Upload any new images first with `deploy/upload-site-assets.sh FILE KEY`.
The four initial `v2/*.webp` images have been uploaded and verified through
`https://mind-assets.uni-robot.cn`. The production build uses that CDN base.

Server release directories are `~/mind-imprint-site-releases/<release-id>`;
`current` points to the active release. Its `compose.yml` controls the site.
Do not use `.deploy-local/deploy-site.sh` for V2: that legacy script rebuilds
`apps/site` and would restore the previous website.

The deployment script starts a temporary candidate on a random loopback port,
checks routes and nginx configuration, then replaces only the site service.
A temporary previous-image tag enables automatic recovery if startup checks
fail. After successful checks it is removed; no old image is retained.
There is no global Docker prune, database migration or product-service restart.

Initial verification: candidate and active container returned 200 for home,
Chinese, about, reading, exploration, courses, research and schools; legacy
English, algorithm, teacher and editions routes returned 301; unknown URL 404.
Public HTTPS home was fetched and opened in a real browser with V2 content.
