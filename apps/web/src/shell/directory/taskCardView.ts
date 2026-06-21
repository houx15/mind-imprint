import type { Task, CardInstance } from "@mind-imprint/contracts";

export interface TaskCardView {
  title: string; status: string; statusStyle: string;
  last: string; barStyle: string; cardsLabel: string;
}

const PILL_ACTIVE = "font-size:11.5px; font-weight:600; color:#4C9A82; background:#E7F3EE; padding:3px 10px; border-radius:999px;";
const PILL_DONE = "font-size:11.5px; font-weight:600; color:#2A3B7A; background:#EDEFF9; padding:3px 10px; border-radius:999px;";

function relTime(iso: string, now: Date): string {
  const diff = now.getTime() - new Date(iso).getTime();
  const min = Math.floor(diff / 60000);
  if (min < 1) return "刚刚";
  if (min < 60) return `${min} 分钟前`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr} 小时前`;
  return `${Math.floor(hr / 24)} 天前`;
}

export function taskCardView(task: Task, cards: CardInstance[], now: Date): TaskCardView {
  const completed = cards.filter((c) => c.status === "completed").length;
  const pct = Math.min(100, completed * 25);
  return {
    title: task.title,
    status: task.status === "evaluated" ? "已评估" : "进行中",
    statusStyle: task.status === "evaluated" ? PILL_DONE : PILL_ACTIVE,
    last: relTime(task.last_active_at, now),
    cardsLabel: `${completed} 张卡`,
    barStyle: `width:${pct}%; height:100%; background:#2A3B7A; border-radius:999px;`,
  };
}
