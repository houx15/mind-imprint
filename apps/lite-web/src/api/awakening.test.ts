import { describe, expect, it, vi } from "vitest";

/**
 * 报告的整形。
 *
 * 只测一件事，而它是**读代码看不出对错**的那件：库里已经写下的 payload 里有
 * `null`，而界面对它调 `.join()`。
 *
 * 🚨 来源是 2026-09-19 的线上走查。Go 把 nil 切片 marshal 成 `null`，
 * 一趟没有长出词的重做因此产出 `diff.stronger = null`，报告页整页白掉。
 * 服务端那一侧已经改成空切片，但**库里那些行改不回去** —— 所以这一层必须挡住，
 * 而且必须有测试守着，否则下一次重构会把它顺手删掉。
 */

vi.mock("./client", () => ({
  apiFetch: vi.fn(),
}));

const { apiFetch } = await import("./client");
const { fetchReport } = await import("./awakening");

describe("报告整形", () => {
  it("把 payload 里每一层的 null 收成空数组", async () => {
    vi.mocked(apiFetch).mockResolvedValue({
      report: {
        version: 1,
        attemptNo: 2,
        // 每一处都写成 null —— 这就是一份旧 payload 的样子。
        pursuing: null,
        drivers: null,
        talent: [{ key: "energy", label: "有能量", cards: null }],
        readings: [{ slug: "s", title: "T", zhTitle: "中", field: "science", tier: 2, why: null }],
        openFields: null,
        answers: null,
        diff: { stronger: null, new: null, previousQuestion: null, daysBetween: null },
      },
    } as never);

    const r = await fetchReport("run-1");

    expect(r.pursuing).toEqual([]);
    expect(r.drivers).toEqual([]);
    expect(r.openFields).toEqual([]);
    expect(r.answers).toEqual([]);
    expect(r.talent[0]!.cards).toEqual([]);
    expect(r.readings[0]!.why).toEqual([]);
    // 这一行就是当时崩掉的地方。
    expect(r.diff!.stronger).toEqual([]);
    expect(r.diff!.new).toEqual([]);
    expect(r.diff!.previousQuestion).toBe("");
    // 旧 payload 里根本没有这个字段，默认必须是 false —— 默认成 true 的话，
    // 每一份老报告都会挂上一句「关键词分析失败」。
    expect(r.selectionFailed).toBe(false);
    expect(r.diff!.daysBetween).toBe(0);

    // 界面真的会这样用它 —— 所以这里也这样用一次。
    expect(() => r.diff!.stronger.join("」「")).not.toThrow();
    expect(() => r.talent.map((p) => p.cards.length)).not.toThrow();
  });

  it("整份报告缺失时给一份能渲染的空报告", async () => {
    vi.mocked(apiFetch).mockResolvedValue({} as never);
    const r = await fetchReport("run-1");
    expect(r.pursuing).toEqual([]);
    expect(r.diff).toBeNull();
    expect(r.question).toBe("");
  });
});
