import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MOCK_EVALUATION_REPORT } from "../EvaluationReport/__fixtures__/mock";
import { ReportPrintButton } from "./ReportPrintButton";

afterEach(cleanup);

describe("ReportPrintButton", () => {
  it("mounts the print document and calls window.print on click", async () => {
    const printSpy = vi.spyOn(window, "print").mockImplementation(() => {});
    vi.useFakeTimers();
    render(<ReportPrintButton report={MOCK_EVALUATION_REPORT} />);
    fireEvent.click(screen.getByText("导出 PDF"));
    expect(document.querySelector(".report-print-root")).not.toBeNull();
    expect(screen.getByTestId("print-cover")).toBeInTheDocument();
    vi.runAllTimers();
    expect(printSpy).toHaveBeenCalledTimes(1);
    vi.useRealTimers();
    printSpy.mockRestore();
  });

  it("removes the print document on afterprint", async () => {
    const printSpy = vi.spyOn(window, "print").mockImplementation(() => {});
    render(<ReportPrintButton report={MOCK_EVALUATION_REPORT} />);
    fireEvent.click(screen.getByText("导出 PDF"));
    expect(document.querySelector(".report-print-root")).not.toBeNull();
    window.dispatchEvent(new Event("afterprint"));
    expect(document.querySelector(".report-print-root")).toBeNull();
    printSpy.mockRestore();
  });

  it("renders the cover identity from the report", () => {
    render(<ReportPrintButton report={MOCK_EVALUATION_REPORT} />);
    fireEvent.click(screen.getByText("导出 PDF"));
    expect(screen.getByText(MOCK_EVALUATION_REPORT.basics.title)).toBeInTheDocument();
    expect(screen.getByText(MOCK_EVALUATION_REPORT.student.name)).toBeInTheDocument();
  });
});
