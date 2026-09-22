package api

// Prompt assembly for writing_turn.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/prompts"
)

// writingRoomHowTo is what the room's screen actually offers, so 印记's
// "what to do next" names real buttons.
//
// 2026-09-18 写作入口走查：她写完三块、剩两块空着，问怎么提交。印记说
// 「提交按钮不在编辑器里」「把空白块删掉」—— 这个房间里删不了块，
// 提交就在成稿那一页。她连着五步找不到出口，clarity 掉到 2。
const writingRoomHowTo = prompts.WritingRoomHowTo

// writingCoachGroundingRules 是**她已经写出东西之后**，才加进上文的两条。
//
// 只在有片段时加：一张白纸上没有句子可引，也没有「她已经写过了」可言。
//
// # 一 · 说她哪里不行，就得指着那一句说
//
// 结构化的那条路（`AI审阅这一段`）早就守着这条：每条意见的 quote 必须
// 逐字出现在她写的东西里，对不上的整条丢掉（validateCommentPoints）。
// **对话这条路一直没有任何约束** —— 于是 2026-09-12 第二十轮走查里，
// 十来条卡壳说的都是同一件事：
//
//	「它不告诉我具体哪一句要改、怎么改才叫立起来」
//	「它说缺少权衡和限定、像绝对断言，但没告诉我具体哪句要改」
//	「印记说『判断没有立起来』，但我开头和结尾都写了啊，不太懂它要我改哪里」
//
// 一句「你的判断没立起来」，她无从下手，也无从反驳 —— 她甚至没法确认
// 印记读的是不是她这一版。
//
// 🚨 这一条**没法在代码里验**（自由对话没有可校验的输出类型），所以它只是
// 一条希望，不是保证 —— 见 [[prompt-output-must-be-verifiable-2026-09-03]]。
// 真正的保证在那条结构化的路上；这里能做的是把她的原文摆在上文里
// （上面那几段就是），让「引一句」成为最省力的选择。
//
// # 二 · 她此刻在写，不在现场
//
// 走查里印记连着几轮让她「去查一下成本」「去问问打饭阿姨」。她的原话：
//
//	「让我去查成本或者问阿姨，但我现在坐在电脑前根本没法去问，只能自己编一个」
//
// **让她去编，是这个产品最不该做的事。** 她手上有的是她见过的、记得的东西；
// 要她去取一件此刻取不到的材料，只会把她推向编造。
//
// # 三 · 指出毛病之后，得给一个动作
//
// 第二十二轮走查里她连着两步在烦这个：
//
//	「它一直让我自己读、自己想，不直接告诉我怎么改，有点烦」
//	「它一直问我觉得是重复还是呼应，又不直接告诉我怎么改，烦死了」
//	「它说结尾只在重复开头，但没告诉我结尾该怎么写才算不重复」
//
// 🚨 这一条要小心读：**她想要的不是答案，而结论也不是「那就把答案给她」。**
// 铁律①在这儿不让步。真正缺的是 story-coach 给那种回复起的名字
// —— "Diagnostic Without Return"：一轮以「你这里不对，你觉得呢」结束，
// 把她的下一步收走了。
//
// 结构化那条路早就守着这条：每条 issue 必须带一句祈使的 `Action`，空的整条
// 丢掉（CommentPoint.Action，writing_comment.go）。对话这条路没有。
// 所以这里补的是同一件事：**给动作，不给那句话**。
// 「把第三句挪到第一句前面」是动作；替她写出那一句，就是替她写作文。
const writingCoachGroundingRules = prompts.WritingCoachGroundingRules
