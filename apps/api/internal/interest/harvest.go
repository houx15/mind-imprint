package interest

import (
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/interests"
	"mindimprint/api/internal/prompts"
)

// maxPerHarvest 是一次完成最多长出几个词。
//
// 三个。一篇阅读长出八个词，一周之后那棵树就是一丛灌木 —— 而树之所以能被看懂，
// 靠的正是上面的词少到她认得出每一个。采集器宁可漏掉一个真兴趣（它下次还会
// 再出现，那时强度反而更有说服力），也不要一次性铺满一根枝。
const maxPerHarvest = 3

// evidenceMinRunes 是「她自己那句话」的最短长度。
//
// 模型偷懒时会把 evidence 填成「有」「是的」这类词来通过非空检查。四个字符是
// 一个低到不会误伤真短句、又高到能挡住敷衍的门槛。
const evidenceMinRunes = 4

// Harvested 是采集器从一次完成里读出来的一个候选领域。
//
// # 它为什么不带中文名
//
// 2026-09-04 之前这里有 TextZh / TextEn / Field 三个字段，由模型填。那意味着
// **模型在命名她的兴趣**，于是长出来的是「例外与代表性」「一个结论要多少证据」
// 这种粒度的词，一周之后那棵树没人认得出。
//
// 现在模型只能交出一个 InterestID，中文名、英文名、所属主枝全部从
// internal/interests 查表得到。模型说不出树上的字，它只能从两百多个已经写好的
// 词里指一个。
type Harvested struct {
	// InterestID 指向 interests.json。落库前过 interests.Exists。
	InterestID string
	// Note 是印记对这个领域之于她的一句话。
	Note string
	// Evidence 是她自己的那句原话。**空的一律丢掉** —— 树的全部说服力都建立在
	// 「这个词不是猜的，这是你说过的话」上面。
	Evidence string
}

// harvestSystemPromptHead 是候选清单之前的那一段。清单由 interests.PromptList()
// 在 BuildHarvestPrompt 里拼进来，不写死在常量里 —— 词表改了 prompt 要跟着改，
// 而两份手抄的清单一定会漂。
const harvestSystemPromptHead = prompts.InterestHarvestSystemPromptHead

// 「怎么算选中一个领域」有两份，因为**处境不一样**。词表、字段要求、JSON 形状、
// 解析器仍然共用一套 —— 分开的只有这一段判据。
//
// 🚨 2026-09-11 实测抓到的：兴趣测试原来整条 system prompt 都复用采集的，于是
// 它把下面 harvestSelectionRules 那两条也带了过去。可那两条的核心是「材料的
// 话题不算」，而**测试里根本没有材料**：她挑的那个作品、她写的那句理由，全都是
// 她自己敲进去的。模型照着这条判据读，得出的结论是「她只是在描述一个角色」，
// 于是返回空数组。
//
// 同一条真实作答（利威尔 / 「在关键时刻依然保持理智，做出自己的选择」），真模型
// 各跑三次：复用采集判据 0/3 长出词，换成下面这份 3/3（decision-making，
// evidence 是她的原话）。学生那边看到的差别是「这次没有长出关键词」和一个词。

const harvestSelectionRules = prompts.InterestHarvestSelectionRules

const quizSelectionRules = prompts.InterestQuizSelectionRules

const harvestSystemPromptTail = prompts.InterestHarvestSystemPromptTail

// BuildHarvestPrompt 拼出采集用的 system 与 user 两段。
//
// kind 是这次完成的是什么（reading / writing / project），它进 prompt 是因为
// 「读完一篇」和「写完一篇」里，什么算她的原话是不一样的：阅读里是她的收获与
// 批注，写作里是她的正文。
func BuildHarvestPrompt(kind, title, body string) (system, user string) {
	label := map[string]string{
		"reading": "学生刚读完的一篇文章，以及学生自己写下的收获与批注",
		"writing": "学生刚写完的一篇文章",
		"project": "学生刚做完、正在复盘的一个项目",
	}[kind]
	if label == "" {
		label = "学生刚完成的一件事"
	}
	system = harvestSystemPromptHead + interests.PromptList() + "\n" + harvestSelectionRules + harvestSystemPromptTail

	var b strings.Builder
	fmt.Fprintf(&b, "下面是%s。\n\n标题：%s\n\n", label, strings.TrimSpace(title))
	b.WriteString("内容：\n")
	b.WriteString(strings.TrimSpace(body))
	b.WriteString("\n")
	return system, b.String()
}

