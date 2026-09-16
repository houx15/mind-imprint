# 完成之后能回看 + 报告值得发出去 · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 读完/写完之后，她能回看整段对话和原文；报告上多出开场序号、对话里的转折时刻、她读的那篇文章；分享做成一颗真按钮，对话是否公开由她单独勾选，发布过的成稿列在她的主页上并能点开。

**Architecture:** 后端三处加数据（`ordinal` 冻结在报告里、`turningPoints` 由模型**只回编号**、`include_transcript` 一位新列），前端三处加界面（完成页的三格切换器、报告的三块新内容、分享面板的勾选 + 主页的作品区）。转折时刻挂在**已经在跑**的那一次 `assess` 调用上，不新增模型调用。

**Tech Stack:** Go 1.23（`net/http` + `pgx` + `sqlc` + `goose`）· React 18 + Vite + TypeScript + Tailwind（`apps/lite-web`）· PostgreSQL · Playwright（看一眼用）

**Spec:** `docs/superpowers/specs/2026-09-16-reading-writing-history-and-sharing-design.md`

## Global Constraints

- **只写逻辑测试**，不给每个渲染元素写断言（`test-logic-not-endless-frontend`）。UI 用真浏览器看，不靠 jsdom 断言。
- **界面文案**：标签是名词（`报告` / `对话` / `原文` / `成稿`），按钮写「做什么」或「做完了」，状态用「已/待/中」，不写文学腔、不用抒情副词、`长出来/长成` 只留给树。见 `AGENTS.md § 界面文案怎么写`。
- **她自己写的字一个都不许截断**（2026-09-12）。截断只允许出现在**喂给模型的 prompt** 里，永远不在渲染出来的页面上。
- **R4 不动**：金句只能是她自己的句子，验的是 `corpus.Text` 的逐字子串。转折时刻**不走引用**，走编号。
- **铁律①** 不受影响：这一版没有任何地方让 AI 写她的正文。
- Go 测试：`CGO_ENABLED=0 go test ./internal/api -timeout 1800s`。🚨 `go test ./... | grep | tail` 的退出码是 tail 的，别那样判成功。
- 迁移号从 **0174** 开始（main 现在在 0173）。
- 新增 sqlc 查询后必须重跑 `sqlc generate`（pin `@v1.27.0`）。
- lite 改动**不许碰 pro**：共用 Go 包与 `queries/`、`migrations/` 目录，新建一个已存在的文件就会毁掉 pro 的代码。

---

### Task 1: 只读对话的渲染逻辑

**Files:**
- Create: `apps/lite-web/src/reports/transcriptLines.ts`
- Test: `apps/lite-web/src/reports/transcriptLines.test.ts`

**Interfaces:**
- Consumes: `LiteMessage`（`@lite/api/readingRoom`）、`coachCardOf`（同文件）
- Produces:
  ```ts
  export type TranscriptLine =
    | { kind: "said"; who: "student" | "coach"; text: string; seq: number }
    | { kind: "card"; seq: number; label: string; answer: string };
  export function transcriptLines(msgs: LiteMessage[]): TranscriptLine[];
  export const CARD_LABELS: Record<string, string>;
  ```

- [ ] **Step 1: Write the failing test**

```ts
import { describe, expect, it } from "vitest";
import { transcriptLines } from "./transcriptLines";
import type { LiteMessage } from "@lite/api/readingRoom";

const m = (seq: number, role: string, content: string, payload?: unknown): LiteMessage =>
  ({ seq, role, content, createdAt: "2026-09-16T00:00:00Z", payload: payload as never });

describe("transcriptLines", () => {
  it("keeps her words and 印记's words apart, in seq order", () => {
    const got = transcriptLines([m(2, "ai", "你为什么这么说？"), m(1, "student", "我觉得人均排放更重要。")]);
    expect(got.map((l) => [l.kind, (l as { who?: string }).who])).toEqual([
      ["said", "student"],
      ["said", "coach"],
    ]);
  });

  it("never truncates her words", () => {
    const long = "我".repeat(3364);
    const got = transcriptLines([m(1, "student", long)]);
    expect((got[0] as { text: string }).text).toHaveLength(3364);
  });

  it("renders a card message as one static line, not a card", () => {
    const got = transcriptLines([
      m(1, "ai", "给你一张卡片", { card: { type: "choose_span", prompt: "挑一句" } }),
      m(2, "student", "第三句"),
    ]);
    expect(got[0]).toEqual({ kind: "card", seq: 1, label: "挑句子", answer: "第三句" });
    expect(got).toHaveLength(1);
  });

  it("drops system messages — they are bookkeeping, not anything anyone said", () => {
    expect(transcriptLines([m(1, "system", "工具已了结")])).toEqual([]);
  });

  it("leaves the answer empty when she never answered the card", () => {
    const got = transcriptLines([m(1, "ai", "给你一张卡片", { card: { type: "word_bank", prompt: "生词" } })]);
    expect(got[0]).toEqual({ kind: "card", seq: 1, label: "生词板", answer: "" });
  });
});
```

- [ ] **Step 2: Run it and watch it fail**

Run: `cd apps/lite-web && node_modules/.bin/vitest run src/reports/transcriptLines.test.ts`
Expected: FAIL — `Failed to resolve import "./transcriptLines"`.

- [ ] **Step 3: Write it**

```ts
// transcriptLines — 把一段存下来的对话摊平成「谁说了什么」。
//
// 这是回看那一屏唯一有判断的地方，所以它是一个纯函数，测试也只测这里。
//
// 三条规矩：
//  1. 顺序按 seq，不按数组顺序（接口回来的顺序不是承诺）。
//  2. 🚨 她的字一个都不截断。2026-09-12 的走查里她为此跟印记说了三次
//     「我的字被截断了」，然后重打了整段。看不见就是没有。
//  3. 带卡片的那一条渲染成一行静态说明，**不挂真卡片组件** —— 这一屏是只读的，
//     摆一张能点的卡片等于给她一颗按下去什么都不会发生的按钮。她当时填的答案
//     就是紧跟着的那条 student 消息，所以那条被这一行吸收掉，不再单独成条。
import { coachCardOf, type LiteMessage } from "@lite/api/readingRoom";

export type TranscriptLine =
  | { kind: "said"; who: "student" | "coach"; text: string; seq: number }
  | { kind: "card"; seq: number; label: string; answer: string };

/** 卡片类型 → 她在屏幕上看到过的那个名字。认不出来的类型退回一个通用词，
 *  而不是把这一条丢掉：丢掉会让对话里缺一段，她会以为是我们弄丢了。 */
export const CARD_LABELS: Record<string, string> = {
  choose_span: "挑句子",
  word_bank: "生词板",
  label_roles: "标注板",
  fill_blank: "填空",
  order_steps: "排顺序",
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
    const text = msg.content.trim();
    if (!text) continue;
    out.push({ kind: "said", who: msg.role === "student" ? "student" : "coach", text: msg.content, seq: msg.seq });
  }
  return out;
}
```

