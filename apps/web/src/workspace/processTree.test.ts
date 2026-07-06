import { describe, it, expect } from "vitest";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { Task, CardInstance } from "@mind-imprint/contracts";
import { deriveProcessTree } from "./processTree";

const task: Task = { id: "t_1", title: "中国是否让地球变得更可持续？", seed: "https://example.org/article", status: "active", created_at: "2026-06-21T09:00:00.000Z", last_active_at: "2026-06-21T09:00:00.000Z" };

function card(id: string, card_id: string, status: CardInstance["status"], at: string, field_values: Record<string, unknown> = {}): CardInstance {
  return { id, card_id, task_id: "t_1", parent_node_id: null, status, field_values, event_trace: [], rubric_tags: [], anchors: [], created_at: at, completed_at: status === "completed" ? at : null };
}

const reg = CARD_REGISTRY;

describe("deriveProcessTree", () => {
  it("task alone → a single task_root", () => {
    const nodes = deriveProcessTree({ task, cards: [], registry: reg });
    expect(nodes).toHaveLength(1);
    expect(nodes[0]).toMatchObject({ id: "t_1", type: "task_root", parent_id: null, title: task.title });
  });

  it("a completed SIFT card → root + a card_use node under it", () => {
    const nodes = deriveProcessTree({ task, cards: [card("ci_1", "sift_craap", "completed", "2026-06-21T09:05:00.000Z", { sift: { stop: "证明中国让地球更可持续" } })], registry: reg });
    expect(nodes).toHaveLength(2);
    expect(nodes[1]).toMatchObject({ id: "ci_1", type: "card_use", parent_id: "t_1", status: "completed", card_id: "sift_craap" });
    expect(nodes[1]!.title).toBe(reg.sift_craap!.name);
    expect(typeof nodes[1]!.sub).toBe("string");
  });

  it("a concession card → a concession node; a reflexivity card → a reflection node", () => {
    const nodes = deriveProcessTree({ task, cards: [
      card("ci_a", "concession", "completed", "2026-06-21T09:06:00.000Z"),
      card("ci_b", "checkpoint", "completed", "2026-06-21T09:07:00.000Z"),
    ], registry: reg });
    expect(nodes.find((n) => n.id === "ci_a")!.type).toBe("concession");
    expect(nodes.find((n) => n.id === "ci_b")!.type).toBe("reflection");
  });

  it("a skipped card still emits a node marked skipped (过程即数据)", () => {
    const nodes = deriveProcessTree({ task, cards: [card("ci_s", "sift_craap", "skipped", "2026-06-21T09:08:00.000Z")], registry: reg });
    const n = nodes.find((x) => x.id === "ci_s")!;
    expect(n.status).toBe("skipped");
    expect(n.sub).toContain("已跳过");
  });

  it("cards are ordered by created_at, task_root first", () => {
    const nodes = deriveProcessTree({ task, cards: [
      card("ci_2", "sift_craap", "completed", "2026-06-21T09:10:00.000Z"),
      card("ci_1", "sift_craap", "completed", "2026-06-21T09:05:00.000Z"),
    ], registry: reg });
    expect(nodes.map((n) => n.id)).toEqual(["t_1", "ci_1", "ci_2"]);
  });

  it("an unknown card_id still emits a card_use node titled by the id (no throw)", () => {
    const nodes = deriveProcessTree({ task, cards: [card("ci_x", "nonexistent-card", "completed", "2026-06-21T09:09:00.000Z")], registry: reg });
    const n = nodes.find((x) => x.id === "ci_x")!;
    expect(n.type).toBe("card_use");
    expect(n.title).toBe("nonexistent-card");
  });
});
