// Package awakening 是觉醒协议的纯逻辑层。
//
// 它**不碰数据库**，也不发 HTTP 请求（模型调用由 internal/api 的胶水层发起，
// 这里只负责拼 prompt、解析回话、算判定）。分层照抄 internal/interest 与
// internal/pbl：领域逻辑可以在没有 Postgres、没有模型 key 的情况下被完整测试。
//
// 文件分工：
//
//	awakening.go  屏、路线、印记助手；跨文件共用的小工具
//	nodes.go      终端那 8 个节点问什么、为什么问
//	dialogue.go   对话那一次调用：prompt、长度、幻引判定
//	selection.go  选词那一次调用：prompt、解析、confirm/grow/open 判定
//	report.go     报告那一次调用：prompt、解析、payload 形状
//	brief.go      她树上已经有什么 —— 喂给终端的那份简报
//
// # 这一版和参考设计的两处分歧
//
// 参考设计（docs/reference/觉醒协议-大模型兴趣探索版-2026-09-18）的终端**没有
// 连过模型**：它的每一句「印记助手」都由五条正则给学生的文本打分，选中一个
// 预置 profile，再套字符串模板拼出来。所以那 8 个问题的措辞可以照用，而对话
// 质量从零开始，必须在上线前跑一次 LIVE_LLM。
//
// 参考设计还让模型自己判 `ready`（做完没有），同时终端又按固定 9 步推进 ——
// 两者对「她什么时候算做完了」说法不一致。这里取消模型自判：**节点推进由
// 服务端算**，模型只负责接住她上一句并把当前节点的问题问出来。
package awakening

import "strings"

/* ── 屏 ─────────────────────────────────────────────────────────────────── */

// Stage 是她走到哪一屏。值进库（awakening_run.stage），所以改名等于改数据。
type Stage string

const (
	StageBoot      Stage = "boot"      // 开场剧情
	StageWorld     Stage = "world"     // 序章：加入 / 暂不加入
	StageWarning   Stage = "warning"   // 印记提醒，引出历史档案
	StageArchive   Stage = "archive"   // 历史档案 01 · 认知让步
	StageDeck      Stage = "deck"      // AI 底牌三实验：顺 / 换 / 偏
	StageRejoin    Stage = "rejoin"    // 翻完三张牌，重新决定
	StageObserver  Stage = "observer"  // 观察者路线的落点
	StageEnergy    Stage = "energy"    // 能量卡牌
	StageNavigator Stage = "navigator" // 选印记助手
	StageTerminal  Stage = "terminal"  // 8 节点兴趣探询
	StageLens      Stage = "lens"      // NOTICE → WONDER → TEST
	StageChallenge Stage = "challenge" // 选一种验证方式
	StageTalent    Stage = "talent"    // 天赋卡牌
	StageReport    Stage = "report"    // 报告
)

// Stages 是全部合法的屏。
//
// 顺序就是**默认推进顺序**，分支在 NextStage 里处理。它不写进数据库的 CHECK：
// 加一屏不该要一次迁移（同 course.category 的理由），所以校验在应用层，
// 由 IsStage 一处负责。
var Stages = []Stage{
	StageBoot, StageWorld, StageWarning, StageArchive, StageDeck,
	StageRejoin, StageObserver, StageEnergy, StageNavigator,
	StageTerminal, StageLens, StageChallenge, StageTalent, StageReport,
}

var stageSet = func() map[Stage]bool {
	m := make(map[Stage]bool, len(Stages))
	for _, s := range Stages {
		m[s] = true
	}
	return m
}()

// IsStage 报告这个值是不是一屏。空串不是 —— 一行新开的 run 由数据库默认成
// 'boot'，客户端没有理由再发一个空的上来。
func IsStage(s string) bool { return stageSet[Stage(s)] }

/* ── 路线 ───────────────────────────────────────────────────────────────── */

// Route 是序章那个选择。
//
// 它决定她看不看 AI 底牌那一段：选 joined 直接进能量卡牌，选 observer 先走
// 档案 → 三实验 → 重新决定。**两条路最后都能到终端**，observer 只是多看了
// 三张牌 —— 这一屏不是一道会答错的题。
type Route string

const (
	RouteNone     Route = ""
	RouteJoined   Route = "joined"
	RouteObserver Route = "observer"
)

func IsRoute(s string) bool {
	switch Route(s) {
	case RouteNone, RouteJoined, RouteObserver:
		return true
	}
	return false
}

/* ── 印记助手 ───────────────────────────────────────────────────────────── */

