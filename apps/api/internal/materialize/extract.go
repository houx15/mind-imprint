package materialize

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

var skipTags = map[string]bool{"script": true, "style": true, "nav": true, "header": true, "footer": true, "aside": true, "noscript": true}
var blockTags = map[string]bool{"p": true, "h1": true, "h2": true, "h3": true, "li": true, "blockquote": true}

// mainTextFloor — how much text a <article>/<main> subtree must yield before we
// trust it over the whole-document scrape. Below this we assume the semantic
// wrapper was empty/decorative and fall back to the full page (#4).
const mainTextFloor = 200

// extractHTML pulls the <title> and the article's readable text. It prefers the
// <article>/<main>/[role=main] subtree when one is present and rich enough —
// which drops site chrome (nav/related/comments) that the plain block scrape
// used to fold in — and otherwise falls back to scraping block elements across
// the whole document (the original behavior). Paragraphs are joined with a
// blank line so Segment can split them.
func extractHTML(body []byte) (title, text string) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", ""
	}
	title = findTitle(doc)
	if main := findMainNode(doc); main != nil {
		if t := scrapeBlocks(main); len(t) >= mainTextFloor {
			return title, t
		}
	}
	return title, scrapeBlocks(doc)
}

// scrapeBlocks concatenates the text of block elements under root, skipping
// script/style/nav/header/footer/aside/noscript. A captured block isn't
// re-descended (no double-counting nested blocks).
func scrapeBlocks(root *html.Node) string {
	var paras []string
	var walk func(n *html.Node, inSkip bool)
	walk = func(n *html.Node, inSkip bool) {
		if n.Type == html.ElementNode {
			if skipTags[n.Data] {
				inSkip = true
			}
			if !inSkip && blockTags[n.Data] {
				if t := textContent(n); t != "" {
					paras = append(paras, t)
				}
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, inSkip)
		}
	}
	walk(root, false)
	return strings.Join(paras, "\n\n")
}

// findTitle returns the first <title>'s text.
func findTitle(doc *html.Node) string {
	var title string
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if title != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "title" {
			title = textContent(n)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return title
}

// findMainNode returns the first <article>, <main>, or role="main" element —
// the semantic main-content wrapper — or nil when the page has none.
func findMainNode(doc *html.Node) *html.Node {
	var found *html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode {
			if n.Data == "article" || n.Data == "main" {
				found = n
				return
			}
			for _, a := range n.Attr {
				if a.Key == "role" && a.Val == "main" {
					found = n
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return found
}

// textContent returns the whitespace-collapsed text of a node's subtree.
func textContent(n *html.Node) string {
	var b strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteString(" ")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
