import type { Task, CardInstance, Evaluation, TraceEvent } from "@mind-imprint/contracts";
import { listTasks, createTask, getTask, type TaskDetail } from "./tasks";
import { activateCard, submitCard, skipCard } from "./cards";
import { runEvaluation, getEvaluation } from "./evaluate";
import { runTurn, type TurnEvent } from "./turn";

export type { TaskDetail, TurnEvent };
export { ApiError } from "./client";

export interface ApiClient {
  listTasks(): Promise<Task[]>;
  createTask(input: { title: string; seed: string | null }): Promise<Task>;
  getTask(id: string): Promise<TaskDetail>;
  activateCard(taskId: string, cardId: string): Promise<CardInstance>;
  submitCard(taskId: string, cardId: string, env: CardInstance): Promise<CardInstance>;
  skipCard(taskId: string, cardId: string, eventTrace: TraceEvent[]): Promise<CardInstance>;
  runEvaluation(taskId: string): Promise<Evaluation>;
  getEvaluation(taskId: string): Promise<Evaluation | null>;
  runTurn(taskId: string, userInput?: string): AsyncGenerator<TurnEvent>;
}

export const api: ApiClient = {
  listTasks, createTask, getTask, activateCard, submitCard, skipCard, runEvaluation, getEvaluation, runTurn,
};
