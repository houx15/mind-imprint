# 教师端空状态（Part E）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把教师端占整页／整段的空状态从一行灰字换成「插图 + 一句话 + 有下一步时给一颗按钮」，素材全部复用已有的 7 张 webp。

**Architecture:** 只动前端。`StudioEmpty`（`apps/lite-web/src/teacher/StudioArtwork.tsx`）加一个可选 `action`，然后把 5 处占整页／整段的空状态换过去。嵌在小面板、表格单元、下拉里的空状态**保持一行字**——插图挤进小格子会比一行字更糟。

**Tech Stack:** React + TypeScript + Vite（`apps/lite-web`），既有的 `Button`、`teacher-studio.css`。

**Spec:** `docs/superpowers/specs/2026-09-16-lite-teacher-workspace-design.md` §7

## Global Constraints

- 只写逻辑测试，不写前端渲染测试（不断言「标题渲染了／类名存在」）。本任务没有值得测的逻辑，**验收靠真浏览器截图**。
- 界面文案守 AGENTS.md §界面文案怎么写：标签是名词；按钮写「做什么／做完了」；状态用已／待／处理中；报错「动词+失败：{后台原话}」；不写文学腔（含代码注释与 commit message）。
- **不新增插图素材。** 只能用 `studentArtwork` 里已有的 7 个 key：`reading`、`writing`、`project`、`quest`、`discovery`、`ideas`、`keepsake`。
- `mk-*` token 不用 Tailwind alpha 语法（用 `color-mix`／`linear-gradient`）；不加左侧色条。
- lite 不得破坏 pro：不动 `apps/web`。

---

## File Structure

| 文件 | 责任 |
|---|---|
| `apps/lite-web/src/teacher/StudioArtwork.tsx` | `StudioEmpty` 加可选 `action` |
| `apps/lite-web/src/teacher/teacher-studio.css` | `.teacher-empty` 下按钮的间距 |
| `apps/lite-web/src/teacher/AssignmentsPage.tsx` | 2 处：暂无班级、暂无作业 |
| `apps/lite-web/src/teacher/AssignmentForm.tsx` | 1 处：暂无班级（整页那处） |
| `apps/lite-web/src/teacher/ParentReportsPage.tsx` | 2 处：暂无班级、暂无家长报告 |
| `apps/lite-web/src/teacher/StudentPage.tsx` | 2 处：暂无作业、暂无家长报告 |
| `apps/lite-web/src/teacher/ClassPage.tsx` | 1 处：已是 `StudioEmpty`，补一颗按钮 |

**不动的地方**（判据：嵌在小面板／表格单元／下拉里 → 保持一行字）：
`AssignmentForm.tsx` 的「暂无学生」（在 `RecipientChecklist` 面板里）、`AssignmentDetailPage.tsx` 的「暂无学生」与「暂无未布置的学生」、`LibraryPicker.tsx` 的「暂无文章」、`WeekSummaryCard.tsx` 的「暂无」、`OutputRecord.tsx` 的「暂无记录」。

---

### Task 1: 空状态换插图

**Files:**
- Modify: `apps/lite-web/src/teacher/StudioArtwork.tsx`
- Modify: `apps/lite-web/src/teacher/teacher-studio.css:53`
- Modify: `apps/lite-web/src/teacher/AssignmentsPage.tsx:117,128`
- Modify: `apps/lite-web/src/teacher/AssignmentForm.tsx:372`
- Modify: `apps/lite-web/src/teacher/ParentReportsPage.tsx:96,107`
- Modify: `apps/lite-web/src/teacher/StudentPage.tsx:244,327`
- Modify: `apps/lite-web/src/teacher/ClassPage.tsx:307`

**Interfaces:**
- Produces: `StudioEmpty({ children, kind?, action? })`，`action?: { label: string; onClick: () => void }`。D1 与后续部分都用这一个组件，不要再造第二个空状态组件。