- [ ] **Step 4: Run it and watch it pass**

Run: `cd apps/lite-web && node_modules/.bin/vitest run src/reports/transcriptLines.test.ts`
Expected: PASS, 5 tests.

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/src/reports/transcriptLines.ts apps/lite-web/src/reports/transcriptLines.test.ts
git commit -m "feat(lite): 只读对话的摊平逻辑"
```

---

### Task 2: 对话那一屏

**Files:**
- Create: `apps/lite-web/src/reports/TranscriptView.tsx`
- Test: 无（纯渲染，按 house rule 不写 jsdom 断言；Task 11 用真浏览器看）

**Interfaces:**
- Consumes: `transcriptLines`（Task 1）
- Produces: `export function TranscriptView({ kind, atomId }: { kind: AtomKind; atomId: string })`

- [ ] **Step 1: 写组件**

```tsx
// TranscriptView — 她读完/写完之后回头看的那段对话。
//
// 只读是结构性的，不是禁用出来的：这一屏里没有输入框、没有重试、没有任何会发
// 请求的控件，连一条能写的路径都不存在。
//
// 取数走房间本来就有的那条接口（GET /readings/{id}/messages 与写作孪生），
// 不新增接口。失败时说一句实话并给一颗真能按的按钮 —— 一句她照做不了的提示
// 比不说更糟（2026-09-11 走查，她为「稍后刷新」找了四步不存在的按钮）。
import { useCallback, useEffect, useState } from "react";
import { apiErrorText } from "../api/errorText";
import { listReadingMessages } from "@lite/api/readingRoom";
import { listWritingMessages } from "@lite/api/writingRoom";
import type { AtomKind } from "@lite/api/reports";
import type { LiteMessage } from "@lite/api/readingRoom";
import { useAlive } from "@lite/shared/useAlive";
import { transcriptLines } from "./transcriptLines";
import { LiteChatMarkdown } from "../readings/LiteChatMarkdown";

