// transcriptLines — 把一段存下来的对话摊平成「谁说了什么」。
//
// 这是回看那一屏唯一有判断的地方，所以它是一个纯函数，测试也只测这里。
//
// 三条规矩：
//
//  1. 顺序按 seq，不按数组顺序。接口回来的顺序不是承诺。
//
//  2. 🚨 她的字一个都不截断。2026-09-12 的走查里她为此跟印记说了三次
//     「我的字被截断了」，然后重打了整段。看不见就是没有。
//
//  3. 带卡片的那一条渲染成一行静态说明，**不挂真卡片组件** —— 回看那一屏是
//     只读的，在上面摆一张能点的卡片，等于给她一颗按下去什么都不会发生的
//     按钮。她当时填的答案就是紧跟着的那条 student 消息，所以那一条被这一行
//     吸收掉，不再单独成条。
//
// 「不是一张能用的卡片」和「没有卡片」是同一回事：coachCardOf 判半张卡片为
// null（没有选项的 choose_span、没有词的生词板），那条消息就照常当成印记说过
// 的一句话渲染。回看里凭空少一条，比多一行说明糟得多。
import { coachCardOf, type LiteMessage } from "@lite/api/readingRoom";

export type TranscriptLine =
  | { kind: "said"; who: "student" | "coach"; text: string; seq: number }
  | { kind: "card"; seq: number; label: string; answer: string };

/** 卡片类型 → 她在屏幕上看见过的那个名字。
 *
 *  🚨 这张表跟着 `COACH_CARD_TYPES`（api/readingRoom.ts）走，服务端加一种卡片
 *  时两处都要加。认不出来的类型退回一个通用词而不是丢掉那一条 —— 丢掉会让
 *  对话里缺一段，她会以为是我们弄丢了。 */
export const CARD_LABELS: Record<string, string> = {
  choose_span: "挑句子",
  pick_in_article: "在文章里挑",
  short_text: "写一句",
  label_roles: "标注板",
  word_bank: "生词板",
  order_events: "排序板",
};

export function transcriptLines(msgs: LiteMessage[]): TranscriptLine[] {
  const sorted = [...msgs].sort((a, b) => a.seq - b.seq);
  const out: TranscriptLine[] = [];
  for (let i = 0; i < sorted.length; i++) {
    const msg = sorted[i];
    if (!msg || msg.role === "system") continue;
    const card = msg.role === "ai" ? coachCardOf(msg) : null;
    if (card) {
      const next = sorted[i + 1];
      const answer = next && next.role === "student" ? next.content : "";
      if (answer) i++;
      out.push({ kind: "card", seq: msg.seq, label: CARD_LABELS[card.type] ?? "卡片", answer });
      continue;
    }
    if (msg.content.trim() === "") continue;
    out.push({
      kind: "said",
      who: msg.role === "student" ? "student" : "coach",
      text: msg.content,
      seq: msg.seq,
    });
  }
  return out;
}
