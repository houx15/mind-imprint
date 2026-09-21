package api

import "strings"

// 她说「想不出来」，印记答得很好，她看到的却是一个转不动的终端。
//
// # 实测出来的
//
// 2026-09-21 真学生走查第二、第四趟，**都**红在规划的第 5 轮，都是 502
// `model_unavailable`。生产日志说清楚了那不是上游抖动：
//
//	level=WARN msg="writing plan turn: reply unparseable"
//	stop_reason="stop" reply_bytes=603
//	reply_head="想不出来没关系，这不用你凭空想——咱们去找。现在这里没有搜索框，
//	            你在浏览器里查一下，查到后把那句话或数字和出处告诉我……"
//	level=WARN msg="writing plan turn: retry also unparseable"
//
// 模型**答得很好**，只是没把那句话装进 JSON 信封里。重问一次，第二次还是
// 一句人话。于是她拿到的是一句「后台错误」。
//
// 🚨 注意它发生在**哪两轮**：第 4、第 5 轮，她连说了两次「想不出来」。
// 那两轮图上没有新东西可加（`add` 是空的），模型于是就直接说话了。
// 也就是说：**她最需要人接住的那一刻，正是这个房间最容易掉链子的那一刻。**
//
// # 为什么已有的救援够不着
//
// `salvageWritingPlanReply` 救的是**断掉的 JSON**：它先要读到一个 `{`，
// 才能往下捡字段。整份从头到尾就是一句人话，它第一个 token 就退出来了。
//
// # 把整份当成 reply，是不是在编？
//
// 不是。交出去的每一个字都是模型自己写的那句话，`add` 是空的 ——
// 她的图一个节点都不会多，一个都不会少。这和
// `salvageWritingPlanReply` 的判断是同一条：
//
//	这一轮的钱已经花掉了，而坏掉的地方不是内容本身，是它被装在哪里。
//
// 🚨 方向仍然偏向报错：`looksLikeAWholeSentence` 这道关一个字都不放松。
// 宁可让她重发一次（她刚说的话还在输入框里），也不要把半句话摆给她看
// （[[ai-errors-must-surface-never-fake]]、2026-09-12 那次救援回归）。

// writingPlanProseMinRunes 一句话短到这个程度就不像一轮陪练发言了。
//
// 十二个字。实测那两次是 603 和 788 字节（两百来字），离这条线很远；
// 定这么低只是为了挡住模型偶尔吐出来的一个孤零零的词。
const writingPlanProseMinRunes = 12

// writingPlanReplyFromProse 整份就是一句人话时，把它当成这一轮的 reply。
//
// 一个节点都不加：模型这一轮本来就没打算往图上放东西（`add` 空正是它
// 忘记信封的场合）。判不准就返回 false，照旧报错。
func writingPlanReplyFromProse(s string) (writingPlanReply, bool) {
	t := strings.TrimSpace(s)
	if len([]rune(t)) < writingPlanProseMinRunes {
		return writingPlanReply{}, false
	}
	// 🚨 长得像 JSON 的一律不碰。断掉的 JSON 归 salvageWritingPlanReply 管，
	// 它会**只**取 reply 那个字段；从这里走会把满屏的 {"add":[... 原样
	// 印到她脸上。
	if looksLikeJSONIsh(t) {
		return writingPlanReply{}, false
	}
	// 说完了的才给她看。这道关不放松。
	if !looksLikeAWholeSentence(t) {
		return writingPlanReply{}, false
	}
	return writingPlanReply{Reply: t}, true
}

// looksLikeJSONIsh 认出「这其实是一份（可能断了的）JSON」。
//
// 只认结构记号，不做解析 —— 解析已经在上面失败过了。
func looksLikeJSONIsh(t string) bool {
	if strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[") {
		return true
	}
	// 信封里的字段名。出现任何一个，就说明这是一份写坏了的 JSON，
	// 而不是模型在跟她说话。
	for _, k := range []string{`"reply"`, `"add"`, `"ready"`, `"kind"`, `"parentId"`, `"parent_id"`} {
		if strings.Contains(t, k) {
			return true
		}
	}
	return false
}
