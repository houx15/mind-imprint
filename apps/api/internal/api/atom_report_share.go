package api

// atom_report_share.go — Task 5: a student can choose to make her finished
// report publicly viewable, and the product asked for a QR code someone can
// scan. This file is the mint / revoke / public-read side of that.
//
// This is the one file in the lite edition that publishes a child's work to
// the open internet, so the binding ruling (R1, design spec) governs every
// choice here: anyone with the link, no login, no expiry, revocable at any
// time, and her name may appear. She is a minor — the compensating controls
// below are the ONLY protection there is, and each is load-bearing:
//
//  1. The token is unguessable: 16 random bytes from crypto/rand as 32 hex
//     chars, NEVER the atom id (an id is guessable from any other link she
//     has ever shared, and it addresses a row she owns).
//  2. Revocation is immediate and total: setting share_token back to NULL
//     (SetAtomReportShare, Task 1) means the public route finds nothing on
//     the very next request — no cache, no grace period.
//  3. The payload is the report and nothing else: no account, no
//     transcript, no article, no draft body, no ids that address anything
//     else she owns. A public endpoint that leaks one extra field leaks it
//     to everyone, forever — resist any temptation to "just include a bit
//     more context" on the public route.
//
// The share/revoke endpoints reuse ensureAtomReport (atom_report.go, Task
// 4) as the ONE generator — this file never re-derives a report.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// newShareToken mints 16 random bytes as 32 hex chars. NOT the atom id: an
// id is guessable from any other link she has ever shared, and it addresses
// a row she owns. This token addresses one report and nothing else.
func newShareToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// publicShareURL builds the absolute link to the student-facing share page
// — lite-web's own client-side route `/s/:token` (design spec), NOT this
// API's own host, which is a different origin in production
// (mind-lite.uni-robot.cn vs. the API host). Prefers the request's own
// Origin header, since that IS the frontend origin that just issued this
// POST; falls back to the first configured CORS origin (Deps.CORSOrigins —
// the platform's own allowlist of SPA origins), and finally to the
// request's own scheme+host so a dev server with neither still gets a
// usable link rather than an empty one.
func publicShareURL(r *http.Request, corsOrigins []string, token string) string {
	origin := r.Header.Get("Origin")
	if origin == "" && len(corsOrigins) > 0 {
		origin = corsOrigins[0]
	}
	if origin == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		origin = scheme + "://" + r.Host
	}
	return strings.TrimRight(origin, "/") + "/s/" + token
}

// shareAtomReportFor is the shared body behind POST /api/v1/readings/{id}/report/share
// and its writing twin. loadOwnedAtomRow, NOT loadOwnedAtom (atom_heartbeat.go's
// precedent): sharing only ever makes sense on a FINISHED atom, and
// loadOwnedAtom's non-GET gate would 403 exactly that case before this
// handler could even look at it.
//
// Re-sharing an already-shared report returns the EXISTING token, never a
// freshly minted one — minting a second would silently break a link she has
// already sent someone.
func (a *API) shareAtomReportFor(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		at, ok := a.loadOwnedAtomRow(w, r, kind)
		if !ok {
			return
		}
		u, _ := UserFromContext(r.Context())
		entitled, err := HasEntitlement(r.Context(), u)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if !entitled {
			httpx.WriteError(w, r, httpx.ErrNotEntitled())
			return
		}

		// Run to completion even if she navigates away mid-call — same
		// posture as getAtomReportFor, since this may generate the report
		// (one model call) on its way to minting a share token.
		ctx, cancel := detachedModelCtx(r)
		defer cancel()

		row, found, err := a.ensureAtomReport(ctx, u.ID, at.ID, kind)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if !found {
			httpx.WriteError(w, r, httpx.ErrConflict("这次还没有完成，暂时无法分享。"))
			return
		}

		token := ""
		if row.ShareToken != nil && *row.ShareToken != "" {
			// Already shared — hand back the SAME link rather than minting
			// a second one (see file/func comment).
			token = *row.ShareToken
		} else {
			token, err = newShareToken()
			if err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			if _, err := a.d.Queries.SetAtomReportShare(ctx, sqlc.SetAtomReportShareParams{
				AtomID: at.ID, ShareToken: &token,
			}); err != nil {
				httpx.WriteError(w, r, err)
				return
			}
		}

		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"token": token,
			"url":   publicShareURL(r, a.d.CORSOrigins, token),
		})
	}
}

// revokeAtomShareFor is the shared body behind DELETE /api/v1/readings/{id}/report/share
// and its writing twin. Idempotent: revoking a report that was never shared
// — or one whose report row does not exist yet at all — is still a 204, not
// an error. SetAtomReportShare is an UPDATE ... WHERE atom_id = $2 :one, so
// when no atom_report row exists yet the RETURNING clause matches nothing
// and pgx surfaces that as ErrNoRows; that is exactly the "nothing to
// revoke" case, not a failure.
func (a *API) revokeAtomShareFor(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		at, ok := a.loadOwnedAtomRow(w, r, kind)
		if !ok {
			return
		}
		if _, err := a.d.Queries.SetAtomReportShare(r.Context(), sqlc.SetAtomReportShareParams{
			AtomID: at.ID, ShareToken: nil,
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *API) shareReadingReport() http.HandlerFunc  { return a.shareAtomReportFor("reading") }
func (a *API) revokeReadingReport() http.HandlerFunc { return a.revokeAtomShareFor("reading") }
func (a *API) shareWritingReport() http.HandlerFunc  { return a.shareAtomReportFor("writing") }
func (a *API) revokeWritingReport() http.HandlerFunc { return a.revokeAtomShareFor("writing") }

// getPublicReport is GET /api/v1/public/reports/{token} — registered WITHOUT
// protected/liteOnly (see api.go): this is the one route in the lite
// edition that is deliberately reachable by anyone, with no session at all.
//
// An unknown token and a revoked token must be indistinguishable: both fall
// through to httpx.WriteError's pgx.ErrNoRows -> plain 404 mapping, with no
// detail that would tell a stranger a report ever existed at this token.
//
// The response is `{"report": <the stored jsonb>}` and NOTHING else — no
// account, no transcript, no article, no draft body, no ids that address
// anything else she owns. Do not widen this payload; see the file comment.
func (a *API) getPublicReport(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	row, err := a.d.Queries.GetAtomReportByShareToken(r.Context(), &token)
	if err != nil {
		httpx.WriteError(w, r, err) // pgx.ErrNoRows -> plain 404, no detail
		return
	}
	// The share link is the case this MATTERS for: without `piece` the viewer
	// gets a page of statistics and no way to the article at all, which is the
	// one thing they opened the link to read. See reportWithPiece — it fills
	// the field in from her draft for any report stored before it existed, and
	// leaves everything else in the blob untouched, so the payload's key set
	// (pinned by TestPublicPayloadCarriesNothingExtra) is unchanged.
	// The two-phase generator's bookkeeping (prosePending / proseClaimedAt)
	// is stripped here, not merely left unrendered. A visitor cannot act on
	// either one: the polling that clears `prosePending` is the OWNER's
	// report page re-asking an authenticated endpoint, and a public page that
	// saw the flag could only either poll an endpoint that never changes for
	// it, or render a "still working" state that never resolves. Keeping the
	// public key set exactly what it was before the split is also what
	// TestPublicPayloadCarriesNothingExtra is for.
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"report": json.RawMessage(stripProseBookkeeping(a.reportWithPiece(r.Context(), row.AtomID, row.Report))),
	})
}
