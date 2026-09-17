import type { Page } from "@playwright/test";
import path from "node:path";
import { readScreen, screenKey } from "./screen";
import { think, type WriteAction, type WriteStudent } from "./brain";

/**
 * 模型演的学生在写作房间里走 N 步 —— 从 writewalk.spec.ts 里抽出来，好让
 * entries.spec.ts 的每个入口共用同一只眼睛和同一双手。
 *
 * 那几条「走查自己的毛病」的注释都留在 writewalk.spec.ts 里（字段序号、
 * 对话框按回车、正文框 blur 保存、屏幕逐字比较），这里照着做，不再重复。
 *
 * `until` 返回真就停：入口走查要的是「走到完成」，走到了就不再花钱。
 */

const FIELD_SELECTOR = "input:visible, textarea:visible";

export type WalkRow = {
  step: number;
  read: string;
  clarity?: number;
  taught?: number;
  snag?: string;
  written?: number;
  action?: WriteAction;
};

export type WalkResult = { log: WalkRow[]; nudges: number; reached: boolean };

export async function settleRoom(page: Page) {
  await page
    .waitForFunction(
      () => (document.body?.innerText ?? "").trim() !== "" || document.querySelectorAll("button").length > 0,
      null,
      { timeout: 15_000 },
    )
    .catch(() => {});
  await page
    .waitForFunction(
      () => !document.querySelector('[aria-label="印记正在打字"]') && !document.querySelector("button[aria-busy]"),
      null,
      { timeout: 180_000 },
    )
    .catch(() => {});
  await page.waitForTimeout(300);
}

export async function walkRoom(
  page: Page,
  student: WriteStudent,
  opts: { steps: number; out: string; tag: string; until: () => Promise<boolean>; goal?: string },
): Promise<WalkResult> {
  const log: WalkRow[] = [];
  const recent: string[] = [];
  let note: string | undefined;
  let lastKey = "";
  let sameFor = 0;
  let nudges = 0;

  for (let step = 1; step <= opts.steps; step++) {
    await settleRoom(page);
    if (await opts.until()) return { log, nudges, reached: true };
    const screen = await readScreen(page);
    const key = screenKey(screen);
    sameFor = key === lastKey ? sameFor + 1 : 0;
    lastKey = key;
    if (sameFor >= 5) {
      nudges++;
      note =
        "你已经在这一屏上停了好几步，屏幕一直没变。别再改那几个框了，" +
        "直接按下面那几个按钮里能往下走的那一个。";
    } else if (sameFor >= 2) {
      note = "上一步之后屏幕没有变化。";
    }
    const beat = await think({ student, screen, recent, note, goal: opts.goal });
    note = undefined;
    const written = screen.proseBoxes.reduce((n, f) => n + f.value.length, 0);
    log.push({ step, ...beat, written });
    const a = beat.action;
    console.log(
      `[${opts.tag}][${step}] clarity=${beat.clarity} taught=${beat.taught} ${a.kind}` +
        `${a.kind === "say" ? "：" + a.text.slice(0, 50) : ""}` +
        ` · 已写${written}字${beat.snag ? ` · snag: ${beat.snag}` : ""}`,
    );
    recent.push(
      `${a.kind}${a.kind === "say" ? "：" + a.text.slice(0, 40) : ""}${a.kind === "write" ? "：写了" + a.text.length + "字" : ""}${
        a.kind === "click" ? "：" + (screen.buttons[a.button]?.label ?? "?") : ""
      }`,
    );
    if (recent.length > 6) recent.shift();

    if (a.kind === "stuck" || a.kind === "leave") {
      await page.screenshot({ path: path.join(opts.out, `walk-${step}-${a.kind}.png`), fullPage: true }).catch(() => {});
      // 卡住不是终点 —— 给一句老师的话，接着走。离开才算走完。
      if (a.kind === "leave") break;
      nudges++;
      note = "老师走过来看了一眼：往下走的路一般是右上角或底部那颗主按钮。";
      continue;
    }
    if (a.kind === "wait") {
      await page.waitForTimeout(2500);
      continue;
    }
    if (a.kind === "say") {
      const idx = screen.chatBox?.i;
      if (idx === undefined) {
        note = "这一屏没有跟印记说话的地方。";
        continue;
      }
      const box = page.locator(FIELD_SELECTOR).nth(idx);
      await box.fill(a.text).catch(() => {});
      await box.press("Enter").catch(() => {});
      await page.waitForTimeout(800);
      continue;
    }
    if (a.kind === "write") {
      const box = page.locator(FIELD_SELECTOR).nth(a.field);
      if (!(await box.count())) {
        note = `没有第 ${a.field} 号框。`;
        continue;
      }
      await box.fill(a.text).catch(() => {});
      if (a.field === screen.chatBox?.i) await box.press("Enter").catch(() => {});
      else await box.blur().catch(() => {});
      await page.waitForTimeout(900);
      continue;
    }
    if (a.kind === "click") {
      const b = page.locator("button:visible").nth(a.button);
      if (!(await b.count())) {
        note = `没有第 ${a.button} 号按钮。`;
        continue;
      }
      // An animated element never counts as "stable"; a real hand clicks it anyway.
      await b.click({ timeout: 8000 }).catch(() => b.click({ force: true, timeout: 4000 }).catch(() => {}));
      await page.waitForTimeout(800);
      continue;
    }
    if (a.kind === "place") {
      const chip = page.locator(".mk-board__loose .mk-board__chip").nth(a.chip);
      const bin = page.locator(".mk-board__bin").nth(a.bin);
      if (!(await chip.count()) || !(await bin.count())) {
        note = "板上没有那张卡片或那个格子。";
        continue;
      }
      await chip.click().catch(() => {});
      await bin.click().catch(() => {});
      await page.waitForTimeout(400);
    }
  }
  await settleRoom(page);
  return { log, nudges, reached: await opts.until() };
}

export function summarize(r: WalkResult) {
  const scored = r.log.filter((b) => typeof b.clarity === "number");
  const avg = (k: "clarity" | "taught") =>
    Number((scored.reduce((s, b) => s + (b[k] ?? 0), 0) / (scored.length || 1)).toFixed(2));
  return {
    steps: r.log.length,
    clarity: avg("clarity"),
    taught: avg("taught"),
    wrote: Math.max(0, ...r.log.map((b) => b.written ?? 0)),
    nudges: r.nudges,
    reached: r.reached,
    snags: r.log.filter((b) => b.snag).map((b) => `[${b.step}] ${b.snag}`),
  };
}
