package api

import "strings"

// 「我觉得它一直在挑衅我。」
//
// # 这条是谁说的
//
// 产品负责人 2026-09-21 转述同事的原话：
//
//	「也是我自己用起来很不满意的地方，尤其是请印记看一看那个部分，
//	  我觉得它一直在挑衅我」
//
// 同一天的真学生走查里，一趟就有四句：
//
//	「前四段**白立的**主张在这里松了手」
//	「读者读完最后一段会以为你其实没下过判断；**可你**第 1 张**明明**说了……」
//	「读者会觉得你已经**站到对面**去了……全篇的结论**跟着塌了**」
//	「**换成谁**来写都成立，你开头那句主张**等于没说**」
//
// 每一条的判断都是**对的**。坏的是它们被写成了对她努力的判决，而不是
// 对文字的描述加下一步。「你写了四段，四段白写」和「这三处各指一个方向，
// 读者会不确定你站哪边」说的是同一件事，前一句让人想摔键盘。
//
// # 为什么已有的那道闸拦不住
//
// `personDirectedVerdict` 拦的是「你很懒」这种直接评价人的话，而上面四句
// **字面上全都在说文字**，一条都不踩。判据又一次比真失败窄了一圈
//（[[detector-must-target-the-real-failure]] 的第三次现场）。
//
// # 🚨 这一条**重问一次**，绝不丢掉
//
// `personDirectedVerdict` 命中就整条丢掉，所以它的表必须极短 ——
// 丢掉一条真反馈比留下一句难听话更糟。这里反过来：命中只是**再问一次**，
// 内容一个字都不会少。于是这张表可以写得宽一点，误判的代价是一次调用，
// 不是她那条意见没了。这正是 2026-09-21 刚修完的那个 bug 的教训
//（模型给了好意见，被服务端丢掉，她拿到一句空话）。

// writingHostilePhrases 把她做过的事判成白做、或者抓包式的说法。
//
// 🚨 只收**写死她努力**和**抓包**这两类。普通的转折（「却」「但是」「不过」）
// 一个都不收 —— 指出问题本来就要转折，收了它每一轮都要重问。
var writingHostilePhrases = []string{
	// 判她做过的事等于没做
	"等于没说", "等于没写", "等于白", "白写", "白立", "白费", "白搭", "等于零",
	// 灾难化：一处不好说成整篇完了
	"跟着塌", "结论塌", "全篇塌", "垮了", "崩了", "全盘皆输",
	// 「谁写都一样」——把她的字判成没有她也成立
	"换成谁", "换谁写", "谁来写都", "谁都能写", "换个人写也",
	// 抓包
	//
	// 🚨 「明明」不能光秃秃地收。2026-09-21 走查里印记写过一句
	//「能不能把这句话换个说法，让它**明明白白**是在说短视频让人变笨」——
	// 那是个正常的副词，收了它就要为一句好话白跑一次重问，
	// 而且重问的提示还会让它把那句好话改掉。
	// 抓包的形状是「明明」后面跟着一个她说过/写过的动作。
	"你明明", "明明说", "明明写", "明明是", "明明已经",
	"可你却", "你倒是", "自相矛盾得",
	// 全盘否定
	"一文不值", "毫无意义", "没有任何意义", "根本谈不上", "完全站不住", "一无是处",
	// 英文
	"pointless", "worthless", "wasted effort", "falls apart", "anyone could have written",
	"you contradict yourself", "says nothing at all",
}

// writingHostileTone 在这一份意见里找一句挑衅的说法，找到就交出那个词。
//
// 查总评，也查每条意见的 text 和 action —— 三处她都会读到。
func writingHostileTone(parsed writingCommentResult) string {
	if p := hostilePhraseIn(parsed.Summary); p != "" {
		return p
	}
	for _, pt := range parsed.Points {
		if p := hostilePhraseIn(pt.Text); p != "" {
			return p
		}
		if p := hostilePhraseIn(pt.Action); p != "" {
			return p
		}
	}
	return ""
}

func hostilePhraseIn(s string) string {
	if s == "" {
		return ""
	}
	low := strings.ToLower(s)
	for _, p := range writingHostilePhrases {
		if strings.Contains(low, p) {
			return p
		}
	}
	return ""
}
