package api

// course_ship_assets_test.go — whitebox unit tests for validateShipAssets,
// the pre-publish asset gate's ship-side wiring. Deps.OSS is a concrete
// *oss.Service (not fakeable without a real bucket — see course_ship_test.go's
// header note), so this file exercises validateShipAssets directly against a
// stub shipAssetGetter instead of going through the live HTTP handler: it is
// the exact function postCourseShip calls with a.d.OSS in place of the stub,
// so this is equivalent coverage without a real OSS dependency. Pure/DB-free.

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// stubShipAssetGetter is a minimal in-memory double for shipAssetGetter.
type stubShipAssetGetter struct {
	objects map[string][]byte // key -> bytes; a missing key => GetObject error
}

func (s *stubShipAssetGetter) GetObject(_ context.Context, key string) ([]byte, error) {
	data, ok := s.objects[key]
	if !ok {
		return nil, errors.New("object not found: " + key)
	}
	return data, nil
}

func shipDefWithInteractiveHtml(source string) []byte {
	return []byte(`{"schemaVersion":"2.0","course":{"id":"c","title":"t","language":"en","estimatedMinutes":1,` +
		`"objectives":[],"parts":[{"slices":[{"blocks":[{"id":"blk1","type":"interactiveHtml",` +
		`"source":"` + source + `","protocolVersion":"1.0","aspectRatio":"4:3"}]}]}]}}`)
}

func TestValidateShipAssetsCleanWhenSelfContained(t *testing.T) {
	slug := "clean-course"
	getter := &stubShipAssetGetter{objects: map[string][]byte{
		courseAssetKey(slug, "interaction.html"): []byte(`<html><body><script>console.log("ok");</script></body></html>`),
	}}

	issues := validateShipAssets(context.Background(), getter, slug, shipDefWithInteractiveHtml("interaction.html"))
	if len(issues) != 0 {
		t.Fatalf("want 0 issues for a self-contained asset, got %d: %+v", len(issues), issues)
	}
}

func TestValidateShipAssetsBlocksExternalFetch(t *testing.T) {
	slug := "leaky-course"
	getter := &stubShipAssetGetter{objects: map[string][]byte{
		courseAssetKey(slug, "interaction.html"): []byte(`<script>fetch("https://evil.example.com/x");</script>`),
	}}

	issues := validateShipAssets(context.Background(), getter, slug, shipDefWithInteractiveHtml("interaction.html"))
	if len(issues) != 1 {
		t.Fatalf("want 1 blocking issue, got %d: %+v", len(issues), issues)
	}
	if issues[0].BlockID != "blk1" || issues[0].AssetPath != "interaction.html" {
		t.Fatalf("issue = %+v, want blockId=blk1 assetPath=interaction.html", issues[0])
	}
	if !strings.Contains(issues[0].Message, "fetch(") {
		t.Fatalf("issue message = %q, want it to mention fetch(", issues[0].Message)
	}
}

func TestValidateShipAssetsBlocksMissingObject(t *testing.T) {
	slug := "missing-asset-course"
	getter := &stubShipAssetGetter{objects: map[string][]byte{}}

	issues := validateShipAssets(context.Background(), getter, slug, shipDefWithInteractiveHtml("interaction.html"))
	if len(issues) != 1 {
		t.Fatalf("want 1 blocking issue for a missing object, got %d: %+v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Message, "not found") {
		t.Fatalf("issue message = %q, want it to mention not found", issues[0].Message)
	}
}

func TestValidateShipAssetsCleanWhenNoInteractiveHtmlBlocks(t *testing.T) {
	slug := "no-html-course"
	getter := &stubShipAssetGetter{objects: map[string][]byte{}}
	def := []byte(`{"schemaVersion":"2.0","course":{"id":"c","title":"t","language":"en","estimatedMinutes":1,` +
		`"objectives":[],"parts":[{"slices":[{"narrations":[{"text":"hi","audio":"n1.mp3"}]}]}]}}`)

	issues := validateShipAssets(context.Background(), getter, slug, def)
	if len(issues) != 0 {
		t.Fatalf("want 0 issues when there are no interactiveHtml blocks (and no OSS calls made), got %d: %+v", len(issues), issues)
	}
}
