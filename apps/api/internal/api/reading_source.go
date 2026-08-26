package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// reading_source.go — the article the student is reading. One reading, one
// article: PUT replaces it wholesale (a student who pastes twice meant the
// second one).

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
	var req struct {
		Title string `json:"title"`
		Text  string `json:"text"`
		URL   string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	title := strings.TrimSpace(req.Title)
	body := strings.TrimSpace(req.Text)
	srcURL := strings.TrimSpace(req.URL)

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