export function TranscriptView({ kind, atomId }: { kind: AtomKind; atomId: string }) {
  const [msgs, setMsgs] = useState<LiteMessage[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const alive = useAlive();

  const load = useCallback(() => {
    setError(null);
    const p = kind === "reading" ? listReadingMessages(atomId) : listWritingMessages(atomId);
    p.then((m) => alive() && setMsgs(m)).catch((e) => alive() && setError(apiErrorText(e)));
  }, [kind, atomId, alive]);

  useEffect(load, [load]);

  if (error) {
    return (
      <div className="mk-rp-measure flex flex-wrap items-center gap-2 py-8">
        <p className="text-mk-body text-mk-danger">读取失败：{error}</p>
        <button type="button" onClick={load} className="rounded-mk-full border border-mk-border px-3 py-1 text-mk-small text-mk-secondary">
          再试一次
        </button>
      </div>
    );
  }
  if (!msgs) return <p className="mk-rp-measure py-8 text-mk-small text-mk-muted">处理中。</p>;

  const lines = transcriptLines(msgs);
  if (lines.length === 0) {
    return <p className="mk-rp-measure py-8 text-mk-body text-mk-muted">这一次没有留下对话。</p>;
  }

  return (
    <div className="mk-rp-measure flex flex-col gap-4 py-8">
      {lines.map((line) =>
        line.kind === "card" ? (
          <div key={line.seq} className="rounded-mk-lg border border-mk-border bg-mk-surface px-5 py-4">
            <p className="text-mk-label text-mk-faint">印记给了一张卡片 · {line.label}</p>
            {line.answer && <p className="mt-2 whitespace-pre-wrap text-mk-body text-mk-ink">{line.answer}</p>}
          </div>
        ) : (
          <div key={line.seq} className={line.who === "student" ? "flex flex-col items-end gap-1" : "flex flex-col gap-1"}>
            <span className="text-mk-label text-mk-faint">{line.who === "student" ? "我" : "印记"}</span>
            <div
              className="max-w-[46rem] rounded-mk-lg px-5 py-4 text-mk-body text-mk-ink"
              style={{ background: line.who === "student" ? "var(--mk-accent-50)" : "var(--mk-surface)" }}
            >
              {line.who === "student" ? (
                <p className="whitespace-pre-wrap">{line.text}</p>
              ) : (
                <LiteChatMarkdown text={line.text} />
              )}
            </div>
          </div>
        ),
      )}
    </div>
  );
}
```

- [ ] **Step 2: 确认取数函数的真实名字**

Run: `grep -n "export async function list.*Messages\|export function list.*Messages" apps/lite-web/src/api/readingRoom.ts apps/lite-web/src/api/writingRoom.ts`
如果名字不是 `listReadingMessages` / `listWritingMessages`，改成真名，**不要**新写一个接口函数。同样确认 `LiteChatMarkdown` 的 prop 名（可能是 `children` 或 `source`）与 `useAlive` 的路径。

- [ ] **Step 3: typecheck**

Run: `cd apps/lite-web && node_modules/.bin/tsc --noEmit`
Expected: 0 errors.

- [ ] **Step 4: Commit**

```bash
git add apps/lite-web/src/reports/TranscriptView.tsx
git commit -m "feat(lite): 只读的对话回看"
```

---

### Task 3: 完成页的三格切换器（阅读）

**Files:**
- Create: `apps/lite-web/src/reports/FinishedTabs.tsx`
- Modify: `apps/lite-web/src/readings/ReadingRoomHost.tsx`（`FinishedReadingPanel`）

**Interfaces:**
- Produces:
  ```tsx
  export type FinishedTab = "report" | "transcript" | "source";
  export function FinishedTabs({ tabs, active, onPick }: {
    tabs: { id: FinishedTab; label: string }[];
    active: FinishedTab;
    onPick: (id: FinishedTab) => void;
  }): JSX.Element;
  ```

- [ ] **Step 1: 写切换器**

```tsx
// FinishedTabs — 完成页上的三格。切的是同一页里的三屏，不进 URL：
// /readings/:id 已经是一条会被分享、会被教师端引用的深链接，多一段 ?view=
// 会让「同一条链接对不同人打开不同屏」变成新的一类 bug，而这三屏之间的切换
// 没有需要被链接的价值。
//
// 标签是名词（界面文案规则 1）。
export type FinishedTab = "report" | "transcript" | "source";

export function FinishedTabs({
  tabs,
  active,
  onPick,
}: {
  tabs: { id: FinishedTab; label: string }[];
  active: FinishedTab;
  onPick: (id: FinishedTab) => void;
}) {
  return (
    <div role="tablist" className="flex gap-1 rounded-mk-full border border-mk-border bg-mk-surface p-1">
      {tabs.map((t) => {
        const on = t.id === active;
        return (
          <button
            key={t.id}
            role="tab"
            aria-selected={on}
            type="button"
            onClick={() => onPick(t.id)}
            className="rounded-mk-full px-4 py-1.5 text-mk-small transition-colors duration-[120ms] ease-mk"
            style={on ? { background: "var(--mk-accent-100)", color: "var(--mk-accent-700)" } : { color: "var(--mk-muted)" }}
          >
            {t.label}
          </button>
        );
      })}
    </div>
  );
}
```

- [ ] **Step 2: 接进 `FinishedReadingPanel`**

在 `apps/lite-web/src/readings/ReadingRoomHost.tsx` 的 `FinishedReadingPanel` 里：

1. `import { useState } from "react";`（若尚未引入）、`import { FinishedTabs, type FinishedTab } from "../reports/FinishedTabs";`、`import { TranscriptView } from "../reports/TranscriptView";`、`import { ReadingArticle } from "../reports/ReadingArticle";`
2. 组件顶部加 `const [tab, setTab] = useState<FinishedTab>("report");`
3. 那条细横条（`已完成` chip 那一行）下面插入：

```tsx
<div className="mk-rp-measure pt-5">
  <FinishedTabs
    tabs={[
      { id: "report", label: "报告" },
      { id: "transcript", label: "对话" },
      { id: "source", label: "原文" },
    ]}
    active={tab}
    onPick={setTab}
  />
</div>
```

4. 现在的 `<ReportPanel …/>` + `<ReadingQuestions …/>` 整块包进 `{tab === "report" && (…)}`；
   再加 `{tab === "transcript" && <TranscriptView kind="reading" atomId={reading.id} />}`
   和 `{tab === "source" && <ReadingArticle readingId={reading.id} />}`。
5. 更新 `FinishedReadingPanel` 顶上那段 doc comment：它现在写着「READ-ONLY BY
   CONSTRUCTION … 没有房间，因此没有任何能改它的东西」。这一条**仍然成立**，
   要补一句说明新增的两屏同样只读、且没有任何会写的路径 —— 不要让注释和代码
   打架。

- [ ] **Step 3: 确认 `ReadingArticle` 的 props**

Run: `sed -n 1,40p apps/lite-web/src/reports/ReadingArticle.tsx`
按它真实的 props 传（可能要的是 blocks 而不是 readingId；如果它需要正文，就用
`getReadingSource(reading.id)` 取，和阅读室同一条路）。

- [ ] **Step 4: typecheck + 提交**

```bash
cd apps/lite-web && node_modules/.bin/tsc --noEmit
git add apps/lite-web/src/reports/FinishedTabs.tsx apps/lite-web/src/readings/ReadingRoomHost.tsx
git commit -m "feat(lite): 读完之后能回看对话和原文"
```

---

### Task 4: 完成页的三格切换器（写作）

**Files:**
- Modify: `apps/lite-web/src/writings/FinishedWritingPage.tsx:293` 一带

- [ ] **Step 1: 接进去**

和 Task 3 同样的三步，标签换成 `报告` / `对话` / `成稿`：`source` 那一格渲染这一页**本来就在渲染的成稿正文**（不新建组件，把它现有的正文块搬进那一格），`transcript` 渲染 `<TranscriptView kind="writing" atomId={writing.id} />`，`report` 包住现有的 `<ReportPanel kind="writing" atomId={writing.id} />`。

🚨 这一页有版本、修改/放弃修改、截止锁定这些真会写的控件（`lite-finished-writing-versions`）。**它们属于成稿那一格，不要被切换器藏掉，也不要复制到另外两格。**

- [ ] **Step 2: typecheck + 提交**

```bash
cd apps/lite-web && node_modules/.bin/tsc --noEmit
git add apps/lite-web/src/writings/FinishedWritingPage.tsx
git commit -m "feat(lite): 写完之后能回看对话"
```

---

### Task 5: `ordinal` — 这是第几篇

**Files:**
- Modify: `apps/api/internal/store/queries/atom.sql`、`apps/api/internal/api/atom_report.go`、`apps/lite-web/src/api/reports.ts`
- Test: `apps/api/internal/api/atom_report_internal_test.go`

**Interfaces:**
- Produces: `liteReportDTO.Ordinal int \`json:"ordinal,omitempty"\``；sqlc `CountFinishedReadingsByUser(ctx, userID) (int64, error)`、`CountFinishedWritingsByUser`

- [ ] **Step 1: 加查询**

在 `apps/api/internal/store/queries/atom.sql` 末尾：

```sql
-- name: CountFinishedReadingsByUser :one
-- 报告上那句「第 8 篇」的来源。数的是她完成过的篇数，在**生成这份报告的那个
-- 事务里**数一次，然后冻结进报告的 JSON —— 重新数会让她三个月前那份报告今天
-- 变成「第 20 篇」，那是在改她的过去。
SELECT count(*) FROM reading r
JOIN atom a ON a.id = r.atom_id
WHERE a.user_id = $1 AND r.status = 'finished';

-- name: CountFinishedWritingsByUser :one
SELECT count(*) FROM writing w
JOIN atom a ON a.id = w.atom_id
WHERE a.user_id = $1 AND w.status = 'finished';
```

Run: `cd apps/api && sqlc generate`（pin `@v1.27.0`）

- [ ] **Step 2: 写失败的测试**

`apps/api/internal/api/atom_report_internal_test.go` 追加：

```go
func TestReportOrdinalFreezesIntoTheStoredBlob(t *testing.T) {
	// 冻结的判据不是「函数返回几」，而是「存下来的 JSON 里有没有这个数」——
	// 报告是一整块存下来的 jsonb，序号一旦写进去就再也不会被重新数。
	dto := liteReportDTO{Version: 1, Kind: "reading", Ordinal: 8}
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	var back liteReportDTO
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Ordinal != 8 {
		t.Fatalf("ordinal did not survive the round trip: %d", back.Ordinal)
	}
	// 0 是「这份报告早于这个字段」，必须整个键缺席，而不是渲染成「第 0 篇」。
	if bytes.Contains(mustJSON(t, liteReportDTO{Version: 1}), []byte("ordinal")) {
		t.Error("a report with no ordinal must omit the key entirely")
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
```

- [ ] **Step 3: 跑，看它编译不过**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestReportOrdinal -timeout 1800s`
Expected: FAIL — `dto.Ordinal undefined`。

- [ ] **Step 4: 加字段并填上**

`liteReportDTO` 里，`FinishedAt` 之后：

```go
	// Ordinal 是她完成这一篇时已完成的篇数 —— 报告开场那句「第 8 篇」用的数。
	//
	// 存的是数字不是句子：报告是整块存下来的 JSON，一句话写进去就永远改不动了
	// （统计标签已经为此踩过一次，statLabels.ts 就是补丁）。人称也因此能由前端
	// 按语境定：她自己看是「我和印记一起读的第 8 篇」，公开页上访客看到的是
	// 「Phoebe 和印记一起读的第 8 篇」。
	//
	// omitempty：早于这个字段的报告整个键缺席，前端据此不渲染那一句，而不是
	// 渲染成「第 0 篇」。
	Ordinal int `json:"ordinal,omitempty"`
```

在 `buildReadingReportDTO` 里（`rd` 取出来之后）：

```go
	// 在这个事务里数一次，然后冻结。见 CountFinishedReadingsByUser 的注释。
	ordinal, err := qtx.CountFinishedReadingsByUser(ctx, userID)
	if err != nil {
		return liteReportDTO{}, err
	}
```
并在最后组装 DTO 时加上 `Ordinal: int(ordinal),`。`buildWritingReportDTO` 同理，用 `CountFinishedWritingsByUser`。

前端 `apps/lite-web/src/api/reports.ts`：`LiteReport` 加 `ordinal: number;`，`RawLiteReport` 把它列进可缺省的那一组，`normalizeReport` 里 `ordinal: raw.ordinal ?? 0`。

- [ ] **Step 5: 跑，绿**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestReportOrdinal -timeout 1800s`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/store/queries/atom.sql apps/api/internal/store/sqlc apps/api/internal/api/atom_report.go apps/api/internal/api/atom_report_internal_test.go apps/lite-web/src/api/reports.ts
git commit -m "feat(api): 报告记下这是她的第几篇"
```

---

### Task 6: 转折时刻 — 模型只回编号

**Files:**
- Modify: `apps/api/internal/api/atom_report.go`
- Test: `apps/api/internal/api/atom_report_internal_test.go`

**Interfaces:**
- Produces:
  ```go
  type reportTurningPoint struct {
      Turn    int    `json:"turn"`
      Why     string `json:"why"`
      Student string `json:"student"`
      Coach   string `json:"coach"`
  }
  func numberedTurns(msgs []sqlc.AtomMessage) []turnPair            // 她的发言 + 紧跟的那条回复
  func buildTurnsBlock(pairs []turnPair) string                      // 喂给模型的编号块
  func resolveTurningPoints(picks []modelTurnPick, pairs []turnPair) []reportTurningPoint
  ```

- [ ] **Step 1: 写失败的测试**

```go
func TestResolveTurningPointsTakesTextFromTheRowsNotTheModel(t *testing.T) {
	pairs := numberedTurns([]sqlc.AtomMessage{
		{Seq: 1, Role: "student", Content: "我觉得人均排放更能说明责任。"},
		{Seq: 2, Role: "ai", Content: "那总量还重要吗？"},
		{Seq: 3, Role: "system", Content: "工具已了结"},
		{Seq: 4, Role: "student", Content: "重要，但它们回答的是两个问题。"},
		{Seq: 5, Role: "ai", Content: "把这句写进结论试试。"},
	})
	if len(pairs) != 2 {
		t.Fatalf("numbered %d turns, want 2: %+v", len(pairs), pairs)
	}

	got := resolveTurningPoints([]modelTurnPick{
		{Turn: 2, Why: "她在这里把两个问题分开了"},
		{Turn: 2, Why: "重复的编号"},      // 重复 — 丢
		{Turn: 9, Why: "越界"},          // 不在窗口里 — 丢
		{Turn: 0, Why: "编号从 1 开始"},  // 越界 — 丢
		{Turn: 1, Why: "   "},          // 没有理由 — 丢
	}, pairs)

	if len(got) != 1 {
		t.Fatalf("kept %d, want 1: %+v", len(got), got)
	}
	// 逐字：正文来自行，不来自模型。
	if got[0].Student != "重要，但它们回答的是两个问题。" || got[0].Coach != "把这句写进结论试试。" {
		t.Errorf("text did not come from the rows verbatim: %+v", got[0])
	}
	if got[0].Why != "她在这里把两个问题分开了" {
		t.Errorf("why: %q", got[0].Why)
	}
}

func TestTurnsBlockNumbersHerTurnsAndSaysWhoSpoke(t *testing.T) {
	pairs := numberedTurns([]sqlc.AtomMessage{
		{Seq: 1, Role: "student", Content: "我觉得人均排放更重要。"},
		{Seq: 2, Role: "ai", Content: "为什么？"},
	})
	block := buildTurnsBlock(pairs)
	if !strings.Contains(block, "1.") || !strings.Contains(block, "她：") || !strings.Contains(block, "你：") {
		t.Fatalf("block does not read as a numbered transcript:\n%s", block)
	}
}

func TestTurningPointsSurviveAModelThatOnlyAnswersWithNumbers(t *testing.T) {
	reply, ok := parseReportReply(`{"moments":[],"gains":[],"summary":"",
		"turningPoints":[{"turn":3,"why":"她改了主意"}]}`)
	if !ok {
		t.Fatal("did not parse")
	}
	if len(reply.TurningPoints) != 1 || reply.TurningPoints[0].Turn != 3 {
		t.Fatalf("turningPoints: %+v", reply.TurningPoints)
	}
}
```

- [ ] **Step 2: 跑，看它失败**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestResolveTurningPoints|TestTurnsBlock|TestTurningPoints' -timeout 1800s`
Expected: FAIL — 这些标识符都还不存在。

- [ ] **Step 3: 实现**

在 `atom_report.go` 里（`reportMoment` 附近）：

```go
// reportTurningPoint 是对话里的一处转折：她说的那一句、印记接的那一句，
// 外加模型写的一句「这里发生了什么」。
//
// 🚨 **模型唯一能写的字是 Why。** Student 与 Coach 是服务端按编号从
// atom_message 里逐字取出来的。这是 (B) 方案的全部意义：让模型引对话原话，
// 它就会引她没说过的句子（2026-09-12 已经为此栽过一次，见
// prompt-twice-then-make-it-checkable）；让它只回编号，引错这件事**结构上
// 不存在**。改这一段之前先想清楚你是不是在把 (A) 方案偷偷放回来。
type reportTurningPoint struct {
	Turn    int    `json:"turn"`
	Why     string `json:"why"`
	Student string `json:"student"`
	Coach   string `json:"coach"`
}

// turnPair 是一次来回：她的一条，加紧跟着的第一条印记回复（可能没有）。
type turnPair struct {
	Student string
	Coach   string
}

// modelTurnPick 是模型允许回的形状 —— 一个编号加一句话，没有正文。
type modelTurnPick struct {
	Turn int    `json:"turn"`
	Why  string `json:"why"`
}

// numberedTurns 把一段对话摊成「她说一句、印记接一句」的序列，编号从 1 开始。
// system 那种记账消息不参与编号 —— 它不是任何人说的话。
func numberedTurns(msgs []sqlc.AtomMessage) []turnPair {
	sorted := append([]sqlc.AtomMessage(nil), msgs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Seq < sorted[j].Seq })
	var pairs []turnPair
	for i := 0; i < len(sorted); i++ {
		if sorted[i].Role != "student" || strings.TrimSpace(sorted[i].Content) == "" {
			continue
		}
		p := turnPair{Student: sorted[i].Content}
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Role == "student" {
				break
			}
			if sorted[j].Role == "ai" && strings.TrimSpace(sorted[j].Content) != "" {
				p.Coach = sorted[j].Content
				break
			}
		}
		pairs = append(pairs, p)
	}
	return pairs
}

// turnPromptCap 是**喂给模型**的每条上限。渲染到报告上的永远是完整原文：
// 截断只允许发生在 prompt 里，她看见的那一份一个字都不能少。
const turnPromptCap = 300

func buildTurnsBlock(pairs []turnPair) string {
	var b strings.Builder
	for i, p := range pairs {
		fmt.Fprintf(&b, "%d. 她：%s\n", i+1, capRunes(p.Student, turnPromptCap))
		if p.Coach != "" {
			fmt.Fprintf(&b, "   你：%s\n", capRunes(p.Coach, turnPromptCap))
		}
	}
	return b.String()
}

func capRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}

// resolveTurningPoints 把模型挑的编号换成行里的原文。越界、重复、没写理由的
// 全部丢掉，最多留 3 条。丢掉是静默的，因为这一节本来就是可有可无的：报告
// 永远不因为它失败（见文件头「a report must never be blocked on prose」）。
func resolveTurningPoints(picks []modelTurnPick, pairs []turnPair) []reportTurningPoint {
	seen := map[int]bool{}
	var out []reportTurningPoint
	for _, p := range picks {
		if p.Turn < 1 || p.Turn > len(pairs) || seen[p.Turn] {
			continue
		}
		why := strings.TrimSpace(p.Why)
		if why == "" {
			continue
		}
		seen[p.Turn] = true
		pair := pairs[p.Turn-1]
		out = append(out, reportTurningPoint{Turn: p.Turn, Why: why, Student: pair.Student, Coach: pair.Coach})
		if len(out) == 3 {
			break
		}
	}
	return out
}
```

`reportModelReply` 加一行 `TurningPoints []modelTurnPick \`json:"turningPoints"\``；
`reportProse` 加 `TurningPoints []reportTurningPoint`；
`liteReportDTO` 加 `TurningPoints []reportTurningPoint \`json:"turningPoints,omitempty"\``。

`generateReportProse` 的签名多收一个 `pairs []turnPair`，返回时填
`TurningPoints: resolveTurningPoints(reply.TurningPoints, pairs)`；两个 builder
在调用处传 `numberedTurns(msgs)`（写作那边传 `writingAllMessages` 的结果，和
corpus 用的是同一批消息）。

`buildReportPrompt` 多收 `turns string`，在 corpus 之后追加：

```go
	if strings.TrimSpace(turns) != "" {
		b.WriteString("\n【对话记录（只用来挑编号）】\n")
		b.WriteString(turns)
		b.WriteString("\n")
	}
```

`liteReportSystem` 在「你要做三件事」改成四件，并加第 4 条：

```
4. turningPoints：从【对话记录】里挑出最多 3 处**转折** —— 她改了主意的那一处、
   她问出关键问题的那一处、你指出她读错了而她接住了的那一处。**只回编号**：
   {"turn": 编号, "why": "这里发生了什么"}。why 一句话，说清楚**这一处为什么是
   转折**，不要复述她说了什么（她的原话会照原样印在旁边）。编号必须是【对话
   记录】里真实出现过的那个数字。挑不出来就给空数组，不要硬凑。
```

输出示例那一行改成：
```
{"moments":[...],"gains":[...],"summary":"...","turningPoints":[{"turn":3,"why":"..."}]}
```

- [ ] **Step 4: 跑，绿**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestResolveTurningPoints|TestTurnsBlock|TestTurningPoints' -timeout 1800s`
Expected: PASS

- [ ] **Step 5: 全量 Go 测试**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -timeout 1800s`
Expected: PASS（`buildReportPrompt` / `generateReportProse` 改了签名，调用点要跟着改）

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/atom_report.go apps/api/internal/api/atom_report_internal_test.go
git commit -m "feat(api): 报告记下对话里的转折时刻，正文取自行不取自模型"
```

---

### Task 7: 文章入口

**Files:**
- Modify: `apps/api/internal/api/atom_report.go`
- Test: `apps/api/internal/api/atom_report_internal_test.go`

**Interfaces:**
- Produces:
  ```go
  type reportArticle struct {
      SourceURL string `json:"sourceUrl,omitempty"`
      Host      string `json:"host,omitempty"`
      Excerpt   string `json:"excerpt,omitempty"`
  }
  func buildReportArticle(src sqlc.ReadingSource, blocks []string) *reportArticle
  ```

- [ ] **Step 1: 写失败的测试**

```go
func TestReportArticleNeverCarriesTheWholeBody(t *testing.T) {
	body := strings.Repeat("这是正文。", 400) // 2000 字
	got := buildReportArticle(sqlc.ReadingSource{Body: body, SourceUrl: ptr("https://www.nature.com/articles/x")}, SplitBlocks(body))
	if got == nil {
		t.Fatal("nil")
	}
	if len([]rune(got.Excerpt)) > reportExcerptCap {
		t.Fatalf("excerpt is %d runes, cap is %d", len([]rune(got.Excerpt)), reportExcerptCap)
	}
	if strings.Contains(got.Excerpt, body) {
		t.Error("the excerpt contains the whole body — the public page would be republishing the article")
	}
	if got.Host != "nature.com" {
		t.Errorf("host: %q", got.Host)
	}
}

func TestReportArticleIsAbsentWhenThereIsNoSource(t *testing.T) {
	if got := buildReportArticle(sqlc.ReadingSource{}, nil); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
}
```

（`ptr` 若仓库里已有同名助手就用现成的；没有就在测试文件里写 `func ptr[T any](v T) *T { return &v }`，先 grep 一次避免重名。）

- [ ] **Step 2: 跑，失败**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestReportArticle -timeout 1800s`

- [ ] **Step 3: 实现**

```go
// reportExcerptCap —— 公开页上这篇文章只能露这么多。
//
// 🚨 报告是她的记录，不是一次转载。分级阅读库是第三方素材，把正文整篇挂到一条
// 谁都能打开的链接上是另一回事。她自己划过的那些句子已经逐字摆在「我的笔记」
// 和「我用透镜查到的」两节里 —— 那才是这篇文章在这份报告上的分量。
const reportExcerptCap = 200

type reportArticle struct {
	SourceURL string `json:"sourceUrl,omitempty"`
	Host      string `json:"host,omitempty"`
	Excerpt   string `json:"excerpt,omitempty"`
}

func buildReportArticle(src sqlc.ReadingSource, blocks []string) *reportArticle {
	url := ""
	if src.SourceUrl != nil {
		url = strings.TrimSpace(*src.SourceUrl)
	}
	first := ""
	if len(blocks) > 0 {
		first = capRunes(blocks[0], reportExcerptCap)
	}
	if url == "" && first == "" {
		return nil
	}
	return &reportArticle{SourceURL: url, Host: hostOf(url), Excerpt: first}
}
```

`hostOf` 已经在 `pbl_site.go` 里（同一个 package），直接用；若它在别的 package，就照它的实现在这里复用，不要复制第二份解析。

`liteReportDTO` 加 `Article *reportArticle \`json:"article,omitempty"\``，
`buildReadingReportDTO` 组装时加 `Article: buildReportArticle(src, blocks)`（`src` 和 `blocks` 那里已经有了）。写作不填这一项。

- [ ] **Step 4: 跑，绿 + 全量**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -timeout 1800s`

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/atom_report.go apps/api/internal/api/atom_report_internal_test.go
git commit -m "feat(api): 阅读报告带上她读的那篇文章的入口"
```

---

### Task 8: 报告页把这三块渲染出来

**Files:**
- Modify: `apps/lite-web/src/reports/ReportView.tsx`、`apps/lite-web/src/api/reports.ts`

- [ ] **Step 1: 前端类型**

`reports.ts` 加：

```ts
export type ReportTurningPoint = { turn: number; why: string; student: string; coach: string };
export type ReportArticle = { sourceUrl?: string; host?: string; excerpt?: string };
```
`LiteReport` 加 `turningPoints: ReportTurningPoint[];` 与 `article: ReportArticle | null;`，
`RawLiteReport` 列进可缺省组，`normalizeReport` 填 `turningPoints: raw.turningPoints ?? []`、`article: raw.article ?? null`。

- [ ] **Step 2: 开场那一句**

`ReportView` 多一个可选 prop `viewer: "owner" | "guest"`（默认 `"owner"`），
在阅读分支的 `journal-cover` 里、标题上方渲染：

```tsx
<Opening ordinal={report.ordinal} name={report.studentName} viewer={viewer} kind={report.kind} />
```

```tsx
/** 开场那一句。ordinal 为 0 = 这份报告早于那个字段，整句不渲染 —— 「第 0 篇」
 *  比没有更糟。人称按语境定：她自己看是「我」，访客看到的是她的名字。 */
function Opening({ ordinal, name, viewer, kind }: { ordinal: number; name: string; viewer: "owner" | "guest"; kind: LiteReport["kind"] }) {
  if (ordinal <= 0) return null;
  const who = viewer === "owner" ? "我" : name;
  const verb = kind === "reading" ? "一起读的第" : "一起写的第";
  const unit = kind === "reading" ? "篇文章" : "篇文章";
  return <p className="mk-rp-opening text-mk-h3 text-mk-accent-700">这是{who}和印记{verb} {ordinal} {unit}！</p>;
}
```

- [ ] **Step 3: 数据带上移**

阅读分支里把 `<ReportVisualSummary stats={stats} />` 从 `journal-cover` **下面**移到
`journal-cover` 的**左栏内**（`Opening` + 标题 + 数据带同一列，插画在右栏）。
写作分支把它移到 `<Hero/>` 正下方，位置不变即可（那一支本来就在上部）。

- [ ] **Step 4: 转折时刻**

在 `<Moments/>` 之后插入 `<TurningPoints points={report.turningPoints} name={report.studentName} viewer={viewer} />`：

```tsx
/** 对话里的转折。每一块都标明两句话分别是谁说的 —— 这一节是这份报告上唯一
 *  同时印着她的话和印记的话的地方，混在一起就是把印记的话记在她名下。 */
function TurningPoints({ points, name, viewer }: { points: ReportTurningPoint[]; name: string; viewer: "owner" | "guest" }) {
  if (points.length === 0) return null;
  const me = viewer === "owner" ? "我" : name;
  return (
    <section className="flex flex-col gap-4">
      <SectionTitle>对话里的转折</SectionTitle>
      {points.map((p, i) => {
        const { bg, fg } = macaron(i + 4);
        return (
          <div key={p.turn} className="mk-rp-rise rounded-mk-lg px-6 py-6" style={{ background: bg, ...rise(i + 4) }}>
            <p className="text-mk-label" style={{ color: fg }}>{p.why}</p>
            <p className="mt-4 text-mk-label text-mk-faint">{me}</p>
            <p className="mt-1 whitespace-pre-wrap text-mk-body-lg text-mk-ink">{p.student}</p>
            {p.coach && (
              <>
                <p className="mt-4 text-mk-label text-mk-faint">印记</p>
                <p className="mt-1 whitespace-pre-wrap text-mk-body text-mk-secondary">{p.coach}</p>
              </>
            )}
          </div>
        );
      })}
    </section>
  );
}
```

- [ ] **Step 5: 文章入口**

在 `<NotesAndLenses/>` 之前插入：

```tsx
function ArticleEntry({ article, title }: { article: ReportArticle | null; title: string }) {
  if (!article) return null;
  return (
    <section className="rounded-mk-lg border border-mk-border bg-mk-surface px-6 py-5">
      <h2 className="text-mk-label text-mk-faint">我读的这篇</h2>
      <p className="mt-2 text-mk-h3 text-mk-ink">{title}</p>
      {article.excerpt && <p className="mt-2 text-mk-body text-mk-muted">{article.excerpt}</p>}
      {article.sourceUrl && (
        <a className="mt-3 inline-block text-mk-small text-mk-accent-700 underline" href={article.sourceUrl} target="_blank" rel="noreferrer noopener">
          原文{article.host ? ` · ${article.host}` : ""}
        </a>
      )}
    </section>
  );
}
```

- [ ] **Step 6: typecheck + 提交**

```bash
cd apps/lite-web && node_modules/.bin/tsc --noEmit && node_modules/.bin/vitest run
git add apps/lite-web/src/reports/ReportView.tsx apps/lite-web/src/api/reports.ts
git commit -m "feat(lite): 报告加开场、对话里的转折、我读的这篇"
```

---

### Task 9: `include_transcript` — 对话是否公开

**Files:**
- Create: `apps/api/internal/store/migrations/0174_atom_report_include_transcript.sql`
- Modify: `apps/api/internal/store/queries/atom.sql`、`apps/api/internal/api/atom_report_share.go`
- Test: `apps/api/internal/api/atom_report_share_internal_test.go` + 既有的 `TestPublicPayloadCarriesNothingExtra`

- [ ] **Step 1: 迁移**

```sql
-- +goose Up
-- 对话是否跟着这条分享链接一起公开。默认 false —— 她不选，就什么都没多出去。
--
-- 🚨 这一列推翻了 atom_report_share.go 顶上那条「no transcript」的旧裁定
-- （2026-09-16，产品负责人明确要她自己能选）。仍然成立的是另外三条：token
-- 不可猜、撤销立刻生效、没有第三件东西能顺带溜进公开负载。
ALTER TABLE atom_report ADD COLUMN include_transcript boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE atom_report DROP COLUMN include_transcript;
```

- [ ] **Step 2: 查询**

`SetAtomReportShare` 改成同时写这一位，并在撤销时归 false：

```sql
-- name: SetAtomReportShare :one
UPDATE atom_report
SET share_token = sqlc.narg(share_token),
    shared_at = CASE WHEN sqlc.narg(share_token)::text IS NULL THEN NULL ELSE now() END,
    -- 撤销 = 两件事一起收回。一条撤掉又重开的链接不该继承上一次的公开范围。
    include_transcript = CASE WHEN sqlc.narg(share_token)::text IS NULL THEN false ELSE sqlc.arg(include_transcript) END
WHERE atom_id = sqlc.arg(atom_id)
RETURNING *;
```

Run: `cd apps/api && sqlc generate`，然后修所有调用点（撤销那处传 `IncludeTranscript: false`）。

- [ ] **Step 3: 写失败的测试**

```go
func TestPublicPayloadOmitsTheTranscriptKeyWhenSheDidNotAskForIt(t *testing.T) {
	// 断在真的 JSON 上，不是断在结构体上：漏出去的是字节，不是字段。
	body := publicReportBody([]byte(`{"version":1,"kind":"reading"}`), nil)
	if bytes.Contains(body, []byte("transcript")) {
		t.Fatalf("a report shared without the transcript must not carry the key: %s", body)
	}
}

func TestPublicPayloadLabelsEveryLineWhenSheDidAskForIt(t *testing.T) {
	body := publicReportBody([]byte(`{"version":1,"kind":"reading"}`), []publicTranscriptLine{
		{Who: "student", Text: "我觉得人均排放更重要。"},
		{Who: "coach", Text: "为什么？"},
	})
	var got struct {
		Transcript []struct{ Who, Text string } `json:"transcript"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Transcript) != 2 || got.Transcript[0].Who != "student" || got.Transcript[1].Who != "coach" {
		t.Fatalf("every line must name who said it: %+v", got.Transcript)
	}
}
```

- [ ] **Step 4: 实现**

`atom_report_share.go`：

```go
// publicTranscriptLine —— 公开出去的那份对话里的一条。Who 永远在，永远不省略：
// 这是公开页上唯一同时印着她的话和印记的话的地方。
type publicTranscriptLine struct {
	Who  string `json:"who"` // "student" | "coach"
	Text string `json:"text"`
}