// Guide 是她选的那个印记助手。
//
// 代号（不是中文名）进库，因为 prompt 的语气段落和音频文件名都按它索引：
// NOVA → hot-*.m4a，SAGE → sage-*.m4a，KIRO → dark-*.m4a。
type Guide string

const (
	GuideNova Guide = "NOVA" // 热血同好 · 鼓励陪伴
	GuideSage Guide = "SAGE" // 资深向导 · 证据启发
	GuideKiro Guide = "KIRO" // 腹黑军师 · 反方陪练
)

// GuideProfile 是一个印记助手在界面上和 prompt 里的全部内容。
//
// 中文名与那句自我介绍是她在选择屏上看见的字；Style 只进 prompt，她看不见。
type GuideProfile struct {
	ID      Guide
	Zh      string
	Label   string
	Body    string
	Quote   string
	Accent  string
	// AudioPrefix 是这个助手的语音文件前缀（hot / sage / dark）。
	AudioPrefix string
	// Style 写进 system prompt 的语气段落。
	Style string
}

// Guides 是三选一的全部。难度递增，但三条都通向同一个终端。
var Guides = []GuideProfile{
	{
		ID: GuideNova, Zh: "热血同好", Label: "鼓励陪伴 · 难度 1",
		Body:  "先发现亮点，再陪你走一步。",
		Quote: "一起去探索未知的领域吧！",
		Accent: "#55e6ff", AudioPrefix: "hot",
		Style: "语气有能量、亲切、敏锐。先回应她说的一个具体细节，再问下一问。不要堆夸奖。",
	},
	{
		ID: GuideSage, Zh: "资深向导", Label: "证据启发 · 难度 2",
		Body:  "给你线索，再带你比较证据。",
		Quote: "我会为你提供线索，但路要你自己走。",
		Accent: "#9e8cff", AudioPrefix: "sage",
		Style: "语气温和、克制，不用感叹号。先准确复述她刚说的那一点，再提出下一问。",
	},
	{
		ID: GuideKiro, Zh: "腹黑军师", Label: "反方陪练 · 难度 3",
		Body:  "比较证据，检查条件，考虑反例。",
		Quote: "我们一起比较不同的解释，看看各自有什么证据。",
		Accent: "#ff7189", AudioPrefix: "dark",
		Style: "语气直接、认真、尊重她。联系她说过的具体内容，请她比较不同解释、核对证据或考虑适用条件。不要挑衅、嘲讽或评价她本人，也不要用暗示她本该知道答案的反问。",
	},
}

// GuideByID 查一个印记助手。第二个返回值为 false 表示这个代号不存在。
func GuideByID(id string) (GuideProfile, bool) {
	for _, g := range Guides {
		if string(g.ID) == id {
			return g, true
		}
	}
	return GuideProfile{}, false
}

// GuideOrDefault 查一个印记助手，查不到给 NOVA。
//
// 调用点是 prompt 拼装：一行 navigator 为空的 run（她还没走到选择屏就退出了，
// 后来又被恢复）不该让一次对话调用崩掉。
func GuideOrDefault(id string) GuideProfile {
	if g, ok := GuideByID(id); ok {
		return g
	}
	return Guides[0]
}

/* ── 共用小工具 ─────────────────────────────────────────────────────────── */

// truncRunes 按**字符**截断，不按字节。
//
// 🚨 按字节切会把一个汉字切成两半，留下一个替换符。这个函数只用在喂给模型的
// 上文上，**绝不用在她自己写的字上** —— 她的字一个都不要切
// （memory: observation-tool-is-the-bug-2026-09-12）。
func truncRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// foldSpace 把连续空白（含换行）折成一个空格。
//
// 逐字比对之前都要过这一道：模型经常重新换行，而那不该算作转述。
// 和 interest.KeepGrounded 用的是同一个做法。
func foldSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

// runeLen 数字符数。界面上说「还差多少字」用的也是它，所以两边的数一致。
func runeLen(s string) int { return len([]rune(s)) }

// replaceAll 是 strings.ReplaceAll 的转出口，省得 nodes.go 再 import 一次。
func replaceAll(s, old, new string) string { return strings.ReplaceAll(s, old, new) }

// punct 是判「这句话去掉套话之后还剩不剩东西」时要忽略的字符。
//
// 中英标点都列上：她的输入法给什么就是什么，而一句只剩「，。……」的回答
// 和一句空回答是同一件事。
const punct = "，。！？；：、,.!?;:…—－-～~ 　\t\n\r\"'「」『』（）()《》<>【】[]"

// stripPunct 去掉全部标点与空白。
func stripPunct(s string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(punct, r) {
			return -1
		}
		return r
	}, s)
}
