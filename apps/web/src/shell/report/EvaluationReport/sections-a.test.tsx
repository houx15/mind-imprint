import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Abstract } from "./Abstract";
import { Header } from "./Header";
import { Timeline } from "./Timeline";
import { MOCK_EVALUATION_REPORT } from "./__fixtures__/mock";

describe("Header", () => {
  it("renders a milestone date and a counter value", () => {
    render(<Header basics={MOCK_EVALUATION_REPORT.basics} title={MOCK_EVALUATION_REPORT.basics.title} />);

    // basics.milestones.started = "2026-08-01T09:00:00Z" -> "08-01"
    expect(screen.getByText("08-01")).toBeInTheDocument();
    // basics.counters.aiTurns = 612
    expect(screen.getByText("612")).toBeInTheDocument();
  });
});

describe("Abstract", () => {
  it("renders **emphasis** as a styled span and a recommended-course reason", () => {
    render(<Abstract abstract={MOCK_EVALUATION_REPORT.abstract} />);

    const emphasized = screen.getByText("没有直接接受标题叙事");
    expect(emphasized.tagName).toBe("EM");

    expect(screen.getByText(MOCK_EVALUATION_REPORT.abstract.recommendedCourses[0]!.reason)).toBeInTheDocument();
  });
});

describe("Timeline", () => {
  it("renders an event kind label and an AI x N badge", () => {
    render(<Timeline events={MOCK_EVALUATION_REPORT.events} />);

    // events[0].kind = "chat" -> EVENT_KIND.chat.label
    expect(screen.getByText("对话")).toBeInTheDocument();
    expect(screen.getByText("AI ×24")).toBeInTheDocument();
  });
});