type harvestReply struct {
	Keywords []struct {
		ID       string `json:"id"`
		Note     string `json:"note"`
		Evidence string `json:"evidence"`
	} `json:"keywords"`
}

// ParseHarvestReply 读采集器的回话。
//
// 丢弃规则，按这个顺序：
//
//  1. 解析不出 JSON → **报错**，调用方不长任何词。绝不返回一个像样的假关键词
//     ——「你关心修理权」这种编出来的观察，比没有观察糟得多，而且她无从反驳。
//  2. id 不在词表里 → 丢。**这是闭表那条不变量的执行点。** 模型很会造
//     "esports" "coral-reefs" 这种看起来合理的 id；靠 prompt 劝它不要造是在
//     期望模型守规矩，而不是让规矩成立。
//  3. evidence 空或太短 → 丢。**这条是树的地基**：没有原话的词是装饰。
//  4. 同一次里重复的 id → 只留第一个。
//  5. 超过三个 → 截断。
func ParseHarvestReply(raw string) ([]Harvested, error) {
	body, err := sliceJSONObject(raw)
	if err != nil {
		return nil, err
	}
	var rep harvestReply
	if err := json.Unmarshal(body, &rep); err != nil {
		return nil, fmt.Errorf("harvest reply is not the expected object: %w", err)
	}

	out := make([]Harvested, 0, maxPerHarvest)
	seen := map[string]bool{}
	for _, k := range rep.Keywords {
		id := strings.TrimSpace(k.ID)
		ev := strings.TrimSpace(k.Evidence)
		if !interests.Exists(id) || seen[id] {
			continue
		}
		if len([]rune(ev)) < evidenceMinRunes {
			continue
		}
		seen[id] = true
		out = append(out, Harvested{
			InterestID: id,
			Note:       strings.TrimSpace(k.Note),
			Evidence:   ev,
		})
		if len(out) == maxPerHarvest {
			break
		}
	}
	return out, nil
}

// KeepGrounded 丢掉那些 evidence **并非真的出自她所写内容**的关键词。
//
// # 它挡的是哪一件事（2026-09-03 由真模型实测发现）
//
// 解析器只检查 evidence 的长度，检查不了它的**出处**。一次真实调用里，模型把
// prompt 里我自己写的那句脚手架文字
//
//	「她喜欢的是：《进击的巨人》里的利威尔」
//
// 当作她的原话返回了。它长度合格、语义通顺、能过解析器的每一道检查 —— 然后被
// 挂在她的树上，标签写着「你自己写的」。**那是一句谎话，而且是她无从反驳的
// 那种。**
//
// 靠改 prompt 去劝模型不要这么干是不够的：那是在期望模型守规矩，而不是让规矩
// 成立。所以这里改成一条**可验证的不变量**：evidence 必须逐字出现在她真的写下
// 的文字里，否则这个词不落库。
//
// corpus 是她自己写的全部内容（阅读的收获与批注、写作的正文、测试里她填的两栏）。
// 比对前把空白折叠掉：模型经常重新换行，那不该算作转述。
func KeepGrounded(hs []Harvested, corpus string) []Harvested {
	flat := foldSpace(corpus)
	out := make([]Harvested, 0, len(hs))
	for _, h := range hs {
		if strings.Contains(flat, foldSpace(h.Evidence)) {
			out = append(out, h)
		}
	}
	return out
}

// foldSpace 把所有连续空白（含换行）折成一个空格，便于逐字比对。
func foldSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
