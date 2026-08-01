package materialize

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
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

// cleanDOI trims wrapping punctuation a DOI never ends in.
func cleanDOI(s string) string {
	return strings.TrimRight(strings.TrimSpace(s), ".,;)")
}

// resolveDOI asks Crossref for a DOI's canonical landing URL and title.
// Best-effort: any failure returns ("","") so the caller just fetches the
// original URL — DOI resolution can only help, never regress.
func (f *HTTPFetcher) resolveDOI(ctx context.Context, doi string) (resolvedURL, title string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, crossrefBase+url.PathEscape(doi), nil)
	if err != nil {
		return "", ""
	}
	// Crossref asks bots to identify themselves; a contactable UA joins the
	// polite pool. No key required.
	req.Header.Set("User-Agent", "MindImprint/1.0 (material fetch; +https://mind-imprint.uni-robot.cn)")
	req.Header.Set("Accept", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", ""
	}
	var out struct {
		Message struct {
			Title    []string `json:"title"`
			URL      string   `json:"URL"`
			Resource struct {
				Primary struct {
					URL string `json:"URL"`
				} `json:"primary"`
			} `json:"resource"`
		} `json:"message"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return "", ""
	}
	if len(out.Message.Title) > 0 {
		title = strings.TrimSpace(out.Message.Title[0])
	}
	resolvedURL = out.Message.Resource.Primary.URL
	if resolvedURL == "" {
		resolvedURL = out.Message.URL
	}
	return resolvedURL, title
}
