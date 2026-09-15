package quotematch

import (
	"reflect"
	"testing"
)

func TestNormalizeIgnoresSpacingAndCommonPunctuation(t *testing.T) {
	cases := []struct{ a, b string }{
		{"上周五我在食堂门口数了一下，有六个桶是满的。", "上周五我在食堂门口数了一下有六个桶是满的"},
		{"Hello, World!", "hello world"},
		{"a\nb\tc", "abc"},
	}
	for _, tc := range cases {
		if got := Normalize(tc.a); got != Normalize(tc.b) {
			t.Errorf("Normalize(%q) = %q, Normalize(%q) = %q, want equal", tc.a, Normalize(tc.a), tc.b, Normalize(tc.b))
		}
	}
}

func TestNormalizeLowercases(t *testing.T) {
	if got := Normalize("ABC"); got != "abc" {
		t.Fatalf("Normalize(ABC) = %q", got)
	}
}

func TestStripQuotedSpans(t *testing.T) {
	cases := []struct{ in, want string }{
		{"这一段缺一个「让步」。", "这一段缺一个。"},
		{"引用『看到什么就拿什么』这句", "引用这句"},
		{"她说“我觉得可以”了", "她说了"},
		{`By "more" I mean two things`, `By "more" I mean two things`}, // straight quotes untouched
		{"「一」和「二」都没了", "和都没了"},
		{"没有引号的一句话", "没有引号的一句话"},
		{"只有一个「开头没有关上", "只有一个「开头没有关上"}, // unpaired: kept
	}
	for _, tc := range cases {
		if got := StripQuotedSpans(tc.in); got != tc.want {
			t.Errorf("StripQuotedSpans(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExtractQuotedSpans(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"corner brackets", "这一段缺一个「让步」。", []string{"让步"}},
		{"title brackets", "引用『看到什么就拿什么』这句", []string{"看到什么就拿什么"}},
		{"curly double quotes", "她说“我觉得可以”了", []string{"我觉得可以"}},
		{"straight double quotes not recognised", `By "more" I mean two things`, nil},
		{"multiple spans in order", "「一」和「二」", []string{"一", "二"}},
		{"no quotes", "没有引号的一句话", nil},
		{"unpaired open bracket ignored", "只有一个「开头没有关上", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractQuotedSpans(tc.in)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ExtractQuotedSpans(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
