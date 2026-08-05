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

// ReferencedWorks is a two-step fetch: first resolve the DOI to the work
// OBJECT (to read its referenced_works id list, not just its own id), then
// batch-fetch those ids in one openalex_id:<W1>|<W2>|... filter query. The
// test server branches on the query string to serve the right shape for each
// step; the ids on the second request must be stripped to their bare "W_"
// form (not the full "https://openalex.org/W_" URL).
func TestReferencedWorks_TwoStep(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "filter=openalex_id") {
			if !strings.Contains(r.URL.RawQuery, "W1") || !strings.Contains(r.URL.RawQuery, "W2") {
				t.Fatalf("openalex_id filter missing bare ids: %s", r.URL.RawQuery)
			}
			if strings.Contains(r.URL.RawQuery, "openalex.org") {
				t.Fatalf("openalex_id filter must use bare ids, not full URLs: %s", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `{"results":[
              {"doi":"https://doi.org/10.3000/a","title":"Ref A","publication_year":2018,
               "primary_location":{"source":{"display_name":"Journal A"},"landing_page_url":"https://a.example"},
               "authorships":[{"author":{"display_name":"A A"}}],
               "abstract_inverted_index":{"About":[0],"A":[1]}},
              {"doi":"https://doi.org/10.3000/b","title":"Ref B","publication_year":2019,
               "primary_location":{"source":{"display_name":"Journal B"},"landing_page_url":"https://b.example"},
               "authorships":[{"author":{"display_name":"B B"}}],
               "abstract_inverted_index":{"About":[0],"B":[1]}}
            ]}`)
			return
		}
		// DOI lookup step: a single work object carrying referenced_works —
		// not a `results` envelope.
		_, _ = io.WriteString(w, `{"id":"https://openalex.org/W99","referenced_works":["https://openalex.org/W1","https://openalex.org/W2"]}`)
	}))
	defer srv.Close()
	old := openAlexBase
	openAlexBase = srv.URL + "/works"
	defer func() { openAlexBase = old }()
	f := newUnguardedFetcher()
	got := f.ReferencedWorks(context.Background(), "10.1/x", 5)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d: %+v", len(got), got)
	}
	if got[0].Title != "Ref A" || got[0].Year != "2018" || got[0].Journal != "Journal A" {
		t.Fatalf("bad parse[0]: %+v", got[0])
	}
	if got[1].Title != "Ref B" || got[1].Year != "2019" || got[1].Journal != "Journal B" {
		t.Fatalf("bad parse[1]: %+v", got[1])
	}
}

// If the DOI lookup fails, or the resolved work has no referenced_works,
// ReferencedWorks must not attempt the batched second call.
func TestReferencedWorks_DOILookupFailReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	old := openAlexBase
	openAlexBase = srv.URL + "/works"
	defer func() { openAlexBase = old }()
	f := newUnguardedFetcher()
	got := f.ReferencedWorks(context.Background(), "10.1/x", 5)
	if got != nil {
		t.Fatalf("want nil, got %+v", got)
	}
}

// CitingWorks is a two-step fetch: resolve the DOI to the work's id, then
// filter by cites:<bare id> (again the bare "W_" form, not the full URL).
func TestCitingWorks_TwoStep(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "filter=cites") {
			if got := r.URL.Query().Get("filter"); got != "cites:W9" {
				t.Fatalf("cites filter must use the bare id: %q", got)
			}
			_, _ = io.WriteString(w, `{"results":[{"doi":"https://doi.org/10.4000/c","title":"Citing Paper",
              "publication_year":2022,"primary_location":{"source":{"display_name":"Journal C"},"landing_page_url":"https://c.example"},
              "authorships":[{"author":{"display_name":"C C"}}],
              "abstract_inverted_index":{"Cites":[0],"it":[1]}}]}`)
			return
		}
		// DOI lookup step: a single work object, keyed by "id".
		_, _ = io.WriteString(w, `{"id":"https://openalex.org/W9"}`)
	}))
	defer srv.Close()
	old := openAlexBase
	openAlexBase = srv.URL + "/works"
	defer func() { openAlexBase = old }()
	f := newUnguardedFetcher()
	got := f.CitingWorks(context.Background(), "10.1/x", 5)
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	if got[0].Title != "Citing Paper" || got[0].Year != "2022" || got[0].Journal != "Journal C" {
		t.Fatalf("bad parse: %+v", got[0])
	}
}

// If the DOI lookup fails, CitingWorks must not attempt the second call.
func TestCitingWorks_DOILookupFailReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	old := openAlexBase
	openAlexBase = srv.URL + "/works"
	defer func() { openAlexBase = old }()
	f := newUnguardedFetcher()
	got := f.CitingWorks(context.Background(), "10.1/x", 5)
	if got != nil {
		t.Fatalf("want nil, got %+v", got)
	}
}