// publicReportBody 把存下来的报告字节和（可选的）对话拼成公开负载。
// transcript 为 nil 时**连键都不出现**。
func publicReportBody(report []byte, transcript []publicTranscriptLine) []byte { … }
```

`getPublicReport` 在 `row.IncludeTranscript` 为真时 `ListAtomMessages(ctx, row.AtomID)`，
用 Task 6 的 `numberedTurns` 之外的一条简单映射（`role == "student"` → `student`，
`role == "ai"` → `coach`，`system` 丢掉，空白丢掉）生成 lines。

分享接口 `POST .../report/share` 接受可选 body `{"includeTranscript": true}`，
已分享时只改这一位、**token 不变**。

- [ ] **Step 5: 改那段裁定注释**

`atom_report_share.go` 文件头第 3 条现在写着 payload「no transcript」。改写成记录
这次反转：日期、是谁定的、以及**仍然成立的那三条**。不要删掉它——一段说谎的注释
比没有注释更危险，一段记着「这里曾经是另一条规矩」的注释才是有用的。

- [ ] **Step 6: 跑全量 + 明确地改 `TestPublicPayloadCarriesNothingExtra`**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -timeout 1800s`
那条测试钉着公开负载的键集合，现在要把 `transcript` 加进允许集合，**并在测试里
写清楚为什么它可以在那儿**（她自己勾的，默认关，撤销即收回）。

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/store/migrations/0174_atom_report_include_transcript.sql apps/api/internal/store/queries/atom.sql apps/api/internal/store/sqlc apps/api/internal/api/atom_report_share.go apps/api/internal/api/atom_report_share_internal_test.go apps/api/internal/api/atom_report_share_test.go
git commit -m "feat(api): 对话是否公开由她自己勾"
```

---

### Task 10: 分享面板 + 公开页

**Files:**
- Modify: `apps/lite-web/src/reports/SharePanel.tsx`、`apps/lite-web/src/reports/ReportActions.tsx`、`apps/lite-web/src/reports/PublicReportPage.tsx`、`apps/lite-web/src/api/reports.ts`

- [ ] **Step 1: 分享做成一颗带字的按钮**

`ReportActions.tsx` 里的分享图标换成 `分享` 按钮（保留图标做前缀）。导出那颗照旧。

- [ ] **Step 2: 勾选**

`SharePanel` 的 `{phase:"on"}` 分支里，链接与二维码下面加：

```tsx
<label className="mt-4 flex items-start gap-2 text-mk-small text-mk-secondary">
  <input type="checkbox" checked={includeTranscript} onChange={(e) => void setTranscript(e.target.checked)} />
  <span>
    公开我和印记的对话
    <span className="block text-mk-faint">勾选之后，拿到这个链接的人能读到这次完整的对话。停止分享时一起收回。</span>
  </span>
