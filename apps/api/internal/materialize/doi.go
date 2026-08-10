package materialize

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// #4 · DOI resolution. Academic links are the biggest source of "取不到正文":
// a bare https://doi.org/... redirects to a publisher page that is usually a
// PDF or a JS/paywall wall. Crossref (free, no key) resolves a DOI to the
// publisher's canonical landing URL + the article title, so we can fetch the
// landing page directly (more often real HTML) and always recover the title.

// crossrefBase is the works endpoint; a package var so tests can point it at an
// httptest server.
var crossrefBase = "https://api.crossref.org/works/"

// doiPattern matches a DOI (10.<registrant>/<suffix>) — permissive on the
// suffix, stopping at whitespace/quote/angle chars.
var doiPattern = regexp.MustCompile(`10\.\d{4,9}/[^\s"'<>]+`)

// extractDOI pulls a DOI out of a doi.org URL (path is the DOI) or any URL whose
// path/query contains one. Returns ("", false) when there's no DOI to resolve.
func extractDOI(u *url.URL) (string, bool) {
	host := strings.ToLower(u.Host)
	if host == "doi.org" || host == "dx.doi.org" || strings.HasSuffix(host, ".doi.org") {
		if doi := cleanDOI(strings.TrimPrefix(u.Path, "/")); doi != "" {
			return doi, true
		}
	}
	if m := doiPattern.FindString(u.Path + " " + u.RawQuery); m != "" {
		return cleanDOI(m), true
	}
	return "", false
}

// ExtractDOI is extractDOI, exported for callers outside this package —
// dig's citation/cited modes (api.digExploration) need a paper's DOI (pulled
// off its reference.Url) before they can call ReferencedWorks/CitingWorks.
func ExtractDOI(u *url.URL) (string, bool) {
	return extractDOI(u)
}

// cleanDOI trims wrapping punctuation a DOI never ends in.
func cleanDOI(s string) string {
	return strings.TrimRight(strings.TrimSpace(s), ".,;)")
}

// DetectDOI pulls a DOI out of a raw student-typed string — a bare DOI
// ("10.1126/science.aap9559"), a doi.org URL, or any URL/text containing one.
// Returns ("", false) when there's nothing DOI-shaped. Exported so the
// add-source path (createReference) can resolve metadata when a student pastes
// a DOI into the 链接/DOI field.
func DetectDOI(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if u, err := url.Parse(raw); err == nil {
		if doi, ok := extractDOI(u); ok {
			return doi, true
		}
	}
	if m := doiPattern.FindString(raw); m != "" {
		return cleanDOI(m), true
	}
	return "", false
}

// ResolveDOI is resolveDOI, exported so callers outside this package can turn a
// DOI into bibliographic metadata (title/author/year/journal/abstract) without
// fetching full text. Best-effort: nil on any failure.
func (f *HTTPFetcher) ResolveDOI(ctx context.Context, doi string) *DOIMeta {
	return f.resolveDOI(ctx, doi)
}

// DOIMeta is the bibliographic metadata Crossref returns for a DOI (#4). Any
// field may be empty. Surfaced to the student even when full text can't be
// fetched: fill the annotated bib from Author/Year/Journal, read the Abstract,
// then paste the full text.
type DOIMeta struct {
	URL      string // publisher landing (may be "")
	Title    string
	Author   string // "Given Family; Given Family; …"
	Year     string
	Journal  string
	Abstract string // plain text (JATS markup stripped)
}

// jatsTag strips JATS/XML tags from a Crossref abstract.
var jatsTag = regexp.MustCompile(`<[^>]+>`)

// resolveDOI asks Crossref for a DOI's landing URL + bibliographic metadata.
// Best-effort: any failure returns nil so the caller just fetches the original
// URL — DOI resolution can only help, never regress.
func (f *HTTPFetcher) resolveDOI(ctx context.Context, doi string) *DOIMeta {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, crossrefBase+url.PathEscape(doi), nil)
	if err != nil {
		return nil
	}
	// Crossref asks bots to identify themselves; a contactable UA joins the
	// polite pool. No key required.
	req.Header.Set("User-Agent", "MindImprint/1.0 (material fetch; +https://mind-imprint.uni-robot.cn)")
	req.Header.Set("Accept", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var out struct {
		Message struct {
			Title          []string `json:"title"`
			ContainerTitle []string `json:"container-title"`
			Abstract       string   `json:"abstract"`
			URL            string   `json:"URL"`
			Author         []struct {
				Given  string `json:"given"`
				Family string `json:"family"`
			} `json:"author"`
			Issued struct {
				DateParts [][]int `json:"date-parts"`
			} `json:"issued"`
			Resource struct {
				Primary struct {
					URL string `json:"URL"`
				} `json:"primary"`
			} `json:"resource"`
		} `json:"message"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil
	}
	m := &DOIMeta{}
	if len(out.Message.Title) > 0 {
		m.Title = strings.TrimSpace(out.Message.Title[0])
	}
	if len(out.Message.ContainerTitle) > 0 {
		m.Journal = strings.TrimSpace(out.Message.ContainerTitle[0])
	}
	if a := out.Message.Abstract; a != "" {
		m.Abstract = strings.TrimSpace(jatsTag.ReplaceAllString(a, ""))
	}
	names := make([]string, 0, len(out.Message.Author))
	for _, a := range out.Message.Author {
		n := strings.TrimSpace(a.Given + " " + a.Family)
		if n != "" {
			names = append(names, n)
		}
	}
	m.Author = strings.Join(names, "; ")
	if len(out.Message.Issued.DateParts) > 0 && len(out.Message.Issued.DateParts[0]) > 0 {
		if y := out.Message.Issued.DateParts[0][0]; y > 0 {
			m.Year = strconv.Itoa(y)
		}
	}
	m.URL = out.Message.Resource.Primary.URL
	if m.URL == "" {
		m.URL = out.Message.URL
	}
	return m
}
