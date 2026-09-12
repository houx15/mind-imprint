package api

import (
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 她只在对话里说过、没写进作品里的那一句。
//
// 第三十六、三十七两轮各撞一次，措辞几乎一样：
//
//	「它引用的那句「全校一天倒掉的饭真的很多」在我现在框里的字里没找到」
//	「印记引用的那句『看到什么就拿什么』在我现在的框[0]里根本找不到」
//
// 注意她找的地方是**框里** —— 她把印记的引文理解成「我正文里的句子」，
// 这个理解是对的。
func TestWritingQuote_TalkOnlyIsNotTreatedAsWritten(t *testing.T) {
	snips := []sqlc.WritingSnippet{{Text: "上周三中午我在二楼靠窗那排吃饭，前面那个男生红烧肉一口没动。"}}
	msgs := []sqlc.AtomMessage{
		{Role: "student", Content: "我觉得大家就是看到什么就拿什么，根本没想过吃不吃得完。"},
		{Role: "ai", Content: "印记自己说的话不算数，绝不能进语料。"},
	}

	written := writingWrittenCorpus(snips, "")
	said := writingSaidCorpus(msgs)

	reply := "你说的「看到什么就拿什么」这句很准，把它写进第 1 块里。"

	// 合起来看：不是幻引 —— 她**确实**说过这句话。
	if got := firstGhostQuote(reply, written+said); got != "" {
		t.Fatalf("她说过的话被判成了幻引：%q", got)
	}
	// 只看她写下来的：这一句不在，所以要提醒印记说清出处。
	if got := firstGhostQuote(reply, written); got == "" {
		t.Fatal("这句话只在对话里，判据没认出来 —— 她会去正文里找，找不到")
	}
}

// 引她正文里真有的句子，两条判据都该放行，一次重试都不该发生。
func TestWritingQuote_QuotingHerWritingIsClean(t *testing.T) {
	snips := []sqlc.WritingSnippet{{Text: "前面那个男生红烧肉一口没动，米饭扒拉两口就全倒了。"}}
	msgs := []sqlc.AtomMessage{{Role: "student", Content: "随便说点别的"}}

	written := writingWrittenCorpus(snips, "")
	said := writingSaidCorpus(msgs)
	reply := "你第 1 块里「前面那个男生红烧肉一口没动」这一句最扎眼，接着往下写。"

	if got := firstGhostQuote(reply, written+said); got != "" {
		t.Errorf("她正文里就有这句，却被判成幻引：%q", got)
	}
	if got := firstGhostQuote(reply, written); got != "" {
		t.Errorf("她正文里就有这句，却被判成「只说过没写过」：%q", got)
	}
}

// 🚨 印记自己说过的话，两份语料里都不能有 —— 幻引的来源就是它的上文。
func TestWritingQuote_NeitherCorpusCarriesTheCoachsOwnWords(t *testing.T) {
	msgs := []sqlc.AtomMessage{{Role: "ai", Content: "「这是印记自己编出来的一整句话」"}}
	if said := writingSaidCorpus(msgs); said != "" {
		t.Fatalf("印记自己的话进了语料：%q", said)
	}
	if w := writingWrittenCorpus(nil, ""); w != "" {
		t.Fatalf("没有段落也没有成稿，语料该是空的：%q", w)
	}
}

// 成稿那一份也算「她写下来的」。
func TestWritingWrittenCorpus_IncludesTheDraft(t *testing.T) {
	written := writingWrittenCorpus(nil, "她在成稿里写的这一整句话摆在这里。")
	reply := "「她在成稿里写的这一整句话摆在这里」这句已经立住了。"
	if got := firstGhostQuote(reply, written); got != "" {
		t.Fatalf("成稿里的句子被判成没写过：%q", got)
	}
}
