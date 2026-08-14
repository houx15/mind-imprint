import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

// jsdom doesn't implement IntersectionObserver; the real EvaluationReportView
// (rendered on the "ready" path) uses it via Ruler.tsx — stub it here the
// same way src/shell/report/EvaluationReport/index.test.tsx does.
class IntersectionObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
// eslint-disable-next-line @typescript-eslint/no-explicit-any
(globalThis as any).IntersectionObserver = IntersectionObserverStub;

vi.mock("@/api/evaluationReport", async (orig) => {
  const real = await orig<typeof import("@/api/evaluationReport")>();
  return {
    ...real,
    getEvaluationReport: vi.fn(),
    generateEvaluationReport: vi.fn(),
  };
});

import { getEvaluationReport, generateEvaluationReport } from "@/api/evaluationReport";
import { EvaluationReportPage } from "@/shell/report/EvaluationReportPage";
import { MOCK_EVALUATION_REPORT } from "@/shell/report/EvaluationReport/__fixtures__/mock";

const getMock = getEvaluationReport as unknown as ReturnType<typeof vi.fn>;
const generateMock = generateEvaluationReport as unknown as ReturnType<typeof vi.fn>;

describe("EvaluationReportPage", () => {
  beforeEach(() => {
    // resetAllMocks (not clearAllMocks) — the "ready" test above sets a
    // persistent `mockResolvedValue`, which clearAllMocks wouldn't erase.
    vi.resetAllMocks();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("ready: renders the report body straight off the first GET, never calling generate", async () => {
    getMock.mockResolvedValue({ status: "ready", report: MOCK_EVALUATION_REPORT });

    render(<EvaluationReportPage projectId="p1" onBack={vi.fn()} />);

    expect(await screen.findByTestId("evaluation-report")).toBeInTheDocument();
    expect(getMock).toHaveBeenCalledWith("p1");
    expect(generateMock).not.toHaveBeenCalled();
  });

  it("null row: POSTs generate once, and renders straight through when it comes back ready", async () => {
    getMock.mockResolvedValueOnce(null);
    generateMock.mockResolvedValueOnce({ status: "ready", report: MOCK_EVALUATION_REPORT });

    render(<EvaluationReportPage projectId="p1" onBack={vi.fn()} />);

    expect(await screen.findByTestId("evaluation-report")).toBeInTheDocument();
    expect(generateMock).toHaveBeenCalledWith("p1");
  });

  it("generating: shows the spinner + rotating caption, polls GET every 3s, and stops once it flips to ready", async () => {
    vi.useFakeTimers();
    getMock
      .mockResolvedValueOnce({ status: "generating" })
      .mockResolvedValueOnce({ status: "generating" })
      .mockResolvedValueOnce({ status: "ready", report: MOCK_EVALUATION_REPORT });

    await act(async () => {
      render(<EvaluationReportPage projectId="p1" onBack={vi.fn()} />);
    });
    expect(getMock).toHaveBeenCalledTimes(1);
    expect(generateMock).not.toHaveBeenCalled();
    expect(screen.getByText("印记正在梳理这个项目的过程记录……")).toBeInTheDocument();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000);
    });
    expect(getMock).toHaveBeenCalledTimes(2);
    expect(screen.queryByTestId("evaluation-report")).toBeNull();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000);
    });
    expect(getMock).toHaveBeenCalledTimes(3);
    // Fake timers are active, so `findBy*`'s internal polling never fires on
    // its own — the state update already flushed inside `act` above, so a
    // synchronous query is enough (and avoids hanging).
    expect(screen.getByTestId("evaluation-report")).toBeInTheDocument();

    // Polling has stopped — another tick doesn't fire a 4th GET.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000);
    });
    expect(getMock).toHaveBeenCalledTimes(3);
  });

  it("out-of-order regression: never has more than one GET in flight, so a stale slow response can't land after a later one and regress the UI back to generating", async () => {
    vi.useFakeTimers();

    // Manually-controlled promises (instead of mockResolvedValueOnce) so we
    // can hold a poll response open indefinitely and prove nothing else
    // fires while it's outstanding.
    const resolvers: ((v: unknown) => void)[] = [];
    getMock.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolvers.push(resolve);
        }),
    );

    await act(async () => {
      render(<EvaluationReportPage projectId="p1" onBack={vi.fn()} />);
    });
    expect(getMock).toHaveBeenCalledTimes(1);

    // Resolve the initial GET as "generating" — schedules the next poll 3s
    // out, but must not fire it early.
    await act(async () => {
      resolvers[0]!({ status: "generating" });
    });
    expect(getMock).toHaveBeenCalledTimes(1);

    // The scheduled poll fires (2nd GET) — leave it UNRESOLVED.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000);
    });
    expect(getMock).toHaveBeenCalledTimes(2);

    // Advance well past another whole poll interval while the 2nd request
    // is still pending. With the old `setInterval` implementation this
    // would fire an overlapping 3rd request before the 2nd settled — the
    // fix (self-rescheduling `setTimeout`) must not: the next poll is only
    // scheduled once the current one settles, so there must be no 3rd call.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10000);
    });
    expect(getMock).toHaveBeenCalledTimes(2);

    // The (slow) 2nd request finally settles as the terminal "ready" state.
    await act(async () => {
      resolvers[1]!({ status: "ready", report: MOCK_EVALUATION_REPORT });
    });
    expect(screen.getByTestId("evaluation-report")).toBeInTheDocument();

    // Further timer advances must not resurrect polling or regress the UI
    // back to "generating" — this is the exact bug the fix closes: a stale
    // response landing after the terminal one used to be able to do this.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10000);
    });
    expect(getMock).toHaveBeenCalledTimes(2);
    expect(screen.getByTestId("evaluation-report")).toBeInTheDocument();
    expect(screen.queryByText("印记正在梳理这个项目的过程记录……")).toBeNull();
  });

  it("failed: shows the empty state with a 重试 action that re-runs the fetch", async () => {
    getMock.mockResolvedValueOnce({ status: "failed" });

    render(<EvaluationReportPage projectId="p1" onBack={vi.fn()} />);

    expect(await screen.findByText("报告暂时无法生成")).toBeInTheDocument();
    const retryBtn = screen.getByText("重试");

    getMock.mockResolvedValueOnce({ status: "ready", report: MOCK_EVALUATION_REPORT });
    await userEvent.click(retryBtn);

    expect(await screen.findByTestId("evaluation-report")).toBeInTheDocument();
    expect(getMock).toHaveBeenCalledTimes(2);
  });

  it("error: a thrown fetch also lands on the empty state (not just an explicit failed envelope)", async () => {
    getMock.mockRejectedValueOnce(new Error("network"));

    render(<EvaluationReportPage projectId="p1" onBack={vi.fn()} />);

    expect(await screen.findByText("报告暂时无法生成")).toBeInTheDocument();
  });

  it("keeps the top bar (返回 + disabled 导出 PDF) across every state", async () => {
    getMock.mockResolvedValueOnce({ status: "failed" });
    const onBack = vi.fn();

    render(<EvaluationReportPage projectId="p1" onBack={onBack} />);
    await screen.findByText("报告暂时无法生成");

    expect(screen.getByText("返回")).toBeInTheDocument();
    const exportBtn = screen.getByText("导出 PDF");
    expect(exportBtn.closest("button")).toBeDisabled();

    await userEvent.click(screen.getByText("返回"));
    expect(onBack).toHaveBeenCalled();
  });
});
