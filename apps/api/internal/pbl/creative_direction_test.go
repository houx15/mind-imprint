package pbl

import (
	"context"
	"encoding/json"
	"mindimprint/api/internal/gateway"
	"strings"
	"testing"
)

func TestHeroGenerationUsesChoicesWithoutCandidatesOrTrialRecords(t *testing.T) {
	doc := CreativeDirection{Stage: "hero", Feeling: "科幻", Motifs: []string{"温室"}, PendingMotif: "还没添加的海底", Suggestions: []string{"未选择的城堡"}, IncludeProcess: true,
		Hero:  &HeroBrief{Mode: "code", Scene: "太空温室", Action: "点击花朵", Prompt: "制作太空温室"},
		Trial: &HeroTrial{VersionID: "previous-version", Observation: "只用于试用判断"}}
	before, _ := json.Marshal(doc)
	for _, kind := range []string{"prompt", "code"} {
		t.Run(kind, func(t *testing.T) {
			response := `{"prompt":"制作太空温室"}`
			if kind == "code" {
				response = `{"html":"<!doctype html><html><body>太空温室</body></html>"}`
			}
			provider := gateway.NewStubProvider([]gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: response}, {Kind: gateway.EventDone}})
			var err error
			if kind == "prompt" {
				_, _, err = GenerateHeroPrompt(context.Background(), provider, gateway.Resolved{}, doc)
			} else {
				_, _, err = GenerateHeroCode(context.Background(), provider, gateway.Resolved{}, doc, "学生", "", "", nil)
			}
			if err != nil {
				t.Fatal(err)
			}
			input := provider.LastRequest.Messages[1].Content
			for _, unwanted := range []string{"还没添加的海底", "pendingMotif", "未选择的城堡", "只用于试用判断", "previous-version", "suggestions", "includeProcess", "trial", "stage"} {
				if strings.Contains(input, unwanted) {
					t.Fatalf("%s leaked into generation: %s", unwanted, input)
				}
			}
			for _, wanted := range []string{"科幻", "温室", "太空温室", "点击花朵", "制作太空温室"} {
				if !strings.Contains(input, wanted) {
					t.Fatalf("student choice missing: %s", wanted)
				}
			}
			after, _ := json.Marshal(doc)
			if string(before) != string(after) {
				t.Fatal("generation changed saved student document")
			}
		})
	}
}

func TestCreativeDirectionSeparatesStudentChoices(t *testing.T) {
	doc, err := NormalizeCreativeDirection(CreativeDirection{Stage: "motifs", Hero: &HeroBrief{Mode: "image", Scene: "宇宙里的猫", Prompt: "画宇宙里的猫"}, Feeling: "  可爱与宇宙  ", Motifs: []string{"猫咪", "飞船"}, Suggestions: []string{"星球"}}, true)
	if err != nil || doc.Feeling != "  可爱与宇宙  " || len(doc.Motifs) != 2 {
		t.Fatal(doc, err)
	}
	for _, bad := range []CreativeDirection{{Stage: "unknown"}, {Stage: "feeling", Feeling: strings.Repeat("字", 2001)}, {Stage: "motifs", Motifs: []string{"树", "树"}}} {
		if _, err := NormalizeCreativeDirection(bad, false); err == nil {
			t.Fatal("invalid accepted", bad)
		}
	}
	if _, err := NormalizeCreativeDirection(CreativeDirection{Stage: "feeling"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := NormalizeCreativeDirection(CreativeDirection{Stage: "motifs", Feeling: "酷炫"}, true); err == nil {
		t.Fatal("empty student choice accepted")
	}
}

func TestHeroBriefDraftAndCompletion(t *testing.T) {
	doc := CreativeDirection{Stage: "hero", Feeling: "植物和科幻", Motifs: []string{"太空花园"}, Hero: &HeroBrief{Mode: "mixed", Scene: "透明温室漂浮在宇宙里", Action: "点击花朵打开作品", Prompt: "画透明温室，花朵可点击打开作品"}}
	if _, err := NormalizeCreativeDirection(doc, true); err != nil {
		t.Fatal(err)
	}
	doc.Hero.Prompt = ""
	if _, err := NormalizeCreativeDirection(doc, false); err != nil {
		t.Fatal("draft rejected", err)
	}
	if _, err := NormalizeCreativeDirection(doc, true); err == nil {
		t.Fatal("missing prompt accepted")
	}
	doc.Hero.Mode = "invalid"
	if _, err := NormalizeCreativeDirection(doc, false); err == nil {
		t.Fatal("bad mode accepted")
	}
}
