import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { CARD_REGISTRY } from "@mind-imprint/contracts";

vi.mock("../../api", async (orig) => {
  const real = await orig<typeof import("../../api")>();
  return {
    ...real,
    api: {
      ...real.api,
      listThreads: vi.fn(),
      createThread: vi.fn(),
      getMessages: vi.fn(),
      chatTurn: vi.fn(),
      submitChatCard: vi.fn(),
      skipChatCard: vi.fn(),
    },
  };
});

import { api } from "../../api";
import { ChatContainer } from "./ChatContainer";

const thread = { id: "t1", title: "关于气候论证的追问", createdAt: "2026-07-10T09:00:00Z" };
const userMsg = { id: "m1", role: "user" as const, content: "我该怎么反驳这个反例？", modality: "text" as const, createdAt: "2026-07-10T09:00:01Z" };
const aiMsg = { id: "m2", role: "assistant" as const, content: "先说说，这个反例具体指向你论证里的哪一步？", modality: "text" as const, createdAt: "2026-07-10T09:00:05Z" };

async function* gen(events: unknown[]) {
  for (const e of events) yield e;
}

const COMPOSER_PLACEHOLDER = "把你正在想的、卡住的、好奇的，说给它听……";

describe("Chat surface", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the binding disclosure copy (pill, subtitle, footer)", async () => {
    (api.listThreads as any).mockResolvedValue([thread]);
    (api.getMessages as any).mockResolvedValue([userMsg, aiMsg]);
    render(<ChatContainer />);

    expect(await screen.findByText("自由对话 · AI 只提问，不替你下结论")).toBeInTheDocument();
    expect(screen.getByText("计入成长评估")).toBeInTheDocument();
    expect(screen.getByTitle("这些对话会成为你成长评估的一部分")).toBeInTheDocument();
    expect(screen.getByText("AI 会陪你把想法想深，但不替你得出结论 · 你的对话只属于你")).toBeInTheDocument();
  });

  it("loads the active thread's messages and renders both bubbles", async () => {
    (api.listThreads as any).mockResolvedValue([thread]);
    (api.getMessages as any).mockResolvedValue([userMsg, aiMsg]);
    render(<ChatContainer />);

    expect(await screen.findByText(userMsg.content)).toBeInTheDocument();
    expect(screen.getByText(aiMsg.content)).toBeInTheDocument();
  });

  it("sends a message, calls chatTurn, and streams the reply into an assistant bubble", async () => {
    (api.listThreads as any).mockResolvedValue([thread]);
    (api.getMessages as any).mockResolvedValue([]);
    (api.chatTurn as any).mockImplementation(() =>
      gen([
        { type: "reply", body: "先" },
        { type: "reply", body: "先说说这个反例具体是什么。" },
        { type: "done" },
      ]),
    );
    render(<ChatContainer />);
    await screen.findByText("对话历史");

    fireEvent.change(screen.getByPlaceholderText(COMPOSER_PLACEHOLDER), { target: { value: "帮我想想这个反例" } });
    fireEvent.click(screen.getByLabelText("发送"));

    await waitFor(() => expect(api.chatTurn).toHaveBeenCalledWith("t1", "帮我想想这个反例"));
    expect(await screen.findByText("先说说这个反例具体是什么。")).toBeInTheDocument();
    expect(screen.getByText("帮我想想这个反例")).toBeInTheDocument();
  });

  it("renders the in-thread card offer when chatTurn yields a card event", async () => {
    (api.listThreads as any).mockResolvedValue([thread]);
    (api.getMessages as any).mockResolvedValue([]);
    (api.chatTurn as any).mockImplementation(() =>
      gen([
        { type: "reply", body: "这个来源的可信度值得核查一下。" },
        { type: "card", cardInstanceId: "ci1", cardId: "craap", materialId: "" },
        { type: "done" },
      ]),
    );
    render(<ChatContainer />);
    await screen.findByText("对话历史");

    fireEvent.change(screen.getByPlaceholderText(COMPOSER_PLACEHOLDER), { target: { value: "这篇文章可信吗" } });
    fireEvent.click(screen.getByLabelText("发送"));

    const spec = CARD_REGISTRY["craap"]!;
    expect(await screen.findByText(spec.name)).toBeInTheDocument();
    expect(screen.getByText(spec.category)).toBeInTheDocument();
  });

  it("shows an honest empty state with no threads: 新对话 button, no fabricated history", async () => {
    (api.listThreads as any).mockResolvedValue([]);
    render(<ChatContainer />);

    expect(await screen.findByRole("button", { name: /新对话/ })).toBeInTheDocument();
    expect(screen.getByText("对话历史")).toBeInTheDocument();
    expect(screen.queryByText(userMsg.content)).toBeNull();
    expect(screen.queryByText(aiMsg.content)).toBeNull();
    expect(api.getMessages).not.toHaveBeenCalled();
  });
});
