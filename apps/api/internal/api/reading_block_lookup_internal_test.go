package api

import "testing"

// 线上走查（2026-09-17）：点的是 prolonged，卡片讲的是 starved to death。
// 逐字核对只保证词在这一段里，保证不了是她点的那个。
func TestLookupCardMustBeTheTappedWord(t *testing.T) {
	cards := []readingWord{
		{Term: "starved to death", Meaning: "饿死"},
		{Term: "prolonged", Meaning: "持续很久的"},
	}
	if c := lookupCardFor(cards, "prolonged"); c == nil || c.Term != "prolonged" {
		t.Errorf("点的是 prolonged，拿到 %+v", c)
	}
	// 词组里包含她点的那个词：对的，prompt 就是这么要求的。
	if c := lookupCardFor(cards, "Starved"); c == nil || c.Term != "starved to death" {
		t.Errorf("点的是 starved，应该拿到它所在的词组，拿到 %+v", c)
	}
	// 没有一张讲她那个词：nil，调用点据此重问 / 失败。
	if c := lookupCardFor(cards, "photosynthesize"); c != nil {
		t.Errorf("没有讲 photosynthesize 的卡，却拿到 %+v", c)
	}
	// 按整词比，不按子串：点 to，不该落到 starved to death 以外的什么「toward」上；
	// 点 star 也不该匹配 starved。
	if c := lookupCardFor(cards, "star"); c != nil {
		t.Errorf("star 不是 starved，拿到 %+v", c)
	}
	// 点的是一个词组：整组要连着出现在 term 里。
	if c := lookupCardFor(cards, "to death"); c == nil || c.Term != "starved to death" {
		t.Errorf("to death 是 starved to death 的一段，拿到 %+v", c)
	}
	if c := lookupCardFor(cards, "death to"); c != nil {
		t.Errorf("顺序反了不该匹配：%+v", c)
	}
	if c := lookupCardFor(cards, ""); c != nil {
		t.Errorf("空词不该匹配任何卡：%+v", c)
	}
	if c := lookupCardFor([]readingWord{{Term: "O’Connor", Meaning: "人名"}}, "o’connor"); c == nil {
		t.Error("撇号在词里，大小写不同，应该认得出")
	}
}
