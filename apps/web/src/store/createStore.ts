import type { Task } from "@mind-imprint/contracts";
import { StoreState, EMPTY_STATE } from "./schema";
import { STORE_KEY, type RawStorage } from "./storage";

export interface CreateStoreOptions {
  storage: RawStorage;
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
  const storage = opts.storage;

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
  };
}
