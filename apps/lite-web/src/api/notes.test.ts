import { describe, expect, it } from "vitest";
import { noteKindMeta, type Note } from "./notes";

function note(id: string, cluster: string): Note {
  return {
    id,
    kind: "observation",
    body: id,
    author: "student",
    edited: false,
    cluster,
    imageKey: "",
    x: 0,
    y: 0,
    createdAt: "2026-09-01T00:00:00Z",
  };
}

describe("noteKindMeta", () => {
  it("has a plain-language label for every kind", () => {
    for (const kind of ["observation", "quote", "assumption", "question", "idea"] as const) {
      expect(noteKindMeta(kind).label).not.toBe("");
      expect(noteKindMeta(kind).kind).toBe(kind);
    }
  });

  // 🚨 标签必须说的是这个 kind 本身，不能只是"非空"。
  //
  // 上面那条"每个 kind 都有标签"全绿了整整一版，而 observation 挂的是
  // 「观察结论」、quote 挂的是「实际观察」——两个最要紧的类型是错位的。
  // 2026-09-02 线上实测：她数出来的人数存成了 quote，同桌的原话存成了
  // observation，回灌给印记的于是是「她记下的别人的原话：走廊上我数了 23 个
  // 人」。这种错编译得过、界面看着正常、别的测试全绿，只有数据是反的。
  //
  // 服务端 gatherPblToolWork 用同一套 kind 造句（"她记下的别人的原话"、
  // "她在板上记的实际观察"），所以这里错一个字，印记那边就跟着错。
  it("labels each kind as the thing that kind actually is", () => {
    // quote = 别人说的原话。标签里必须有「原话」或「说」。
    expect(noteKindMeta("quote").label).toMatch(/原话|说/);
    // observation = 她亲眼看到的事实，不是从事实里得出的结论。
    expect(noteKindMeta("observation").label).toMatch(/观察/);
    expect(noteKindMeta("observation").label).not.toMatch(/结论/);
    // 推论是她自己想出来的，不能和「观察」共用一个词。
    expect(noteKindMeta("assumption").label).toMatch(/推论|猜/);
    expect(noteKindMeta("question").label).toMatch(/问题/);
  });

  // 五个标签互不相同，且没有两个是彼此的子串——「观察结论」和「实际观察」
  // 同时在场的时候，她根本分不出该点哪个。
  it("gives five labels nobody could confuse for each other", () => {
    const labels = (["observation", "quote", "assumption", "question", "idea"] as const).map(
      (k) => noteKindMeta(k).label,
    );
    expect(new Set(labels).size).toBe(labels.length);
    for (const a of labels) {
      for (const b of labels) {
        if (a !== b) expect(a.includes(b)).toBe(false);
      }
    }
  });
});
