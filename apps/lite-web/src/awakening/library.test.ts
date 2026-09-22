import { describe, expect, it } from "vitest";

import type { AwakeningThread } from "../api/awakening";
import { LIBRARY } from "./content";
import { libraryRows, threadName, whenLabel } from "./library";

const thread = (over: Partial<AwakeningThread> = {}): AwakeningThread => ({
  id: "t1",
  title: "",
  firstText: "",
  turnCount: 0,
  summarized: false,
  reportCount: 0,
  navigator: "SAGE",
  stage: "terminal",
  lastTurnAt: "",
  updatedAt: "2026-09-21T02:00:00Z",
  ...over,
});

describe("threadName", () => {
  it("用她挑定的名字", () => {
    expect(threadName({ title: "潮汐发电为什么少", firstText: "最近老是刷到…" })).toBe(
      "潮汐发电为什么少",
    );
  });

  // 🚨 没起名的线索退回她自己写的第一句 —— 否则库里是一排空行，
  // 而那几条线索她一条都认不出来。
  it("没起名就用她自己写的第一句", () => {
    expect(threadName({ title: "  ", firstText: "最近老是刷到潮汐发电的视频" })).toBe(
      "最近老是刷到潮汐发电的视频",
    );
  });

  // 退回来的那一句按标签宽度裁：库是一张表，一条两行的行认不快。
  // 裁的只是这一行显示的字 —— 点进去看到的还是她写的全文。
  it("她的长句子按第一个句读断开，再按宽度裁", () => {
    expect(
      threadName({
        title: "",
        firstText: "最近老是刷到潮汐发电的视频，一个海湾里的闸门一开一合就能发电",
      }),
    ).toBe("最近老是刷到潮汐发电的视频");
    expect(
      threadName({ title: "", firstText: "我一直在想为什么潮汐这么规律却几乎没有地方用它发电" }),
    ).toBe("我一直在想为什么潮汐这么规律");
  });
});

describe("whenLabel", () => {
  const now = new Date("2026-09-21T09:00:00+08:00");

  // 🚨 比的是日期，不是小时数。昨夜十一点到今早八点只隔九小时，
  // 但对她来说那是「昨天」。
  it("昨天夜里算昨天，不算今天", () => {
    expect(whenLabel("2026-09-20T23:00:00+08:00", now)).toBe(LIBRARY.yesterday);
    expect(whenLabel("2026-09-21T01:00:00+08:00", now)).toBe(LIBRARY.today);
  });

  it("再往前按天数说", () => {
    expect(whenLabel("2026-09-18T15:00:00+08:00", now)).toBe(
      LIBRARY.daysAgo.replace("{n}", "3"),
    );
  });

  it("没有时间戳就不显示", () => {
    expect(whenLabel("", now)).toBe("");
    expect(whenLabel("not-a-date", now)).toBe("");
  });
});

describe("libraryRows", () => {
  const now = new Date("2026-09-21T09:00:00+08:00");

  it("在做的那条显示进度，总结过的显示已总结", () => {
    const rows = libraryRows(
      [
        thread({ id: "a", title: "潮汐发电", turnCount: 3, lastTurnAt: "2026-09-21T08:00:00+08:00" }),
        thread({ id: "b", firstText: "桌游规则总是被改", summarized: true, reportCount: 1, turnCount: 8 }),
      ],
      now,
    );
    expect(rows[0]!.state).toBe(LIBRARY.progress.replace("{n}", "3"));
    expect(rows[0]!.when).toBe(LIBRARY.today);
    expect(rows[1]!.state).toBe(LIBRARY.summarized);
    // 🚨 总结过不等于只读 —— 界面据此仍然给「接着问」。
    expect(rows[1]!.summarized).toBe(true);
    expect(rows[1]!.name).toBe("桌游规则总是被改");
    expect(rows[1]!.unnamed).toBe(true);
  });

  it("总结过不止一次时说出有几份", () => {
    const rows = libraryRows([thread({ summarized: true, reportCount: 2 })], now);
    expect(rows[0]!.extra).toBe(LIBRARY.reports.replace("{n}", "2"));
  });

  // 一条刚开、一个字都没写的线索：名字和原话都没有，那一行仍然要有东西。
  it("什么都还没有的线索也有东西可显示", () => {
    const rows = libraryRows([thread()], now);
    expect(rows[0]!.name).toBe(LIBRARY.unnamed);
    expect(rows[0]!.state).toBe(LIBRARY.progress.replace("{n}", "0"));
  });

  // 八问答满之后进度不该写成「已答 9 / 8」。
  it("进度不超过总问数", () => {
    const rows = libraryRows([thread({ turnCount: 11 })], now);
    expect(rows[0]!.state).toBe(LIBRARY.progress.replace("{n}", "8"));
  });
});
