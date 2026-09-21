package promptlib

import "testing"

func TestTargetWords(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		// 库里真有的那几种写法。
		{"不少于800字", 800},
		{"不少于600字", 600},
		{"at least 100 words", 100},
		{"at least 250 words", 250},
		{"80词左右", 80},
		{"100词左右", 100},
		{"不超过60词", 60},
		{"不少于80词", 80},
		// 区间取上限。
		{"120-150词", 150},
		{"600-800之间", 800},

		// 🚨 解不出来就是 0，不猜。
		{"", 0},
		{"字数不限", 0},
		{"二选一", 0},      // 2 太小，不是字数
		{"共3题", 0},      // 3 是题数
		{"不超过5个自然段", 0}, // 5 是段数
	}
	for _, c := range cases {
		if got := TargetWords(c.in); got != c.want {
			t.Fatalf("TargetWords(%q) = %d，该是 %d", c.in, got, c.want)
		}
	}
}

// 整份库过一遍：解出来的要么是 0，要么落在一个像字数的区间里。
//
// 🚨 守的是「不会给她一个离谱的目标」——把分值（60）或年份（2026）
// 当成字数写进去，她那边会被一个不存在的要求追着跑。
func TestTargetWords_WholeLibraryIsSane(t *testing.T) {
	all, err := All()
	if err != nil {
		t.Fatal(err)
	}
	solved, blank := 0, 0
	for _, p := range all {
		n := TargetWords(p.WordLimit)
		if n == 0 {
			blank++
			continue
		}
		solved++
		if n < wordsMin || n > 5000 {
			t.Fatalf("%s 的「%s」解成了 %d 字", p.ID, p.WordLimit, n)
		}
	}
	t.Logf("解出目标字数的 %d 道，空着的 %d 道", solved, blank)
	if solved == 0 {
		t.Fatal("一道都没解出来 —— 这个解析器等于没接上")
	}
}
