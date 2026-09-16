package pbl

import (
	"context"
	"encoding/json"
	"testing"

	"mindimprint/api/internal/gateway"
)

func TestGeneratedScriptSyntax(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"static", "<p>我的主页</p>", true},
		{"modern JS", `<script>const title = document.querySelector('h1'); title?.focus();</script>`, true},
		{"module", `<script type="module">const result = await Promise.resolve(1);</script>`, true},
		{"JSON data", `<script type="application/ld+json">{"name":"我"}</script>`, true},
		{"does not execute", `<script>throw new Error('must never execute');</script>`, true},
		// Reduced from the real generated flower keydown handler: an extra }
		// stopped every flower from being created, despite complete HTML/body.
		{"flower regression", `<script>el.addEventListener('keydown',function(e){if(e.key==='Enter'){goToProcess();}}if(e.key==='Escape'){closeModal()}});</script>`, false},
		{"second script", `<script>const x=1;</script><script>const y = ;</script>`, false},
		{"repaired handler", `<script>el.addEventListener('keydown',function(e){if(e.key==='Enter'){goToProcess();}if(e.key==='Escape'){closeModal()}});</script>`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			document := "<html><body>" + tc.body + "</body></html>"
			output, _ := json.Marshal(map[string]string{"html": document})
			stub := gateway.NewStubProvider([]gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: string(output)}, {Kind: gateway.EventDone}})
			got, _, err := GenerateHeroCode(context.Background(), stub, gateway.Resolved{}, CreativeDirection{}, "test", "", "", nil)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
			if !tc.valid && got != "" {
				t.Fatal("invalid code returned as a usable result")
			}
			if tc.valid && got != document {
				t.Fatal("validation rewrote generated source")
			}
		})
	}
}
