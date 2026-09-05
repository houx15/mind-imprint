// Package interests 是领域词表 —— 她说得出的那一层。
//
// 和 internal/disciplines 是配套的两张表，各占一层：
//
//	interests    金融 · 裁缝 · 游戏 —— 一个人会说自己「对 ___ 感兴趣」的词
//	disciplines  经济学 · 材料科学 · 交互设计 —— 学校管这件事叫什么
//
// 一个领域连着 2 到 4 门学科，这条边 (Interest.Disciplines) 是**写在数据里的**，
// 不靠模型判定。所以「你的游戏和金融，底下是同一根 —— 概率」这句话查表就能说，
// 不花一次调用，也不会今天这么连明天那么连。
//
// # 为什么是闭表
//
// 2026-09-02 那版采集器让模型自由造词，造出来的是「例外与代表性」「一个结论要
// 多少证据」这种粒度 —— 一周之后那棵树是一丛谁也认不出的灌木。闭表把「词不零碎」
// 从 prompt 里的一句劝告变成一条**可验的不变量**：Exists 说不认识，这个词就不
// 落库。同 disciplines.IsField、interest.KeepGrounded。
//
// # 为什么条目这么瘦
//
// 只有 ID / Field / Zh / En / Disciplines 五个字段，每个都有真实读者：
//
//	ID           存在 interest_keyword 上，落库前过 Exists
//	Field        这片叶子挂在哪根枝上
//	Zh / En      显示
//	Disciplines  它扎在哪几条根上
//
// 早期草稿里还有 Aliases 和 Asks，都删了。别名的唯一用处是喂 prompt，而把一千
// 个别名塞进 system 会让每次采集的输入涨三倍；Asks 则和学科自己的 Asks 重复，
// 界面上显示的是后者。
//
// # 为什么有两份一模一样的 interests.json
//
// 和 disciplines 同一个理由：真相源在 packages/contracts/interests/（前端构建期
// import 同一份），go:embed 出不了本包目录，所以这里放一份逐字节副本，
// TestEmbeddedCopyMatchesSourceOfTruth 守着两者不漂移。改词表请改
// packages/contracts/interests/build.py 再跑它，它会同时写两份。
package interests

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

//go:embed interests.json
var interestsJSON []byte

// Interest 是一个领域。
type Interest struct {
	ID    string `json:"id"`
	Field string `json:"field"`
	Zh    string `json:"zh"`
	En    string `json:"en"`
	// Disciplines 是它扎进的学科 id，指向 disciplines.json。至少两条。
	// 打错一个 id 不会编译报错，只会让一片叶子连不到任何根 ——
	// TestEveryDisciplineIDIsReal 守着这件事。
	Disciplines []string `json:"disciplines"`
}

var (
	once    sync.Once
	all     []Interest
	byID    map[string]Interest
	loadErr error
)

func load() {
	once.Do(func() {
		if err := json.Unmarshal(interestsJSON, &all); err != nil {
			loadErr = err
			return
		}
		byID = make(map[string]Interest, len(all))
		for _, it := range all {
			byID[it.ID] = it
		}
	})
}

// LoadErr 返回解析 interests.json 时的错误（正常情况下是 nil）。
func LoadErr() error { load(); return loadErr }

// All 返回全部领域，按文件里的顺序。
func All() []Interest { load(); return all }

// ByID 按 id 查一个领域。
func ByID(id string) (Interest, bool) {
	load()
	it, ok := byID[id]
	return it, ok
}

// Exists 报告 id 在不在表里。**这是闭表那条不变量的执行点**：采集器返回的每个
// id 都要过这一关，过不了就丢掉，绝不落库。
func Exists(id string) bool {
	load()
	_, ok := byID[id]
	return ok
}

// ByField 返回一根主枝下的全部领域。
func ByField(field string) []Interest {
	load()
	out := make([]Interest, 0, 48)
	for _, it := range all {
		if it.Field == field {
			out = append(out, it)
		}
	}
	return out
}

// PromptList 把整张表写成喂给采集器的候选清单。
//
// 按主枝分组，每组一行，因为**分组本身就是给模型的信息**：它在挑「游戏」的时候
// 看得见同组还有「编程」「机器人」，比一串两百多个词的流水账更容易挑对。
//
// 只带 id 和中文名。带上英文名会让输入长一倍而不增加任何辨识度；带上学科边会
// 诱导模型去解释自己的选择，而我们不要它的解释，只要它选的那个 id。
func PromptList() string {
	load()
	fields := []string{"formal", "science", "making", "society", "humanities", "arts", "self"}
	labels := map[string]string{
		"formal":     "数学与形式",
		"science":    "科学与自然",
		"making":     "技术与创造",
		"society":    "社会与世界",
		"humanities": "人文与写作",
		"arts":       "艺术与表达",
		"self":       "自我与成长",
	}
	var b strings.Builder
	for _, f := range fields {
		rows := ByField(f)
		if len(rows) == 0 {
			continue
		}
		b.WriteString(labels[f])
		b.WriteString("：")
		for i, it := range rows {
			if i > 0 {
				b.WriteString(" · ")
			}
			b.WriteString(it.Zh)
			b.WriteString("(")
			b.WriteString(it.ID)
			b.WriteString(")")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// IDs 返回全部 id，排序后返回，便于测试与日志里稳定比对。
func IDs() []string {
	load()
	out := make([]string, 0, len(all))
	for _, it := range all {
		out = append(out, it.ID)
	}
	sort.Strings(out)
	return out
}
