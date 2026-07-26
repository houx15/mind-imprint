import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { CARD_REGISTRY } from "@mind-imprint/contracts";

vi.mock("@/api", async (orig) => {
  const real = await orig<typeof import("@/api")>();
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
      getChatAssessment: vi.fn(),
      generateChatAssessment: vi.fn(),
    },
  };
});

import { api } from "@/api";
import { ChatContainer } from "@/shell/chat/ChatContainer";

const thread = { id: "t1", title: "关于气候论证的追问", createdAt: "2026-07-10T09:00:00Z" };
const userMsg = { id: "m1", role: "user" as const, content: "我该怎么反驳这个反例？", modality: "text" as const, createdAt: "2026-07-10T09:00:01Z" };
const aiMsg = { id: "m2", role: "assistant" as const, content: "先说说，这个反例具体指向你论证里的哪一步？", modality: "text" as const, createdAt: "2026-07-10T09:00:05Z" };

async function* gen(events: unknown[]) {
  for (const e of events) yield e;
}

const COMPOSER_PLACEHOLDER = "把你正在想的、卡住的、好奇的，说给它听……";

// `handleSend`'s `for await` loop over `api.chatTurn`'s async generator
// drives its state updates purely off microtasks (the mock generator has no
// real timer/IO) — but as a fire-and-forget async function, nothing hands
// its promise chain back to the caller, so `waitFor`/`findBy*`'s real
// wall-clock budget is the only thing standing between the test and the
// chain settling. Under full-suite CPU contention that budget can lose even
// at a generous 5000ms (this test used to pin exactly that number and still
// flaked ~1 run in 3). A macrotask (setTimeout) only runs once the JS engine
// has fully drained the microtask queue — including anything newly queued
// while draining — so awaiting one deterministically waits out the whole
// chain regardless of how long the engine takes to get there, bounded only
// by vitest's own per-test timeout, not a number tuned by hand here.
async function flush() {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

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
    // "对话历史" is a static label, always in the very first render — awaiting
    // it (as this test used to) proves nothing about whether the mount-time
    // load chain (listThreads → setActiveThreadId → the getMessages effect
    // it triggers) has actually settled. If that chain's own `setEntries([])`
    // resolves AFTER this test's send already added the user/assistant
    // bubbles, it silently wipes them back to empty — a real race, not a
    // test-only artifact. `flush` (see this file's doc comment) drains that
    // whole chain deterministically before we interact.
    await flush();

    fireEvent.change(screen.getByPlaceholderText(COMPOSER_PLACEHOLDER), { target: { value: "帮我想想这个反例" } });
    fireEvent.click(screen.getByLabelText("发送"));

    await waitFor(() => expect(api.chatTurn).toHaveBeenCalledWith("t1", "帮我想想这个反例"));
    // Deterministically drain the generator's whole microtask chain (see
    // this file's `flush` doc comment) instead of racing a wall-clock
    // findByText timeout — the assertions below can now be plain sync
    // queries because the condition they depend on has actually settled.
    await flush();
    expect(screen.getByText("先说说这个反例具体是什么。")).toBeInTheDocument();
    expect(screen.getByText("帮我想想这个反例")).toBeInTheDocument();
  });

  it("sends into an empty surface: creates the thread, keeps the optimistic turn, never hydrates over it (Finding C)", async () => {
    // No threads yet → activeThreadId is null. The first send must create a
    // thread AND stream into it without the [activeThreadId] load-effect racing
    // its own getMessages over the optimistic/streamed entries. Before the fix,
    // setActiveThreadId(newId) fired getMessages(newId) which resolved after the
    // optimistic setEntries and wiped both bubbles (the first message vanished).
    (api.listThreads as any).mockResolvedValue([]);
    (api.getMessages as any).mockResolvedValue([]); // would clobber if the effect ran it
    (api.createThread as any).mockResolvedValue({ id: "t-new", title: "", createdAt: "2026-07-26T09:00:00Z" });
    (api.chatTurn as any).mockImplementation(() =>
      gen([
        { type: "reply", body: "先说说，你这个想法最不确定的地方在哪？" },
        { type: "done" },
      ]),
    );
    render(<ChatContainer />);
    await screen.findByRole("button", { name: /新对话/ });

    fireEvent.change(screen.getByPlaceholderText(COMPOSER_PLACEHOLDER), { target: { value: "我想聊聊气候论证" } });
    fireEvent.click(screen.getByLabelText("发送"));

    await waitFor(() => expect(api.chatTurn).toHaveBeenCalledWith("t-new", "我想聊聊气候论证"));
    await flush();

    // Both the user's message and the streamed reply survive — nothing wiped.
    expect(screen.getByText("我想聊聊气候论证")).toBeInTheDocument();
    expect(screen.getByText("先说说，你这个想法最不确定的地方在哪？")).toBeInTheDocument();
    // The freshly-created thread is never hydrated from the server (which is
    // what would have raced and clobbered the optimistic turn).
    expect(api.getMessages).not.toHaveBeenCalled();
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
    // Same mount-time race as the previous test — drain the initial
    // listThreads/getMessages chain before sending (see this file's `flush`
    // doc comment and the previous test's comment for why).
    await flush();

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

  it("disables the report button while no assistant turn exists", async () => {
    (api.listThreads as any).mockResolvedValue([thread]);
    (api.getMessages as any).mockResolvedValue([userMsg]); // user-only, no assistant reply yet
    render(<ChatContainer />);

    const reportButton = await screen.findByRole("button", { name: "生成本次对话的思维印记" });
    expect(reportButton).toBeDisabled();
    fireEvent.click(reportButton);
    expect(screen.queryByText("本次对话评估")).toBeNull();
  });

  it("enables the report button once an assistant turn exists and opens the report on click", async () => {
    (api.listThreads as any).mockResolvedValue([thread]);
    (api.getMessages as any).mockResolvedValue([userMsg, aiMsg]);
    (api.getChatAssessment as any).mockResolvedValue(null);
    render(<ChatContainer />);

    const reportButton = await screen.findByRole("button", { name: "生成本次对话的思维印记" });
    await waitFor(() => expect(reportButton).not.toBeDisabled());
    fireEvent.click(reportButton);
    expect(await screen.findByText("本次对话评估")).toBeInTheDocument();
  });
});
