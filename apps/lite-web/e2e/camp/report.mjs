/**
 * report.mjs —— 把四个学生的走查记录汇成一份能读的东西。
 *
 *   node apps/lite-web/e2e/camp/report.mjs > docs/2026-09-04-camp-dry-run.md
 *
 * 汇的是三件事，其余一概不汇：
 *   1. 每一天走到没走到，停在哪儿；
 *   2. clarity / support 的曲线，以及所有 ≤2 的时刻——那是她坐在屏幕前不知道
 *      该干什么的时刻；
 *   3. 她自己写下来的 snag（哪一处看不懂、哪一处本该有东西却没有）。
 *
 * 🚨 不做统计。四个学生不是样本，是四个具体的人，平均值会把差异抹掉。
 */
import fs from "node:fs";
import path from "node:path";

const DIR = "apps/lite-web/e2e/.camp";
const files = fs.existsSync(DIR) ? fs.readdirSync(DIR).filter((f) => f.endsWith(".json")) : [];
if (!files.length) {
  console.error(`${DIR} 里没有走查记录 —— 先跑 camp/camp.spec.ts`);
  process.exit(1);
}

const logs = files.map((f) => JSON.parse(fs.readFileSync(path.join(DIR, f), "utf8")));

const bar = (n) => "█".repeat(n) + "·".repeat(5 - n);
const mins = (ms) => `${Math.round(ms / 60000)} 分钟`;

const out = [];
out.push("# 四天营 · 模拟学生走查");
out.push("");
out.push(`日期：2026-09-04　　学生：${logs.length} 个　　全程由一个只看得见屏幕的模型操作。`);
out.push("");
out.push("> 扮演学生的模型（qwen3.8-max）拿不到项目 id、拿不到接口、不知道产品有几关。");
out.push("> 它每一步只拿到两样东西：这一屏渲染出来的字，和能按的按钮。看不懂就记下来。");
out.push("");

/* ── 一张总表 ─────────────────────────────────────────────────────────── */
out.push("## 四天，谁走到了哪儿");
out.push("");
out.push("| 学生 | 第一天 | 第二天 | 第三天 | 第四天 | 停在哪儿 |");
out.push("|---|---|---|---|---|---|");
for (const l of logs) {
  const cell = (d) => {
    const day = l.days.find((x) => x.day === d);
    if (!day) return "—";
    return day.done ? "✅" : "❌";
  };
  const stopped = l.days.find((d) => !d.done);
  out.push(
    `| **${l.student.name}** | ${cell(1)} | ${cell(2)} | ${cell(3)} | ${cell(4)} | ` +
      `${stopped ? `第${stopped.day}天：${stopped.stoppedBecause}` : "四天走完"} |`,
  );
}
out.push("");

/* ── 每个人 ───────────────────────────────────────────────────────────── */
for (const l of logs) {
  out.push(`## ${l.student.name}`);
  out.push("");
  out.push(`用他/她问的是：${l.student.probes}`);
  out.push("");

  for (const day of l.days) {
    const cl = day.steps.map((s) => s.beat.clarity);
    const su = day.steps.map((s) => s.beat.support);
    const avg = (a) => (a.length ? (a.reduce((x, y) => x + y, 0) / a.length).toFixed(1) : "—");
    out.push(`### ${day.title}`);
    out.push("");
    out.push(`白板上写的：「${day.brief}」`);
    out.push("");
    out.push(
      `**${day.done ? "走到了" : "没走到"}** · ${day.steps.length} 步 · ${mins(day.ms)} · ` +
        `清楚 ${avg(cl)}/5 · 被托住 ${avg(su)}/5` +
        (day.stoppedBecause ? `\n\n停在：${day.stoppedBecause}` : ""),
    );
    out.push("");

    // 曲线：一步一格，看得出她是在哪一段开始糊涂的。
    if (day.steps.length) {
      out.push("```");
      out.push("步  清楚 托住  她当时以为自己在看什么");
      for (const s of day.steps) {
        const flag = s.beat.clarity <= 2 ? " ←" : "  ";
        out.push(
          `${String(s.n).padStart(2)} ${bar(s.beat.clarity)} ${bar(s.beat.support)}${flag} ` +
            (s.beat.read || "").slice(0, 46),
        );
      }
      out.push("```");
      out.push("");
    }

    // 她自己写下来的每一处看不懂。这一段是全篇最该读的。
    const snags = day.steps.filter((s) => s.beat.snag && s.beat.snag.trim());
    if (snags.length) {
      out.push("**她说看不懂的地方：**");
      out.push("");
      for (const s of snags) out.push(`- 第 ${s.n} 步：${s.beat.snag}`);
      out.push("");
    }

    // 老师介入。三级 = 老师替她点了，等于这一步产品没把人送到。
    if (day.nudges && day.nudges.length) {
      out.push(`**老师介入 ${day.nudges.length} 次：**`);
      out.push("");
      for (const g of day.nudges) {
        const what = g.level === 1 ? "只说目标" : g.level === 2 ? "报了坐标" : "替她点了";
        out.push(`- 第 ${g.step} 步（${what}）：「${g.text}」`);
      }
      out.push("");
    }

    const stucks = day.steps.filter((s) => s.beat.action.kind === "stuck");
    if (stucks.length) {
      out.push("**她卡住的地方：**");
      out.push("");
      for (const s of stucks) out.push(`- 第 ${s.n} 步：${s.beat.action.why}`);
      out.push("");
    }

    // 按了没反应的。
    const dead = day.steps.filter((s) => s.outcome.startsWith("没按成") || s.outcome.includes("按不动"));
    if (dead.length) {
      out.push("**按不动 / 按不成的：**");
      out.push("");
      for (const s of dead) out.push(`- 第 ${s.n} 步：${s.outcome}`);
      out.push("");
    }
  }
}

console.log(out.join("\n"));
