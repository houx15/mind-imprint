package pbl

import (
	"strings"
	"testing"
)

// 🚨 每一问是冲着哪件事去的，必须跟着问题一起活下来。
//
// 复盘最容易变成一张感想表：问题看着都对，落到任何项目上都成立，她于是答
// 「挺好的」。把她当初写下的那句话摆在问题上面，她答的就不再是「我有什么收获」，
// 而是「我现在怎么看我当时写的这句话」——那是两件事。
//
// 这两列（anchor_kind / anchor_ref）0111 就加了，之前一直被写成 free / ""。
func TestParseLookback_KeepsTheEvidenceEachQuestionAsksAbout(t *testing.T) {
	qs, err := parseLookback(`{"questions":[
	  {"section":"what","prompt":"你后来是怎么跟同桌说的？",
	   "evidence":"她把问题定成了：同桌需要在拿东西出来之前就知道它大概能卖掉"},
	  {"section":"how","prompt":"听到他说怕丢面子时你心里什么感觉？","evidence":""}
	]}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(qs) != 2 {
		t.Fatalf("问题数不对：%+v", qs)
	}
	if !strings.Contains(qs[0].Evidence, "同桌需要在拿东西出来之前") {
		t.Fatalf("出处丢了：%+v", qs[0])
	}
	// 「感受如何」那一段冲着她本人问，没有出处也仍然是合法的一问——不能因此
	// 被丢掉，否则六段会缺一段。
	if qs[1].Prompt == "" {
		t.Fatalf("没有出处的那一问被丢了：%+v", qs)
	}
	if qs[1].Evidence != "" {
		t.Fatalf("凭空补了一个出处：%q", qs[1].Evidence)
	}
}

// 出处要原样带回，不能被 trim 以外的任何加工动过——她读到的应该就是她当初
// 写下的那句话。
func TestParseLookback_EvidenceIsCarriedVerbatim(t *testing.T) {
	const line = "关于「义卖摆在哪儿」，她选了「操场角落」，因为时间宽裕"
	qs, err := parseLookback(`{"questions":[{"section":"moment",
	  "prompt":"当时你犹豫了多久？","evidence":"  ` + line + `  "}]}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if qs[0].Evidence != line {
		t.Fatalf("出处被改写了：\n想要：%s\n拿到：%s", line, qs[0].Evidence)
	}
}

// 🚨 prompt 里必须真的把这件事要求出去。少了这一句，模型不会凭空开始给出处，
// 而两列又会静悄悄地退回空字符串——正是它们过去一直的样子。
func TestLookbackSystem_AsksForTheEvidenceLine(t *testing.T) {
	if !strings.Contains(lookbackSystem, "evidence") {
		t.Fatal("system prompt 里没有 evidence 这一格")
	}
	if !strings.Contains(lookbackSystem, "原样抄") {
		t.Fatal("没要求原样抄回那一行——改写过的出处她认不出是自己写的")
	}
}

// 🚨 上文里每一行都以「  · 」开头，模型「原样抄回」时会把圆点一起抄走。
// 她看到的于是是「当时你写的是：· 午休想安静待着的同学…」——一个凭空冒出来的
// 符号，会让她以为这句话不是自己写的。2026-09-03 线上第一次生成就是这样。
func TestParseLookback_StripsTheBulletTheModelCopiedAlong(t *testing.T) {
	qs, err := parseLookback(`{"questions":[{"section":"moment","prompt":"当时你怎么想的？",
	  "evidence":"  · 午休想安静待着的同学 需要 一个待得住的地方"}]}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if strings.HasPrefix(qs[0].Evidence, "·") || strings.HasPrefix(qs[0].Evidence, " ") {
		t.Fatalf("圆点没削掉：%q", qs[0].Evidence)
	}
	// 只削前缀，内容一个字都不能少。
	if !strings.HasPrefix(qs[0].Evidence, "午休想安静待着的同学") {
		t.Fatalf("削过头了：%q", qs[0].Evidence)
	}
}
