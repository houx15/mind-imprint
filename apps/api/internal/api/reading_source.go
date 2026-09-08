package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// reading_source.go — the article the student is reading. One reading, one
// article: PUT replaces it wholesale (a student who pastes twice meant the
// second one) — but ONLY while nothing is anchored into it yet. See
// refuseIfAnchored.

// refuseIfAnchored blocks a source replacement once ANY process evidence hangs
// off this reading: a card, a margin note, or a single line of transcript.
//
// Block ids are POSITIONAL ("b1" is simply the first paragraph) and anchors
// carry rune offsets into the body they were made against. Swapping the
// article underneath therefore does not orphan those rows — which would at
// least be visible — it silently RE-POINTS every one of them at whatever
// prose now happens to occupy those coordinates. A CRAAP card would come back
// hanging off a sentence the student never read, and 铁律④ says that row is
// evidence a report gets generated from.
//
// Not reachable from today's UI (the room offers the paste box only when the
// reading has no article), so this costs a student nothing; it exists because
// the endpoint is reachable without the UI, and because "not reachable today"
// is not a property that survives a redesign.
//
// The empty case is deliberately permissive: pasting the wrong thing and
// immediately re-pasting is a normal correction, and nothing points at the old
// text yet.
func (a *API) refuseIfAnchored(w http.ResponseWriter, r *http.Request, atomID uuid.UUID) bool {
	n, err := a.d.Queries.CountAtomEvidence(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return true
	}
	if n > 0 {
		httpx.WriteError(w, r, httpx.ErrSourceLocked())
		return true
	}
	return false
}

type sourceDTO struct {
	Title     string  `json:"title"`
	SourceURL string  `json:"sourceUrl"`
	Blocks    []Block `json:"blocks"`
}

func (a *API) putReadingSourceLite(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	// Before anything else — refusing costs nothing, and the URL branch below
	// would otherwise burn a server-side fetch on a replacement we will reject.
	if a.refuseIfAnchored(w, r, at.ID) {
		return
	}
	var req struct {
		Title string `json:"title"`
		Text  string `json:"text"`
		URL   string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	title := strings.TrimSpace(req.Title)
	body := strings.TrimSpace(req.Text)
	srcURL := strings.TrimSpace(req.URL)

	// 🚨 已经有正文了就别再抓一次。
	//
	// 从地图进来的那一篇，正文在建的时候就已经放好了（feed 自带，见
	// mintReadingForPlanet）。而前端在「现在读」之后照样会带着 url 调一次这里 ——
	// 那一次会去抓原页面，抓到就把好好的正文覆盖成另一份，抓不到（走查里就是
	// 403/400）就直接报错，把一篇本来能读的文章变成一条红字。
	//
	// 只挡「带 url、不带正文」这一种。她**粘**一份新的进来（body 非空）照旧
	// 覆盖 —— 那是她明确要换掉这一篇，和这条无关。
	if body == "" && srcURL != "" {
		if existing, err := a.d.Queries.GetReadingSource(r.Context(), at.ID); err == nil &&
			len(SplitBlocks(existing.Body)) > 0 {
			httpx.WriteJSON(w, http.StatusOK, sourceDTO{
				Title: existing.Title, SourceURL: derefOr(existing.SourceUrl, ""),
				Blocks: SplitBlocks(existing.Body),
			})
			return
		}
	}

	// A URL is fetched server-side through the same guarded fetcher the pro
	// side uses; a pasted body is taken as-is.
	if body == "" && srcURL != "" {
		if a.d.Fetcher == nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("fetch_unavailable", "暂时无法抓取链接，请直接粘贴正文。", nil))
			return
		}
		fetchedTitle, text, _, err := a.d.Fetcher.FetchReadable(r.Context(), srcURL)
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("fetch_failed", "这个链接抓不到正文，请直接粘贴。", nil))
			return
		}
		body = strings.TrimSpace(text)
		if title == "" {
			title = fetchedTitle
		}
	}

	if len(SplitBlocks(body)) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_text", "先把文章正文放进来。", nil))
		return
	}
	if title == "" {
		title = "未命名文章"
	}

	row, err := a.d.Queries.UpsertReadingSource(r.Context(), sqlc.UpsertReadingSourceParams{
		AtomID: at.ID, Title: title, Body: body, SourceUrl: nullableText(srcURL),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sourceDTO{
		Title: row.Title, SourceURL: srcURL, Blocks: SplitBlocks(row.Body),
	})
}

func (a *API) getReadingSourceLite(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetReadingSource(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err) // pgx.ErrNoRows → 404: nothing pasted yet
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sourceDTO{
		Title: row.Title, SourceURL: derefOr(row.SourceUrl, ""), Blocks: SplitBlocks(row.Body),
	})
}

// nullableText returns a *string for s: nil for an empty/blank string (so the
// store column persists NULL), the trimmed value's address otherwise.
func nullableText(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
