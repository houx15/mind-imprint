package api

// course_ship_assets.go — Slice 9 Task 1: the pre-publish asset gate wired
// into postCourseShip (course_ship.go). Border-walks the stored
// CourseDefinition-2.0 JSON for every interactiveHtml block's `source`,
// downloads that object from OSS, and runs ValidateHtmlSelfContained
// (html_selfcontain.go) over its bytes. A missing object, or any
// self-containment issue, is a BLOCKING publish issue — closes the
// tractable core of review P1-12 + the publish-time half of P1-10.
//
// Deliberately a border walk, not a deep re-validation of the definition:
// packages/contracts already gated PUT (id/type/source shape), so this stage
// only reads the three fields (type, source, id) it needs to find
// interactiveHtml assets and check them.

import (
	"context"
	"encoding/json"
	"fmt"
)

// shipAssetGetter is the narrow OSS slice this stage needs — an object
// download, exactly as agent.CourseAudioStore is the narrow slice
// GenerateCourseAudio needs (see course_audio.go's header). *oss.Service
// already satisfies it via its GetObject(ctx, key) method, so no adapter
// type is needed at the call site; tests use a stub instead of a real bucket.
type shipAssetGetter interface {
	GetObject(ctx context.Context, key string) ([]byte, error)
}

// shipAssetIssue is one blocking publish issue, keyed by the asset path and
// the block that references it, for the 422 response body.
type shipAssetIssue struct {
	AssetPath string `json:"assetPath"`
	BlockID   string `json:"blockId,omitempty"`
	Message   string `json:"message"`
}

// shipDefinitionDoc is the narrow border-walk shape of a stored
// CourseDefinition-2.0 document: only what's needed to find interactiveHtml
// blocks and their source path. Every other authored field is deliberately
// ignored here — deep validation is packages/contracts' job.
type shipDefinitionDoc struct {
	Course struct {
		Parts []struct {
			Slices []struct {
				Blocks []struct {
					ID     string `json:"id"`
					Type   string `json:"type"`
					Source string `json:"source"`
				} `json:"blocks"`
			} `json:"slices"`
		} `json:"parts"`
	} `json:"course"`
}

// validateShipAssets border-walks def for every interactiveHtml block,
// downloads its declared source object from OSS via getter, and validates
// that it's self-contained. Returns every blocking issue found; nil/empty
// means clean (including when def has no interactiveHtml blocks at all, or
// fails to unmarshal — this stage never second-guesses definition validity,
// it only adds new blocking reasons on top of an already-valid PUT).
func validateShipAssets(ctx context.Context, getter shipAssetGetter, slug string, def []byte) []shipAssetIssue {
	var doc shipDefinitionDoc
	if err := json.Unmarshal(def, &doc); err != nil {
		return nil
	}

	var issues []shipAssetIssue
	for _, part := range doc.Course.Parts {
		for _, slice := range part.Slices {
			for _, block := range slice.Blocks {
				if block.Type != "interactiveHtml" || block.Source == "" {
					continue
				}

				key := courseAssetKey(slug, block.Source)
				data, err := getter.GetObject(ctx, key)
				if err != nil {
					issues = append(issues, shipAssetIssue{
						AssetPath: block.Source,
						BlockID:   block.ID,
						Message:   fmt.Sprintf("asset not found in storage: %v", err),
					})
					continue
				}

				for _, si := range ValidateHtmlSelfContained(data) {
					issues = append(issues, shipAssetIssue{
						AssetPath: block.Source,
						BlockID:   block.ID,
						Message:   si.Message,
					})
				}
			}
		}
	}
	return issues
}
