import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { WorkspaceView } from "./WorkspaceView";
import type { Store } from "../store/createStore";
import { createStore, makeMemoryStorage } from "../store";
import type { Conversation, ConvState, ConvPhase } from "../agent/createConversation";
import type { Task, Message, CardInstance } from "@mind-imprint/contracts";

// ─── Fake store ───────────────────────────────────────────────────────────────

function makeTask(overrides: Partial<Task> = {}): Task {
  return {
    id: "t1",
    title: "中国是否让地球变得更可持续？",
    seed: null,
    status: "active",
    created_at: "2024-01-01T00:00:00.000Z",
    last_active_at: "2024-01-01T00:00:00.000Z",
    ...overrides,
  };
}

function makeCardInstance(overrides: Partial<CardInstance> = {}): CardInstance {
  return {
    id: "ci-sift",
    card_id: "sift_craap",
    task_id: "t1",
    parent_node_id: null,
    status: "active",
    field_values: {},
    event_trace: [],
    rubric_tags: [],
    created_at: "2024-01-01T00:00:00.000Z",
    completed_at: null,
    ...overrides,
  };
}

function makeStore(overrides: {
  task?: Task;
  messages?: Message[];
  cards?: CardInstance[];
} = {}): Store {
  const task = overrides.task ?? makeTask();
  const messages: Message[] = overrides.messages ?? [];
  const cards: CardInstance[] = overrides.cards ?? [];

  const storeState = { version: 1 as const, tasks: [task], messages, cards, evaluations: [] };

  return {
    getSnapshot: () => storeState,
    subscribe: (listener: () => void) => {
      // Return unsubscribe
      return () => {};
    },
    createTask: vi.fn() as any,
    getTask: (id: string) => (id === task.id ? task : undefined),
    listTasks: () => [task],
    updateTask: vi.fn() as any,
    appendMessage: vi.fn() as any,
    listMessages: (task_id: string) => messages.filter((m) => m.task_id === task_id),
    putCard: vi.fn() as any,
    getCard: (id: string) => cards.find((c) => c.id === id),
    listCards: (task_id: string) => cards.filter((c) => c.task_id === task_id),
    putEvaluation: vi.fn() as any,
    listEvaluations: vi.fn() as any,
    getLatestEvaluation: vi.fn() as any,
  };
}

// ─── Fake conversation ────────────────────────────────────────────────────────

function makeConversation(phase: ConvPhase = "idle", pendingCardId?: string): Conversation {
  let state: ConvState = { taskId: "t1", phase, pendingCardId };
  const listeners = new Set<() => void>();

  return {
    getSnapshot: () => state,
    subscribe: (listener: () => void) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    send: vi.fn(),
    openCard: vi.fn(),
    submitCard: vi.fn(),
    skipCard: vi.fn(),
  };
}

// ─── Tests ────────────────────────────────────────────────────────────────────

