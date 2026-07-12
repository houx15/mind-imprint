import type { CoachMessage } from "./state";
import { studioTurn as defaultStudioTurn, postDisposition as defaultPostDisposition, type StudioTurnEvent } from "../api/studioTurn";

type Snapshot = { messages: CoachMessage[]; sending: boolean; error: string | null; disposableInterventionId: string | null };
type Deps = { projectId: string; api?: { studioTurn: typeof defaultStudioTurn; postDisposition: typeof defaultPostDisposition } };

export function createStudioConversation({ projectId, api }: Deps) {
  const turn = api?.studioTurn ?? defaultStudioTurn;
  const dispose = api?.postDisposition ?? defaultPostDisposition;
  let state: Snapshot = { messages: [], sending: false, error: null, disposableInterventionId: null };
  const listeners = new Set<() => void>();
  const emit = () => listeners.forEach((l) => l());
  const set = (p: Partial<Snapshot>) => { state = { ...state, ...p }; emit(); };

  async function send(text: string) {
    set({ messages: [...state.messages, { kind: "student", body: text }], sending: true, error: null });
    try {
      for await (const e of turn(projectId, text) as AsyncGenerator<StudioTurnEvent>) {
        if (e.type === "intervention") {
          set({
            messages: [...state.messages, { kind: "ai", body: e.body, tag: e.criterion || undefined, anchor: e.anchor || undefined }],
            disposableInterventionId: e.interventionId,
          });
        } else if (e.type === "error") {
          set({ error: e.message });
        }
      }
    } catch {
      set({ error: "对话失败，请重试" });
    } finally {
      set({ sending: false });
    }
  }

  async function disposeIntervention(action: "accept" | "rewrite" | "reject", reason: string) {
    if (!state.disposableInterventionId) return;
    await dispose(projectId, state.disposableInterventionId, action, reason);
  }

  return {
    getSnapshot: () => state,
    subscribe: (l: () => void) => { listeners.add(l); return () => listeners.delete(l); },
    send,
    dispose: disposeIntervention,
  };
}
