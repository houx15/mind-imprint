package gateway

import (
	"context"
	"testing"
)

func TestMuxProviderDispatch(t *testing.T) {
	ds := NewStubProvider([]StreamEvent{{Kind: EventTextDelta, TextDelta: "ds"}, {Kind: EventDone}})
	an := NewStubProvider([]StreamEvent{{Kind: EventTextDelta, TextDelta: "an"}, {Kind: EventDone}})
	m := NewMuxProvider(map[string]Provider{"deepseek": ds, "anthropic": an})

	res, err := Collect(context.Background(), m, Resolved{Provider: "anthropic"}, ChatRequest{})
	if err != nil || res.Text != "an" { t.Fatalf("want an, got %q err=%v", res.Text, err) }

	if _, err := m.Stream(context.Background(), Resolved{Provider: "nope"}, ChatRequest{}); err == nil {
		t.Fatal("want error for unknown provider")
	}
}
