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
