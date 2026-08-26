package evalbench

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"mindimprint/api/internal/evalreport"
	"mindimprint/api/internal/rubric"
)

// renderMarkdownReport renders the compact, human-facing report.md deliverable.
// It only reads in-memory experiment state: rendering neither changes experiment
// results nor performs model calls.
func renderMarkdownReport(c Config, manifest Manifest, summary Summary, states map[string][]*variantState) string {
	var b strings.Builder
	b.WriteString("# EvalBench 实验结果摘要\n\n")
	b.WriteString("> 面向内部研发与产品。本摘要只呈现实验观测与统计口径，不提供方案推荐或行动指引。详细审计内容见 [report-details.md](report-details.md)。\n\n")

	renderCompactExperimentScope(&b, c, manifest)
	renderCoreComparison(&b, summary)
	renderCompactAlignment(&b, c, summary, states)
	renderCompactPerformanceAndCost(&b, summary)
	renderCompactReliability(&b, c, summary, states)
	renderArtifactIndex(&b)
	return b.String()
}

// renderDetailedMarkdownReport renders the complete report-details.md audit
// attachment. Candidate text is preserved here, including any original
// suggestions, but it is not an EvalBench recommendation.
func renderDetailedMarkdownReport(c Config, manifest Manifest, summary Summary, states map[string][]*variantState) string {
	var b strings.Builder
	b.WriteString("# EvalBench 实验详细报告\n\n")
	b.WriteString("> 面向内部研发与产品。此附件汇总离线实验结果；学生过程原话和模型输出仅应在受控环境中查看。Candidate 中出现的建议或下一步属于模型原始输出，并非 EvalBench 对评估方案的建议。\n\n")
	b.WriteString("[返回实验结果摘要](report.md)\n\n")

	renderExperimentOverview(&b, c, manifest)
	renderVariantOverview(&b, summary)
	renderVariantCostComparison(&b, summary)
	renderComparisonOverview(&b, summary)

	b.WriteString("## Case 详情\n\n")
	for _, cs := range c.Cases {
		b.WriteString("### " + mdInline(cs.ID) + "\n\n")
		for _, v := range c.Variants {
			state := stateForVariant(states[cs.ID], v.ID)
			if state == nil {
				fmt.Fprintf(&b, "#### %s\n\n未找到该 variant 的运行状态。\n\n", mdInline(v.ID))
				continue
			}
			renderCaseVariant(&b, cs.ID, v, state, manifest.Pricing)
		}
	}

	renderReproducibility(&b, c, manifest)
	return b.String()
}

func renderCompactExperimentScope(b *strings.Builder, c Config, manifest Manifest) {
	b.WriteString("## 实验范围\n\n")
	b.WriteString("| 项目 | 值 |\n| --- | --- |\n")
	row(b, "实验名称", c.Name)
	row(b, "实验 ID", manifest.ExperimentID)
	row(b, "运行状态", manifest.Status)
	row(b, "开始 / 完成（UTC）", formatTime(manifest.StartedAt)+" / "+formatTimePointer(manifest.CompletedAt))
	row(b, "Case / Variant", fmt.Sprintf("%d / %d", len(c.Cases), len(c.Variants)))
	row(b, "每组合目标完整成功次数", fmt.Sprintf("%d", c.SuccessfulRuns))
	row(b, "Candidate 模型", compactCandidateModels(c, manifest))
	row(b, "Comparator", compactComparator(c, manifest))
	b.WriteString("\n")
}

func compactCandidateModels(c Config, manifest Manifest) string {
	parts := make([]string, 0, len(c.Variants))
	for _, v := range c.Variants {
		model, ok := manifest.Models[v.Model]
		if !ok {
			parts = append(parts, v.ID+"：未提供")
			continue
		}
		parts = append(parts, v.ID+"："+model.Provider+"/"+model.Model)
	}
	if len(parts) == 0 {
		return "未提供"
	}
	return strings.Join(parts, "；")
}

func compactComparator(c Config, manifest Manifest) string {
	model, ok := manifest.Models[c.Comparator.Model]
	if !ok {
		return "未提供"
	}
	parts := []string{model.Provider + "/" + model.Model}
	if c.Comparator.PromptVersion != "" {
		parts = append(parts, "prompt="+c.Comparator.PromptVersion)
	}
	return strings.Join(parts, "；")
}

func renderCoreComparison(b *strings.Builder, summary Summary) {
	b.WriteString("## 核心比较\n\n")
	b.WriteString("对齐率为所有已有 comparator 结果中，`aligned / 可比较项` 的比例；不表示绝对正确率。耗时只统计完整成功 run；成本包含成功、失败和重试调用。\n\n")
	b.WriteString("| Variant | 可比对齐率 | 对齐 / 可比较项 | 过度 / 不足 | 不可比较 / 人工复核 | 完整成功 | 端到端中位耗时 | Candidate / 完整成功 | 总成本 / 完整成功 |\n| --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, v := range summary.Variants {
		comparison := aggregateComparisonTotals(v.Comparison)
		fmt.Fprintf(b, "| %s | %s | %d / %d | %d / %d | %d / %d | %d/%d（%s） | %s | %s | %s |\n",
			mdInline(v.ID), rateText(comparison.AlignedRate), comparison.Aligned, comparison.Comparable,
			comparison.Overstates, comparison.Understates, comparison.NotComparable, comparison.ManualReview,
			v.SuccessfulRuns, v.Attempts, percent(v.SuccessRate), medianDurationText(v.WallMs),
			costPerSuccessText(v.CandidateCostPerSuccessUSD), costPerSuccessText(v.TotalCostPerSuccessUSD))
	}
	b.WriteString("\n")
}

