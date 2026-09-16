package pbl

import (
	"context"
	"encoding/json"
	"mindimprint/api/internal/gateway"
	"strings"
	"testing"
)

func TestHeroImageSourceMustBeAnActualImage(t *testing.T) {
	for _, tc := range []struct {
		html  string
		valid bool
	}{
		{ImageHeroHTML("<script>bad()</script>", `" onerror="bad()`), true},
		{"<html><body><p>" + HeroImageSource + "</p></body></html>", false},
		{`<html><body><template><img src="` + HeroImageSource + `"></template></body></html>`, false},
		{"<html><body>" + strings.Repeat(`<img src="`+HeroImageSource+`">`, 4) + "</body></html>", false},
	} {
		output, _ := json.Marshal(map[string]string{"html": tc.html})
		stub := gateway.NewStubProvider([]gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: string(output)}, {Kind: gateway.EventDone}})
		result, _, err := GenerateHeroCode(context.Background(), stub, gateway.Resolved{}, CreativeDirection{}, "test", "", "", nil, HeroImageSource)
		if (err == nil) != tc.valid {
			t.Fatalf("valid=%v err=%v", tc.valid, err)
		}
		if tc.valid && strings.Contains(result, "<script>bad()") {
			t.Fatal("student input became code")
		}
	}
}