describe("WorkspaceView", () => {
  describe("Breadcrumb", () => {
    it("shows 返回所有任务 link", () => {
      const store = makeStore();
      const conv = makeConversation();
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      expect(screen.getByText(/返回所有任务/)).toBeInTheDocument();
    });

    it("shows the task title", () => {
      const store = makeStore();
      const conv = makeConversation();
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      expect(screen.getByText("中国是否让地球变得更可持续？")).toBeInTheDocument();
    });

    it("shows 进行中 badge", () => {
      const store = makeStore();
      const conv = makeConversation();
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      expect(screen.getByText("进行中")).toBeInTheDocument();
    });

    it("shows card count badge", () => {
      const cards = [makeCardInstance({ status: "completed" })];
      const store = makeStore({ cards });
      const conv = makeConversation();
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      expect(screen.getByText(/1 张工具卡/)).toBeInTheDocument();
    });

    it("calls onBack when 返回所有任务 is clicked", async () => {
      const onBack = vi.fn();
      const store = makeStore();
      const conv = makeConversation();
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={onBack} />);
      await userEvent.click(screen.getByText(/返回所有任务/));
      expect(onBack).toHaveBeenCalled();
    });
  });

  describe("Composer", () => {
    it("renders the textarea with correct placeholder", () => {
      const store = makeStore();
      const conv = makeConversation();
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      expect(screen.getByPlaceholderText("把你的想法发给陪练……")).toBeInTheDocument();
    });

    it("renders the send button", () => {
      const store = makeStore();
      const conv = makeConversation();
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      // Send button is identified by its role and accessible name or the SVG inside
      const sendBtn = screen.getByRole("button", { name: /发送/ });
      expect(sendBtn).toBeInTheDocument();
    });

    it("send button is disabled when phase is awaiting_llm", () => {
      const store = makeStore();
      const conv = makeConversation("awaiting_llm");
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      expect(screen.getByRole("button", { name: /发送/ })).toBeDisabled();
    });

    it("send button is enabled when phase is idle", () => {
      const store = makeStore();
      const conv = makeConversation("idle");
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      expect(screen.getByRole("button", { name: /发送/ })).not.toBeDisabled();
    });

    it("calls conversation.send when text is typed and send button clicked", async () => {
      const store = makeStore();
      const conv = makeConversation("idle");
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      const textarea = screen.getByPlaceholderText("把你的想法发给陪练……");
      await userEvent.type(textarea, "我的论点");
      await userEvent.click(screen.getByRole("button", { name: /发送/ }));
      expect(conv.send).toHaveBeenCalledWith("我的论点");
    });
  });

  describe("TreePanel", () => {
    it("shows the 过程树 heading when open", () => {
      const store = makeStore();
      const conv = makeConversation();
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      expect(screen.getByText("过程树")).toBeInTheDocument();
    });

    it("shows the 只读 badge when panel is open", () => {
      const store = makeStore();
      const conv = makeConversation();
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      expect(screen.getByText("只读")).toBeInTheDocument();
    });

    it("shows the empty-state copy", () => {
      const store = makeStore();
      const conv = makeConversation();
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      expect(screen.getByText("边做边长 · 随评估归并枝节")).toBeInTheDocument();
    });

    it("toggles tree closed when collapse button is clicked", async () => {
      const store = makeStore();
      const conv = makeConversation();
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      // The 只读 badge should be visible when open
      expect(screen.getByText("只读")).toBeInTheDocument();
      // Click the collapse/toggle button (the chevron in the tree header)
      const toggleBtn = screen.getByRole("button", { name: /折叠过程树/ });
      await userEvent.click(toggleBtn);
      // After closing, 只读 badge should not be visible
      expect(screen.queryByText("只读")).not.toBeInTheDocument();
    });

    it("toggles tree back open after closing", async () => {
      const store = makeStore();
      const conv = makeConversation();
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      const toggleBtn = screen.getByRole("button", { name: /折叠过程树/ });
      await userEvent.click(toggleBtn);
      // Tree is closed, now click the expand button on the closed panel
      const expandBtn = screen.getByRole("button", { name: /展开过程树/ });
      await userEvent.click(expandBtn);
      expect(screen.getByText("只读")).toBeInTheDocument();
    });
  });

  describe("Live process tree growth", () => {
    it("shows empty-state when there are no cards yet", () => {
      const realStore = createStore({ storage: makeMemoryStorage() });
      realStore.createTask({ title: "中国是否让地球变得更可持续？", seed: null });
      const taskId = realStore.listTasks()[0]!.id;
      const conv = makeConversation();
      render(<WorkspaceView store={realStore} conversation={conv} taskId={taskId} onBack={() => {}} />);
      // Empty-state footer is always visible
      expect(screen.getByText("边做边长 · 随评估归并枝节")).toBeInTheDocument();
      // Card name should NOT be visible yet
      expect(screen.queryByText("SIFT×CRAAP 信息核查")).not.toBeInTheDocument();
    });

    it("shows the card node after putCard with a completed sift_craap instance", () => {
      const realStore = createStore({ storage: makeMemoryStorage() });
      realStore.createTask({ title: "中国是否让地球变得更可持续？", seed: null });
      const taskId = realStore.listTasks()[0]!.id;
      const conv = makeConversation();
      const { rerender } = render(
        <WorkspaceView store={realStore} conversation={conv} taskId={taskId} onBack={() => {}} />,
      );
      // Confirm no card node yet
      expect(screen.queryByText("SIFT×CRAAP 信息核查")).not.toBeInTheDocument();

      // Add a completed sift_craap card to the store
      act(() => {
        realStore.putCard({
          id: "ci-sift-live",
          card_id: "sift_craap",
          task_id: taskId,
          parent_node_id: null,
          status: "completed",
          field_values: {},
          event_trace: [],
          rubric_tags: [],
          created_at: new Date().toISOString(),
          completed_at: new Date().toISOString(),
        });
      });

      // The tree should now show the card node title from CARD_REGISTRY
      expect(screen.getByText("SIFT×CRAAP 信息核查")).toBeInTheDocument();
    });
  });

  describe("CardSheetHost", () => {
    it("does NOT show the bottom sheet when phase is idle", () => {
      const store = makeStore();
      const conv = makeConversation("idle");
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      expect(screen.queryByText("现在轮到你想")).not.toBeInTheDocument();
    });

    it("shows the bottom sheet when phase is card_active with an active card instance", () => {
      const cards = [makeCardInstance({ status: "active" })];
      const store = makeStore({ cards });
      const conv = makeConversation("card_active", "ci-sift");
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      expect(screen.getByText("现在轮到你想")).toBeInTheDocument();
    });

    it("shows the card name in the bottom sheet header", () => {
      const cards = [makeCardInstance({ status: "active" })];
      const store = makeStore({ cards });
      const conv = makeConversation("card_active", "ci-sift");
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      // The card name from CARD_REGISTRY for sift_craap appears — may appear in both
      // the tree panel and the card sheet header, so allow multiple matches.
      expect(screen.getAllByText("SIFT×CRAAP 信息核查").length).toBeGreaterThan(0);
    });

    it("mounts the CardRenderer — a known field label from the active card appears", () => {
      const cards = [makeCardInstance({ status: "active" })];
      const store = makeStore({ cards });
      const conv = makeConversation("card_active", "ci-sift");
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      // SIFT×CRAAP has a step title rendered by CardRenderer
      expect(screen.getByText("SIFT · 横向找更多来源")).toBeInTheDocument();
    });

    it("calls conversation.skipCard when the bottom sheet close button is clicked", async () => {
      const cards = [makeCardInstance({ status: "active" })];
      const store = makeStore({ cards });
      const conv = makeConversation("card_active", "ci-sift");
      render(<WorkspaceView store={store} conversation={conv} taskId="t1" onBack={() => {}} />);
      const closeBtn = screen.getByRole("button", { name: /关闭/ });
      await userEvent.click(closeBtn);
      expect(conv.skipCard).toHaveBeenCalledWith("ci-sift");
    });
  });
});
