import { describe, it, expect } from "vitest";
import { serializeCardForRefeed } from "../src/refeed";
import { CARD_REGISTRY } from "../src/registry";
import type { CardInstance } from "../src/envelope";

const money = CARD_REGISTRY["money-trail"]!;

function inst(status: CardInstance["status"], field_values: Record<string, unknown>): CardInstance {
  return {
    id: "ci_1", card_id: "money-trail", task_id: "t_1", parent_node_id: null,
    status, field_values, event_trace: [], rubric_tags: [], anchors: [],
    created_at: "2026-06-21T10:00:00.000Z", completed_at: null,
  };
}

describe("serializeCardForRefeed", () => {
  it("skipped → only identity + status, no steps", () => {
    const p = serializeCardForRefeed(money, inst("skipped", {}));
    expect(p).toEqual({ card_id: "money-trail", card_name: money.name, status: "skipped" });
  });

  it("completed → labels paired with field_values, repeatable_group remapped to label:value", () => {
    const p = serializeCardForRefeed(money, inst("completed", {
      main: {
        claim: "糖对健康无害",
        chain: [
          { who: "糖业协会", role: "资助者", source: "会员企业会费" },
          { who: "某大学实验室", role: "发布者", source: "协会资助的课题" },
        ],
        meaning: "出资方利益与结论一致，需找独立来源交叉验证。",
      },
    }));
    expect(p.card_id).toBe("money-trail");
    expect(p.status).toBe("completed");
    const step = p.steps!.find((s) => s.title === money.steps[0]!.title)!;
    // the claim field's answer is paired with its label
    expect(step.answers.find((a) => a.label.startsWith("要溯源"))!.value).toBe("糖对健康无害");
    // the repeatable_group is an array of {item-label: value}
    const chain = step.answers.find((a) => a.label.startsWith("资金"))!.value as Array<Record<string, unknown>>;
    expect(chain[0]).toEqual({ "节点（谁）": "糖业协会", "这是哪一环": "资助者", "它的钱 / 利益从哪来？": "会员企业会费" });
  });

  it("omits unfilled fields (no empty answers)", () => {
    const p = serializeCardForRefeed(money, inst("completed", { main: { claim: "x" } }));
    const step = p.steps!.find((s) => s.title === money.steps[0]!.title)!;
    expect(step.answers.every((a) => a.value !== undefined && a.value !== "")).toBe(true);
  });
});
