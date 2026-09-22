package guidance

import (
	"strings"
	"testing"
)

func TestResolveReturnsTheMostSpecificText(t *testing.T) {
	r := NewRegistry()
	r.Add(SlotSkeleton, Scope{Surface: SurfaceWrite, Lang: "zh"}, "中文骨架")
	r.Add(SlotSkeleton, Scope{Surface: SurfaceWrite, Lang: "en"}, "英文骨架")

	got, err := r.Resolve(Key{Surface: SurfaceWrite, Lang: "en"}, SlotSkeleton)
	if err != nil {
		t.Fatalf("取不到：%v", err)
	}
	if got[SlotSkeleton] != "英文骨架" {
		t.Errorf("拿到 %q，想要英文骨架", got[SlotSkeleton])
	}
}

// 🚨 这一条是这个包存在的理由之一。2026-09-22「按语言挑」那次的教训：
// 把中文那几条挡住之后英文那边空了 —— 少给一整块，而线上看起来只是
// 印记话变少了，没有人会去查。取不到必须炸，不许给空串。
func TestResolveErrorsRatherThanReturningEmpty(t *testing.T) {
	r := NewRegistry()
	r.Add(SlotSkeleton, Scope{Surface: SurfaceWrite, Lang: "zh"}, "中文骨架")

	_, err := r.Resolve(Key{Surface: SurfaceWrite, Lang: "en"}, SlotSkeleton)
	if err == nil {
		t.Fatal("英文没有登记，Resolve 必须报错")
	}
	// 报错要说清是哪个槽、哪一次 —— 半夜看日志的人得能直接定位。
	if !strings.Contains(err.Error(), string(SlotSkeleton)) {
		t.Errorf("报错里没有槽名：%v", err)
	}
	if !strings.Contains(err.Error(), "en") {
		t.Errorf("报错里没有 Key 的内容：%v", err)
	}
}

func TestResolveTakesSeveralSlotsAtOnce(t *testing.T) {
	r := NewRegistry()
	r.Add(SlotKinds, Scope{Surface: SurfaceWrite}, "节点类型")
	r.Add(SlotMaterial, Scope{Surface: SurfaceWrite}, "材料")

	got, err := r.Resolve(Key{Surface: SurfaceWrite, Lang: "zh"}, SlotKinds, SlotMaterial)
	if err != nil {
		t.Fatalf("取不到：%v", err)
	}
	if len(got) != 2 {
		t.Errorf("要两个槽，拿到 %d 个", len(got))
	}
}

// 默认注册表里，写作立题那四种组合都要取得齐。
func TestDefaultRegistryCoversWritingPlan(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		for _, genre := range []string{"argument", "narrative"} {
			k := Key{Surface: SurfaceWrite, Lang: lang, Genre: genre}
			got, err := Default().Resolve(k, SlotKinds, SlotMaterial, SlotSkeleton)
			if err != nil {
				t.Errorf("%s/%s 取不齐：%v", lang, genre, err)
				continue
			}
			for slot, text := range got {
				if strings.TrimSpace(text) == "" {
					t.Errorf("%s/%s 的 %s 是空的", lang, genre, slot)
				}
			}
		}
	}
}
