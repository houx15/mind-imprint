import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { CARD_REGISTRY } from "@mind-imprint/contracts";

vi.mock("@/workspace/api/workspace", () => ({
  reflectProjectCard: vi.fn(async () => ({ cardInstanceId: "ci-1", reply: "你这个问题问得挺准——它能再收窄一点吗？" })),
  persistProjectCard: vi.fn(async () => "ci-legacy"),
  dismissProposal: vi.fn(async () => {}),
}));

import { CoachCardPanel } from "@/workspace/blocks/CoachCardPanel";
import { reflectProjectCard, persistProjectCard } from "@/workspace/api/workspace";

const reflectMock = vi.mocked(reflectProjectCard);
const persistMock = vi.mocked(persistProjectCard);

describe("CoachCardPanel", () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it("renders an AI card offer and consumes it on dismiss", () => {
    const onConsumed = vi.fn();
    render(
      <CoachCardPanel
        projectId="p1"
        proposal={{ cardId: "concession", reason: "one_sided", nudgeText: "要不要打开「先说对方最强的版本」？" }}
        onProposalConsumed={onConsumed}
      />,
    );
    expect(screen.getByText(/要不要打开/)).toBeTruthy();
    // The chip has a 跳过 dismiss control; clicking it consumes the proposal.
    fireEvent.click(screen.getByRole("button", { name: "跳过" }));
    expect(onConsumed).toHaveBeenCalled();
  });

  it("self-summon: the card shelf is always visible (#8), defaulting to the thinking deck", () => {
    render(<CoachCardPanel projectId="p1" proposal={null} onProposalConsumed={() => {}} />);
    // #8 · the deck is no longer hidden behind a 工具卡 toggle — the
    // default thinking cards show as tappable chips immediately.
    expect(screen.getByRole("button", { name: "事实/观点/价值判断卡" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "让步段 · 以退为进" })).toBeTruthy();
  });

  it("honors a phase-scoped deck (#17/#18)", () => {
    render(<CoachCardPanel projectId="p1" proposal={null} onProposalConsumed={() => {}} deck={["question-card", "search-plan"]} />);
    expect(screen.getByRole("button", { name: "提问卡" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "检索方向审视" })).toBeTruthy();
    // a card outside the passed deck is not offered here
    expect(screen.queryByRole("button", { name: "事实/观点/价值判断卡" })).toBeNull();
  });
});

describe("CoachCardPanel · Slice 2 reflect path", () => {
  const deckCardId = "question-card";
  const deckCardName = CARD_REGISTRY[deckCardId]!.name;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("submitting a deck card with a surface calls reflectProjectCard and surfaces the AI reply", async () => {
    const onReflected = vi.fn();
    render(
      <CoachCardPanel
        projectId="p1"
        proposal={null}
        onProposalConsumed={() => {}}
        onReflected={onReflected}
        surface="forming"
        deck={[deckCardId]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: deckCardName }));
    fireEvent.click(screen.getByRole("button", { name: /提交/ }));

    await waitFor(() => expect(reflectMock).toHaveBeenCalledTimes(1));
    const call = reflectMock.mock.calls[0]!;
    expect(call[0]).toBe("p1");
    expect(call[1]).toBe(deckCardId);
    expect(call[4]).toBe("forming");
    expect(persistMock).not.toHaveBeenCalled();

    await waitFor(() => expect(onReflected).toHaveBeenCalledTimes(1));
    // reply (2nd arg) is surfaced to the parent thread.
    expect(onReflected.mock.calls[0]![1]).toContain("你这个问题问得挺准");
  });

  it("without a surface it falls back to the legacy persist path", async () => {
    const onLogged = vi.fn();
    render(
      <CoachCardPanel
        projectId="p1"
        proposal={null}
        onProposalConsumed={() => {}}
        onLogged={onLogged}
        deck={[deckCardId]}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: deckCardName }));
    fireEvent.click(screen.getByRole("button", { name: /提交/ }));

    await waitFor(() => expect(persistMock).toHaveBeenCalledTimes(1));
    expect(reflectMock).not.toHaveBeenCalled();
    await waitFor(() => expect(onLogged).toHaveBeenCalledTimes(1));
  });
});
