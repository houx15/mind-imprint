package awakening

import (
	"fmt"
	"mindimprint/api/internal/claritytest"
	"mindimprint/api/internal/gateway"
	"strings"
	"testing"
)

func TestClarityGuideDialogue(t *testing.T) {
	for _, guide := range Guides {
		for _, mode := range []string{"opening", "retry", "last"} {
			t.Run(string(guide.ID)+"/"+mode, func(t *testing.T) {
				in := DialogueInput{Guide: guide, Latest: "我喜欢看鸟，昨天观察到同一只鸟反复把树枝衔进屋檐，我想知道它怎样选择筑巢的地方。"}
				if mode == "retry" {
					in.NodeIndex = 4
					in.Retry = true
					in.Latest = "不知道"
				}
				if mode == "last" {
					in.NodeIndex = 7
					in.Last = true
					in.Latest = "我想做一份校园观鸟地图，让同学知道在哪里观察才不会惊扰它们。"
				}
				system, user := BuildDialoguePrompt(in)
				claritytest.Run(t, gateway.ClassDialogue, gateway.ChatRequest{Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: system}, {Role: gateway.RoleUser, Content: user}}}, func(raw string) error {
					reply := CleanReply(raw)
					if reply == "" {
						return fmt.Errorf("empty reply")
					}
					if mode == "last" && strings.ContainsAny(reply, "?？") {
						return fmt.Errorf("completed interview asks another question")
					}
					for _, phrase := range []string{"脑子转", "无聊", "最狠", "撑", "主张", "站得住"} {
						if strings.Contains(reply, phrase) {
							return fmt.Errorf("inappropriate coaching phrase: %s", phrase)
						}
					}
					if len(HallucinatedQuotes(reply, in.Latest)) > 0 {
						return fmt.Errorf("invented attributed quote")
					}
					return nil
				})
			})
		}
	}
}
