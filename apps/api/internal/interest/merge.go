// Package interest 是兴趣模型的纯逻辑层 —— 归一化、合并、路由、采集解析。
//
// 它**不碰数据库**，也不发 HTTP 请求（模型调用由 internal/api 的胶水层发起，
// 这里只负责拼 prompt 和解析回话）。分层照抄 internal/pbl：领域逻辑可以在没有
// Postgres、没有模型 key 的情况下被完整测试，胶水层才是不好测的那一半。
//
// 三件事在这里：
//
//	merge.go    她的词怎么算同一个词，强度怎么由来源条数推出来
//	router.go   词 → 学科的三档路由（别名 / 共现 / 模型）
//	harvest.go  从一次完成的阅读、写作、项目里采集关键词
package interest

import "mindimprint/api/internal/disciplines"

// strengthLadder 是「来源条数 → 强度读数」的阶梯，来自设计文档 §2.1。
//
// 强度是**读数**（STR 4/5），不是等级，也不是她挣来的分数。它回答的只有一个
// 问题：这个词在她身上出现过多少次。所以它只能由来源条数推出来，任何一处能
// 直接设强度的接口，迟早会有人拿它去「调整」某个词。
//
// 索引 = 来源条数，值 = 强度；超出的落到最后一档。
var strengthLadder = []int{1, 1, 2, 3, 3, 4, 4, 4, 5}

// Strength 把来源条数换算成 1..5 的强度读数。
//
// 阶梯前密后疏是有意的：第二个来源出现，是「这个词不是偶然」这条信息里最重的
// 一次跳变；从第八次到第八十次则几乎不再增加任何信息。
func Strength(sourceCount int) int {
	if sourceCount < 0 {
		sourceCount = 0
	}
	if sourceCount >= len(strengthLadder) {
		return strengthLadder[len(strengthLadder)-1]
	}
	return strengthLadder[sourceCount]
}

// SameKeyword 报告两个词是否该被当成同一个词。
//
// 复用 disciplines.Normalize，所以「去重」「别名命中」「别名唯一性测试」三处
// 共享同一个「相同」的定义。三份各自实现的归一化，是那种在演示当天才会被发现
// 的分歧。
func SameKeyword(a, b string) bool {
	na, nb := disciplines.Normalize(a), disciplines.Normalize(b)
	return na != "" && na == nb
}

// Norm 是 disciplines.Normalize 的转出口，省得每个调用方都去 import 两个包。
func Norm(s string) string { return disciplines.Normalize(s) }
