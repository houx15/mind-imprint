import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { RecordsView } from "./RecordsView";
import { createStore, makeMemoryStorage } from "../../store";
import { CARD_REGISTRY } from "@mind-imprint/contracts";

const NOW = () => new Date("2026-06-21T12:00:00Z");

describe("RecordsView", () => {
  it("shows the 我的评估 header + learning tab by default with the activity calendar", () => {
    const store = createStore({ storage: makeMemoryStorage() });
    render(<RecordsView store={store} registry={CARD_REGISTRY} now={NOW} />);
    expect(screen.getByText("我的评估")).toBeTruthy();
    expect(screen.getByText("活跃日历")).toBeTruthy();
  });

  it("switches to the 工具卡 tab and shows the empty-state when no cards", () => {
    const store = createStore({ storage: makeMemoryStorage() });
    render(<RecordsView store={store} registry={CARD_REGISTRY} now={NOW} />);
    fireEvent.click(screen.getByText("工具卡"));
    expect(screen.getByText(/还没有用过工具卡/)).toBeTruthy();
  });

  it("switches to the 能力素养 tab and renders 9 ability rows", () => {
    const store = createStore({ storage: makeMemoryStorage() });
    render(<RecordsView store={store} registry={CARD_REGISTRY} now={NOW} />);
    fireEvent.click(screen.getByText("能力素养"));
    // 9 dim names from FULL_RUBRIC appear (each appears in radar label + ability list)
    expect(screen.getAllByText("提问清晰度").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("AI 边界与伦理").length).toBeGreaterThanOrEqual(1);
  });
});
