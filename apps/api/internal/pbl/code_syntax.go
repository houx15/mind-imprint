package pbl

import (
	"fmt"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
	"golang.org/x/net/html"
)

// Parse only: generated code is never executed on the server. This catches
// syntax failures, not missing DOM elements, broken interactions or layout.
func validateInlineScripts(document string) error {
	root, err := html.Parse(strings.NewReader(document))
	if err != nil {
		return err
	}
	scriptIndex := 0
	var walk func(*html.Node) error
	walk = func(n *html.Node) error {
		if n.Type == html.ElementNode && n.Data == "template" {
			return nil
		}
		if n.Type == html.ElementNode && n.Data == "script" {
			kind := ""
			for _, attr := range n.Attr {
				if attr.Key == "type" {
					kind = strings.ToLower(strings.TrimSpace(attr.Val))
				}
			}
			switch kind {
			case "", "module", "text/javascript", "application/javascript", "text/ecmascript", "application/ecmascript":
			default:
				return nil // JSON and other data blocks aren't JavaScript.
			}
			scriptIndex++
			var source strings.Builder
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == html.TextNode {
					source.WriteString(child.Data)
				}
			}
			result := api.Transform(source.String(), api.TransformOptions{Loader: api.LoaderJS, Target: api.ESNext, LogLevel: api.LogLevelSilent})
			if len(result.Errors) > 0 {
				problem := result.Errors[0]
				line := 1
				if problem.Location != nil {
					line = problem.Location.Line
				}
				return fmt.Errorf("生成页面代码检查失败：第%d段脚本第%d行：%s", scriptIndex, line, problem.Text)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(root)
}
