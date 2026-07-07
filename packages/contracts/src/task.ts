import { z } from "zod";

export const TaskStatus = z.enum(["active", "evaluated"]);

export const Task = z.object({
  id: z.string(),
  title: z.string(),
  seed: z.string().nullable(),
  status: TaskStatus,
  created_at: z.string(),
  last_active_at: z.string(),
});

export const MessageRole = z.enum(["user", "assistant", "system"]);

export const Message = z.object({
  id: z.string(),
  task_id: z.string(),
  role: MessageRole,
  content: z.string(),
  tool_call: z.unknown().nullable(),
  source: z.enum(["voice"]).nullable().optional(),
  created_at: z.string(),
});

export type TaskStatus = z.infer<typeof TaskStatus>;
export type Task = z.infer<typeof Task>;
export type MessageRole = z.infer<typeof MessageRole>;
export type Message = z.infer<typeof Message>;
