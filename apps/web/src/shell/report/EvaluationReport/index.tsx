import type { EvaluationReport } from "@mind-imprint/contracts";
import { Abstract } from "./Abstract";
import { AxisPanel } from "./AxisPanel";
import { Header } from "./Header";
import { Materials } from "./Materials";
import { PromptLens } from "./PromptLens";
import { Risks } from "./Risks";
import { Ruler, type RulerSection } from "./Ruler";
import { Timeline } from "./Timeline";
import { ToolUsage } from "./ToolUsage";

const SECTIONS: RulerSection[] = [
  { id: "s1", idx: "01", label: "基本信息" },
  { id: "s2", idx: "02", label: "综述" },
  { id: "s3", idx: "03", label: "过程时间线" },
  { id: "s4", idx: "04", label: "材料清单" },
  { id: "s5", idx: "05", label: "认知深度 D" },
  { id: "s6", idx: "06", label: "智识自主 A" },
  { id: "s7", idx: "07", label: "提问透镜" },
  { id: "s8", idx: "08", label: "工具卡与子代理" },
  { id: "s9", idx: "09", label: "风险提示" },
];

export interface EvaluationReportViewProps {
  report: EvaluationReport;
}

/**
 * The whole 过程评估报告 page (Task 8 shell — sections 9–11 fill in real
 * content). Ports `.report`/`.body` from
 * `docs/reference/2026-08-13-eval-report-mockup.html`: sticky ruler 目录 +
 * scrollable body, 9 sections in fixed order, each anchored by `id="sN"`
 * for the ruler's IntersectionObserver scroll-sync.
 */
export function EvaluationReportView({ report }: EvaluationReportViewProps) {
  return (
    <div className="grid grid-cols-[224px_minmax(0,1fr)] items-start" data-testid="evaluation-report">
      <Ruler sections={SECTIONS} />

      <div className="min-w-0 px-12 pb-[140px] pt-8">
        <section id="s1" className="mb-14 scroll-mt-20">
          <Header basics={report.basics} title={report.basics.title} />
        </section>

        <section id="s2" className="mb-14 scroll-mt-20">
          <Abstract abstract={report.abstract} />
        </section>

        <section id="s3" className="mb-14 scroll-mt-20">
          <Timeline events={report.events} />
        </section>

        <section id="s4" className="mb-14 scroll-mt-20">
          <Materials materials={report.materials} />
        </section>

        <section id="s5" className="mb-14 scroll-mt-20">
          <AxisPanel axis="depth" dims={report.depth} />
        </section>

        <section id="s6" className="mb-14 scroll-mt-20">
          <AxisPanel axis="autonomy" dims={report.autonomy} />
        </section>

        <section id="s7" className="mb-14 scroll-mt-20">
          <PromptLens promptLens={report.promptLens} />
        </section>

        <section id="s8" className="mb-14 scroll-mt-20">
          <ToolUsage toolUsage={report.toolUsage} />
        </section>

        <section id="s9" className="mb-14 scroll-mt-20">
          <Risks risks={report.risks} />
        </section>
      </div>
    </div>
  );
}
