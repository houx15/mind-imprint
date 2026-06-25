import type { Task, Message, CardInstance } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

export interface TaskDetail { task: Task; messages: Message[]; cards: CardInstance[] }

export async function listTasks(): Promise<Task[]> {
  const r = await apiFetch<{ tasks: Task[] }>("/api/v1/tasks");
  return r.tasks;
}

export async function createTask(input: { title: string; seed: string | null }): Promise<Task> {
  const r = await apiFetch<{ task: Task }>("/api/v1/tasks", { method: "POST", body: JSON.stringify(input) });
  return r.task;
}

export async function getTask(id: string): Promise<TaskDetail> {
  const r = await apiFetch<TaskDetail>(`/api/v1/tasks/${id}`);
  // Backend omits tool_call when absent; the Message type carries it as nullable.
  return { ...r, messages: r.messages.map((m) => ({ ...m, tool_call: m.tool_call ?? null })) };
}
