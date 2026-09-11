package api

import (
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func ghostCorpus() string {
	return writingQuoteCorpus(
		[]sqlc.WritingSnippet{{Text: "上周五我在食堂门口数了一下，有六个桶是满的。"}},
		"上周五我在食堂门口数了一下，有六个桶是满的。这说明浪费不是个别现象。",
		[]sqlc.AtomMessage{
			{Role: "student", Content: "我觉得主要是打饭的阿姨给太多了"},
			// 🚨 印记自己说过的话**不进语料** —— 幻引正是从它自己上一轮来的，
			// 收进来等于让它自证。
			{Role: "ai", Content: "你正文里那句「食堂每天倒掉五十公斤」写得太满了"},
		},
	)
}

func TestFirstGhostQuote_CatchesASentenceSheNeverWrote(t *testing.T) {
	// 第二十四轮线上真实的那一种：印记引着她早就删掉的句子接着教。
	reply := "你最后那句「食堂每天倒掉五十公斤」下得太绝对了，把它换成你数过的那个数。"
	got := firstGhostQuote(reply, ghostCorpus())
	if got == "" {
		t.Fatal("这句她没写过，应该被认出来")
	}
	if got != "食堂每天倒掉五十公斤" {
		t.Fatalf("认出来的是 %q", got)
	}
}

func TestFirstGhostQuote_LetsHerOwnWordsThrough(t *testing.T) {
	corpus := ghostCorpus()
	for _, reply := range []string{
		// 正文里的原话。
		"你写的「有六个桶是满的」就是一条真材料，把它挪到主张前面。",
		// 只差句末标点 —— 模型复述时最常变的就是这个，不该误伤。
		"「上周五我在食堂门口数了一下」这句放在开头更有力",
		// 她自己在对话里说过的话，印记复述它是正常的。
		"你刚说「我觉得主要是打饭的阿姨给太多了」——那就把它写进第二段。",
		// 太短的引号多半是术语，不是在复述她的句子。
		"这一段缺一个「让步」。",
		// 根本没有引号。
		"把第三句挪到第一句前面。",
	} {
		if got := firstGhostQuote(reply, corpus); got != "" {
			t.Errorf("误伤了：%q\n（回复：%s）", got, reply)
		}
	}
}

// 🚨 印记自己上一轮说过的话不算数。这一条是承重的：幻引的来源就是它自己的
// 上文，如果把 ai 的消息也收进语料，它引自己说过的假话就永远查不出来。
func TestWritingQuoteCorpus_ExcludesTheCoachsOwnWords(t *testing.T) {
	reply := "你正文里那句「食堂每天倒掉五十公斤」还是太满了。"
	if firstGhostQuote(reply, ghostCorpus()) == "" {
		t.Fatal("印记引的是它自己上一轮编的句子，必须能查出来")
	}
}
