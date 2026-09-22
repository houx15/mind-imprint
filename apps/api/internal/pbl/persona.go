package pbl

// persona.go — 第一关：她的页面给谁看。
//
// 产品负责人 2026-09-03：「compose an audience - who is he/she? why he/she knows
// you? what they are willing to see? what you are liking to show? what feeling
// you want to bring to them?」
//
// 这一关回答的是整个项目的驱动问题：**我想让谁，看见我的什么？**
//
// ## 她在这一关不打字
//
// 印记先做出两三个可能的受众，每个带一组关键词；她挑一个、否掉别的、划掉不同意
// 的关键词。判断，不是填写。
//
// ## 候选人从哪来
//
// 从**她真做过的事**来：她读过的、写过的、做过的。凭空生成三个受众会得到三个
// 一模一样的模板人（「对你的领域感兴趣的同龄人」），而那种东西她挑哪个都一样，
// 于是这一关就变成了一次点击。所以生成的输入是她的真实材料，输出必须落回到
// 那些材料上。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// PersonaCandidate 是一个可能的受众。
type PersonaCandidate struct {
	// Label 是一个短称呼。他是一类读者，不是一个具体的人，所以不给名字——
	// 给了名字，她会开始想那个人，而不是想那一类人。
	Label string `json:"label"`
	// WhyKnows：他为什么会知道她、怎么点进这一页。
	WhyKnows string `json:"whyKnows"`
	// Wants：他想在这一页上看到什么。
	Wants string `json:"wants"`
	// Feeling：这一页该给他什么感觉。第三关的配色从这里派生。
	Feeling string `json:"feeling"`
	// Keywords：三到六个。第二关拿它对结构，第三关拿它派生配色。
	Keywords []string `json:"keywords"`
	// Portrait 是画这个人的提示词。印记自己写——它比我们更清楚这三个人差在哪。
	Portrait string `json:"portrait"`
}

// PersonaWanted 是一次生成几个候选。
//
// 三个。两个是一道二选一（她会挑「比较好的那个」而不是想清楚），四个开始变成
// 一份清单——而清单是用来扫的，不是用来判断的。
const PersonaWanted = 3

const personaSystem = `你在帮助中学生选择个人主页的目标读者。根据学生提供的阅读、写作或项目经历，提出两到三个可能的读者类型，供学生比较和选择。这些是候选设想，不是对真实读者需求的调查结论。

各字段用途如下：
label：读者类型的简短名称，不编造具体人名。
whyKnows：这类读者可能在什么场合了解学生或访问主页，联系已提供的经历说明。
wants：这类读者可能希望从主页了解什么。
feeling：希望页面给读者的感受，用一个词或短句说明。
keywords：三到六个描述页面风格的关键词，用于后续选择配色。
portrait：一句扁平插画风格的人物图像提示词，画面不含文字，不描绘具体名人。

候选应在访问场合、关注内容或呈现风格上有实质区别。材料不足时可以减少候选数量，不补造学生经历。
label、whyKnows、wants、feeling 和 keywords 会显示给学生。解释时用「你」称呼学生，用「读者」指代候选受众，清楚区分双方。
只返回 JSON：
{"personas": [{"label":"","whyKnows":"","wants":"","feeling":"","keywords":[],"portrait":""}]}`

