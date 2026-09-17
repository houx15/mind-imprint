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
//  3. The payload is the report, plus ONLY what she herself asked to add.
//     No account, no ids that address anything else she owns, never the
//     article's full body. A public endpoint that leaks one extra field
//     leaks it to everyone, forever — resist any temptation to "just
//     include a bit more context" on the public route.
//
//     🚨 这一条在 2026-09-16 改过，原文是「the report and nothing else: no
//     account, **no transcript**, no article, no draft body」。改它的是产品
//     负责人的决定，不是实现方便：她要学生能公开自己的记录，并在被问到对话
//     记录时选了「她可以单独勾选公开」（而不是「跟报告一起公开」）。
//
//     所以现在多出来的只有一件事，而且它由她自己按：`include_transcript`
//     （迁移 0174）。它默认 false、撤销分享时归 false、只影响 transcript 这
//     一个键。**旧规矩剩下的部分一个字没松**：token 仍然不可猜、撤销仍然立刻
//     生效、正文全文仍然永远不在这里（报告上那一节只有不超过 200 字的摘录，
//     见 reportExcerptCap）。
//
//     留着这段历史是故意的：一段和代码相反的注释比没有注释更危险，而一段
//     记着「这里曾经是另一条规矩、是谁在哪一天改的」的注释，是下一个人判断
//     能不能再放宽一寸时唯一的依据。
//
// The share/revoke endpoints reuse ensureAtomReport (atom_report.go, Task
// 4) as the ONE generator — this file never re-derives a report.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
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

// publicTranscriptLine —— 公开出去的那份对话里的一条。
//
// 🚨 `Who` 永远在，永远不省略。这是公开页上唯一同时印着她的话和印记的话的
// 地方，不标就是把印记的话记在她名下。
type publicTranscriptLine struct {
	Who  string `json:"who"` // "student" | "coach"
	Text string `json:"text"`
}

// publicTranscriptOf 把存下来的消息摊成公开的那一份。system 那种记账消息不是
// 任何人说的话，空白消息也不是 —— 两者都不出现。
//
// 不截断：她自己写的字一个都不切（2026-09-12）。
func publicTranscriptOf(msgs []sqlc.AtomMessage) []publicTranscriptLine {
	sorted := append([]sqlc.AtomMessage(nil), msgs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Seq < sorted[j].Seq })
	out := make([]publicTranscriptLine, 0, len(sorted))
	for _, m := range sorted {
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		switch m.Role {
		case "student":
			out = append(out, publicTranscriptLine{Who: "student", Text: m.Content})
		case "ai":
			out = append(out, publicTranscriptLine{Who: "coach", Text: m.Content})
		}
	}
	return out
}

// publicReportBody 拼出公开负载的字节。
//
// transcript 为 nil 时**连键都不出现** —— 不是空数组。空数组在前端是「有这
// 件事，只是这次没有」，缺席才是「她没公开对话」。一条测试断在字节上守着
// 这个区别。
func publicReportBody(report []byte, transcript []publicTranscriptLine) ([]byte, error) {
	payload := map[string]any{"report": json.RawMessage(report)}
	if len(transcript) > 0 {
		payload["transcript"] = transcript
	}
	return json.Marshal(payload)
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

		// 对话是否一起公开。请求体可以整个没有（老客户端、以及「只是想要
		// 一条链接」那一次），那就是 false —— 默认什么都不多公开。
		//
		// 已经分享过的报告再 POST 一次，走的是同一条路：token 原样带回，
		// 只有这一位跟着她当时勾的状态更新。重新发一个 token 会悄悄弄坏
		// 她已经发出去的链接。
		var body struct {
			IncludeTranscript bool `json:"includeTranscript"`
			// IncludeToolkit：阅读报告上「段落工具」那一节（有她自己写的仿写）
			// 要不要一起公开。2026-09-17 起，和对话一样单独勾选、默认不公开。
			IncludeToolkit bool `json:"includeToolkit"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}

		token := ""
		if row.ShareToken != nil && *row.ShareToken != "" {
			token = *row.ShareToken
		} else {
			token, err = newShareToken()
			if err != nil {
				httpx.WriteError(w, r, err)
				return
			}
		}
		if _, err := a.d.Queries.SetAtomReportShare(ctx, sqlc.SetAtomReportShareParams{
			AtomID: at.ID, ShareToken: &token, IncludeTranscript: body.IncludeTranscript,
			IncludeToolkit: body.IncludeToolkit,
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}

		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"token":             token,
			"url":               publicShareURL(r, a.d.CORSOrigins, token),
			"includeTranscript": body.IncludeTranscript,
			"includeToolkit":    body.IncludeToolkit,
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
		// IncludeTranscript: false 在这里是多余的（那条语句在 share_token 为
		// NULL 时自己会把它写成 false，见 atom.sql），写出来是为了让「撤销
		// 把两件事一起收回」在调用点上也看得见。
		if _, err := a.d.Queries.SetAtomReportShare(r.Context(), sqlc.SetAtomReportShareParams{
			AtomID: at.ID, ShareToken: nil, IncludeTranscript: false, IncludeToolkit: false,
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
	// 她勾了才取，没勾连查都不查 —— 一条不该出现在负载里的数据，最好的状态
	// 是它根本没被读出来过。
	var transcript []publicTranscriptLine
	if row.IncludeTranscript {
		msgs, mErr := a.d.Queries.ListAtomMessages(r.Context(), row.AtomID)
		if mErr != nil {
			// 取不到对话不该让这条链接整个打不开：她公开的主要是那份报告。
			slog.Warn("public report: could not load the transcript she shared", "err", mErr, "atom_id", row.AtomID)
		} else {
			transcript = publicTranscriptOf(msgs)
		}
	}
	report := stripProseBookkeeping(a.reportWithPiece(r.Context(), row.AtomID, row.Report))
	// 段落工具那一节（有她自己写的仿写）只在她勾了的时候跟着公开。
	if !row.IncludeToolkit {
		report = stripReportKey(report, "toolkit")
	}
	body, err := publicReportBody(report, transcript)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// stripReportKey 从存下来的报告里拿掉一个顶层键，别的字节不动；没有这个键就
// 原样返回（不重新编码 —— 她分享出去的字节就是我们服务的字节）。
func stripReportKey(raw []byte, key string) []byte {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return raw
	}
	if _, ok := obj[key]; !ok {
		return raw
	}
	delete(obj, key)
	out, err := json.Marshal(obj)
	if err != nil {
		return raw
	}
	return out
}
