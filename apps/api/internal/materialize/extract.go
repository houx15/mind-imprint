package materialize

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

var skipTags = map[string]bool{"script": true, "style": true, "nav": true, "header": true, "footer": true, "aside": true, "noscript": true}
var blockTags = map[string]bool{"p": true, "h1": true, "h2": true, "h3": true, "li": true, "blockquote": true}

// extractHTML pulls the <title> and the concatenated text of block elements,
// skipping script/style/nav/header/footer/aside/noscript. Paragraphs are joined
// with a blank line so Segment can split them.
func extractHTML(body []byte) (title, text string) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", ""
	}
	var paras []string
	var walk func(n *html.Node, inSkip bool)
	walk = func(n *html.Node, inSkip bool) {
		if n.Type == html.ElementNode {
			if n.Data == "title" && title == "" {
				title = textContent(n)
			}
			if skipTags[n.Data] {
				inSkip = true
			}
			if !inSkip && blockTags[n.Data] {
				if t := textContent(n); t != "" {
					paras = append(paras, t)
				}
				return // captured; don't double-count nested blocks
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, inSkip)
		}
	}
	walk(doc, false)
	return title, strings.Join(paras, "\n\n")
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