</label>
```

`shareReport(kind, atomId, { includeTranscript })` 复用同一条接口；失败时把勾选状态**改回去**并显示后台原话（「记录失败：{后台原话}」），不要静默。

- [ ] **Step 3: 公开页渲染对话**

`PublicReportPage` 在 `view === "record"`（阅读是直接那一屏）下、报告之后渲染
`transcript`（接口带回来才有），每条标 `她` / `印记`。访客那一面用她的名字，不用「我」。
`ReportView` 传 `viewer="guest"`。

- [ ] **Step 4: typecheck + vitest + 提交**

```bash
cd apps/lite-web && node_modules/.bin/tsc --noEmit && node_modules/.bin/vitest run
git add apps/lite-web/src/reports apps/lite-web/src/api/reports.ts
git commit -m "feat(lite): 分享是一颗按钮，对话公开是一个勾选"
```

---

### Task 11: 发布过的作品列在主页上

**Files:**
- Modify: `apps/api/internal/store/queries/pbl_site.sql`、`apps/api/internal/api/pbl_site.go`、`apps/lite-web/src/site/PublicSitePage.tsx`、`apps/lite-web/src/mysite/MySitePage.tsx`

- [ ] **Step 1: 查询带上 token**

`ListSiteWritingsByUser` / `ListSiteReadingsByUser` 各 `LEFT JOIN atom_report rep ON rep.atom_id = a.id`
并多取 `COALESCE(rep.share_token,'')::text AS share_token`。注释里写明：**有 token
就是已发布**，不新建「发布」这张表。

Run: `cd apps/api && sqlc generate`

- [ ] **Step 2: 负载带上 `publicUrl`**

`pbl_site.go` 的 `SiteItem` 多一个 `PublicPath string`（`/s/<token>`，**相对路径**，
理由同 `SharePanel`：绝对地址由浏览器自己的 origin 拼，服务端猜的那个在代理后面
会猜错）。空 token 就空字符串。

- [ ] **Step 3: `/p/:token` 上的作品区**

`PublicSitePage` 在 iframe **外面**、`ProcessComparison` 同一层，加一块原生的
「作品」区，列出 `publicPath` 非空的那些，每张卡片是一个 `<a href={publicPath}>`。

🚨 **不要把它放进 iframe。** 那个 iframe 是 `sandbox="allow-scripts"`，里面的链接
根本跳不动（没有 `allow-popups`，也没有 `allow-top-navigation-by-user-activation`），
而为了让链接能用去放宽一个渲染模型生成 HTML 的沙箱，是拿安全换一行样式。

- [ ] **Step 4: 她自己那一面的发布开关**

`MySitePage` 的每篇完成作品上加一颗 `发布` / `停止发布`，走 Task 9 的同一条接口
（`shareReport` / `unshareReport`）。状态词用「已发布」/「未发布」。

- [ ] **Step 5: typecheck + 全量测试 + 提交**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/api -timeout 1800s
cd ../lite-web && node_modules/.bin/tsc --noEmit && node_modules/.bin/vitest run
git add apps/api/internal/store apps/api/internal/api/pbl_site.go apps/lite-web/src/site/PublicSitePage.tsx apps/lite-web/src/mysite/MySitePage.tsx
git commit -m "feat(lite): 发布过的作品列在主页上并且点得开"
```

