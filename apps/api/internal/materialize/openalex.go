package materialize

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// #A2 · OpenAlex search + related-works. Exploration lets a student widen a
// thin lead (search a topic) or find neighbors of a paper already on the map
// (related works) without leaving the app. Free API, no key. Best-effort:
// every failure degrades to nil/empty so a flaky network never blocks the
// student's own thinking.

// openAlexBase is the works endpoint; a package var so tests can point it at
// an httptest server.
var openAlexBase = "https://api.openalex.org/works"

// WorkMeta is the bibliographic summary surfaced for an OpenAlex work — enough
// to let a student judge relevance before adopting it into the map.
type WorkMeta struct {
	DOI      string
	Title    string
	Authors  string // "A, B, C" joined
	Year     string // string to match Reference.year
	Journal  string
	Abstract string
	URL      string
}

// openAlexWork is the subset of an OpenAlex work object we need.
type openAlexWork struct {
	ID              string `json:"id"`
	Doi             string `json:"doi"`
	Title           string `json:"title"`
	PublicationYear int    `json:"publication_year"`
	PrimaryLocation struct {
		Source struct {
			DisplayName string `json:"display_name"`
		} `json:"source"`
		LandingPageURL string `json:"landing_page_url"`
	} `json:"primary_location"`
	Authorships []struct {
		Author struct {
			DisplayName string `json:"display_name"`
		} `json:"author"`
	} `json:"authorships"`
	AbstractInvertedIndex map[string][]int `json:"abstract_inverted_index"`
	// ReferencedWorks is the list of OpenAlex work ids (full URL form) THIS
	// work cites — only present on the single-work DOI lookup, used by
	// ReferencedWorks to fan out into a batched openalex_id filter query.
	ReferencedWorks []string `json:"referenced_works"`
}

func toWorkMeta(w openAlexWork) WorkMeta {
	m := WorkMeta{
		Title:   strings.TrimSpace(w.Title),
		Journal: strings.TrimSpace(w.PrimaryLocation.Source.DisplayName),
		URL:     w.PrimaryLocation.LandingPageURL,
		DOI:     strings.TrimPrefix(w.Doi, "https://doi.org/"),
	}
	if w.PublicationYear != 0 {
		m.Year = strconv.Itoa(w.PublicationYear)
	}
	names := make([]string, 0, len(w.Authorships))
	for _, a := range w.Authorships {
		if n := strings.TrimSpace(a.Author.DisplayName); n != "" {
			names = append(names, n)
		}
	}
	m.Authors = strings.Join(names, ", ")
	m.Abstract = reconstructAbstract(w.AbstractInvertedIndex)
	return m
}

// reconstructAbstract rebuilds plain text from OpenAlex's inverted index
// (word -> positions), placing each occurrence at its index and joining with
// spaces.
func reconstructAbstract(inv map[string][]int) string {
	if len(inv) == 0 {
		return ""
	}
	maxPos := 0
	for _, positions := range inv {
		for _, p := range positions {
			if p > maxPos {
				maxPos = p
			}
		}
	}
	words := make([]string, maxPos+1)
	for word, positions := range inv {
		for _, p := range positions {
			words[p] = word
		}
	}
	out := make([]string, 0, len(words))
	for _, w := range words {
		if w != "" {
			out = append(out, w)
		}
	}
	return strings.Join(out, " ")
}

// openAlexGet performs a GET against rawURL, decoding the response body into
// out. Best-effort: any failure returns false and leaves out untouched.
func (f *HTTPFetcher) openAlexGet(ctx context.Context, rawURL string, out any) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "MindImprint/1.0 (+https://mind-imprint.uni-robot.cn)")
	req.Header.Set("Accept", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out); err != nil {
		return false
	}
	return true
}

// fetchWorks issues a GET against base with the given query string and
// decodes the `{"results":[...]}` envelope. Best-effort: any failure returns
// nil.
func (f *HTTPFetcher) fetchWorks(ctx context.Context, base, rawQuery string) []openAlexWork {
	var out struct {
		Results []openAlexWork `json:"results"`
	}
	if !f.openAlexGet(ctx, base+"?"+rawQuery, &out) {
		return nil
	}
	return out.Results
}