- [ ] **Step 1: `StudioEmpty` 加 action**

`apps/lite-web/src/teacher/StudioArtwork.tsx` 现在是：

```tsx
export function StudioEmpty({ children, kind = "ideas" }: { children: React.ReactNode; kind?: keyof typeof studentArtwork }) {
  return <div className="teacher-empty"><img src={studentArtwork[kind]} alt="" /><p>{children}</p></div>;
}
```

改成：

```tsx
import { Button } from "@/ui/Button";
import { studentArtwork } from "../learning/StudentArtwork";

/** 教师端的空状态：插图 + 一句话 + 可选的一颗按钮。
 *  只给占整页或整段的空状态用；嵌在小面板、表格单元、下拉里的空状态保持一行字。 */
export function StudioEmpty({
  children,
  kind = "ideas",
  action,
}: {
  children: React.ReactNode;
  kind?: keyof typeof studentArtwork;
  action?: { label: string; onClick: () => void };
}) {
  return (
    <div className="teacher-empty">
      <img src={studentArtwork[kind]} alt="" />
      <p>{children}</p>
      {action && (
        <Button variant="primary" size="sm" onClick={action.onClick}>
          {action.label}
        </Button>
      )}
    </div>
  );
}
```

`StudioHeading` 保持原样，不要改。

- [ ] **Step 2: CSS 补按钮间距**

`apps/lite-web/src/teacher/teacher-studio.css` 第 53 行的 `.teacher-empty` 已经是 `flex-direction:column; gap:8px`，按钮会自然排在下面。只补一条让按钮与文字拉开一点：

```css
.teacher-empty button { margin-top:4px; }
```

写在 `.teacher-empty img` 那一条后面。不要新建 class 体系。

- [ ] **Step 3: 确认改动前先跑一次 typecheck 基线**

Run: `cd apps/lite-web && npx tsc --noEmit`
Expected: PASS（这是基线；后面每一步之后要保持 PASS）

- [ ] **Step 4: AssignmentsPage 两处**

`apps/lite-web/src/teacher/AssignmentsPage.tsx`，先 `import { StudioEmpty } from "./StudioArtwork";`。

第 117 行：

```tsx
) : classes.length === 0 ? (
  <div className="text-mk-body text-mk-muted">暂无班级</div>
```

改成：

```tsx
) : classes.length === 0 ? (
  <StudioEmpty kind="discovery">暂无班级。请联系管理员为你分配班级。</StudioEmpty>
```

第 128 行：

```tsx
) : rows.length === 0 ? (
  <div className="text-mk-body text-mk-muted">暂无作业</div>
```

改成（`onNew` 是这个页面顶部那颗「布置作业」按钮已经在用的回调，复用它，不要新写一个）：

```tsx
) : rows.length === 0 ? (
  <StudioEmpty kind="writing" action={{ label: "布置作业", onClick: () => onNew(classId) }}>
    这个班级还没有作业。
  </StudioEmpty>
```

🚨 打开文件确认顶部那颗按钮的回调名与参数**到底叫什么**，照抄；本步写的 `onNew(classId)` 是示意，名字不符就以文件里的为准。

- [ ] **Step 5: AssignmentForm 一处**

`apps/lite-web/src/teacher/AssignmentForm.tsx` 第 372 行：

```tsx
) : classes.length === 0 ? (
  <div className="mt-6 text-mk-body text-mk-muted">暂无班级</div>
```

改成：

```tsx
) : classes.length === 0 ? (
  <StudioEmpty kind="discovery">暂无班级。请联系管理员为你分配班级。</StudioEmpty>
```

**不要动**第 504 行的「暂无学生」——它在 `RecipientChecklist` 面板里，保持一行字。

- [ ] **Step 6: ParentReportsPage 两处**

第 96 行「暂无班级」→ `<StudioEmpty kind="discovery">暂无班级。请联系管理员为你分配班级。</StudioEmpty>`

