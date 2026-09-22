package api

import "strings"

// writing_board.go —— 板：她用手摆完的那一下，怎么让 印记 知道。
//
// # 板是什么，以及它为什么不是被删掉的那种卡片
//
// 2026-09-11 走查写作房间时最刺眼的一件事：`apps/lite-web/src/writings/` 里
// **一处 pointer 事件、一处拖动、一处重新排序都没有**。而阅读室 2026-09-10
// 已经有两块能拖的板了。产品负责人对「交互」有确定的意思（2026-09-04）：
// **一块能用手摆的板，拖是主要动词。**
//
// 🚨 这不是把 2026-08-27 删掉的工具卡搬回来。那次删的是**卡片货架**——
// 一排常驻在屏幕上、等她挑、挑了要填表的东西（产品原话：
// "in pro edition we don't really use any card, we only use snippet box"）。
// 板长在她已经写完的某一段上，和旁边那颗「AI审阅这一段」是同一类东西，
// 摆完就没了。
//
// # 闭环走的是已经闭好的那一条
//
// 阅读室那两块板从第一天起就走卡片那条路（她摆 → 结果原样变成一条真的学生
// 消息 → 下一轮 印记 对着它说话），所以它们不需要第二套接线。写作这边同理：
// 板摆完之后就是一次普通的 `POST /turn`，`board` 字段只是告诉服务端
// **这一条是摆完一块板产生的**，好在那一轮的上文里加一句说明。
//
// 为什么需要那句说明：`postLiteWritingTurn` 用的是 pro 自己那个
// `agent.ProposeProjectCoachReply`（写作房间没有自己的系统提示词），
// 而那个 producer 是 pro 和 lite 共用的，**不能为了 lite 去改**
// （lite-must-not-break-pro）。能改的是 lite 自己拼的那段 projection，
// 所以这句话加在那里。
//
// 阅读室在这条回路上断过一次（2026-09-03 同事试用：「透镜应用完毕之后，
// 没有响应，没有推进到下一步」），修法是 `readingLensDone`。这里是同一个位置
// 上的同一件事，所以一开始就把它接上。

// writingBoardKinds 是闭表。
//
// 认不出来的值**当没给处理，不报错**：她摆的东西是真的，那条消息本身完全成立，
// 少一句上下文不该把这一轮弄丢。这和阅读室「卡片被丢掉照常成功」是同一个取向。
var writingBoardKinds = map[string]string{
	"role": writingRoleBoardNote,
}

// writingBoardNote 返回这一轮该加进 projection 的那段话；没有板就是空串。
func writingBoardNote(kind string) string {
	return writingBoardKinds[strings.TrimSpace(kind)]
}