---

### Task 12: 真的看一眼 + 真模型跑一次

**Files:**
- Create: `apps/lite-web/e2e/finished-history-walk.spec.ts`
- Delete: `apps/lite-web/harness.html`、`apps/lite-web/src/test/reportHarness.tsx`、`apps/lite-web/shot.mjs`（一次性脚手架）

- [ ] **Step 1: 截图**

用 `e2e/freshAccount.ts` 注册一个新账号（🚨 前提不能靠文件名字母序），走完一次
阅读，在完成页上截三张：**报告**、**对话**、**原文**；再分享一次，分别在
`include_transcript` 开与关的情况下打开 `/s/:token` 各截一张。

判据是**人眼**：报告的开场那句在不在、转折时刻有没有把两个人的话标开、公开页
在没勾选时**一个字的对话都没有**。上一次报告改版在 344 个绿测试底下导出了一张
全白的 PNG——这一步不能省。

- [ ] **Step 2: 真模型**

Run: `cd apps/api && LIVE_LLM=1 CGO_ENABLED=0 go test ./internal/api -run TestLiveReport -timeout 1800s`
（若没有这条 live 测试，就照 `internal/gateway` 里 `TestLive*` 的样子加一条，只断
三件事：`turningPoints` 回得来、`turn` 落在窗口里、`why` 不是空字符串。）