第 107 行「暂无家长报告」→

```tsx
<StudioEmpty kind="keepsake">暂无家长报告。请在学生页面为单个学生生成报告。</StudioEmpty>
```

这一处**不给按钮**：生成入口在学生页，不在这一页，按钮点不到东西。

- [ ] **Step 7: StudentPage 两处**

第 244 行「暂无作业」→ `<StudioEmpty kind="writing">暂无作业</StudioEmpty>`

第 327 行「暂无家长报告」→ `<StudioEmpty kind="keepsake">暂无家长报告</StudioEmpty>`

这两处在 `.teacher-item-section` 里，CSS 已有的 `.teacher-item-section .teacher-empty { flex-direction:row; … }` 会把它们排成横向小图 + 文字，正是想要的效果，不用再改样式。

- [ ] **Step 8: ClassPage 补按钮**

第 307 行现在是：

```tsx
<StudioEmpty>{query.trim() || selectedDays !== null ? "未找到匹配的学生，请修改搜索条件。" : `暂无学生。请将邀请码 ${joinCode ?? "—"} 发给学生。`}</StudioEmpty>
```

保持文案，给「没有学生」那一支加一颗复制邀请码的按钮，并把插图分开——搜索无结果用 `discovery`，没有学生用 `quest`：

```tsx
{query.trim() || selectedDays !== null ? (
  <StudioEmpty kind="discovery">未找到匹配的学生，请修改搜索条件。</StudioEmpty>
) : (
  <StudioEmpty
    kind="quest"
    action={joinCode ? { label: "复制邀请码", onClick: () => void navigator.clipboard.writeText(joinCode) } : undefined}
  >
    {`暂无学生。请将邀请码 ${joinCode ?? "—"} 发给学生。`}
  </StudioEmpty>
)}
```

🚨 `joinCode` 为空时不给按钮——复制一个 `—` 没有意义。

- [ ] **Step 9: typecheck + 既有测试**

Run: `cd apps/lite-web && npx tsc --noEmit && npx vitest run`
Expected: typecheck PASS；既有测试全绿（本任务没有新增测试，见 Global Constraints）

- [ ] **Step 10: 真浏览器看一眼**

写一个一次性的 Playwright harness，登录教师账号，逐个截图这 5 个页面的空状态。

🚨 lite 的滚动在 `<main class="overflow-y-auto">` 里（`LiteApp.tsx:255,303`），`fullPage: true` 会裁——先把视口调大再截。

**看什么：** 插图不糊、不超出容器；文字与插图有间距；按钮在文字下面而不是压在插图上；窄屏（375px）下插图不撑破布局。

截图看过之后删掉 harness，不要留在仓库里。

- [ ] **Step 11: Commit**

```bash
git add apps/lite-web/src/teacher/StudioArtwork.tsx \
        apps/lite-web/src/teacher/teacher-studio.css \
        apps/lite-web/src/teacher/AssignmentsPage.tsx \
        apps/lite-web/src/teacher/AssignmentForm.tsx \
        apps/lite-web/src/teacher/ParentReportsPage.tsx \
        apps/lite-web/src/teacher/StudentPage.tsx \
        apps/lite-web/src/teacher/ClassPage.tsx
git commit -m "feat(lite-web): teacher empty states carry artwork and a next step"
```

---

## Self-Review

**Spec coverage（§7）：** 换 `StudioEmpty` ✓（Step 4–8）；`action` 可选 ✓（Step 1）；只用已有 7 张图 ✓；小面板保持一行字 ✓（File Structure 的「不动的地方」）；文案守 §3.6 ✓。

**Placeholder scan：** 无 TBD／TODO。Step 4 标出了要照抄真实回调名，并说明了原因，不是占位符。

**Type consistency：** `StudioEmpty` 的 `action` 形状在 Step 1 定义，Step 4、8 按同一形状使用；`kind` 取值全部落在 `studentArtwork` 的 7 个 key 内。
