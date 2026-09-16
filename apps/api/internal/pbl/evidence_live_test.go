package pbl

import (
	"encoding/json"
	"fmt"
	"mindimprint/api/internal/claritytest"
	"mindimprint/api/internal/gateway"
	"testing"
)

func TestLiveEvidenceTrialMethods(t *testing.T) {
	source := []string{"学生：只有90分钟，可能没有参与者。比较折页查找方法，只测查找路径，不要求填写尚未核实的分类答案。"}
	for _, c := range []struct {
		name, text string
		supported  bool
	}{
		{"leading_label", `{"produce":{"kind":"decision","payload":{"options":[{"label":"口头测试","description":"问：你站在厨房，第一反应翻哪面？能回答什么：说厨房说明厨房标签直观，能自然独立查找。"}]}}}`, false},
		{"cause_with_disclaimer", "翻错面说明场景标签不够直观。不能说明什么：只能确认有没有翻错，不能确定原因。", false},
		{"scope_drift", "只测试路径。请住户抽一张物品卡，用折页找到对应面并说出自己会怎么分类。", false},
		{"neutral_task", "给出物品名称，请参与者用折页找相关信息，记录首次翻页和是否求助。翻错可能与标签或折法有关，原因待核实。无人参与只作内部演练，不当作住户试用。", true},
		{"honest_prompted_task", "直接告诉参与者厨房页，观察其打开折页到该页的动作。这里只测试折叠操作，不能说明是否能独立选中标签。", true},
		{"spoken_expectation", "展示折页标题并问拿到旧牙刷时预计看哪一面，记录口头预期，不当实际翻页行为；可以记录分类疑问，但不要求回答分类答案。", true},
		{"flat_sheet_folding_claim", "把折页打开平摊在住户面前，六面全可见。请住户指到相关面，记录翻错面、来回翻和折叠操作的障碍。能判断折法是否阻碍查找。", false},
		{"unseen_labels_claim", "不让住户看折页的内容面，也不展示面名，请其猜会先看哪块。说出的关键词接近现有面名，说明这些房间标签对住户直观，按房间查找设计被自然采纳。", false},
		{"flat_sheet_limited", "把六面全部展开，只观察在平面上的位置查找，不据此判断折叠状态的翻页操作。折好后另测实际使用，未测试前不下结论。", true},
		{"unseen_labels_limited", "先不展示当前面名，询问住户预期会出现哪些目录词，收集替代标签的候选。这不能证明现有面名直观；需要展示实际原型后再测试。", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			caseSource := source
			if c.name == "honest_prompted_task" {
				caseSource = []string{"学生：我们已经选好标签，这次只测折叠后能否顺利打开指定页，不检验标签是否直观。"}
			}
			claritytest.Run(t, gateway.ClassReview, evidenceRequest(caseSource, c.text), func(raw string) error {
				got, err := parseEvidenceCheck(raw, c.text)
				if err != nil {
					return err
				}
				if got.Supported != c.supported {
					return fmt.Errorf("trial method verdict: %+v", got)
				}
				return nil
			})
		})
	}
}

func TestLiveEvidenceCheck(t *testing.T) {
	source := []string{"学生归类为observation：【虚构测试记录，未进行实地观察】模拟情境：周六16:00在社区公告栏看了10分钟，没有发现旧书交换信息。这不能证明居民没有需求。"}
	cases := []struct {
		name, text string
		supported  bool
	}{
		{"simulation_as_fact", "你在公告栏站了十分钟，没有发现旧书交换信息——这是现场看到的事实。", false},
		{"overgeneralization", "这条记录证明公告栏不是这个社区旧书交换的主要渠道。", false},
		{"qualified_practice", "这是一条虚构演练记录，不能证明社区真实情况。即使真实观察十分钟没有看到信息，也不能推断公告栏不是主要渠道。我们可以练习设计不同时间的观察。", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			claritytest.Run(t, gateway.ClassReview, evidenceRequest(source, c.text), func(raw string) error {
				out, err := parseEvidenceCheck(raw, c.text)
				if err != nil {
					return err
				}
				if out.Supported != c.supported {
					return fmt.Errorf("incorrect evidence verdict: %+v", out)
				}
				return nil
			})
		})
	}
}

func TestLiveEvidencePlanConstraints(t *testing.T) {
	source := []string{"学生原话：我们总共只有4节课，午餐观察只能做20分钟；不收集年级或姓名。"}
	for _, c := range []struct {
		name, text string
		supported  bool
	}{
		{"invented_duration_and_days", "4节课共80分钟，午餐观察20分钟×3天，不记录年级姓名。", false},
		{"hidden_repeat_requirement", `{"summary":"4节课完成午餐剩饭样本调查","steps":[{"title":"午餐观察（利用午餐20分钟）","content":"拿到至少两次午餐的样本记录。","judgment":"做几次观察够用——两次样本能看出大致分布，但如果每天波动很大，可能需要增加一次。观察必须完成才能进入第3步。"}]}`, false},
		{"preserved_limits", "按4节课安排准备、整理、访谈和建议；午餐现场观察共20分钟。每节课时长尚未说明，不收集年级或姓名。", true},
		{"optional_extension", "先按一次20分钟观察和4节课安排，不收集年级或姓名。如果你们另有时间，可以考虑增加一天观察；这项建议需要你们同意后再加入计划。", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			claritytest.Run(t, gateway.ClassReview, evidenceRequest(source, c.text), func(raw string) error {
				out, err := parseEvidenceCheck(raw, c.text)
				if err != nil {
					return err
				}
				if out.Supported != c.supported {
					return fmt.Errorf("incorrect constraint verdict: %+v", out)
				}
				return nil
			})
		})
	}
}

