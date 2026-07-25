package parent

import "testing"

func TestDBadge(t *testing.T) {
	cases := map[string]string{"L1": "起步", "L2": "发展", "L3": "熟练", "L4": "优秀", "NA": "暂无", "": "暂无", "L9": "暂无"}
	for in, want := range cases {
		if got := DBadge(in); got != want {
			t.Errorf("DBadge(%q)=%q want %q", in, got, want)
		}
	}
}

func TestAState(t *testing.T) {
	cases := map[int]string{0: "暂未观察到", 1: "暂未观察到", 2: "偶有·多在引导后", 3: "偶有·多在引导后", 4: "观察到主动信号", 5: "观察到主动信号", -1: "暂未观察到", 7: "观察到主动信号"}
	for in, want := range cases {
		if got := AState(in); got != want {
			t.Errorf("AState(%d)=%q want %q", in, got, want)
		}
	}
}
