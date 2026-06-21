import { describe, it, expect } from "vitest";
import { taskCardView } from "./taskCardView";
import type { Task, CardInstance } from "@mind-imprint/contracts";

const NOW = new Date("2026-06-21T12:00:00.000Z");
function task(o: Partial<Task> = {}): Task {
  return { id: "t1", title: "中国是否让地球更可持续？", seed: null, status: "active",
    created_at: "2026-06-21T10:00:00.000Z", last_active_at: "2026-06-21T10:00:00.000Z", ...o };
}
function card(status: CardInstance["status"]): CardInstance {
  return { id: Math.random().toString(), card_id: "sift_craap", task_id: "t1", parent_node_id: null,
    status, field_values: {}, event_trace: [], rubric_tags: [], created_at: "x", completed_at: null };
}

describe("taskCardView", () => {
  it("maps active status + relative time + card count", () => {
    const v = taskCardView(task(), [card("completed"), card("completed"), card("skipped")], NOW);
    expect(v.status).toBe("进行中");
    expect(v.last).toBe("2 小时前");
    expect(v.cardsLabel).toBe("2 张卡");
    expect(v.barStyle).toContain("50%"); // 2*25
  });
  it("maps evaluated status", () => {
    expect(taskCardView(task({ status: "evaluated" }), [], NOW).status).toBe("已评估");
  });
});
