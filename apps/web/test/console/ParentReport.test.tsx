import { describe, expect, it, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { ParentReport } from "@/console/ParentReport";
import { api } from "@/api";

const base = {
  cover: { name: "林知远", subject: "研究项目 · 嵌入式广告", klass: "研究组", typeLabel: "项目报告", dateStr: "2026年7月25日", warmLine: "" },
  glance: "", dOverview: "", aOverview: "",
  dRows: [
    { code: "D1", name: "任务理解与问题表述", badge: "优秀", reading: "" },
    { code: "D6", name: "反思与元认知", badge: "暂无", reading: "暂无可计入的证据" },
  ],
  aRows: [
    { code: "A1", name: "方向自主", state: "观察到主动信号", reading: "" },
    { code: "A4", name: "对抗与检验", state: "暂未观察到", reading: "" },
  ],
  opportunity: "", advice: [], prose: null as null,
};

describe("ParentReport", () => {
  it("renders 四台阶 badges and numberless 三态, shows the generate button when prose is null", async () => {
    vi.spyOn(api, "getParentReport").mockResolvedValue(base as never);
    render(<ParentReport classId="c" studentId="s" surface="project" scopeId="p" studentName="林知远" onClose={() => {}} />);
    expect(await screen.findByText("优秀")).toBeTruthy();
    expect(screen.getByText("观察到主动信号")).toBeTruthy();
    expect(screen.getByText("暂无可计入的证据")).toBeTruthy(); // 敢于空白
    // No A-axis number anywhere in the A rows.
    expect(screen.queryByText(/\b[0-5]\s*级/)).toBeNull();
    expect(screen.getByRole("button", { name: /生成家长版正文/ })).toBeTruthy();
  });

  it("composes on click and renders the readings", async () => {
    vi.spyOn(api, "getParentReport").mockResolvedValue(base as never);
    vi.spyOn(api, "generateParentReportProse").mockResolvedValue({
      ...base, prose: "present", glance: "方法对齐、证据充分",
      dRows: [{ code: "D1", name: "任务理解与问题表述", badge: "优秀", reading: "能把宽泛话题收窄。" }],
    } as never);
    render(<ParentReport classId="c" studentId="s" surface="project" scopeId="p" studentName="林知远" onClose={() => {}} />);
    fireEvent.click(await screen.findByRole("button", { name: /生成家长版正文/ }));
    await waitFor(() => expect(screen.getByText("方法对齐、证据充分")).toBeTruthy());
  });
});
