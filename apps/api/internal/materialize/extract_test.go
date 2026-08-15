package materialize

import (
	"strings"
	"testing"
)

// A data table's rows must survive extraction — the scraper used to descend
// past <tr>/<td> (they aren't blockTags) and emit nothing, dropping the whole
// table while a preceding <p> label ("Table 1: …") stayed, so the room jumped
// straight to the next section with no notice (#4).
func TestExtractHTMLKeepsTableRows(t *testing.T) {
	body := []byte(`<html><body><article>
		<p>Table 1: List of analyzed media articles</p>
		<table>
			<caption>List of analyzed media articles</caption>
			<thead><tr><th>Outlet</th><th>Year</th></tr></thead>
			<tbody>
				<tr><td>Xinhua</td><td>2021</td></tr>
				<tr><td>Nature</td><td>2022</td></tr>
			</tbody>
		</table>
		<h2>3.2 Findings</h2>
		<p>The analysis proceeds.</p>
	</article></body></html>`)
	_, text := extractHTML(body)
	for _, want := range []string{"Outlet | Year", "Xinhua | 2021", "Nature | 2022", "List of analyzed media articles"} {
		if !strings.Contains(text, want) {
			t.Fatalf("extracted text missing %q\n--- got ---\n%s", want, text)
		}
	}
	// The following section must still be present and after the table.
	if !strings.Contains(text, "3.2 Findings") {
		t.Fatalf("extracted text missing the following section:\n%s", text)
	}
	if strings.Index(text, "Xinhua | 2021") > strings.Index(text, "3.2 Findings") {
		t.Fatalf("table rows should come before the next section:\n%s", text)
	}
}

// tableToText renders one table as a single cohesive block (caption lead line +
// pipe-joined rows on their own lines), and yields "" for an empty table.
func TestTableToText(t *testing.T) {
	empty := []byte(`<html><body><table><tbody></tbody></table></body></html>`)
	_, got := extractHTML(empty)
	if strings.TrimSpace(got) != "" {
		t.Fatalf("empty table should yield no text, got %q", got)
	}
}
