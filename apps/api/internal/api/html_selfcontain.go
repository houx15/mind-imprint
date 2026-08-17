package api

// html_selfcontain.go — Slice 9 Task 1's pure core: a text/regex analysis of
// an interactiveHtml asset's bytes that flags EXTERNAL network dependencies.
// Closes the "publish-time HTML self-containment" half of review P1-10 (the
// runtime CSP half is deferred — see the slice plan). Deliberately dependency-
// free and side-effect-free (no I/O, no HTML parser) so it is trivially unit
// testable and cheap to run over every interactiveHtml asset at ship time.
//
// What counts as "external": an explicit http:// or https:// scheme. data:
// and blob: URIs, and any scheme-less reference (a relative path, including a
// protocol-relative "//host/path" — no protocol literally present), are
// self-contained-compatible and intentionally NOT flagged; the asset bundle
// (the HTML plus its co-uploaded siblings under the same courses/<slug>/
// prefix) is expected to reference only itself or embed data inline.
//
// This is a border walk, not an HTML/CSS/JS parser: it looks for a fixed set
// of textual patterns known to reach out to the network. It cannot prove a
// document is network-free (obfuscated code can dodge textual patterns), only
// that it isn't OBVIOUSLY reaching out — which is exactly the bar the plan
// asks for as the tractable, pre-real-asset core; deep sandboxing is the
// runtime CSP's job (deferred).

import (
	"fmt"
	"regexp"
)

// SelfContainIssue is one external-network-dependency finding.
type SelfContainIssue struct {
	Message string
}

var (
	// reAttrURL matches an absolute http(s) URL in a src= or href= attribute
	// value, case-insensitively. This single pattern covers every tag that
	// carries one of these two attributes — <script src>, <link href>
	// (including <link rel="stylesheet" href>), <img src>, <iframe src>,
	// <audio src>, <video src>, <source src> — so no per-tag duplicate rule is
	// needed.
	reAttrURL = regexp.MustCompile(`(?i)\b(?:src|href)\s*=\s*(["'])(https?://[^"'<>\s]+)["']`)

	// reFetch matches a fetch(...) call.
	reFetch = regexp.MustCompile(`(?i)\bfetch\s*\(`)

	// reXHR matches use of the XMLHttpRequest constructor.
	reXHR = regexp.MustCompile(`(?i)\bXMLHttpRequest\b`)

	// reESImport matches a static ES module import from an absolute http(s)
	// URL: import ... from "https://...". [^;]*? (not [^;{]*?) deliberately
	// allows a named-import brace group (import { a, b } from "...") between
	// the keywords. Dynamic import("https://...") is also covered.
	reESImportFrom    = regexp.MustCompile(`(?i)\bimport\b[^;]*?\bfrom\s+(["'])(https?://[^"']+)["']`)
	reESImportDynamic = regexp.MustCompile(`(?i)\bimport\s*\(\s*(["'])(https?://[^"']+)["']`)

	// reCSSURL matches a CSS url(...) function referencing an absolute
	// http(s) URL — this alone also covers the url()-form of @import
	// (@import url(https://...)); the string form of @import
	// (@import "https://...") is matched separately below.
	reCSSURL = regexp.MustCompile(`(?i)\burl\(\s*(["']?)(https?://[^"')\s]+)["']?\s*\)`)

	// reCSSImportString matches the bare-string form of @import:
	// @import "https://..." (no url() wrapper).
	reCSSImportString = regexp.MustCompile(`(?i)@import\s+(["'])(https?://[^"']+)["']`)
)

// ValidateHtmlSelfContained scans html's text for external network
// dependencies and returns one issue per distinct dependency found (empty/nil
// = self-contained as far as this text-level check can tell). Pure: no I/O.
func ValidateHtmlSelfContained(html []byte) []SelfContainIssue {
	s := string(html)
	seen := make(map[string]bool)
	var issues []SelfContainIssue
	add := func(msg string) {
		if seen[msg] {
			return
		}
		seen[msg] = true
		issues = append(issues, SelfContainIssue{Message: msg})
	}

	for _, m := range reAttrURL.FindAllStringSubmatch(s, -1) {
		add(fmt.Sprintf("external network reference in src/href attribute: %s", m[2]))
	}
	if reFetch.MatchString(s) {
		add("script calls fetch() — network requests are not self-contained")
	}
	if reXHR.MatchString(s) {
		add("script uses XMLHttpRequest — network requests are not self-contained")
	}
	for _, m := range reESImportFrom.FindAllStringSubmatch(s, -1) {
		add(fmt.Sprintf("ES module import references an external URL: %s", m[2]))
	}
	for _, m := range reESImportDynamic.FindAllStringSubmatch(s, -1) {
		add(fmt.Sprintf("dynamic import() references an external URL: %s", m[2]))
	}
	for _, m := range reCSSURL.FindAllStringSubmatch(s, -1) {
		add(fmt.Sprintf("CSS references an external URL via url(): %s", m[2]))
	}
	for _, m := range reCSSImportString.FindAllStringSubmatch(s, -1) {
		add(fmt.Sprintf("CSS @import references an external stylesheet: %s", m[2]))
	}

	return issues
}
