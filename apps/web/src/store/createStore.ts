import type { Task, Message, MessageRole, CardInstance, Evaluation } from "@mind-imprint/contracts";
import { StoreState, EMPTY_STATE } from "./schema";
import { STORE_KEY, type RawStorage, makeMemoryStorage } from "./storage";

export interface CreateStoreOptions {
  storage?: RawStorage;
  now?: () => string;
  genId?: () => string;
}

export interface Store {
  getSnapshot(): StoreState;
  subscribe(listener: () => void): () => void;
  createTask(input: { title: string; seed: string | null }): Task;
  getTask(id: string): Task | undefined;
  listTasks(): Task[];
  updateTask(id: string, patch: Partial<Pick<Task, "title" | "seed" | "status">>): Task;
  appendMessage(input: { task_id: string; role: MessageRole; content: string; tool_call?: unknown }): Message;
  listMessages(task_id: string): Message[];
  putCard(card: CardInstance): void;
  getCard(id: string): CardInstance | undefined;
  listCards(task_id: string): CardInstance[];
  putEvaluation(evaluation: Evaluation): void;
  listEvaluations(task_id: string): Evaluation[];
  getLatestEvaluation(task_id: string): Evaluation | undefined;
  putTask(task: Task): void;
  putMessage(message: Message): void;
  removeMessage(id: string): void;
  hydrateTask(taskId: string, data: { task: Task; messages: Message[]; cards: CardInstance[]; evaluation?: Evaluation }): void;
}

function load(storage: RawStorage): StoreState {
  const raw = storage.getItem(STORE_KEY);
  if (raw == null) return EMPTY_STATE;
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    console.warn(`[store] corrupt JSON in "${STORE_KEY}"; starting empty`);
    return EMPTY_STATE;
  }
  const result = StoreState.safeParse(parsed);
  if (!result.success) {
    console.warn(`[store] invalid store shape in "${STORE_KEY}"; starting empty`);
    return EMPTY_STATE;
  }
  return result.data;
}

export function createStore(opts: CreateStoreOptions): Store {
  const now = opts.now ?? (() => new Date().toISOString());
  const genId = opts.genId ?? (() => crypto.randomUUID());
  const storage = opts.storage ?? makeMemoryStorage();

  let state: StoreState = load(storage);
  const listeners = new Set<() => void>();

  function commit(next: StoreState): StoreState {
    const parsed = StoreState.parse(next);
    storage.setItem(STORE_KEY, JSON.stringify(parsed));
    state = parsed;
    listeners.forEach((l) => l());
    return parsed;
  }

  return {
    getSnapshot: () => state,
    subscribe(listener) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    createTask({ title, seed }) {
      const ts = now();
      const task: Task = { id: genId(), title, seed, status: "active", created_at: ts, last_active_at: ts };
      commit({ ...state, tasks: [...state.tasks, task] });
      return task;
    },
    getTask: (id) => state.tasks.find((t) => t.id === id),
    listTasks: () => state.tasks,
    updateTask(id, patch) {
      const idx = state.tasks.findIndex((t) => t.id === id);
      if (idx === -1) throw new Error(`[store] unknown task "${id}"`);
      const updated: Task = { ...state.tasks[idx]!, ...patch, last_active_at: now() };
      const tasks = [...state.tasks];
      tasks[idx] = updated;
      commit({ ...state, tasks });
      return updated;
    },
    appendMessage({ task_id, role, content, tool_call }) {
      const message: Message = {
        id: genId(), task_id, role, content,
        tool_call: tool_call ?? null, created_at: now(),
      };
      let tasks = state.tasks;
      const idx = state.tasks.findIndex((t) => t.id === task_id);
      if (idx !== -1) {
        tasks = [...state.tasks];
        tasks[idx] = { ...tasks[idx]!, last_active_at: now() };
      }
      commit({ ...state, tasks, messages: [...state.messages, message] });
      return message;
    },
    listMessages: (task_id) => state.messages.filter((m) => m.task_id === task_id),
    putCard(card) {
      const idx = state.cards.findIndex((c) => c.id === card.id);
      let cards: CardInstance[];
      if (idx === -1) {
        cards = [...state.cards, card];
      } else {
        cards = [...state.cards];
        cards[idx] = card;
      }
      commit({ ...state, cards });
    },
    getCard: (id) => state.cards.find((c) => c.id === id),
    listCards: (task_id) => state.cards.filter((c) => c.task_id === task_id),
    putEvaluation(evaluation) {
      commit({ ...state, evaluations: [...state.evaluations, evaluation] });
    },
    listEvaluations: (task_id) => state.evaluations.filter((e) => e.task_id === task_id),
    getLatestEvaluation: (task_id) => {
      const evals = state.evaluations.filter((e) => e.task_id === task_id);
      if (evals.length === 0) return undefined;
      return evals.reduce((max, e) => (e.created_at > max.created_at ? e : max));
    },
    putTask(task) {
      const idx = state.tasks.findIndex((t) => t.id === task.id);
      const tasks = idx === -1 ? [...state.tasks, task] : state.tasks.map((t) => (t.id === task.id ? task : t));
      commit({ ...state, tasks });
    },
    putMessage(message) {
      const idx = state.messages.findIndex((m) => m.id === message.id);
      const messages = idx === -1 ? [...state.messages, message] : state.messages.map((m) => (m.id === message.id ? message : m));
      commit({ ...state, messages });
    },
    removeMessage(id) {
      commit({ ...state, messages: state.messages.filter((m) => m.id !== id) });
    },
    hydrateTask(taskId, data) {
      const tasks = state.tasks.some((t) => t.id === data.task.id)
        ? state.tasks.map((t) => (t.id === data.task.id ? data.task : t))
        : [...state.tasks, data.task];
      const messages = state.messages.filter((m) => m.task_id !== taskId).concat(data.messages);
      const cards = state.cards.filter((c) => c.task_id !== taskId).concat(data.cards);
      const evaluations = data.evaluation
        ? state.evaluations.filter((e) => e.task_id !== taskId).concat(data.evaluation)
        : state.evaluations;
      commit({ ...state, tasks, messages, cards, evaluations });
    },
  };
}