// GeneratePersonas 从她的真实材料里推出几个候选受众。
func GeneratePersonas(
	ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, material string,
) ([]PersonaCandidate, gateway.ChatUsage, error) {
	if strings.TrimSpace(material) == "" {
		return nil, gateway.ChatUsage{}, errors.New("pbl: no material to build an audience from")
	}
	// 🚨 要一次重试。
	//
	// 这一步是她这个项目的**第一件事**，而模型偶尔会回一批用不了的候选（少字段、
	// 或者干脆给个空数组）。一次失败对她来说就是「生成失败」四个字挡在第一关
	// 门口。重试一次不解决模型的随机性，但把"第一次就撞上"的概率压下去一个量级。
	//
	// 只重一次：两次都不行说明不是抖动（多半是材料太薄），那时候该让她看见那句
	// 报错，而不是转更久的圈。同一个做法见 AssessReport 的 maxAssessAttempts。
	var usage gateway.ChatUsage
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		res, err := gateway.Collect(ctx, prov, resolved, gateway.ChatRequest{
			MaxTokens: 2048,
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: personaSystem},
				{Role: gateway.RoleUser, Content: "他做过的事：\n" + strings.TrimSpace(material)},
			},
		})
		usage.InputTokens += res.Usage.InputTokens
		usage.OutputTokens += res.Usage.OutputTokens
		if err != nil {
			return nil, usage, err
		}
		out, perr := ParsePersonas(res.Text)
		if perr == nil {
			return out, usage, nil
		}
		lastErr = perr
	}
	return nil, usage, lastErr
}

// ParsePersonas 读模型返回的 JSON。
//
// 🚨 解析失败就报错，不给一组兜底的模板人。三个模板受众看起来和真的一模一样，
// 而她照着一个编出来的读者去定整页的调子——错在哪儿她永远看不出来。
// 见 memory · ai-errors-must-surface-never-fake。
func ParsePersonas(raw string) ([]PersonaCandidate, error) {
	// 同 ParsePalettes：数括号，见 jsonwire.go。
	s := firstJSONObject(strings.TrimSpace(raw))
	var out struct {
		Personas []PersonaCandidate `json:"personas"`
	}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("pbl: personas are not JSON: %w；模型回的是：%s",
			err, clip(raw, 400))
	}

	kept := make([]PersonaCandidate, 0, len(out.Personas))
	for _, p := range out.Personas {
		p.Label = strings.TrimSpace(p.Label)
		p.WhyKnows = strings.TrimSpace(p.WhyKnows)
		p.Wants = strings.TrimSpace(p.Wants)
		p.Feeling = strings.TrimSpace(p.Feeling)
		p.Portrait = strings.TrimSpace(p.Portrait)
		p.Keywords = trimAll(p.Keywords)
		// 一个没有称呼、没有理由、或者没有关键词的候选，在界面上是一张残卡，
		// 而她要在三张卡之间做判断。宁可少一张，不要一张残的。
		if p.Label == "" || p.WhyKnows == "" || len(p.Keywords) == 0 {
			continue
		}
		if len(p.Keywords) > maxPersonaKeywords {
			p.Keywords = p.Keywords[:maxPersonaKeywords]
		}
		kept = append(kept, p)
		if len(kept) == PersonaWanted {
			break
		}
	}
	if len(kept) == 0 {
		// 带上原话：一个候选都留不下，可能是模型给了空数组（材料太薄，它老实
		// 地不编），也可能是字段名不对。两者要做的事完全不同，不带原话分不出来。
		return nil, fmt.Errorf("pbl: no usable persona came back；模型回的是：%s", clip(raw, 400))
	}
	return kept, nil
}

// maxPersonaKeywords 是一个受众带几个关键词的上限。
//
// 六个。再多，第三关派生配色时它们会互相矛盾，而她没有办法判断是哪几个在打架。
const maxPersonaKeywords = 6

func trimAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// PortraitPrompt 把候选自己带的那句提示词收紧成真正送去画的那一句。
//
// 🚨 两条是我们加的，不由模型决定：
//
//  1. **不要文字。** 生成模型往图里写字几乎必然出错（尤其中文），而一张写着
//     乱码的画像会让整块板看起来是坏的。
//  2. **不要真实人物、不要照片写实。** 这是一个虚构的读者，画成照片写实的脸，
//     学生会以为那是一个真人。界面上另外还标着「这张画像是生成的」。
func PortraitPrompt(c PersonaCandidate) string {
	base := c.Portrait
	if base == "" {
		base = c.Label
	}
	return base + "。扁平插画风格，柔和暖色，半身像，简洁背景，" +
		"不要任何文字，不要照片写实，不要具体的真实人物。"
}
