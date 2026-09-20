package api

import (
	"testing"

	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/vocab"
)

// 两处词表一字不差。methods.json 的 `genre` 字段由 vocab 读，
// 判断这一篇是什么文体的是 api 这一侧 —— 值分岔的那天，一篇记叙文会拿到
// 一套空的方法库，而且没有任何东西会报错。
func TestWritingGenreWordsMatchVocab(t *testing.T) {
	if genreArgument != vocab.GenreArgument {
		t.Errorf("genreArgument = %q，vocab 那边是 %q", genreArgument, vocab.GenreArgument)
	}
	if genreNarrative != vocab.GenreNarrative {
		t.Errorf("genreNarrative = %q，vocab 那边是 %q", genreNarrative, vocab.GenreNarrative)
	}
}

func outlineRow(kind string) sqlc.WritingOutline {
	return sqlc.WritingOutline{Kind: kind, Depth: writingKindDepth(kind)}
}

// 🚨 板上摆的东西胜过题目里的字，而且拿不准一律议论文。
// 两条都是**方向**，不是偏好：见 writing_genre.go 头上那段。
func TestWritingGenreOf(t *testing.T) {
	narrativeTitle := "记一次难忘的旅行"

	for _, tc := range []struct {
		name    string
		wr      sqlc.Writing
		outline []sqlc.WritingOutline
		want    string
	}{
		{
			name: "什么都没有 → 议论文",
			wr:   sqlc.Writing{Title: "手机该不该带进学校"},
			want: genreArgument,
		},
		{
			name: "题目像记叙文，板上还空着 → 记叙文",
			wr:   sqlc.Writing{Title: narrativeTitle},
			want: genreNarrative,
		},
		{
			// 这一条是整条判据的重点：她已经在按议论文摆了。
			name:    "题目像记叙文，但板上已经有中心论点 → 议论文",
			wr:      sqlc.Writing{Title: narrativeTitle},
			outline: []sqlc.WritingOutline{outlineRow(writingKindThesis)},
			want:    genreArgument,
		},
		{
			name:    "题目像议论文，但板上摆的是场景和细节 → 记叙文",
			wr:      sqlc.Writing{Title: "论坚持的意义"},
			outline: []sqlc.WritingOutline{outlineRow(writingKindScene), outlineRow(writingKindDetail)},
			want:    genreNarrative,
		},
		{
			// 开篇两种文体都有，它一个人说明不了什么。
			name:    "板上只有开篇 → 还是回到题目，题目也不像 → 议论文",
			wr:      sqlc.Writing{Title: "读书的好处"},
			outline: []sqlc.WritingOutline{outlineRow(writingKindOpening)},
			want:    genreArgument,
		},
		{
			name:    "老师布置的那句里有记叙文的说法",
			wr:      sqlc.Writing{Title: "无题", AssignedPrompt: strPtr("请写一件事：那一天我明白了什么")},
			want:    genreNarrative,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := writingGenreOf(tc.wr, tc.outline); got != tc.want {
				t.Errorf("writingGenreOf = %q，want %q", got, tc.want)
			}
		})
	}
}

// 记叙文那四种块也要有标题、有深度、算得出父节点该挂在哪。
// 少一样，她在图上就会看见一张没有名字的卡，或者一张落错层的卡 ——
// R1 修的就是这个（「这个是总结，不是分论点」）。
func TestNarrativeKindsAreComplete(t *testing.T) {
	for _, k := range []string{
		writingKindScene, writingKindDetail, writingKindTurn, writingKindFeeling,
	} {
		if !writingKindValid(k) {
			t.Errorf("%s 不在闭表里", k)
		}
		if writingKindLabel(k, "") == "" {
			t.Errorf("%s 没有标题", k)
		}
		if writingKindGenre(k) != genreNarrative {
			t.Errorf("%s 的文体是 %q，该是记叙文", k, writingKindGenre(k))
		}
	}
	// 细节挂在场景下面；场景挂在开篇下面（记叙文没有中心论点）。
	rows := []sqlc.WritingOutline{
		{Kind: writingKindOpening, Position: 0},
		{Kind: writingKindScene, Position: 1},
	}
	if p := writingKindParentOf(writingKindDetail, rows); p == nil || p.Kind != writingKindScene {
		t.Errorf("细节没挂到场景上：%+v", p)
	}
	if p := writingKindParentOf(writingKindScene, rows); p == nil || p.Kind != writingKindOpening {
		t.Errorf("场景没挂到开篇上：%+v", p)
	}
	// 🚨 议论文那八种的文体不能被这次改动带偏。
	for _, k := range []string{
		writingKindThesis, writingKindPoint, writingKindEvidence, writingKindReference,
		writingKindReasoning, writingKindCounter, writingKindRebuttal, writingKindGap,
	} {
		if writingKindGenre(k) != genreArgument {
			t.Errorf("%s 的文体是 %q，该是议论文", k, writingKindGenre(k))
		}
	}
	// 开篇和结尾两种文体都用。
	for _, k := range []string{writingKindOpening, writingKindClosing} {
		if g := writingKindGenre(k); g != "" {
			t.Errorf("%s 的文体是 %q，该是两种都用（空串）", k, g)
		}
	}
}

// 一篇记叙文绝不该拿到议论文的那套方法，反过来也一样。
// 这是把 lang 那条轴上学到的东西（2026-08-28：英文句式漏进中文作文）
// 用在文体上。
func TestVocabNeverCrossesGenres(t *testing.T) {
	for _, m := range vocab.ForLang("zh", genreNarrative) {
		if m.Genre == genreArgument {
			t.Errorf("记叙文拿到了议论文的方法 %q", m.ID)
		}
	}
	for _, m := range vocab.ForLang("zh", genreArgument) {
		if m.Genre == genreNarrative {
			t.Errorf("议论文拿到了记叙文的方法 %q", m.ID)
		}
	}
	// 记叙文这一边必须真的有东西可教 —— 过滤过头等于把这间屋子关上了。
	var sawDetail bool
	for _, m := range vocab.For("body", "zh", genreNarrative) {
		if m.Category == "detail" {
			sawDetail = true
		}
	}
	if !sawDetail {
		t.Error(`For("body","zh",记叙文) 里一条细节描写都没有 —— 她这一段无从下手`)
	}
	// 议论文这一边要拿得到分析句三法。
	if len(vocab.Analyses("zh")) != 3 {
		t.Errorf("分析句三法应当有 3 条，得到 %d", len(vocab.Analyses("zh")))
	}
	for _, m := range vocab.Analyses("zh") {
		if len(m.Patterns) == 0 {
			t.Errorf("%s 没有句式 —— 三法值钱的地方正是那一句可套的话", m.ID)
		}
	}
}

func strPtr(s string) *string { return &s }