prompt 改了就要真跑一次 —— stub 测试只证明解析器读得懂**我自己写的** JSON。

- [ ] **Step 3: 删脚手架并提交**

```bash
git rm -f apps/lite-web/harness.html apps/lite-web/src/test/reportHarness.tsx apps/lite-web/shot.mjs
git add apps/lite-web/e2e/finished-history-walk.spec.ts
git commit -m "test(lite): 走一遍完成页的三格与两种公开范围"
```

---

## Self-Review

**Spec coverage** — 逐节对过：§1.1 三格 → Task 3/4；§1.2 对话屏 → Task 1/2；§1.3 原文屏 → Task 3；§2.1 ordinal → Task 5；§2.2 数据带上移 → Task 8 Step 3；§2.3 转折时刻 → Task 6 + Task 8 Step 4；§2.4 文章入口 → Task 7 + Task 8 Step 5；§3.1 分享按钮 → Task 10 Step 1；§3.2 勾选 → Task 9 + Task 10；§3.3 主页 → Task 11；§5 验证 → 各 task 内的测试 + Task 12。没有落空的节。

**Placeholder scan** — 无 TBD；每个要写代码的步骤都给了真代码。Task 2/10/11 的若干步是「按现有组件的真实 props 接线」，都给了先 grep 确认真名的动作，而不是让执行者猜。

**Type consistency** — `reportTurningPoint` / `modelTurnPick` / `turnPair` 在 Task 6 定义，Task 8 的前端 `ReportTurningPoint` 字段名与之逐字对应（turn/why/student/coach）；`reportArticle`（Go）↔ `ReportArticle`（TS）同样逐字对应；`FinishedTab` 在 Task 3 定义、Task 4 复用；`publicTranscriptLine.Who` 的两个取值 `student`/`coach` 与前端渲染分支一致。
