package interest

import (
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/disciplines"
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

// Harvested 是采集器从一次完成里读出来的一个候选关键词。
type Harvested struct {
	TextZh string
	TextEn string
	Field  string
	// Note 是印记对这个词之于她的一句话。
	Note string
	// Evidence 是她自己的那句原话。**空的一律丢掉** —— 树的全部说服力都建立在
	// 「这个词不是猜的，这是你说过的话」上面。
	Evidence string
}

const harvestSystemPrompt = `你在读一个中学生刚刚完成的一件事，从里面找出 1-3 个
「她正在关心的东西」，作为她兴趣树上的关键词。

什么算一个好关键词：
- 是一个**她关心的问题或方法**，不是文章的话题标签。
  好：「例外与代表性」「一个结论要多少证据」。
  差：「珊瑚」「环保」「阅读理解」。
- 她自己的话里能找到根据。找不到根据就不要写这个词。

字段要求：
- zh 中文关键词，4-10 字；en 对应的英文说法。
- field 只能是这七个之一：formal（数学与形式）science（科学与自然）
  making（技术与创造）society（社会与世界）humanities（人文与写作）
  arts（艺术与表达）self（自我与成长）。
- note：一句话，对她说，讲这个词在她身上是什么。不超过 40 字。
- evidence：**她自己写的原话**，原样摘录，不要改写、不要总结。
  找不到能作为根据的原话，就不要输出这个词。

宁可只给一个词，也不要凑满三个。

只输出一个 JSON 对象，不要任何解释：
{"keywords":[{"zh":"","en":"","field":"","note":"","evidence":""}]}`

// BuildHarvestPrompt 拼出采集用的 system 与 user 两段。
//
// kind 是这次完成的是什么（reading / writing / project），它进 prompt 是因为
// 「读完一篇」和「写完一篇」里，什么算她的原话是不一样的：阅读里是她的收获与
// 批注，写作里是她的正文。
func BuildHarvestPrompt(kind, title, body string) (system, user string) {
	label := map[string]string{
		"reading": "她刚读完的一篇文章，以及她自己写下的收获与批注",
		"writing": "她刚写完的一篇文章",
		"project": "她刚做完、正在复盘的一个项目",
	}[kind]
	if label == "" {
		label = "她刚完成的一件事"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "下面是%s。\n\n标题：%s\n\n", label, strings.TrimSpace(title))
	b.WriteString("内容：\n")
	b.WriteString(strings.TrimSpace(body))
	b.WriteString("\n")
	return harvestSystemPrompt, b.String()
}

type harvestReply struct {
	Keywords []struct {
		Zh       string `json:"zh"`
		En       string `json:"en"`
		Field    string `json:"field"`
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
//  2. zh 为空 → 丢。
//  3. field 不是七根主枝之一 → 丢。模型偶尔会发明 "magic"、"tech"。
//  4. evidence 空或太短 → 丢。**这条是树的地基**：没有原话的词是装饰。
//  5. 同一次里重复的词 → 只留第一个。
//  6. 超过三个 → 截断。
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
		zh := strings.TrimSpace(k.Zh)
		ev := strings.TrimSpace(k.Evidence)
		if zh == "" || !disciplines.IsField(k.Field) {
			continue
		}
		if len([]rune(ev)) < evidenceMinRunes {
			continue
		}
		n := disciplines.Normalize(zh)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, Harvested{
			TextZh:   zh,
			TextEn:   strings.TrimSpace(k.En),
			Field:    k.Field,
			Note:     strings.TrimSpace(k.Note),
			Evidence: ev,
		})
		if len(out) == maxPerHarvest {
			break
		}
	}
	return out, nil
}
