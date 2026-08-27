# Lite Edition: One Sign-In, Two Front Doors

**Date:** 2026-08-27
**Commits:** `cbc5fd3a` (auth + edition routing + settings), `bd02d31d` (committed nginx installer)

## What was missing

The lite edition shipped its reading room (P1) and writing room (P3) without a
front door. `LiteApp` rendered the reading tab the moment it mounted, and the
API client sent `credentials: "include"` on the assumption that a session
cookie already existed. Opening the app with no session produced a 401 on
every request and offered no way to sign in. That, not any missing feature, is
why lite had never been deployed.

## The shape

Auth is not a lite feature or a pro feature. It is one surface both editions
stand on: `/auth/signin`, `/auth/signup`, `/auth/signout` and `/auth/me` are
ungated on the server — only the edition-specific route groups are wrapped in
`requireEdition`.

So both apps mount the **same** `AuthScreen`. It already accepted its API
client as an injectable prop, so lite passes its own thin wrappers
(`apps/lite-web/src/api/auth.ts`) rather than dragging in pro's whole
project/studio API surface. There is one implementation of signing in, not
two.

### Landing on the wrong host

Which edition a student belongs to is an organisation fact — `schools.edition`
(migration 0093) — and the API already answers 404 for the other edition's
routes. The frontends now apply the same rule: `/auth/me` reports
`school.edition`, and each shell runs `resolveEditionDecision()` on boot.

A student in the wrong app gets a panel that names their edition and then
moves them. It is announced rather than silent because a page that replaces
itself with a different domain reads as a glitch, and the student never learns
which address is actually theirs.

The redirect is cheap because **the session is already shared**. The cookie is
host-only on `mind-api.uni-robot.cn` with `SameSite=Lax`, and every frontend
sits under the same registrable domain (`uni-robot.cn`), which makes them
*same-site* with the API. The student arrives at the other host already signed
in. No cookie attribute, session table, or auth flow changed anywhere in this
work.

### Three decisions that look like bugs

- **An unrecognised or absent edition means stay.** Being in the wrong app is
  recoverable — routes 404 and a teacher can help. Being ejected from the
  *right* app because a field arrived empty or from an API too old to send it
  leaves the student nowhere. Graceful degradation runs toward staying put.
- **Each app's own edition is a compile-time literal**, never read from the
  environment. An env var that failed to be set would make an app believe it
  is the other edition and exile every one of its own students, permanently.
- **The target URL carries `edition_redirect=1`.** If the host we send someone
  to sends them straight back — two deployments misconfigured to the same
  edition, or one DNS name pointing at the other's app — the flag turns an
  infinite browser bounce into a single honest dead end.

### Effect on pro

One behavioural change: pro's `AppShell` runs the same check. `schools.edition`
defaults to `'pro'`, so for every existing account the decision is "stay" and
nothing changes — asserted in both directions in
`apps/web/test/shell/AppShell.test.tsx`. Settings in lite is pro's real
`SettingsView`, mounted under the same accent/background providers, not a copy.

## Deployment topology

```
host nginx (TLS, certbot)
  ├── mind-api.uni-robot.cn   → 127.0.0.1:8090 → api container (Go)
  ├── mind-web.uni-robot.cn   → 127.0.0.1:8091 → web container (full edition)
  ├── mind.uni-robot.cn       → 127.0.0.1:8092 → marketing site (deployed separately)
  └── mind-lite.uni-robot.cn  → 127.0.0.1:8093 → lite-web container (lite edition)
```

- `.deploy-local/deploy.sh lite` ships the lite frontend alone.
- `full` now deploys **api, web and lite**: the two frontends are built from one
  source tree and share components, so shipping half of a change is how the
  editions drift apart.
- `CORS_ORIGINS` in the server's `deploy/.env.prod` now lists both frontends.

### `deploy/install-nginx-sites.sh`

Moved out of the operator's local `.deploy-local/` and into the repo, because
sudo on the ECS requires a password — which means a script piped over ssh
stdin cannot work (sudo wants that stdin itself), and the script has to be run
from the server's own checkout.

It also no longer installs "all sites". The configs in `deploy/nginx/` are
HTTP-only, and certbot adds each host's `:443` block by editing the *installed*
file. An install-everything run while adding a new host would have knocked
`mind-api` and `mind-web` back to plain HTTP. It now installs only the hosts
named on the command line and refuses to run with no arguments.

## TLS: resolved with an Aliyun certificate

Let's Encrypt could not issue for this host. Certbot failed four times, in two
forms — `During secondary validation: DNS problem: query timed out looking up
A`, and once `DNSSEC: DNSKEY Missing: key for validation cn. is marked as
invalid`.

**The fault was on LE's side, not in this domain.** A staging dry run for the
same host *succeeded*, which proves nginx, the A record, inbound reachability
and the HTTP-01 challenge are all correct here. Production differs by
validating from **several vantage points worldwide**; the non-China ones time
out querying `dns13`/`dns14.hichina.com`. `uni-robot.cn` is itself unsigned
(no DS, no DNSKEY), which is normal — the DNSSEC error variant is just LE
failing to fetch `cn.`'s keys in order to prove that.

Note for anyone reaching for the usual advice: **a DNS-01 challenge does not
help here.** It still requires LE to resolve a `_acme-challenge` TXT record
through the same failing path. DNS-01 solves inbound-reachability problems,
and this was not one.

The host therefore carries an **Aliyun-issued DV certificate** (DigiCert
`Encryption Everywhere DV TLS CA - G2`, valid **2026-08-27 → 2026-11-24**),
installed by `deploy/install-tls-cert.sh`. Verified from outside: HTTPS returns
200 with a fully-verifying chain, and HTTP 301-redirects to it.

### ⚠️ This certificate does not auto-renew

Roughly 90 days, and nothing renews it. Before **2026-11-24**:

1. Aliyun console → SSL 证书 → renew for `mind-lite.uni-robot.cn` → download,
   choosing **Nginx** as the server type (gives `<host>.pem` + `<host>.key`).
2. `scp` both to the server's home directory. They are secrets — never commit
   them; `.deploy-local/` and `docs/reference/` are both git-ignored.
3. `cd ~/mind-imprint && sudo bash deploy/install-tls-cert.sh mind-lite.uni-robot.cn`

The script refuses a key that does not match the certificate or one that does
not cover the host, warns on a leaf-only chain, installs the key `600`
root-owned, reloads nginx, and deletes the staging copies.

**The same exposure applies to the certbot-managed hosts.** The
`mind-api` + `mind-web` certificate renews in ~57 days through the same
multi-perspective validation that just failed repeatedly. If LE's reach into
`.cn` stays this unreliable, that renewal can fail too — which would take the
main product down, not just lite. Moving those hosts to Aliyun certificates on
a chosen schedule is safer than discovering it at expiry.
