package materialize

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchWorks_ParsesResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "search=") {
			t.Fatalf("missing search param: %s", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{"results":[{"doi":"https://doi.org/10.1000/x","title":"Carbon in China",
          "publication_year":2023,"primary_location":{"source":{"display_name":"Nature Sustainability"},"landing_page_url":"https://n.example/x"},
          "authorships":[{"author":{"display_name":"Li Wei"}},{"author":{"display_name":"Zhang San"}}],
          "abstract_inverted_index":{"Carbon":[0],"rising":[1]}}]}`)
	}))
	defer srv.Close()
	old := openAlexBase
	openAlexBase = srv.URL + "/works"
	defer func() { openAlexBase = old }()
	f := newUnguardedFetcher()
	got := f.SearchWorks(context.Background(), "china carbon", 5)
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	if got[0].Title != "Carbon in China" || got[0].Year != "2023" || got[0].Journal != "Nature Sustainability" {
		t.Fatalf("bad parse: %+v", got[0])
	}
	if got[0].Authors != "Li Wei, Zhang San" {
		t.Fatalf("authors: %q", got[0].Authors)
	}
	if !strings.HasPrefix(got[0].Abstract, "Carbon rising") {
		t.Fatalf("abstract reconstruct: %q", got[0].Abstract)
	}
	if got[0].DOI != "10.1000/x" {
		t.Fatalf("doi strip: %q", got[0].DOI)
	}
}

func TestSearchWorks_BadStatusReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	old := openAlexBase
	openAlexBase = srv.URL + "/works"
	defer func() { openAlexBase = old }()
	f := newUnguardedFetcher()
	got := f.SearchWorks(context.Background(), "china carbon", 5)
	if got != nil {
		t.Fatalf("want nil, got %+v", got)
	}
}

// RelatedWorks is a two-step fetch: first resolve the DOI to a work id (a
// single work OBJECT — not the `results` envelope), then filter by
// related_to:<id> (which IS a `results` envelope). The test server branches
// on the query string to serve the right shape for each step.
func TestRelatedWorks_TwoStepFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "filter=related_to") {
			_, _ = io.WriteString(w, `{"results":[{"doi":"https://doi.org/10.2000/y","title":"Related Paper",
              "publication_year":2020,"primary_location":{"source":{"display_name":"Journal Y"},"landing_page_url":"https://y.example"},
              "authorships":[{"author":{"display_name":"Wang Fang"}}],
              "abstract_inverted_index":{"Related":[0],"work":[1]}}]}`)
			return
		}
		// DOI lookup step: a single work object, keyed by "id" — not a
		// `results` envelope.
		_, _ = io.WriteString(w, `{"id":"https://openalex.org/W123"}`)
	}))
	defer srv.Close()
	old := openAlexBase
	openAlexBase = srv.URL + "/works"
	defer func() { openAlexBase = old }()
	f := newUnguardedFetcher()
	got := f.RelatedWorks(context.Background(), "10.1/x", 5)
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	if got[0].Title != "Related Paper" || got[0].Year != "2020" || got[0].Journal != "Journal Y" {
		t.Fatalf("bad parse: %+v", got[0])
	}
	if got[0].Authors != "Wang Fang" {
		t.Fatalf("authors: %q", got[0].Authors)
	}
	if got[0].DOI != "10.2000/y" {
		t.Fatalf("doi strip: %q", got[0].DOI)
	}
}

// If the first step (DOI -> work id lookup) fails, RelatedWorks must not
// attempt the second call — best-effort contract holds end to end.
func TestRelatedWorks_DOILookupFailReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	old := openAlexBase
	openAlexBase = srv.URL + "/works"
	defer func() { openAlexBase = old }()
	f := newUnguardedFetcher()
	got := f.RelatedWorks(context.Background(), "10.1/x", 5)
	if got != nil {
		t.Fatalf("want nil, got %+v", got)
	}
}
