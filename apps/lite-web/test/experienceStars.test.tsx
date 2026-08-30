import { render, screen, cleanup, fireEvent, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ExperienceStars } from "@lite/reports/ExperienceStars";

/**
 * 报告底部的五颗星。
 *
 * 🚨 方向是这个组件唯一真正危险的地方。铁律②禁止给学生打分——排名、等第、
 * 连胜一律不做。这五颗星指的是**反过来**：她在评我们这次带读。所以下面钉住
 * 的第一件事不是交互，是文案：屏幕上不许出现任何一个把这看成「她的分数」的
 * 词。一次好心的改写就能把它变成禁止的东西。
 */

let sent: number[];

vi.mock("@lite/api/readingRoom", () => ({
  putReadingRating: async (_id: string, rating: number) => {
    sent.push(rating);
    return rating;
  },
}));

beforeEach(() => {
  sent = [];
});
afterEach(cleanup);

describe("ExperienceStars", () => {
  it("说清楚她评的是这次带读，不是她自己", () => {
    const { container } = render(<ExperienceStars atomId="a1" initial={null} />);
    expect(screen.getByText("这次阅读，你觉得怎么样？")).toBeTruthy();
    expect(screen.getByText(/你在评的是这次带读，不是你自己/)).toBeTruthy();
    // textContent AND innerHTML: an aria-label is invisible to the first and
    // read out loud by a screen reader — that hole let a scored label through
    // a 12-assertion sweep once already.
    const text = `${container.textContent ?? ""} ${container.innerHTML}`;
    for (const banned of ["得分", "分数", "评分", "等第", "排名", "连胜", "满分", "成绩"]) {
      expect(text, `出现了 ${banned}`).not.toContain(banned);
    }
  });

  it("点一下就是答案——没有提交按钮", async () => {
    render(<ExperienceStars atomId="a1" initial={null} />);
    fireEvent.click(screen.getByRole("radio", { name: "4 星" }));
    await waitFor(() => expect(sent).toEqual([4]));
    expect(screen.queryByRole("button", { name: /提交|保存/ })).toBeNull();
    expect(screen.getByText("谢谢你告诉我们。")).toBeTruthy();
  });

  it("改主意也只要一下", async () => {
    render(<ExperienceStars atomId="a1" initial={4} />);
    expect(screen.getByRole("radio", { name: "4 星" }).getAttribute("aria-checked")).toBe("true");
    fireEvent.click(screen.getByRole("radio", { name: "2 星" }));
    await waitFor(() => expect(sent).toEqual([2]));
    expect(screen.getByRole("radio", { name: "2 星" }).getAttribute("aria-checked")).toBe("true");
    expect(screen.getByRole("radio", { name: "4 星" }).getAttribute("aria-checked")).toBe("false");
  });

  it("没打过星和打了一星，长得不一样", () => {
    const { container: blank } = render(<ExperienceStars atomId="a1" initial={null} />);
    expect(blank.querySelectorAll('[aria-checked="true"]')).toHaveLength(0);
    // 「她没说」被画成「她给了最低分」是这个组件唯一会真的伤到人的错法。
    expect(blank.textContent).not.toContain("谢谢你告诉我们。");

    cleanup();
    const { container: one } = render(<ExperienceStars atomId="a1" initial={1} />);
    expect(one.querySelectorAll('[aria-checked="true"]')).toHaveLength(1);
    expect(one.textContent).toContain("谢谢你告诉我们。");
  });
});
