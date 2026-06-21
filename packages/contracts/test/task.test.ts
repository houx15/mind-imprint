import { describe, it, expect } from "vitest";
import { Task, Message } from "../src/task";

const task = {
  id: "t_1", title: "中国是否让地球更可持续？", seed: "https://example.org/article",
  status: "active", created_at: "2026-06-21T10:00:00.000Z", last_active_at: "2026-06-21T10:00:00.000Z",
};
const message = {
  id: "m_1", task_id: "t_1", role: "user", content: "帮我看看这篇文章",
  tool_call: null, created_at: "2026-06-21T10:00:00.000Z",
};

describe("Task", () => {
  it("accepts a valid active task", () => {
    expect(Task.safeParse(task).success).toBe(true);
  });
  it("accepts a null seed", () => {
    expect(Task.safeParse({ ...task, seed: null }).success).toBe(true);
  });
  it("rejects an unknown status", () => {
    expect(Task.safeParse({ ...task, status: "deleted" }).success).toBe(false);
  });
  it("rejects a non-string seed", () => {
    expect(Task.safeParse({ ...task, seed: 42 }).success).toBe(false);
  });
});

describe("Message", () => {
  it("accepts a valid user message", () => {
    expect(Message.safeParse(message).success).toBe(true);
  });
  it("accepts empty content and a null tool_call", () => {
    expect(Message.safeParse({ ...message, content: "", tool_call: null }).success).toBe(true);
  });
  it("accepts an arbitrary tool_call payload", () => {
    expect(Message.safeParse({ ...message, tool_call: { name: "summon_card", args: { card_id: "x" } } }).success).toBe(true);
  });
  it("rejects an unknown role", () => {
    expect(Message.safeParse({ ...message, role: "tool" }).success).toBe(false);
  });
});