// SearchWorks searches OpenAlex for works matching query, returning up to
// limit results. Best-effort: returns nil on any failure.
func (f *HTTPFetcher) SearchWorks(ctx context.Context, query string, limit int) []WorkMeta {
	q := url.Values{}
	q.Set("search", query)
	q.Set("per-page", strconv.Itoa(limit))
	q.Set("mailto", "hi@mind-imprint.uni-robot.cn")
	works := f.fetchWorks(ctx, openAlexBase, q.Encode())
	if works == nil {
		return nil
	}
	metas := make([]WorkMeta, 0, len(works))
	for _, w := range works {
		metas = append(metas, toWorkMeta(w))
	}
	return metas
}

// RelatedWorks resolves doi to its OpenAlex work id, then returns up to limit
// works OpenAlex considers related. Best-effort: returns nil on any failure
// (including the initial doi resolution).
func (f *HTTPFetcher) RelatedWorks(ctx context.Context, doi string, limit int) []WorkMeta {
	id := f.resolveWorkID(ctx, doi)
	if id == "" {
		return nil
	}
	return f.filterWorksMeta(ctx, "related_to:"+id, limit)
}

// ReferencedWorks resolves doi to its OpenAlex work object, then fetches the
// metadata of up to limit works THIS paper cites — its own referenced_works
// id list, batched into one openalex_id:<W1>|<W2>|... filter query (pipe =
// OR). Best-effort: returns nil on any failure, including doi resolution or
// an empty referenced_works list.
func (f *HTTPFetcher) ReferencedWorks(ctx context.Context, doi string, limit int) []WorkMeta {
	w, ok := f.resolveWork(ctx, doi)
	if !ok || len(w.ReferencedWorks) == 0 {
		return nil
	}
	ids := w.ReferencedWorks
	if len(ids) > limit {
		ids = ids[:limit]
	}
	bare := make([]string, 0, len(ids))
	for _, id := range ids {
		bare = append(bare, stripWorkID(id))
	}
	return f.filterWorksMeta(ctx, "openalex_id:"+strings.Join(bare, "|"), limit)
}

// CitingWorks resolves doi to its OpenAlex work id, then returns up to limit
// works that CITE this paper (cites:<workid>). Best-effort: returns nil on
// any failure, including the initial doi resolution.
func (f *HTTPFetcher) CitingWorks(ctx context.Context, doi string, limit int) []WorkMeta {
	id := f.resolveWorkID(ctx, doi)
	if id == "" {
		return nil
	}
	return f.filterWorksMeta(ctx, "cites:"+stripWorkID(id), limit)
}

// filterWorksMeta issues a filter query against openAlexBase (the shared
// second step of every OpenAlex lookup that starts from a resolved work id —
// RelatedWorks/ReferencedWorks/CitingWorks) and maps the results envelope to
// WorkMeta. Best-effort: returns nil on any failure.
func (f *HTTPFetcher) filterWorksMeta(ctx context.Context, filter string, limit int) []WorkMeta {
	q := url.Values{}
	q.Set("filter", filter)
	q.Set("per-page", strconv.Itoa(limit))
	q.Set("mailto", "hi@mind-imprint.uni-robot.cn")
	works := f.fetchWorks(ctx, openAlexBase, q.Encode())
	if works == nil {
		return nil
	}
	metas := make([]WorkMeta, 0, len(works))
	for _, w := range works {
		metas = append(metas, toWorkMeta(w))
	}
	return metas
}

// stripWorkID trims OpenAlex's full URL id form ("https://openalex.org/W123")
// down to the bare id ("W123") filter queries expect.
func stripWorkID(id string) string {
	return strings.TrimPrefix(id, "https://openalex.org/")
}

// resolveWork looks up a single OpenAlex work by DOI and returns the full
// work object (id, referenced_works, ...), or a zero value + false on any
// failure.
func (f *HTTPFetcher) resolveWork(ctx context.Context, doi string) (openAlexWork, bool) {
	var out openAlexWork
	rawURL := openAlexBase + "/doi:" + url.PathEscape(doi) + "?mailto=hi@mind-imprint.uni-robot.cn"
	if !f.openAlexGet(ctx, rawURL, &out) {
		return openAlexWork{}, false
	}
	return out, true
}

// resolveWorkID looks up a single OpenAlex work by DOI and returns its id
// (e.g. "https://openalex.org/W123"), or "" on any failure.
func (f *HTTPFetcher) resolveWorkID(ctx context.Context, doi string) string {
	w, ok := f.resolveWork(ctx, doi)
	if !ok {
		return ""
	}
	return w.ID
}