func renderCompactAlignment(b *strings.Builder, c Config, summary Summary, states map[string][]*variantState) {
	b.WriteString("## 人工 Gold 对齐与错误画像\n\n")
	b.WriteString("比较对象为模型生成的判断、证据、综述、提问透镜和风险；事实字段不参与 comparator。`overstates` 与 `understates` 分别表示相对 Gold 的过度判断和判断不足。\n\n")
	b.WriteString("| Variant | 区域 | 项数 | 对齐 | 过度 | 不足 | 不可比较 | 人工复核 | 可比对齐率 |\n| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |\n")
	for _, v := range summary.Variants {
		rows := aggregateComparisonByArea(v.Comparison)
		if len(rows) == 0 {
			fmt.Fprintf(b, "| %s | 未提供 | 0 | 0 | 0 | 0 | 0 | 0 | 未提供 |\n", mdInline(v.ID))
			continue
		}
		for _, r := range rows {
			fmt.Fprintf(b, "| %s | %s | %d | %d | %d | %d | %d | %d | %s |\n", mdInline(v.ID), comparisonAreaName(r.Area), r.Total, r.Aligned, r.Overstates, r.Understates, r.NotComparable, r.ManualReview, rateText(r.AlignedRate))
		}
	}
	b.WriteString("\n")
	if len(c.Cases) <= 1 {
		return
	}
	b.WriteString("### Case 覆盖\n\n")
	b.WriteString("| Case | Variant | 完整成功 | Comparator 结果 | 对齐 / 可比较项 | 可比对齐率 |\n| --- | --- | --- | ---: | --- | --- |\n")
	for _, cs := range c.Cases {
		for _, v := range c.Variants {
			state := stateForVariant(states[cs.ID], v.ID)
			if state == nil {
				fmt.Fprintf(b, "| %s | %s | 0/0（0.0%%） | 0 | 0 / 0 | 未提供 |\n", mdInline(cs.ID), mdInline(v.ID))
				continue
			}
			attempts := summarizeAttempts(state.attempts)
			comparison := aggregateComparisonTotals(comparisonSummary(state.attempts))
			fmt.Fprintf(b, "| %s | %s | %d/%d（%s） | %d | %d / %d | %s |\n", mdInline(cs.ID), mdInline(v.ID), attempts.successful, attempts.attempts, percent(attemptRate(attempts.successful, attempts.attempts)), attempts.comparisons, comparison.Aligned, comparison.Comparable, rateText(comparison.AlignedRate))
		}
	}
	b.WriteString("\n")
}

func renderCompactPerformanceAndCost(b *strings.Builder, summary Summary) {
	b.WriteString("## 性能与成本\n\n")
	b.WriteString("金额为 USD 估算，使用运行开始时的价格快照；输出 token 包含 reasoning token。成本覆盖不完整时只展示已知小计，不计算每成功 run 成本。\n\n")
	b.WriteString("| Variant | Candidate / Comparator 调用 | Candidate / Comparator token | Candidate TTFT 中位数 | Candidate 支出 | Comparator 支出 | 实验总支出 | 成本覆盖 |\n| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, v := range summary.Variants {
		total := combineCallTotals(v.Candidate, v.Comparator)
		fmt.Fprintf(b, "| %s | %d / %d | %s / %s | %s | %s | %s | %s | %s |\n",
			mdInline(v.ID), v.Candidate.Calls, v.Comparator.Calls, callTokens(v.Candidate), callTokens(v.Comparator), medianDurationText(v.TTFTMs),
			costText(v.Candidate), costText(v.Comparator), costText(total), costCoverageText(total))
	}
	b.WriteString("\n")
}

func renderCompactReliability(b *strings.Builder, c Config, summary Summary, states map[string][]*variantState) {
	b.WriteString("## 可靠性与解释边界\n\n")
	b.WriteString("| Variant | Attempt / 完整成功 / Comparator 结果 | 非成功 attempt 分类 | 异常流 | usage 缺失 | 未定价调用 | Comparator 稳定性 |\n| --- | --- | --- | ---: | ---: | ---: | --- |\n")
	for _, v := range summary.Variants {
		stats := summarizeVariantAttempts(states, v.ID)
		incomplete := v.Candidate.IncompleteStreams + v.Comparator.IncompleteStreams
		usageMissing := v.Candidate.UsageMissing + v.Comparator.UsageMissing
		unpriced := v.Candidate.UnpricedCalls + v.Comparator.UnpricedCalls
		fmt.Fprintf(b, "| %s | %d / %d / %d | %s | %d | %d | %d | %s |\n", mdInline(v.ID), stats.attempts, stats.successful, stats.comparisons, mdInline(errorDistribution(stats.errors)), incomplete, usageMissing, unpriced, compactStability(states, c, v.ID))
	}
	b.WriteString("\n")
	b.WriteString("- 样本量为 " + fmt.Sprintf("%d 个 case，每个组合目标 %d 个完整成功 run。", len(c.Cases), c.SuccessfulRuns) + "\n")
	b.WriteString("- 对齐结果依赖本次人工 Gold 与 comparator；`not_comparable` 和人工复核项不计入可比对齐率。\n")
	b.WriteString("- 本摘要不生成学生总分；智识自主在详细附件中仅作为 0–5 行为计数带展示。\n\n")
}

func renderArtifactIndex(b *strings.Builder) {
	b.WriteString("## 产物索引\n\n")
	b.WriteString("- [详细报告](report-details.md)：代表 run、Candidate 原始输出、attempt 诊断、46 项矩阵、稳定性与可复现信息。\n")
	b.WriteString("- [summary.json](summary.json)：机器可读 variant 汇总。\n")
	b.WriteString("- [manifest.json](manifest.json)：配置、版本、价格快照、输入/Gold hash 与执行顺序。\n\n")
}

type comparisonTotals struct {
	Total, Comparable, Aligned, Overstates, Understates, NotComparable, ManualReview int
	AlignedRate                                                                      *float64
}

func aggregateComparisonTotals(rows []ComparisonSummary) comparisonTotals {
	var total comparisonTotals
	for _, row := range rows {
		total.Total += row.Total
		total.Comparable += row.Comparable
		total.Aligned += row.Aligned
		total.Overstates += row.Overstates
		total.Understates += row.Understates
		total.NotComparable += row.NotComparable
		total.ManualReview += row.ManualReview
	}
	if total.Comparable > 0 {
		rate := float64(total.Aligned) / float64(total.Comparable)
		total.AlignedRate = &rate
	}
	return total
}

type attemptSummary struct {
	attempts, successful, comparisons int
	errors                            map[string]int
}

func summarizeAttempts(attempts []attemptResult) attemptSummary {
	out := attemptSummary{attempts: len(attempts), errors: map[string]int{}}
	for _, attempt := range attempts {
		if attempt.complete && attempt.report != nil && attempt.comparison != nil {
			out.successful++
		}
		if attempt.comparison != nil {
			out.comparisons++
		}
		if attempt.status.Status != "success" {
			out.errors[errorClass(attempt.status.Error)]++
		}
	}
	return out
}

func summarizeVariantAttempts(states map[string][]*variantState, variantID string) attemptSummary {
	out := attemptSummary{errors: map[string]int{}}
	for _, caseStates := range states {
		state := stateForVariant(caseStates, variantID)
		if state == nil {
			continue
		}
		part := summarizeAttempts(state.attempts)
		out.attempts += part.attempts
		out.successful += part.successful
		out.comparisons += part.comparisons
		for class, count := range part.errors {
			out.errors[class] += count
		}
	}
	return out
}