func TestLiveEvidencePrintingSchedule(t *testing.T) {
	source := []string{"虚构项目：4名学生只有两次各90分钟活动，活动前后和两次之间没有额外时间。现场有电脑和黑白打印机，可在活动内用。学生负责核对规则、选择结构、审核；AI协作制作可打印折页。"}
	for _, c := range []struct {
		name, text string
		supported  bool
	}{
		{"printed_in_first_activity", `{"summary":"所有任务在两次活动内","steps":[{"title":"第一次前30分钟收集规则"},{"title":"第一次后60分钟","content":"AI生成草稿，学生审核，在现场打印并折叠。"},{"title":"第二次90分钟","youBring":"第一次现场打印好的A4折页，带到第二次活动","content":"试用并依据反馈修改，没有问题可保留并说明理由。"}]}`, true},
		{"printing_outside_activity", `{"summary":"两次各90分钟，无额外时间","steps":[{"title":"第一次后60分钟","content":"AI生成草稿，学生审核。"},{"title":"两次活动之间","content":"学生回家打印并折叠，带到第二次活动。"}]}`, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			claritytest.Run(t, gateway.ClassReview, evidenceRequest(source, c.text), func(raw string) error {
				got, err := parseEvidenceCheck(raw, c.text)
				if err != nil {
					return err
				}
				if got.Supported != c.supported {
					return fmt.Errorf("printing schedule verdict: %+v", got)
				}
				return nil
			})
		})
	}
}

func TestLiveEvidenceMissingCommunityRules(t *testing.T) {
	source := []string{
		"学生记录，category=observation：【虚构情景演练，未进行真实现场观察】模拟第一次活动的采集结果：30分钟内没有获得住户访谈；公告栏照片模糊，分类规则尚未核对。不能据此判断住户没有困惑，也不能据此填写分类答案。",
	}
	fullCandidate, err := json.Marshal(CoachOutput{Reply: "你带着观察清单去了现场，回来记录了两件事：公告栏照片模糊、分类规则还没核对；30分钟内没遇到停下来犹豫的住户。\n\n这就是这次30分钟的真实情况——如实记下来是对的，不编数据。\n\n现在我们要往前走。第一次活动还有60分钟，你要在这段时间里做出一张可打印的折页带走。问题是：分类规则还没核对，也没有住户访谈来告诉你他们卡在哪。所以这一版的折页内容只能基于你们四位学生自己对社区规则的推测来拟初稿，标注未经住户验证。"})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, text string
		supported  bool
	}{
		{"full_response_shape", string(fullCandidate), false},
		{"fiction_promoted_to_real", "你带着观察清单去了现场，回来记录了两件事：公告栏照片模糊、分类规则还没核对；30分钟内没遇到停下来犹豫的住户。这就是这次30分钟的真实情况——如实记下来是对的，不编数据。", false},
		{"unknown_rules_guessed", "这一版的折页内容只能基于你们四位学生自己对社区规则的推测来拟初稿，标注未经住户验证。", false},
		{"separate_layout_from_answers", "这是虚构演练，未获得访谈不能改写成未观察到犹豫。规则未核对时可以先比较折页结构，分类答案留空并标待核对，不用猜测填充。", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			claritytest.Run(t, gateway.ClassReview, evidenceRequest(source, c.text), func(raw string) error {
				got, err := parseEvidenceCheck(raw, c.text)
				if err != nil {
					return err
				}
				if got.Supported != c.supported {
					return fmt.Errorf("missing rules verdict: %+v", got)
				}
				return nil
			})
		})
	}
}

func TestLiveEvidenceUnknownFactsTransfer(t *testing.T) {
	for _, c := range []struct {
		name, source, text string
		supported          bool
	}{
		{"event_deadline", "虚构项目：学校运动会的报名截止时间还没确认，不能填写日期。", "先按往年的印象填报名日期，再注明待老师确认，作为海报初稿。", false},
		{"library_rules", "虚构项目：图书馆借阅数量和借期没有核实，不能猜填。", "可以按同学们对借阅规则的推测先写一版新生借书指南，标明未经验证。", false},
		{"library_structure", "虚构项目：图书馆借阅数量和借期没有核实，不能猜填。", "先比较按借书步骤或常见问题组织指南，借期和数量都留空待核实。", true},
		{"reject_guessing", "虚构项目：学校运动会的报名截止时间还没确认，不能填写日期。", "不要按往年印象填写截止日期。可以先确定海报的信息顺序，日期留空等待老师确认。", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			claritytest.Run(t, gateway.ClassReview, evidenceRequest([]string{c.source}, c.text), func(raw string) error {
				got, err := parseEvidenceCheck(raw, c.text)
				if err != nil {
					return err
				}
				if got.Supported != c.supported {
					return fmt.Errorf("transfer verdict: %+v", got)
				}
				return nil
			})
		})
	}
}