func attemptRate(successful, attempts int) float64 {
	if attempts == 0 {
		return 0
	}
	return float64(successful) / float64(attempts)
}

func errorDistribution(errors map[string]int) string {
	if len(errors) == 0 {
		return "无"
	}
	keys := make([]string, 0, len(errors))
	for class := range errors {
		keys = append(keys, class)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, class := range keys {
		parts = append(parts, fmt.Sprintf("%s × %d", class, errors[class]))
	}
	return strings.Join(parts, "；")
}

func compactStability(states map[string][]*variantState, c Config, variantID string) string {
	var rates []float64
	for _, cs := range c.Cases {
		state := stateForVariant(states[cs.ID], variantID)
		if state == nil {
			continue
		}
		successful := successfulAttemptReports(state.attempts)
		if len(successful) < 2 {
			continue
		}
		attempts := make([]attemptResult, 0, len(successful))
		for _, attempt := range successful {
			attempts = append(attempts, *attempt)
		}
		for _, item := range buildStability(attempts) {
			if item.AgreementRate != nil && item.Runs >= 2 {
				rates = append(rates, *item.AgreementRate)
			}
		}
	}
	if len(rates) == 0 {
		return "不可评估（每个 case 少于 2 个完整成功 run）"
	}
	sort.Float64s(rates)
	mean := 0.0
	for _, rate := range rates {
		mean += rate
	}
	mean /= float64(len(rates))
	return fmt.Sprintf("%d 项；平均 %s；最低 %s", len(rates), percent(mean), percent(rates[0]))
}

func medianDurationText(d Distribution) string {
	if d.Median == nil {
		return "未提供"
	}
	return durationText(int64(*d.Median))
}

func formatTimePointer(value *time.Time) string {
	if value == nil {
		return "未提供"
	}
	return formatTime(*value)
}

func renderExperimentOverview(b *strings.Builder, c Config, manifest Manifest) {
	b.WriteString("## 实验概览\n\n")
	b.WriteString("| 项目 | 值 |\n| --- | --- |\n")
	row(b, "实验名称", c.Name)
	row(b, "实验 ID", manifest.ExperimentID)
	row(b, "运行状态", manifest.Status)
	row(b, "开始时间（UTC）", formatTime(manifest.StartedAt))
	if manifest.CompletedAt == nil {
		row(b, "完成时间（UTC）", "未提供")
	} else {
		row(b, "完成时间（UTC）", formatTime(*manifest.CompletedAt))
	}
	row(b, "Case 数", fmt.Sprintf("%d", len(c.Cases)))
	row(b, "每个组合目标完整成功次数", fmt.Sprintf("%d", c.SuccessfulRuns))
	row(b, "每个组合最大 attempt 数", fmt.Sprintf("%d", c.MaxAttempts))
	row(b, "Git commit", valueOrMissing(manifest.GitCommit))
	row(b, "工作区", map[bool]string{true: "dirty", false: "clean"}[manifest.Dirty])
	b.WriteString("\n### 模型与评估器\n\n")
	b.WriteString("| 类型 | ID | Provider / Model / 版本 |\n| --- | --- | --- |\n")
	modelIDs := make([]string, 0, len(manifest.Models))
	for id := range manifest.Models {
		modelIDs = append(modelIDs, id)
	}
	sort.Strings(modelIDs)
	for _, id := range modelIDs {
		m := manifest.Models[id]
		row3(b, "模型", id, m.Provider+" / "+m.Model)
	}
	for _, v := range c.Variants {
		d := manifest.Evaluators[v.ID]
		parts := []string{valueOrMissing(d.ImplementationVersion)}
		if d.PromptVersion != "" {
			parts = append(parts, "prompt="+d.PromptVersion)
		}
		if d.PromptSHA256 != "" {
			parts = append(parts, "prompt sha256="+d.PromptSHA256)
		}
		row3(b, "评估器", v.ID, strings.Join(parts, "；"))
	}
	row3(b, "Comparator", c.Comparator.Model, "prompt="+c.Comparator.PromptVersion)
	versionIDs := make([]string, 0, len(manifest.Versions))
	for id := range manifest.Versions {
		versionIDs = append(versionIDs, id)
	}
	sort.Strings(versionIDs)
	for _, id := range versionIDs {
		row3(b, "运行组件", id, manifest.Versions[id])
	}
	b.WriteString("\n")
	renderPricingSnapshot(b, manifest.Pricing)
}

func renderPricingSnapshot(b *strings.Builder, pricing map[string]PriceSnapshot) {
	b.WriteString("### 价格快照与估算口径\n\n")
	b.WriteString("成本为 USD 估算，按运行开始时共享 gateway 价格表的保守 cache-miss 费率计算；不含折扣、税费或汇率。输出 token 包含 provider 报告的 reasoning token。\n\n")
	if len(pricing) == 0 {
		b.WriteString("未提供价格快照。\n\n")
		return
	}
	b.WriteString("| Provider / Model | 定价状态 | 输入 USD / 1M token | 输出 USD / 1M token |\n| --- | --- | ---: | ---: |\n")
	keys := make([]string, 0, len(pricing))
	for key := range pricing {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		price := pricing[key]
		if !price.Priced {
			fmt.Fprintf(b, "| %s | 未定价 | 未提供 | 未提供 |\n", mdInline(key))
			continue
		}
		fmt.Fprintf(b, "| %s | 已定价（%s） | $%.6f | $%.6f |\n", mdInline(key), mdInline(price.Currency), price.InputPerMillionUSD, price.OutputPerMillionUSD)
	}
	b.WriteString("\n")
}

func renderVariantOverview(b *strings.Builder, summary Summary) {
	b.WriteString("## Variant 运行概览\n\n")
	b.WriteString("| Variant | 状态 | 完整成功 | Candidate 调用 | Comparator 调用 | Candidate token | Comparator token | 端到端耗时 | TTFT | 异常流 / usage 缺失 |\n")
	b.WriteString("| --- | --- | --- | ---: | ---: | --- | --- | --- | --- | --- |\n")
	for _, v := range summary.Variants {
		issues := fmt.Sprintf("%d / %d", v.Candidate.IncompleteStreams+v.Comparator.IncompleteStreams, v.Candidate.UsageMissing+v.Comparator.UsageMissing)
		fmt.Fprintf(b, "| %s | %s | %d/%d（%s） | %d | %d | %s | %s | %s | %s | %s |\n",
			mdInline(v.ID), mdInline(v.Status), v.SuccessfulRuns, v.Attempts, percent(v.SuccessRate),
			v.Candidate.Calls, v.Comparator.Calls, callTokens(v.Candidate), callTokens(v.Comparator),
			distributionText(v.WallMs), distributionText(v.TTFTMs), mdInline(issues))
	}
	b.WriteString("\n")
}

func renderVariantCostComparison(b *strings.Builder, summary Summary) {
	b.WriteString("## Variant 成本比较\n\n")
	b.WriteString("实际实验支出包含所有成功、失败和重试 attempt。Candidate/成功 run 衡量评估器运行效率；总成本/成功 run 另包含 comparator，反映实验预算。\n\n")
	b.WriteString("| Variant | Candidate 支出 | Comparator 支出 | 实验总支出 | Candidate / 完整成功 | 总成本 / 完整成功 | 成本覆盖 |\n| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, v := range summary.Variants {
		total := combineCallTotals(v.Candidate, v.Comparator)
		fmt.Fprintf(b, "| %s | %s | %s | %s | %s | %s | %s |\n",
			mdInline(v.ID), costText(v.Candidate), costText(v.Comparator), costText(total),
			costPerSuccessText(v.CandidateCostPerSuccessUSD), costPerSuccessText(v.TotalCostPerSuccessUSD), costCoverageText(total))
	}
	b.WriteString("\n")
}

func renderComparisonOverview(b *strings.Builder, summary Summary) {
	b.WriteString("## 与人工 Gold 的对齐概览\n\n")
	b.WriteString("仅比较模型生成的判断、证据、建议、综述、提问透镜和风险；事实字段不交由 comparator 评判。`aligned` 仅表示与本次人工 Gold 对齐，不代表绝对正确。\n\n")
	for _, v := range summary.Variants {
		b.WriteString("### " + mdInline(v.ID) + "\n\n")
		rows := aggregateComparisonByArea(v.Comparison)
		if len(rows) == 0 {
			b.WriteString("尚无成功的 comparator 结果。\n\n")
			continue
		}
		b.WriteString("| 区域 | 项数 | 对齐 | 过度判断 | 判断不足 | 不可比较 | 人工复核 | 对齐率 |\n| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |\n")
		for _, r := range rows {
			fmt.Fprintf(b, "| %s | %d | %d | %d | %d | %d | %d | %s |\n", comparisonAreaName(r.Area), r.Total, r.Aligned, r.Overstates, r.Understates, r.NotComparable, r.ManualReview, rateText(r.AlignedRate))
		}
		b.WriteString("\n")
	}
}

func renderCaseVariant(b *strings.Builder, caseID string, v VariantConfig, state *variantState, pricing map[string]PriceSnapshot) {
	b.WriteString("#### " + mdInline(v.ID) + "\n\n")
	b.WriteString("| Attempt | 状态 | 耗时 | 调用数 | 内部重试 | Candidate 成本 | Comparator 成本 | Comparison | 错误分类 | 详情 |\n| ---: | --- | --- | ---: | ---: | --- | --- | --- | --- | --- |\n")
	attempts := append([]attemptResult(nil), state.attempts...)
	sort.SliceStable(attempts, func(i, j int) bool { return attempts[i].status.Attempt < attempts[j].status.Attempt })
	for _, a := range attempts {
		candidateCost := addCalls(CallTotals{}, a.candidate, pricing)
		comparatorCost := addCalls(CallTotals{}, a.comparator, pricing)
		fmt.Fprintf(b, "| %d | %s | %s | %d | %d | %s | %s | %s | %s | %s |\n",
			a.status.Attempt, mdInline(a.status.Status), durationText(a.status.WallMs), a.status.CallCount, a.status.InternalRetry,
			mdTableCell(costText(candidateCost)), mdTableCell(costText(comparatorCost)), mdTableCell(valueOrMissing(a.status.Comparison)), mdInline(errorClass(a.status.Error)), mdTableCell(valueOrMissing(a.status.Error)))
	}
	if len(state.attempts) == 0 {
		b.WriteString("| — | 未执行 | 未提供 | 0 | 0 | $0.000000 | $0.000000 | 未提供 | 未提供 | 未提供 |\n")
	}
	b.WriteString("\n")
	renderCallCostBreakdown(b, state.attempts, pricing)

	representative, diagnostic := representativeAttempt(state.attempts)
	if representative == nil {
		b.WriteString("没有可展示的 candidate report；请查看上表错误和 `evaluator-calls/` 原始记录。\n\n")
		return
	}
	if diagnostic {
		b.WriteString("> **诊断预览**：该 report 未满足完整成功条件，不计入成功率；仅用于定位候选输出或 comparator 问题。\n\n")
	} else {
		fmt.Fprintf(b, "> **代表 run**：attempt-%03d（按 attempt 编号选择第一个完整成功 run，避免选择性挑选）。\n\n", representative.status.Attempt)
	}
	renderCandidateReport(b, *representative.report)
	renderComparatorDetails(b, *representative, caseID, v.ID)
	renderStability(b, state.attempts)
}

type callCostGroup struct {
	scope    string
	purpose  string
	provider string
	model    string
	calls    []CallRecord
}

func renderCallCostBreakdown(b *strings.Builder, attempts []attemptResult, pricing map[string]PriceSnapshot) {
	b.WriteString("##### 调用成本明细\n\n")
	groups := map[string]*callCostGroup{}
	add := func(scope string, calls []CallRecord) {
		for _, call := range calls {
			key := strings.Join([]string{scope, call.Purpose, call.Provider, call.Model}, "\x00")
			group := groups[key]
			if group == nil {
				group = &callCostGroup{scope: scope, purpose: call.Purpose, provider: call.Provider, model: call.Model}
				groups[key] = group
			}
			group.calls = append(group.calls, call)
		}
	}
	for _, attempt := range attempts {
		add("Candidate", attempt.candidate)
		add("Comparator", attempt.comparator)
	}
	if len(groups) == 0 {
		b.WriteString("未发起模型调用；已知成本为 $0.000000。\n\n")
		return
	}
	ordered := make([]*callCostGroup, 0, len(groups))
	for _, group := range groups {
		ordered = append(ordered, group)
	}
	sort.Slice(ordered, func(i, j int) bool {
		left := strings.Join([]string{ordered[i].scope, ordered[i].purpose, ordered[i].provider, ordered[i].model}, "\x00")
		right := strings.Join([]string{ordered[j].scope, ordered[j].purpose, ordered[j].provider, ordered[j].model}, "\x00")
		return left < right
	})
	b.WriteString("| Scope | Purpose | Provider / Model | 调用 | Token | 成本估算 | 覆盖 |\n| --- | --- | --- | ---: | --- | --- | --- |\n")
	for _, group := range ordered {
		totals := addCalls(CallTotals{}, group.calls, pricing)
		fmt.Fprintf(b, "| %s | %s | %s / %s | %d | %s | %s | %s |\n", mdInline(group.scope), mdInline(valueOrMissing(group.purpose)), mdInline(valueOrMissing(group.provider)), mdInline(valueOrMissing(group.model)), totals.Calls, callTokens(totals), costText(totals), costCoverageText(totals))
	}
	b.WriteString("\n")
}

func representativeAttempt(attempts []attemptResult) (*attemptResult, bool) {
	var successful *attemptResult
	for i := range attempts {
		a := &attempts[i]
		if a.complete && a.report != nil && a.comparison != nil && (successful == nil || a.status.Attempt < successful.status.Attempt) {
			successful = a
		}
	}
	if successful != nil {
		return successful, false
	}
	var diagnostic *attemptResult
	for i := range attempts {
		a := &attempts[i]
		if a.report != nil && (diagnostic == nil || a.status.Attempt < diagnostic.status.Attempt) {
			diagnostic = a
		}
	}
	return diagnostic, diagnostic != nil
}

func renderCandidateReport(b *strings.Builder, r evalreport.Report) {
	b.WriteString("##### Candidate EvaluationReport\n\n")
	b.WriteString("| 字段 | 值 |\n| --- | --- |\n")
	row(b, "Schema 版本", fmt.Sprintf("%d", r.Version))
	row(b, "Report ID", r.ReportID)
	row(b, "Project ID", r.ProjectID)
	row(b, "题目", r.Basics.Title)
	row(b, "任务类型", r.Basics.Type)
	row(b, "学生", valueOrMissing(r.Student.Name)+"（"+valueOrMissing(r.Student.ID)+"）")
	row(b, "生成时间（UTC）", formatISOTime(r.GeneratedAt))
	row(b, "开始 / 结束（UTC）", formatISOTime(r.Basics.StartDate)+" / "+formatISOTimePointer(r.Basics.EndDate))
	row(b, "AI 轮次 / 阅读材料 / 写作词数 / AI 评论 / 编辑", fmt.Sprintf("%d / %d / %d / %d / %d", r.Basics.Counters.AITurns, r.Basics.Counters.MaterialsRead, r.Basics.Counters.WordsWritten, r.Basics.Counters.AICommentCount, r.Basics.Counters.EditCount))
	row(b, "里程碑：开始", formatISOTimePointer(r.Basics.Milestones.Started))
	row(b, "里程碑：框架完成", formatISOTimePointer(r.Basics.Milestones.FrameworkFinished))
	row(b, "里程碑：提案完成", formatISOTimePointer(r.Basics.Milestones.ProposalFinished))
	row(b, "里程碑：写作完成", formatISOTimePointer(r.Basics.Milestones.WritingFinished))
	row(b, "里程碑：项目完成", formatISOTimePointer(r.Basics.Milestones.ProjectFinished))
	b.WriteString("\n###### 综述与建议\n\n")
	renderLabeledParagraph(b, "全过程综述", r.Abstract.Overview)
	renderLabeledParagraph(b, "材料", r.Abstract.MaterialSentence)
	renderLabeledParagraph(b, "写作", r.Abstract.WritingSentence)
	renderLabeledParagraph(b, "AI 边界", r.Abstract.AISentence)
	renderLabeledParagraph(b, "建议", r.Abstract.SuggestionParagraph)
	if len(r.Abstract.SuggestionSentences) > 0 {
		b.WriteString("**下一步**\n\n")
		for _, s := range r.Abstract.SuggestionSentences {
			b.WriteString("- " + mdParagraph(s) + "\n")
		}
		b.WriteString("\n")
	}
	if len(r.Abstract.RecommendedCourses) > 0 {
		b.WriteString("**推荐课程**\n\n| 课程 | 原因 |\n| --- | --- |\n")
		for _, course := range r.Abstract.RecommendedCourses {
			row(b, course.CourseID, course.Reason)
		}
		b.WriteString("\n")
	}

	renderEvents(b, r.Events)
	renderMaterials(b, r.Materials)
	renderAxes(b, r)
	renderPromptLens(b, r.PromptLens)
	renderToolUsage(b, r.ToolUsage)
	renderRisks(b, r.Risks)
}

func renderLabeledParagraph(b *strings.Builder, label, value string) {
	b.WriteString("**" + mdInline(label) + "**\n\n")
	b.WriteString(mdParagraph(value) + "\n\n")
}

func renderEvents(b *strings.Builder, events []evalreport.EventEntry) {
	b.WriteString("###### 过程事件\n\n")
	if len(events) == 0 {
		b.WriteString("未提供。\n\n")
		return
	}
	b.WriteString("| 时间（UTC） | 类型 | AI 轮次 | 摘要 | 引用 |\n| --- | --- | ---: | --- | --- |\n")
	for _, e := range events {
		ref := "未提供"
		if e.Ref != nil {
			ref = refText(*e.Ref)
		}
		fmt.Fprintf(b, "| %s | %s | %d | %s | %s |\n", mdInline(formatISOTime(e.TS)), mdInline(e.Kind), e.AITurns, mdTableCell(e.Summary), mdTableCell(ref))
	}
	b.WriteString("\n")
}

func renderMaterials(b *strings.Builder, materials []evalreport.MaterialEntry) {
	b.WriteString("###### 材料与来源状态\n\n")
	if len(materials) == 0 {
		b.WriteString("未提供。\n\n")
		return
	}
	b.WriteString("| 材料 ID | 添加时间（UTC） | 来源 | URL | 使用位置 | 最终状态 | 备注 / 可支持内容 | 不可支持内容 |\n| --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, m := range materials {
		url, usedIn := "未提供", "未提供"
		if m.URL != nil {
			url = pointerValue(m.URL)
		}
		if m.UsedIn != nil {
			usedIn = refText(*m.UsedIn)
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s | %s | %s | %s | %s |\n", mdInline(m.MaterialID), mdInline(formatISOTime(m.AddedAt)), mdTableCell(m.Source), mdTableCell(url), mdTableCell(usedIn), mdInline(m.FinalStatus), mdTableCell(m.Comment), mdTableCell(m.CannotSupport))
	}
	b.WriteString("\n")
}

func renderAxes(b *strings.Builder, r evalreport.Report) {
	b.WriteString("###### 认知深度（不合成总分）\n\n")
	depthNames := map[string]string{}
	for _, d := range rubric.DepthDims() {
		depthNames[d.ID] = d.Name
	}
	for _, d := range r.Depth {
		fmt.Fprintf(b, "**%s · %s — L%d**\n\n", mdInline(d.ID), mdInline(depthNames[d.ID]), d.Level)
		b.WriteString(mdParagraph(d.Summary) + "\n\n")
		renderEvidence(b, d.Evidence)
		renderSuggestion(b, d.Suggestion)
	}

	b.WriteString("###### 智识自主（0–5 行为计数带，不是质量总分）\n\n")
	autonomyNames := map[string]string{}
	for _, a := range rubric.AutonomySignals() {
		autonomyNames[a.ID] = a.Name
	}
	for _, a := range r.Autonomy {
		fmt.Fprintf(b, "**%s · %s — %d/5**\n\n", mdInline(a.ID), mdInline(autonomyNames[a.ID]), a.Band)
		b.WriteString(mdParagraph(a.Summary) + "\n\n")
		renderEvidence(b, a.Evidence)
		renderSuggestion(b, a.Suggestion)
	}
}

func renderEvidence(b *strings.Builder, evidence []evalreport.EvidenceItem) {
	if len(evidence) == 0 {
		b.WriteString("证据：未提供。\n\n")
		return
	}
	for i, e := range evidence {
		fmt.Fprintf(b, "证据 %d（%s）\n\n", i+1, mdInline(valueOrMissing(e.ID)))
		b.WriteString("时间：" + mdInline(formatISOTime(e.TS)) + "；阶段：" + mdInline(valueOrMissing(e.Stage)) + "\n\n")
		b.WriteString(mdQuote(e.Quote) + "\n\n")
		b.WriteString("观察：" + mdParagraph(e.Observation) + "\n\n")
		if strings.TrimSpace(e.Boundary) != "" {
			b.WriteString("边界：" + mdParagraph(e.Boundary) + "\n\n")
		}
	}
}

func renderSuggestion(b *strings.Builder, suggestion string) {
	b.WriteString("建议：" + mdParagraph(suggestion) + "\n\n")
}

func renderPromptLens(b *strings.Builder, lens evalreport.PromptLens) {
	b.WriteString("###### 提问透镜\n\n")
	b.WriteString(mdParagraph(lens.Summary) + "\n\n")
	if len(lens.Prompts) == 0 {
		b.WriteString("代表性提示词：未提供。\n\n")
		return
	}
	for i, p := range lens.Prompts {
		attention := "否"
		if p.Attention {
			attention = "是"
		}
		fmt.Fprintf(b, "**提示词 %d · %s · 需关注：%s**\n\n", i+1, mdInline(p.Stage), attention)
		b.WriteString(mdQuote(p.Quote) + "\n\n")
		b.WriteString("观察：" + mdParagraph(p.Observation) + "\n\n")
		b.WriteString("关联领域：" + mdInline(strings.Join(p.RelatedDomains, "、")) + "；引用：" + mdInline(refText(p.Ref)) + "\n\n")
	}
}

func renderToolUsage(b *strings.Builder, tools []evalreport.ToolUsageEntry) {
	b.WriteString("###### 工具使用\n\n")
	if len(tools) == 0 {
		b.WriteString("未提供。\n\n")
		return
	}
	b.WriteString("| 工具 | 阶段 | 目的 | 摘要 |\n| --- | --- | --- | --- |\n")
	for _, t := range tools {
		fmt.Fprintf(b, "| %s（%s） | %s | %s | %s |\n", mdInline(t.Name), mdInline(t.ToolID), mdInline(t.Stage), mdTableCell(t.Purpose), mdTableCell(t.Summary))
	}
	b.WriteString("\n")
}

func renderRisks(b *strings.Builder, risks []evalreport.RiskEntry) {
	b.WriteString("###### 风险\n\n")
	if len(risks) == 0 {
		b.WriteString("未识别到风险。\n\n")
		return
	}
	for i, r := range risks {
		ref := "未提供"
		if r.Ref != nil {
			ref = refText(*r.Ref)
		}
		fmt.Fprintf(b, "**风险 %d · %s**\n\n", i+1, mdInline(r.Type))
		b.WriteString("行为：" + mdParagraph(r.Behaviour) + "\n\n")
		b.WriteString("建议：" + mdParagraph(r.Suggestion) + "\n\n")
		b.WriteString("引用：" + mdInline(ref) + "\n\n")
	}
}

func renderComparatorDetails(b *strings.Builder, a attemptResult, caseID, variantID string) {
	b.WriteString("##### Comparator 详情\n\n")
	if a.comparison == nil {
		b.WriteString("未提供：该 attempt 没有通过 comparator。\n\n")
		return
	}
	items := orderedComparisonItems(*a.comparison)
	var flagged []ComparisonItem
	for _, item := range items {
		if item.Comparison != "aligned" || item.ManualReview {
			flagged = append(flagged, item)
		}
	}
	if len(flagged) == 0 {
		b.WriteString("所有 46 个比较项均为 aligned，且无人工复核标记。\n\n")
	} else {
		b.WriteString("###### 需关注项\n\n")
		for _, item := range flagged {
			manual := "否"
			if item.ManualReview {
				manual = "是"
			}
			fmt.Fprintf(b, "**%s / %s / %s — %s（置信度：%s；人工复核：%s）**\n\n", mdInline(comparisonAreaName(item.Area)), mdInline(item.Code), mdInline(item.Aspect), mdInline(item.Comparison), mdInline(item.Confidence), manual)
			b.WriteString("Gold 摘录：\n\n" + mdQuote(item.GoldExcerpt) + "\n\n")
			b.WriteString("Candidate 摘录：\n\n" + mdQuote(item.CandidateExcerpt) + "\n\n")
			b.WriteString("原因：" + mdParagraph(item.Reason) + "\n\n")
		}
	}
	b.WriteString("###### 完整 46 项矩阵\n\n")
	b.WriteString("| 区域 | Code | Aspect | 结论 | 置信度 | 人工复核 | 原因 |\n| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, item := range items {
		manual := "否"
		if item.ManualReview {
			manual = "是"
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s | %s | %s | %s |\n", mdInline(comparisonAreaName(item.Area)), mdInline(item.Code), mdInline(item.Aspect), mdInline(item.Comparison), mdInline(item.Confidence), manual, mdTableCell(item.Reason))
	}
	b.WriteString("\n原始 comparison：`cases/" + mdInline(caseID) + "/" + mdInline(variantID) + fmt.Sprintf("/attempt-%03d/comparison.json`。\n\n", a.status.Attempt))
}

func renderStability(b *strings.Builder, attempts []attemptResult) {
	b.WriteString("##### 多次运行稳定性\n\n")
	successful := successfulAttemptReports(attempts)
	if len(successful) == 0 {
		b.WriteString("没有可用于稳定性分析的完整成功 run。\n\n")
		return
	}
	b.WriteString("完整成功 run 数：" + fmt.Sprintf("%d", len(successful)) + "。\n\n")
	b.WriteString("###### D/A 判定分布\n\n")
	b.WriteString("| 维度 | 观测分布 |\n| --- | --- |\n")
	for _, row := range axisDistribution(successful) {
		row2(b, row.id, row.values)
	}
	b.WriteString("\n###### Comparator verdict 稳定性\n\n")
	stability := orderedStability(buildStability(attempts))
	if len(stability) == 0 {
		b.WriteString("未提供。\n\n")
		return
	}
	b.WriteString("| 区域 | Code | Aspect | Run 数 | 一致率 | Verdict 分布 |\n| --- | --- | --- | ---: | --- | --- |\n")
	for _, s := range stability {
		fmt.Fprintf(b, "| %s | %s | %s | %d | %s | %s |\n", mdInline(comparisonAreaName(s.Area)), mdInline(s.Code), mdInline(s.Aspect), s.Runs, rateText(s.AgreementRate), mdInline(verdictDistribution(s.Verdicts)))
	}
	b.WriteString("\n")
}

func orderedComparisonItems(c Comparison) []ComparisonItem {
	byKey := make(map[string]ComparisonItem, len(c.Items))
	for _, item := range c.Items {
		byKey[comparisonKey(item)] = item
	}
	items := make([]ComparisonItem, 0, len(c.Items))
	seen := make(map[string]bool, len(c.Items))
	for _, expected := range ExpectedComparisonItems() {
		key := comparisonKey(expected)
		if item, ok := byKey[key]; ok {
			items = append(items, item)
			seen[key] = true
		}
	}
	for _, item := range SortedComparisonItems(c) {
		if !seen[comparisonKey(item)] {
			items = append(items, item)
		}
	}
	return items
}

func orderedStability(items []StabilityItem) []StabilityItem {
	byKey := make(map[string]StabilityItem, len(items))
	for _, item := range items {
		byKey[strings.Join([]string{item.Area, item.Code, item.Aspect}, "/")] = item
	}
	ordered := make([]StabilityItem, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, expected := range ExpectedComparisonItems() {
		key := comparisonKey(expected)
		if item, ok := byKey[key]; ok {
			ordered = append(ordered, item)
			seen[key] = true
		}
	}
	for _, item := range items {
		key := strings.Join([]string{item.Area, item.Code, item.Aspect}, "/")
		if !seen[key] {
			ordered = append(ordered, item)
		}
	}
	return ordered
}

type axisDistributionRow struct {
	id     string
	values string
}

func axisDistribution(attempts []*attemptResult) []axisDistributionRow {
	depth := map[string]map[int]int{}
	autonomy := map[string]map[int]int{}
	for _, a := range attempts {
		for _, d := range a.report.Depth {
			if depth[d.ID] == nil {
				depth[d.ID] = map[int]int{}
			}
			depth[d.ID][d.Level]++
		}
		for _, x := range a.report.Autonomy {
			if autonomy[x.ID] == nil {
				autonomy[x.ID] = map[int]int{}
			}
			autonomy[x.ID][x.Band]++
		}
	}
	rows := make([]axisDistributionRow, 0, 12)
	for _, d := range rubric.DepthDims() {
		rows = append(rows, axisDistributionRow{id: d.ID + " · " + d.Name, values: levelDistribution(depth[d.ID], "L")})
	}
	for _, a := range rubric.AutonomySignals() {
		rows = append(rows, axisDistributionRow{id: a.ID + " · " + a.Name, values: levelDistribution(autonomy[a.ID], "")})
	}
	return rows
}

func successfulAttemptReports(attempts []attemptResult) []*attemptResult {
	var out []*attemptResult
	for i := range attempts {
		a := &attempts[i]
		if a.complete && a.report != nil && a.comparison != nil {
			out = append(out, a)
		}
	}
	return out
}

func renderReproducibility(b *strings.Builder, c Config, manifest Manifest) {
	b.WriteString("## 可复现信息\n\n")
	b.WriteString("| 项目 | SHA-256 / 值 |\n| --- | --- |\n")
	row(b, "配置", manifest.ConfigHash)
	row(b, "Rubric", manifest.RubricHash)
	for _, cs := range c.Cases {
		row(b, "输入："+cs.ID, manifest.Inputs[cs.ID])
		row(b, "Gold："+cs.ID, manifest.Gold[cs.ID])
	}
	b.WriteString("\n### 执行顺序\n\n")
	if len(manifest.Execution) == 0 {
		b.WriteString("未提供。\n\n")
	} else {
		for _, step := range manifest.Execution {
			b.WriteString("1. `" + mdInline(step) + "`\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("### 机器可读产物\n\n")
	b.WriteString("- `summary.json`：variant 汇总和调用统计。\n")
	b.WriteString("- `manifest.json`：配置、版本、价格快照、输入/Gold hash 和执行顺序。\n")
	b.WriteString("- `cases/<case-id>/input.json`、`gold-report.json`：冻结输入与人工 Gold。\n")
	b.WriteString("- `cases/<case-id>/<variant>/attempt-XXX/report.json`、`comparison.json`、`status.json`：逐次 candidate、对齐和状态。\n")
	b.WriteString("- `evaluator-calls/`、`comparator-calls/`：请求、原始输出和观测 metadata；可能含学生过程数据。\n\n")
}

func stateForVariant(states []*variantState, variantID string) *variantState {
	for _, state := range states {
		if state.config.ID == variantID {
			return state
		}
	}
	return nil
}

func aggregateComparisonByArea(rows []ComparisonSummary) []ComparisonSummary {
	byArea := map[string]*ComparisonSummary{}
	for _, row := range rows {
		r := byArea[row.Area]
		if r == nil {
			r = &ComparisonSummary{Area: row.Area}
			byArea[row.Area] = r
		}
		r.Total += row.Total
		r.Comparable += row.Comparable
		r.Aligned += row.Aligned
		r.Overstates += row.Overstates
		r.Understates += row.Understates
		r.NotComparable += row.NotComparable
		r.ManualReview += row.ManualReview
	}
	order := []string{"depth", "autonomy", "abstract", "promptLens", "risks"}
	out := make([]ComparisonSummary, 0, len(byArea))
	for _, area := range order {
		if r := byArea[area]; r != nil {
			if r.Comparable > 0 {
				rate := float64(r.Aligned) / float64(r.Comparable)
				r.AlignedRate = &rate
			}
			out = append(out, *r)
		}
	}
	return out
}

func comparisonAreaName(area string) string {
	switch area {
	case "depth":
		return "认知深度"
	case "autonomy":
		return "智识自主"
	case "abstract":
		return "综述"
	case "promptLens":
		return "提问透镜"
	case "risks":
		return "风险"
	default:
		return area
	}
}

func errorClass(err string) string {
	switch {
	case strings.TrimSpace(err) == "":
		return "无"
	case strings.Contains(err, "not JSON") || strings.Contains(err, "invalid JSON"):
		return "JSON 无法解析"
	case strings.Contains(err, "stream"):
		return "Provider 流失败"
	case strings.Contains(err, "deadline exceeded") || strings.Contains(err, "timeout"):
		return "调用超时"
	case strings.Contains(err, "incomplete"):
		return "报告不完整"
	default:
		return "运行错误"
	}
}

func callTokens(c CallTotals) string {
	if c.InputTokens == nil || c.OutputTokens == nil {
		return "未提供"
	}
	parts := []string{fmt.Sprintf("入 %d / 出 %d", *c.InputTokens, *c.OutputTokens)}
	if c.ReasoningTokens != nil && c.ContentTokens != nil {
		parts = append(parts, fmt.Sprintf("推理 %d / 内容 %d", *c.ReasoningTokens, *c.ContentTokens))
	}
	return strings.Join(parts, "；")
}

func costText(totals CallTotals) string {
	if totals.CostUSD != nil {
		return fmt.Sprintf("$%.6f", *totals.CostUSD)
	}
	parts := make([]string, 0, 3)
	if totals.CostedCalls > 0 {
		parts = append(parts, fmt.Sprintf("已知小计 $%.6f（%d/%d）", totals.KnownCostUSD, totals.CostedCalls, totals.Calls))
	}
	if totals.UsageMissing > 0 {
		parts = append(parts, fmt.Sprintf("usage 缺失 %d", totals.UsageMissing))
	}
	if totals.UnpricedCalls > 0 {
		parts = append(parts, fmt.Sprintf("未定价 %d", totals.UnpricedCalls))
	}
	if len(parts) == 0 {
		return "未提供"
	}
	return strings.Join(parts, "；")
}

func costPerSuccessText(cost *float64) string {
	if cost == nil {
		return "未提供"
	}
	return fmt.Sprintf("$%.6f", *cost)
}

func costCoverageText(totals CallTotals) string {
	if totals.Calls == 0 {
		return "0/0 已计价"
	}
	parts := []string{fmt.Sprintf("%d/%d 已计价", totals.CostedCalls, totals.Calls)}
	if totals.UsageMissing > 0 {
		parts = append(parts, fmt.Sprintf("usage 缺失 %d", totals.UsageMissing))
	}
	if totals.UnpricedCalls > 0 {
		parts = append(parts, fmt.Sprintf("未定价 %d", totals.UnpricedCalls))
	}
	return strings.Join(parts, "；")
}

func distributionText(d Distribution) string {
	if d.Mean == nil {
		return "未提供"
	}
	return fmt.Sprintf("min %s；median %s；mean %s；max %s", durationText(int64(*d.Min)), durationText(int64(*d.Median)), durationText(int64(*d.Mean)), durationText(int64(*d.Max)))
}

func durationText(ms int64) string {
	if ms < 0 {
		return "未提供"
	}
	return fmt.Sprintf("%s（%d ms）", (time.Duration(ms) * time.Millisecond).String(), ms)
}

func rateText(rate *float64) string {
	if rate == nil {
		return "未提供"
	}
	return percent(*rate)
}

func percent(value float64) string { return fmt.Sprintf("%.1f%%", value*100) }

func levelDistribution(values map[int]int, prefix string) string {
	if len(values) == 0 {
		return "未提供"
	}
	keys := make([]int, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s%d × %d", prefix, key, values[key]))
	}
	return strings.Join(parts, "；")
}

func verdictDistribution(values map[string]int) string {
	if len(values) == 0 {
		return "未提供"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s × %d", key, values[key]))
	}
	return strings.Join(parts, "；")
}

func refText(ref evalreport.Ref) string {
	parts := []string{ref.ID}
	if ref.Label != "" {
		parts = append(parts, ref.Label)
	}
	if ref.TS != "" {
		parts = append(parts, formatISOTime(ref.TS))
	}
	return strings.Join(parts, " · ")
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "未提供"
	}
	return t.UTC().Format(time.RFC3339)
}

func pointerValue(value *string) string {
	if value == nil || *value == "" {
		return "未提供"
	}
	return *value
}

func formatISOTime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "未提供"
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	return parsed.UTC().Format(time.RFC3339Nano)
}

func formatISOTimePointer(value *string) string {
	if value == nil {
		return "未提供"
	}
	return formatISOTime(*value)
}

func valueOrMissing(value string) string {
	if strings.TrimSpace(value) == "" {
		return "未提供"
	}
	return value
}

func row(b *strings.Builder, left, right string) { row2(b, left, right) }

func row2(b *strings.Builder, left, right string) {
	fmt.Fprintf(b, "| %s | %s |\n", mdTableCell(left), mdTableCell(right))
}

func row3(b *strings.Builder, one, two, three string) {
	fmt.Fprintf(b, "| %s | %s | %s |\n", mdTableCell(one), mdTableCell(two), mdTableCell(three))
}

func mdTableCell(value string) string {
	value = mdParagraph(value)
	value = strings.ReplaceAll(value, "|", "\\|")
	return strings.ReplaceAll(value, "\n", "<br>")
}

func mdInline(value string) string {
	return strings.ReplaceAll(mdParagraph(value), "\n", " ")
}

func mdParagraph(value string) string {
	value = strings.TrimSpace(html.EscapeString(value))
	if value == "" {
		return "未提供"
	}
	value = strings.NewReplacer("\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]").Replace(value)
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ">") || strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "*") {
			indent := line[:len(line)-len(trimmed)]
			lines[i] = indent + "\\" + trimmed
		}
	}
	return strings.Join(lines, "\n")
}

func mdQuote(value string) string {
	lines := strings.Split(mdParagraph(value), "\n")
	for i, line := range lines {
		lines[i] = "> " + line
	}
	return strings.Join(lines, "\n")
}
