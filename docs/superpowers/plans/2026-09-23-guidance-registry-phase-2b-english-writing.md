# 二期 b · 英文写作 —— 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让写英文议论文的学生在立题那一步先把题目拆成 TOPIC + TASK 两项任务；同时补上「改一个已有班级的年级」那两块界面，把二期 a 留下的半截功能做完。

**Architecture:** 教学正文进 `internal/prompts` 的一个新常量，由 `internal/guidance` 按 `{write, en, argument}` 登记成 `SlotCoach`，`writingPlanSystemFor` 用一个**可选**的槽（取不到就整节不出现，和阅读那边 `buildGenreCoachSection` 同一条纪律）把它填进模板新开的 `@@COACH@@` 洞。中文那三条分支一个字节都不变。年级那件事后端**已经做完了**（`patchClass` 收 grade、校验、落库、有测试），缺的只有 TS 客户端方法和两块界面。

**Tech Stack:** Go 1.26（`~/sdk/go1.26.0/bin/go`，`CGO_ENABLED=0`）、React + TypeScript + Vite、vitest。

**Spec:** `docs/superpowers/specs/2026-09-22-teaching-guidance-registry-design.md`（§6「二期 · 英文写作」、§6.5「第三方资料怎么用」、§8.6「二期 a 做完之后留下的东西」）

---

## Global Constraints

逐条抄自 spec 与 AGENTS.md，每个任务都受它们约束。

- **发给模型的字里只有指令。** 理由、出处、日期、谁提的意见一律写在 Go 注释里，不进提示词常量。AGENTS.md「提示词怎么写」§2。
- **陈述句，不是训斥。** 用「在什么情况下该怎么做」，不用「绝不许……」。§3。
- **不打比喻，直接说那个东西的名字。** §4；界面文案规则 10 同样适用于提示词。
- **术语只从注册表来。** 新正文里出现的块名必须逐字存在于 `WritingPlanEnglishArgumentKinds`（thesis statement / topic sentence / commentary / counterargument / refutation），方法名必须逐字存在于 `packages/contracts/vocab/methods.json`。§5。
- **不收那份资料里的填空模板。** `In contemporary society, xxx has become an increasingly widely discussed issue on social media.` 给她一句能直接粘进去的开头，就是替她把这篇写了一部分。本期**一个都不收**。spec §6「二期」第三条、§7。
- **教法照学，文字自己写。** `docs/reference/writing-teaching/` 下的资料是别人发布的作品，`英语作文批改/SKILL.md` 里还埋着 12 处水印与授权指纹。教学内容照收，句子不许整段粘进 `internal/prompts`。spec §6.5。
- **中文那三条分支的装配结果必须逐字节不变。** `zh/argument`、`zh/narrative`、`en/narrative` 三种组合的 `writingPlanSystemFor` 输出和今天完全一样；`cmd/promptinspect` 的 `bench/compose/lite-writing-plan` 基线哈希不许动。
- **印记仍然用中文跟她说话。** 换的是教的内容，不是说话的语言。术语用英文，解释用中文。
- **界面文案：标签是名词，按钮写「做什么」或「做完了」，状态用「已/待/中」。** 不写文学腔。AGENTS.md「界面文案怎么写」。
- **报错：动词+失败，再接后台原话。** 例：`修改年级失败：{后台原话}`。

## 🚨 门（每个碰前端的任务都要跑全这四条）

二期 a 的终审抓到两条红着的门而七个任务的评审一条都看不见（spec §8.6「门要开得比一个 app 宽」）：`apps/web` 的类型改动会打断 `apps/lite-web`（lite 的 `@/*` 解析到 `apps/web/src`），而整个二期 a 没有一个任务跑过 vitest。

```
cd apps/lite-web && npm run typecheck
cd apps/web && npm run typecheck
cd apps/web && npm run build     # vite build 不做类型检查
cd apps/web && npx vitest run
```

碰后端的任务：

```
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go build ./...
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./internal/guidance/... ./internal/prompts/... ./cmd/promptinspect/...
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./internal/api -run 'Writing|Grade|Class' -timeout 1800s
```

## 🚨 一条把 spec 的话改窄了的裁定（控制者 2026-09-23）

spec §8.6 写着「二期 b 的内容做出来，一个学生也收不到」。**那句话只有在二期 b 登记按年级分的内容时才成立** —— 我当时写得过头了。

本期**不登记任何一行 `Grades` 的教学内容**。理由：产品负责人对学段的原话是「by putting 小学作文 here I don't want to be compatible with 小学 now. we are still for 中学 ... it is only a reference」——他要的是建班时能记下学段，不是一套按年级分叉的教学内容。手上两份参考资料里，初中那份是中考提示作文 + 范文 + 模板的路子，和我们的 IB 方向对不上，照它分叉等于凭空造一套差异。

**由此而来的三条：**

1. `TestGradeDoesNotChangeAnythingYet` **留着**，仍然是真的。只把它注释里「这一期」改成「到二期 b 为止」。
2. `coverage_test.go` 的 want 表**仍按 `lang/genre` 两轴键**，不必拆成三轴。spec §8.6 坑 1 说的那件事本期不会发生 —— 但它逐字仍然有效，谁第一次加 `Grades` 行，谁先拆 want 表。
3. 年级那两块界面**照做**。它不是本期教学内容的前提，它是二期 a 的半截功能：`patchClass` 收 grade 却没有任何界面调得到它，而线上两个班都是批量导入建的、年级全是空串。**任何一期开始登记年级内容之前，这两块界面必须先在。**

## 🚨 另一件 spec 没说、但改变了任务形状的事

**线上两个班都是 lite 版。** 班级详情有**两块**界面，不是一块：

- `apps/web/src/console/ClassDetailView.tsx` —— pro 控制台，管理员用，能管所有班。
- `apps/lite-web/src/teacher/ClassPage.tsx` —— lite 教师端，**真正带这两个班的老师在这里**。

两块都要有。它们共用 `@/api` 的同一个客户端方法（lite 的 `@/*` 解析到 `apps/web/src`），所以客户端只写一份；界面各写各的，**控件不共用**：pro 用 `@/ui` 的 `Select`，lite 用 `./controls/Select`（memory「No native form controls」：lite 教师页不用浏览器原生控件）。

---

## 文件清单

**改：**

- `apps/web/src/api/classes.ts` —— 加 `setClassGrade`，加导出的 `CLASS_GRADE_OPTIONS`
- `apps/web/src/api/index.ts` —— `ApiClient` 接口 + import/export + `api` 对象
- `apps/web/src/console/ClassesView.tsx` —— 删掉本地那份 `GRADE_OPTIONS`，改成引用共用的那份
- `apps/web/src/console/ClassDetailView.tsx` —— `Client` 的 Pick 表 + 年级那一格
- `apps/lite-web/src/teacher/ClassPage.tsx` —— 年级那一格
- `apps/api/internal/prompts/api_writing_plan_lang.go` —— 新常量
- `apps/api/internal/prompts/api_writing_plan.go` —— 模板上开 `@@COACH@@`
- `apps/api/internal/prompts/catalog.go` —— 登记新常量
- `apps/api/internal/guidance/registry.go` —— 登记 `SlotCoach` 写作面那一行
- `apps/api/internal/api/writing_plan_prompt.go` —— 可选槽的取用
- `apps/api/internal/api/writing_grade.go` + `writing_plan.go` —— 两个班年级对不上时记一行日志
- `apps/api/internal/guidance/guidance_test.go`、`coverage_test.go` —— 测试名里的 `Stage` 改成 `Grade`
- `apps/api/internal/api/writing_grade_key_test.go` —— 注释改一句

**测试：**

- `apps/web/test/api/classes.test.ts`
- `apps/web/test/console/ClassDetailView.test.tsx`
- `apps/lite-web` 的教师端测试（见 Task 3 步骤 1：先确认这个仓库把它放在哪）
- `apps/api/internal/guidance/coverage_test.go`
- `apps/api/internal/api/writing_plan_internal_test.go`

---

### Task 1: 客户端方法 `setClassGrade` + 共用的年级选项表

**Files:**
- Modify: `apps/web/src/api/classes.ts`
- Modify: `apps/web/src/api/index.ts`
- Modify: `apps/web/src/console/ClassesView.tsx:10-18`（删本地副本）、`:134-145`（改引用）
- Test: `apps/web/test/api/classes.test.ts`

**Interfaces:**
- Produces: `setClassGrade(id: string, grade: string): Promise<ClassSummary>`，PATCH `/api/v1/classes/${id}`，body `{ grade }`。空串合法，表示清掉年级。
- Produces: `CLASS_GRADE_OPTIONS: { value: string; label: string }[]`，从 `@/api` 导出。第一项是 `{ value: "", label: "未填写" }`。
- Consumes: 后端 `patchClass`（`apps/api/internal/api/classes.go:152`）已经收 `grade *string`、已经校验（`validateClassGrade`）、已经落库（`SetClassGrade`）。**后端本任务一行都不用改。**

- [ ] **Step 1: 写失败的测试**

在 `apps/web/test/api/classes.test.ts` 里，紧挨着现有的 `"regenerateJoinCode PATCHes {regenerate_join_code:true}"` 那条之后加。这个文件已有两个辅助函数，**用它们，别另起一套 mock**：

```ts
const ok = (body: unknown, status = 200) => ...   // 第 6-7 行
const callOf = (spy: any, i = 0) => spy.mock.calls[i] as unknown as [string, RequestInit];
```

加这三条：

```ts
  it("setClassGrade PATCHes {grade}", async () => {
    const spy = ok({ class: { id: "c1", name: "11A", join_code: "AB-CD", school_id: "s1", created_at: "z", grade: "junior2", grade_label: "初二" } });
    vi.stubGlobal("fetch", spy);
    const c = await setClassGrade("c1", "junior2");
    expect(c.grade_label).toBe("初二");
    expect(callOf(spy)[0]).toContain("/api/v1/classes/c1");
    expect(callOf(spy)[1].method).toBe("PATCH");
    expect(callOf(spy)[1].body).toBe(JSON.stringify({ grade: "junior2" }));
  });

  // 🚨 空串是合法值，它的意思是「清掉年级」。写成 `if (grade)` 提前 return
  // 就会让「改回未填写」这个动作静默失败。
  it("setClassGrade sends an empty string to clear the grade", async () => {
    const spy = ok({ class: { id: "c1", name: "11A", join_code: "AB-CD", school_id: "s1", created_at: "z", grade: "", grade_label: "" } });
    vi.stubGlobal("fetch", spy);
    await setClassGrade("c1", "");
    expect(callOf(spy)[1].body).toBe(JSON.stringify({ grade: "" }));
  });

  it("CLASS_GRADE_OPTIONS starts with the empty option and covers the closed set", () => {
    expect(CLASS_GRADE_OPTIONS[0]).toEqual({ value: "", label: "未填写" });
    expect(CLASS_GRADE_OPTIONS.map((o) => o.value)).toEqual(
      ["", "junior1", "junior2", "junior3", "senior1", "senior2", "senior3"],
    );
  });
```

文件顶部的 import 是 `from "@/api/classes"`（**不是** `@/api`），把 `setClassGrade` 和 `CLASS_GRADE_OPTIONS` 加进那一串。

- [ ] **Step 2: 跑测试，确认它红**

```
cd apps/web && npx vitest run test/api/classes.test.ts
```

预期：红，报 `setClassGrade is not a function` 或 import 失败。

- [ ] **Step 3: 写实现**

在 `apps/web/src/api/classes.ts` 里，`ClassSummary` 接口正下方（第 11 行 `}` 之后）加：

```ts
// 与后端闭表（apps/api/internal/api/class_grade.go 的 classGrades）一致。
// 放在这里而不是某一个页面里：pro 的班级列表、pro 的班级详情、lite 的教师端
// 三处都要用同一份，而 lite 的 `@/*` 解析到 apps/web/src，所以它拿得到。
// 第一项代表「不填」—— 它是一个合法值，不是占位符。
export const CLASS_GRADE_OPTIONS: { value: string; label: string }[] = [
  { value: "", label: "未填写" },
  { value: "junior1", label: "初一" },
  { value: "junior2", label: "初二" },
  { value: "junior3", label: "初三" },
  { value: "senior1", label: "高一" },
  { value: "senior2", label: "高二" },
  { value: "senior3", label: "高三" },
];
```

在 `regenerateJoinCode` 那个函数正下方加：

```ts
// 🚨 grade 传空串是合法的，意思是把年级清掉（后端 validateClassGrade 认空串）。
// 不要在这里加 `if (!grade) return`。
export async function setClassGrade(id: string, grade: string): Promise<ClassSummary> {
  const r = await apiFetch<{ class: ClassSummary }>(`/api/v1/classes/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ grade }),
  });
  return r.class;
}
```

在 `apps/web/src/api/index.ts` 里四处改动：

1. 第 5-8 行那个 `import { ... } from "./classes";` 里，把 `setClassGrade` 和 `CLASS_GRADE_OPTIONS` 加进值的那一串。
2. `ApiClient` 接口里，`regenerateJoinCode(id: string): Promise<ClassSummary>;` 那一行下面加：
   ```ts
   setClassGrade(id: string, grade: string): Promise<ClassSummary>;
   ```
3. `export const api: ApiClient = {` 里，`listClasses, createClass, getClass, renameClass, regenerateJoinCode, removeEnrollment,` 那一行末尾加上 `setClassGrade,`。
4. 把选项表转出去给页面用：在 `export { ApiError } from "./client";` 附近加 `export { CLASS_GRADE_OPTIONS };`（第 1 步已经把它 import 进来了，所以用这种写法，**不要**再写一遍 `export { CLASS_GRADE_OPTIONS } from "./classes";` —— 两种只能留一种）。

- [ ] **Step 4: 把 `ClassesView.tsx` 改成引用共用的那份**

删掉 `apps/web/src/console/ClassesView.tsx` 第 10-18 行那个本地的 `const GRADE_OPTIONS = [...]`（连它上面那行注释一起删），改从这个文件现有的 api 来源引入 `CLASS_GRADE_OPTIONS`（**照它已有的 import 写法，是 `"../api"` 还是 `"@/api"` 照抄**），并把第 134-145 行那个 `<Select ... options={GRADE_OPTIONS} />` 改成 `options={CLASS_GRADE_OPTIONS}`。

🚨 **不要动那一段的其余部分。** 那里有一条注释解释为什么不给 `Select` 传 `placeholder`（共用 `Select` 会把 placeholder 渲染成一个 disabled 的 `value=""` 选项，排在调用方的选项前面，于是 `未填写` 永远显示不出来）。那条注释和那个可见的 `<label>年级</label>` 都留着。

- [ ] **Step 5: 跑测试，确认它绿**

```
cd apps/web && npx vitest run test/api/classes.test.ts test/console/ClassesView.test.tsx
```

预期：全绿。

🚨 **更正（控制者，Task 1 评审之后）**：这里原本写着「`ClassesView.test.tsx` 里那条 `grade-picker` 的测试必须仍然绿」—— **那条测试不存在**，我在没 grep 的情况下断言了它。真正盖住这次搬家的是 `create flow calls createClass and surfaces the new join code`（第 42-51 行），它断言 `createClass` 收到 `grade: ""`，间接证明 `CLASS_GRADE_OPTIONS[0].value === ""` 一路传对了。**那一条必须绿。**

- [ ] **Step 6: 跑全四条门**

```
cd apps/lite-web && npm run typecheck
cd apps/web && npm run typecheck
cd apps/web && npm run build
cd apps/web && npx vitest run
```

四条都必须绿。**lite 那条是真的会红的** —— 二期 a 就栽在它上面。

- [ ] **Step 7: 提交**

暂存这四个文件并提交，信息写：`feat(classes): 客户端能改年级了，选项表挪到共用的地方`

- `apps/web/src/api/classes.ts`
- `apps/web/src/api/index.ts`
- `apps/web/src/console/ClassesView.tsx`
- `apps/web/test/api/classes.test.ts`

（**只暂存这几个具体文件，不要 `git add -A`。**）

---

### Task 2: pro 控制台的班级详情能改年级

**Files:**
- Modify: `apps/web/src/console/ClassDetailView.tsx:8-20`（Pick 表）、`:216-227`（邀请码那一行下面）
- Test: `apps/web/test/console/ClassDetailView.test.tsx`

**Interfaces:**
- Consumes: Task 1 的 `setClassGrade(id, grade)` 与 `CLASS_GRADE_OPTIONS`。
- Consumes: `detail.class.grade`（内部取值）与 `detail.class.grade_label`（给人看的字，后端算好的）。两个字段**已经在 `ClassSummary` 上了**，这个文件今天一个都没读。

- [ ] **Step 1: 共用 `Select` 的事实（已核实，照这个写，不用再查）**

`apps/web/src/ui/forms.tsx:130-160`：它渲染的是**原生 `<select>`**，并且 `...rest` 原样摊到那个 `<select>` 上（`SelectProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, "value" | "onChange">`）。所以 `id` 和 `data-testid` 都能透传，`userEvent.selectOptions` 直接可用。

它的 `placeholder` 会渲染成 `<option value="" disabled>`，**排在调用方选项之前** —— 这就是为什么不给它传 `placeholder`。

- [ ] **Step 2: 写失败的测试**

在 `apps/web/test/console/ClassDetailView.test.tsx` 里。这个文件已有的夹具：`detail(over?)` 造 `ClassDetail`（班名是 `"11 年级 A"`，`grade` / `grade_label` 都已经是 `""`），`makeClient(d, roster?)` 造 client（一堆 `vi.fn()`），`noop`，用的是 `userEvent` 不是 `fireEvent`。

先在 `makeClient` 的返回对象里加一行（在 `regenerateJoinCode: vi.fn(),` 旁边）：

```ts
    setClassGrade: vi.fn(),
```

再在 `describe("ClassDetailView mutations")` 里加两条：

```tsx
  it("sets the grade and shows the new label", async () => {
    const client = makeClient(detail());
    client.setClassGrade.mockResolvedValue({ ...detail().class, grade: "junior2", grade_label: "初二" });
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} onOpenStudent={() => {}} onOpenReport={noop} />);
    await userEvent.selectOptions(await screen.findByTestId("class-grade-picker"), "junior2");
    await waitFor(() => expect(client.setClassGrade).toHaveBeenCalledWith("c1", "junior2"));
    expect((await screen.findByTestId("class-grade-picker")) as HTMLSelectElement).toHaveValue("junior2");
  });

  // 🚨「未填写」是一个可以选回去的值，不是一个不能选的占位符。
  // 给 Select 传了 placeholder 的话，这一条会红 —— 那正是它存在的理由。
  it("choosing 未填写 clears the grade", async () => {
    const client = makeClient(detail({ class: { ...detail().class, grade: "junior2", grade_label: "初二" } }));
    client.setClassGrade.mockResolvedValue({ ...detail().class, grade: "", grade_label: "" });
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} onOpenStudent={() => {}} onOpenReport={noop} />);
    await userEvent.selectOptions(await screen.findByTestId("class-grade-picker"), "");
    await waitFor(() => expect(client.setClassGrade).toHaveBeenCalledWith("c1", ""));
  });
```

- [ ] **Step 3: 跑测试，确认它红**

```
cd apps/web && npx vitest run test/console/ClassDetailView.test.tsx
```

预期：红，`class-grade-picker` 找不到。

- [ ] **Step 4: 写实现**

在 `apps/web/src/console/ClassDetailView.tsx` 里：

1. 第 6 行 `import { Button } from "@/ui";` 改成 `import { Button, Select } from "@/ui";`
2. 第 3 行 `import { ApiError } from "../api";` 改成 `import { ApiError, CLASS_GRADE_OPTIONS } from "../api";`
3. `type Client = Pick<ApiClient, ...>` 的那一串里，`| "regenerateJoinCode"` 下面加一行 `| "setClassGrade"`
4. `doRegen()` 函数（第 108-120 行）正下方加：

```tsx
  // 年级改了就直接存，不做「编辑 / 保存」两步 —— 这是一个七选一的下拉，
  // 没有可打错的中间态。失败时把后台原话摆出来（AGENTS.md 文案第 8 条）。
  async function doSetGrade(grade: string) {
    setMutationError(null);
    setBusy(true);
    try {
      const updated = await client.setClassGrade(classId, grade);
      setDetail((d) => (d ? { ...d, class: updated } : d));
    } catch (e) {
      setMutationError(e instanceof ApiError ? `修改年级失败：${e.message}` : "修改年级失败");
    } finally {
      setBusy(false);
    }
  }
```

5. 邀请码那一行（第 216-227 行那个 `<div style={{ display: "flex", ... marginTop: 14 ...}}>`）的**结束 `</div>` 之后**、`{mutationError && (` 之前，加：

```tsx
        <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 14 }}>
          <label htmlFor="class-grade-picker" style={{ fontSize: 13, fontWeight: 700, color: "var(--mk-ink)" }}>年级</label>
          {/* 🚨 不传 placeholder：共用 Select 会把它渲染成一个 disabled 的
              value="" 选项，排在 CLASS_GRADE_OPTIONS[0]（未填写）前面，于是
              「未填写」这个状态永远显示不出来。可见的 <label> 代替它。 */}
          <Select
            id="class-grade-picker"
            data-testid="class-grade-picker"
            value={c.grade}
            onChange={(v) => void doSetGrade(v)}
            options={CLASS_GRADE_OPTIONS}
          />
        </div>
```

`id` / `data-testid` 会经 `...rest` 落到原生 `<select>` 上（Step 1 已核实），不用额外包一层。

- [ ] **Step 5: 跑测试，确认它绿**

```
cd apps/web && npx vitest run test/console/ClassDetailView.test.tsx
```

- [ ] **Step 6: 跑全四条门**

```
cd apps/lite-web && npm run typecheck
cd apps/web && npm run typecheck
cd apps/web && npm run build
cd apps/web && npx vitest run
```

- [ ] **Step 7: 提交**

暂存 `apps/web/src/console/ClassDetailView.tsx` 与 `apps/web/test/console/ClassDetailView.test.tsx`，信息写：`feat(console): 班级详情能改年级`

---

### Task 3: lite 教师端的班级页能改年级

**Files:**
- Modify: `apps/lite-web/src/teacher/ClassPage.tsx`（state 约第 105-118 行、handler 约第 186 行 `doRegen` 之后、JSX 约第 283-308 行 `teacher-invite-strip` 里）
- Test: **本任务不加新测试。** 理由见 Step 1。

**Interfaces:**
- Consumes: Task 1 的 `api.setClassGrade` 与 `CLASS_GRADE_OPTIONS`（经 `@/api`，lite 的 `@/*` 解析到 `apps/web/src`）。
- Consumes: lite 自己的下拉 `Select`，来自 `./controls/Select`（`apps/lite-web/src/teacher/controls/Select.tsx`）。**不要用 `@/ui` 的那个** —— lite 教师页不用 pro 的控件，也不用浏览器原生控件（memory「No native form controls」）。它的签名是：
  ```ts
  Select<T extends string>({ value, options, onChange, ariaLabel?, placeholder?, disabled?, size?, className? })
  // options: { value: T; label: string }[]
  ```

- [ ] **Step 1: 为什么这一任务不写测试（已核实，不用再查）**

`apps/lite-web` 的教师端测试**全是纯逻辑**：`src/teacher/*.test.ts`（`assignmentLogic`、`rubricLogic`、`weekNav`、`teacherRouting`……）。顶层 `test/` 里那些 `.tsx` 渲染测试都是学生端的。`ClassPage` 今天一条测试都没有，教师端也没有任何页面级 render harness。

这一格新增的逻辑只有「把下拉选中的值原样发给后端，空串也发」——那已经由 Task 1 的两条客户端测试钉住了。为一个下拉在 lite 教师端开**第一条**页面渲染测试，正是 AGENTS.md 明令不做的那类（「不要为每个组件写『标题渲染了 / prop 传下去了』这类断言」）。

**替代的验证**：四条门 + 收尾时**真的打开那一屏看一眼**（AGENTS.md：UI 用真浏览器看，不靠 jsdom 断言；memory `look-at-the-image-not-the-assertions-2026-09-22`：绿是必要条件，不是结论）。收尾清单里有这一条。

也**不要**为此新建 `e2e/harness/` 下的看图台 —— 那五个都是学生端的面，为一个下拉再造一个教师端看图台不划算。

- [ ] **Step 2: 写实现**

在 `apps/lite-web/src/teacher/ClassPage.tsx` 里：

1. import：把 `CLASS_GRADE_OPTIONS` 并进现有的 `import { api } from "@/api";`，再加
   ```ts
   import { Select } from "./controls/Select";
   ```
2. state：`const [joinCode, setJoinCode] = useState<string | null>(null);` 下面加
   ```ts
   const [grade, setGrade] = useState("");
   ```
   并在**加载班级信息的那个 effect 里**（`api.getClass` 回来、`setName(...)` / `setJoinCode(...)` 的那几行旁边，约第 140-165 行）加一行把年级也读进来。变量名照那段现有代码里对班级对象的叫法。
3. `doRegen()` 正下方加：

```tsx
  // 年级是一个七选一的下拉，改了就存，不做两步确认。
  async function doSetGrade(next: string) {
    setMutationError(null);
    setBusy(true);
    try {
      const updated = await api.setClassGrade(classId, next);
      setGrade(updated.grade);
    } catch (e) {
      setMutationError(`修改年级失败：${errorText(e)}`);
    } finally {
      setBusy(false);
    }
  }
```

4. JSX：在 `teacher-invite-strip` 那个 `<div>` 里、`renaming` 为假的那一支中，`<span className="teacher-invite-spacer" />` **之前**加：

```tsx
                  <span style={{ display: "inline-flex", alignItems: "center", gap: 8 }}>
                    <span>年级</span>
                    <Select
                      value={grade}
                      options={CLASS_GRADE_OPTIONS}
                      onChange={(v) => void doSetGrade(v)}
                      ariaLabel="年级"
                      size="sm"
                      disabled={busy}
                    />
                  </span>
```

🚨 **不给 lite 的 `Select` 传 `placeholder`。** 它的 `placeholder` 只在选中项找不到时才显示，而 `CLASS_GRADE_OPTIONS[0]` 的 value 就是 `""`，所以空年级会正确显示成「未填写」—— 传了 placeholder 既没用，还多一个会漂移的字符串。

- [ ] **Step 3: 跑 lite 自己的测试，确认一条都没被碰坏**

```
cd apps/lite-web && npm run test
```

（lite 的 `test` 脚本就是 `vitest run`。）

- [ ] **Step 4: 跑全四条门**

```
cd apps/lite-web && npm run typecheck
cd apps/web && npm run typecheck
cd apps/web && npm run build
cd apps/web && npx vitest run
```

- [ ] **Step 5: 提交**

暂存 `apps/lite-web/src/teacher/ClassPage.tsx` 和你改/建的那个测试文件，信息写：`feat(lite-teacher): 班级页能改年级`

---

### Task 4: 英文议论文的题目拆解 —— 正文常量

**Files:**
- Modify: `apps/api/internal/prompts/api_writing_plan_lang.go`（文件末尾）
- Modify: `apps/api/internal/prompts/catalog.go:52`（`api.writingPlanEnglishNarrativeKinds` 那一行之后）
- Test: `apps/api/internal/prompts/catalog_test.go` 里已有的 `TestCatalogAndAssemblySourcesExist` 会自动覆盖，不用新写

**Interfaces:**
- Produces: `prompts.WritingPlanEnglishTaskSplit string` —— Task 5 会把它登记进 `guidance`。

**这一段正文的来源与边界：** 来自 `docs/reference/writing-teaching/english-writing/思维印记-英文写作逻辑框架搭建.md` 的「题目拆解」一节：TOPIC+TASK 两段式、四类 TASK、四类都可归约成 two tasks。**那份资料里的四段式整句模板一句都不收**（`In contemporary society, xxx has become...`）—— 给她一句能直接粘进去的开头就是替她写了一部分，spec §7 明令不收。这段话写进 Go 注释，不进提示词。

- [ ] **Step 1: 加常量**

在 `apps/api/internal/prompts/api_writing_plan_lang.go` 末尾追加：

> 🚨 **这一段是 2026-09-23 评审之后的第二版**（提交 `2c6046e2`）。第一版有三处会和
> 同一份装配好的提示词打架：无条件的次序、一句按自己那张表就不成立的普遍断言、
> 以及把 counterargument + refutation 叫成 topic sentence。理由写在常量上面的注释里。

```go
// WritingPlanEnglishTaskSplit 是英文议论文立题那一步多出来的一节：先把题目
// 拆成 TOPIC 和 TASK，再确认 TASK 有几项。
//
// 来源：docs/reference/writing-teaching/english-writing/思维印记-英文写作逻辑框架搭建.md
// 的「题目拆解」。同事给的那份资料里最有价值的就是这一段 —— 四类 TASK
// （agree / discuss / advantage / reason&solution）都可以归约成两项任务。
//
// 🚨 那份资料里的四段式**整句模板**一句都没收（`In contemporary society,
// xxx has become an increasingly widely discussed issue on social media.`）：
// 给她一句能直接粘进去的开头，就是替她把这篇写了一部分。雅思考官对背诵式
// spec §7「不收整句填空模板」。
//
// 🚨 只挂在 {write, en, argument} 上。中文议论文的题目不是这个形状，
// 英文记叙文没有 TASK 可拆。
//
// 🚨 2026-09-23 评审推翻了第一版的三处，都是这一节和它周围那份提示词打架：
//
//  1. 第一版写「讨论 thesis statement 之前，先和学生把这两部分分开」——
//     无条件的次序。而这一族里其余每个常量都带着条件闸（Skeleton 那两份：
//     「学生尚未说明观点时，不要求她先选结构名称」），WritingPlanSystem
//     的「你怎么问」更是被专门改成「已经说清的内容直接整理进图，不再要求
//     她换个说法重说」。一个开口就把观点和理由说全了的学生，会被这一节
//     拉回去做一遍拆题练习。现在改成按她说到哪儿分两支。
//  2. 第一版写「这四种都可以拆成两项任务」，而按它自己那张表，
//     discuss both views 写着三件事，advantages and disadvantages 的第三件
//     （哪边更重要）只在题目写 outweigh 时才要答。两种没给归约示范的恰好
//     就是有歧义的那两种，模型会自己挑一种，挑出来的图不一样。现在四种
//     逐条写清楚要回答几件事，并把那句普遍断言换成「数清楚这一道题要
//     回答几件事」——那才是这段资料真正有用的地方。
//  3. 第一版写「每一项任务在图上对应一条或一组 topic sentence」，但它自己
//     举的例子第二项是「反方的理由、为什么不成立」，那是 counterargument
//     加 refutation，不是 topic sentence。这正是同一份提示词里
//     「结合学生内容辨别节点类型」那一节要防的漂移。
const WritingPlanEnglishTaskSplit = `## 题目里的 TOPIC 与 TASK

英文议论文的题目由两部分组成：TOPIC 给出话题和语境，TASK 给出这篇要完成的任务。

学生还没说明这篇要回答什么时，你把题目分成这两部分说给她听，再用一个问题确认这个 TASK 要回答几件事。她已经说明了观点和理由，就直接整理进图，不要求她重做一遍这个拆分；这时把 TASK 当作对照表，看她的计划有没有漏掉其中一件。

常见的四种 TASK，以及每一种要回答的几件事：

- agree / disagree：你的立场（同意、不同意，或在什么范围内同意），以及为什么反方的理由不足以改变它。
- discuss both views and give your opinion：一方的理由、另一方的理由，以及自己的看法。题目写了 give your opinion，第三件就是必答的。
- advantages and disadvantages：优点和缺点。题目写成 Do the advantages outweigh the disadvantages 时，还要回答哪一边更重要；只写 Discuss 时不要求她下这个判断。
- reasons and solutions：原因，以及对应的办法。

数清楚这一道题要回答几件事，比判断它属于哪一类更要紧。

这几件事对应图上不同的节点类型：
- 她自己这一方的理由是 topic sentence。
- 反方的理由是 counterargument，她对它的回应是 refutation —— 这两样都不是 topic sentence。
- thesis statement 要覆盖题目要求的全部内容，不只是其中一件。

题目里限定的范围（人群、地点、时间）保留在 thesis statement 和 topic sentence 里，不要换成更大的说法。

学生自己命题、题目里没有明确的 TASK 时，请她说明这篇要回答哪个问题，再按同样的方式数清楚它包含几件事。`
```

- [ ] **Step 2: 登记进目录**

在 `apps/api/internal/prompts/catalog.go` 里 `api.writingPlanEnglishNarrativeKinds` 那一行（第 52 行）之后加：

```go
		{ID: "api.writingPlanEnglishTaskSplit", Source: "internal/prompts/api_writing_plan_lang.go", Consumer: "internal/api/writing_plan_prompt.go", Text: WritingPlanEnglishTaskSplit},
```

🚨 `Consumer` 写的是 `writing_plan_prompt.go`（`writingPlanSystemFor` 在那儿），不是 `writing_plan_lang_prompt.go`。`TestCatalogAndAssemblySourcesExist` 会 `os.Stat` 这两个路径，写错就红。

- [ ] **Step 3: 跑测试**

```
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./internal/prompts/...
```

预期：绿。

- [ ] **Step 4: 对一遍术语**

新正文里出现的块名只有 `thesis statement` 和 `topic sentence`，两个都逐字存在于 `WritingPlanEnglishArgumentKinds`。**新正文里一个方法名都没有**，所以不用对 `methods.json`。确认这一点：

```
cd apps/api && grep -n "并列论证\|对比论证\|留个悬念\|先承认，再反驳\|总—分\|起承转合" internal/prompts/api_writing_plan_lang.go
```

新常量那一段里一条都不该命中。

- [ ] **Step 5: 提交**

暂存 `apps/api/internal/prompts/api_writing_plan_lang.go` 与 `apps/api/internal/prompts/catalog.go`，信息写：`feat(prompts): 英文议论文的题目拆解 —— TOPIC 与 TASK`

---

### Task 5: 把它接进立题的提示词 —— 一个可选的槽

**Files:**
- Modify: `apps/api/internal/prompts/api_writing_plan.go`（模板上开 `@@COACH@@`）
- Modify: `apps/api/internal/guidance/registry.go`（写作面的 `SlotCoach` 一行）
- Modify: `apps/api/internal/api/writing_plan_prompt.go:56-81`（`writingPlanSystemFor`）
- Test: `apps/api/internal/guidance/coverage_test.go`、`apps/api/internal/api/writing_plan_internal_test.go`

**Interfaces:**
- Consumes: Task 4 的 `prompts.WritingPlanEnglishTaskSplit`。
- Consumes: `guidance.SlotCoach`（已存在，阅读面在用）、`guidance.Registry.Resolve`（取不到返回 error）。
- Produces: `writingPlanSystemFor(genre, lang, grade)` 的输出在 `en/argument` 上多一节；其余三种组合**逐字节不变**。

**🚨 为什么是「可选」：** `Resolve` 取不到返回 error 而不是空串，这是一期定下的纪律（空串在线上的样子是「印记话变少了」，查不出来）。但「这一篇没有这一节」是**正常**情况，不是故障 —— 阅读面的 `buildGenreCoachSection`（`internal/api/reading_genre.go`）已经是这个形状：议论文和认不出来的体裁本来就没有带读说明，取不到就返回空串。写作面照它办。

**🚨 为什么这样开洞不碰成本契约：** 这一节住在 **system prompt** 里，而 system prompt 在一篇文章的所有轮次之间不变（genre 和 lang 在一次写作里不变）。易变的东西（计时、还差多少字、板的状态）全在 user message 里，一个都没动。前缀没被打碎。

- [ ] **Step 1: 写失败的测试（装配那一条）**

在 `apps/api/internal/api/writing_plan_internal_test.go` 的 `TestWritingPlanSystemFor_EveryGenreAndLangAssembles` 里：

先把占位符检查那一行加上 `@@COACH@@`：

```go
			for _, ph := range []string{"@@KINDS@@", "@@MATERIAL@@", "@@SKELETON@@", "@@COACH@@", "%d"} {
```

再在 `// 文体不许串台。` 那一段**之前**加：

```go
			// 🚨 题目拆解只挂在英文议论文上。中文议论文的题目不是这个形状，
			// 英文记叙文没有 TASK 可拆 —— 串到那三条分支上就是给错教学内容，
			// 而她看不出来（印记仍然在用中文跟她说话）。
			const taskSplitMark = "TOPIC 与 TASK"
			wantTaskSplit := lang == langEnglish && genre == genreArgument
			if has := strings.Contains(got, taskSplitMark); has != wantTaskSplit {
				t.Errorf("%s：题目拆解那一节 在=%v，应该 在=%v", name, has, wantTaskSplit)
			}
```

🚨 用 `has :=`，**不要**写成 `got := strings.Contains(got, ...)` —— 那会遮蔽外层那个装着整段提示词的 `got`。

- [ ] **Step 2: 写失败的测试（注册表那两条）**

在 `apps/api/internal/guidance/coverage_test.go` 末尾加：

```go
// 🚨 写作面的 SlotCoach 只有一行，所以它的判据是**在哪几个组合上取得到**，
// 而不是「取到的非空」。取得到的地方多一个，就是一批学生悄悄换了教学内容。
func TestWriteCoachIsOnlyOnEnglishArgument(t *testing.T) {
	grades := []string{"", "junior1", "junior2", "junior3", "senior1", "senior2", "senior3"}
	for _, lang := range []string{"zh", "en", ""} {
		for _, genre := range []string{"argument", "narrative", "report", "explain", ""} {
			for _, grade := range grades {
				k := Key{Surface: SurfaceWrite, Lang: lang, Genre: genre, Grade: grade}
				got, err := Default().Resolve(k, SlotCoach)
				wantIt := lang == "en" && genre == "argument"
				if !wantIt {
					if err == nil {
						t.Errorf("%+v 取到了写作面的带读说明，但只有英文议论文该有", k)
					}
					continue
				}
				if err != nil {
					t.Errorf("%+v 该取到题目拆解，却取不到：%v", k, err)
					continue
				}
				if got[SlotCoach] != prompts.WritingPlanEnglishTaskSplit {
					t.Errorf("%+v 取到的不是题目拆解那一段", k)
				}
			}
		}
	}
}

// 🚨 两面不许串台。Scope.Matches 第一条比的就是 Surface，但在写作面加了
// SlotCoach 之前，没有任何东西测过「阅读面那三行还在不在」。
func TestReadCoachStillResolvesAfterWriteCoachWasAdded(t *testing.T) {
	want := map[string]string{
		"report":    prompts.ReadingCoachGenreReport,
		"explain":   prompts.ReadingCoachGenreExplain,
		"narrative": prompts.ReadingCoachGenreNarrative,
	}
	for genre, text := range want {
		// 阅读面的 Key 不带 Lang —— buildGenreCoachSection 就是这么调的。
		for _, lang := range []string{"", "zh", "en"} {
			k := Key{Surface: SurfaceRead, Lang: lang, Genre: genre}
			got, err := Default().Resolve(k, SlotCoach)
			if err != nil {
				t.Errorf("%+v 取不到带读说明：%v", k, err)
				continue
			}
			if got[SlotCoach] != text {
				t.Errorf("%+v 取到的不是 %s 该用的那一段", k, genre)
			}
		}
	}
	// 阅读面的议论文本来就没有带读说明 —— 加了写作面那一行之后仍然没有。
	if _, err := Default().Resolve(Key{Surface: SurfaceRead, Genre: "argument"}, SlotCoach); err == nil {
		t.Error("阅读面的议论文取到了带读说明，它本来就不该有")
	}
}
```

- [ ] **Step 3: 跑测试，确认它们红**

```
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./internal/guidance/... -run 'CoachIsOnly|ReadCoachStill'
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./internal/api -run 'EveryGenreAndLangAssembles' -timeout 1800s
```

预期：`TestWriteCoachIsOnlyOnEnglishArgument` 红（英文议论文取不到），装配那条红（`TOPIC 与 TASK` 不在）。

- [ ] **Step 4: 模板上开洞**

在 `apps/api/internal/prompts/api_writing_plan.go` 的 `WritingPlanSystem` 里，把

```
右边有一张思维导图，会随着她说的话逐步展开。你每轮说的话和你往图上加的节点，都出现在她眼前。

## 你怎么问
```

改成

```
右边有一张思维导图，会随着她说的话逐步展开。你每轮说的话和你往图上加的节点，都出现在她眼前。

@@COACH@@
## 你怎么问
```

🚨 `@@COACH@@` **自己单独一行**，它下面紧跟 `## 你怎么问`（中间没有空行）。这样取不到时删掉 `"@@COACH@@\n"` 整行，剩下的字节和今天**完全一样**；取到时换成正文，正文上面那个空行就是它和上一段之间的分隔。

- [ ] **Step 5: 登记那一行**

在 `apps/api/internal/guidance/registry.go` 里，`r.Add(SlotSkeleton, Scope{Surface: SurfaceWrite}, prompts.WritingPlanSkeletonZH)` 那一行之后、`// ── 阅读带读说明 ──` 那一段之前，加：

```go
	// 🚨 写作面的带读说明只有英文议论文有一行：题目拆解（TOPIC + TASK）。
	// 中文议论文的题目不是这个形状，两种记叙文没有 TASK 可拆 —— 所以这里
	// **不登记兜底行**。取不到不是故障，是这一篇本来就没有这一节，由
	// writingPlanSystemFor 把「没有」翻译成「整节不出现」。
	// 和阅读面的 SlotCoach 同一条纪律（议论文那边也没有登记）。
	r.Add(SlotCoach, Scope{Surface: SurfaceWrite, Lang: "en", Genres: []string{"argument"}},
		prompts.WritingPlanEnglishTaskSplit)
```

- [ ] **Step 6: 取用那一段**

在 `apps/api/internal/api/writing_plan_prompt.go` 的 `writingPlanSystemFor` 里，`english := lang == langEnglish` 那一行**之前**加：

```go
	// 这一篇的带读说明是**可选**的：今天只有英文议论文有一节（题目拆解）。
	// 取不到不是故障 —— 中文议论文和两种记叙文本来就没有这一节，所以这里
	// 不记日志、不兜底，直接把那一行占位符整行删掉。阅读面的
	// buildGenreCoachSection 是同一个形状。
	coach := ""
	if p, cerr := guidance.Default().Resolve(k, guidance.SlotCoach); cerr == nil {
		coach = p[guidance.SlotCoach]
	}
```

然后在三条 `strings.Replace` 之后、`s = strings.Replace(s, "%d", ...)` 之前，加：

```go
	if coach == "" {
		// 整行删掉 —— 留下一个空行会让中文那三条分支和今天差一个字节，
		// promptinspect 的基线会红。
		s = strings.Replace(s, "@@COACH@@\n", "", 1)
	} else {
		// 🚨 多补一个 \n。模板上那一行是 "@@COACH@@\n## 你怎么问"，而常量
		// 结尾是「……的句子。」没有换行 —— 直接替换会得到
		// "句子。\n## 你怎么问"，小标题前面没有空行，和这份提示词里其余
		// 每一处分节都不一样。补上之后是 "句子。\n\n## 你怎么问"。
		s = strings.Replace(s, "@@COACH@@", coach+"\n", 1)
	}
```

**核对过的事实**（控制者 2026-09-23 实测，不用再查）：模板里那一段今天逐字节是

```
……都出现在她眼前。\n\n## 你怎么问\n
```

开洞之后是 `……她眼前。\n\n@@COACH@@\n## 你怎么问\n`。所以：
- 取不到 → 删掉 `"@@COACH@@\n"` → 回到 `……她眼前。\n\n## 你怎么问\n`，**和今天一字节不差**。
- 取到了 → 换成 `coach+"\n"` → `……她眼前。\n\n<正文>\n\n## 你怎么问\n`，两侧各一个空行。

🚨 `k` 是函数开头已经建好的那个 `guidance.Key`，直接复用，别再造一个。

- [ ] **Step 7: 跑测试，确认它们绿**

```
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./internal/guidance/...
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./internal/api -run 'Writing|Grade' -timeout 1800s
```

全绿。特别确认这三条仍然绿：
- `TestWritingPlanSystemForLeavesNoPlaceholder`（成品里不许残留 `@@`）
- `TestGradeDoesNotChangeAnythingYet`（填不填年级结果一样 —— 本期仍然成立）
- `TestEveryCombinationResolves`（三个必需槽一个都没动）

- [ ] **Step 8: 🚨 parity —— 中文那条必须一个字节都没变**

```
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./cmd/promptinspect/...
```

**必须绿，而且不许重录基线。** `bench/compose/lite-writing-plan` 是中文议论文，它的 sha256 变了就说明 Step 4 的开洞或 Step 6 的删除写错了（多半是留了个空行）。真红了就去看渲染出来的字：

```
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go run ./cmd/promptinspect --help
```

（先看这个命令自己的参数怎么写，`cmd/promptinspect/README.txt` 里有。）

- [ ] **Step 9: 提交**

暂存这五个文件，信息写：`feat(guidance): 题目拆解接进立题 —— 一个可选的槽，中文那三条不变`

- `apps/api/internal/prompts/api_writing_plan.go`
- `apps/api/internal/guidance/registry.go`
- `apps/api/internal/guidance/coverage_test.go`
- `apps/api/internal/api/writing_plan_prompt.go`
- `apps/api/internal/api/writing_plan_internal_test.go`

---

### Task 6: 两处带着走的小账

**Files:**
- Modify: `apps/api/internal/api/writing_grade.go`（`gradeFromClasses` 的签名）
- Modify: `apps/api/internal/api/writing_plan.go:598` 附近（调用方 + 一行日志）
- Modify: `apps/api/internal/guidance/guidance_test.go`、`apps/api/internal/guidance/coverage_test.go`（测试名里的 `Stage` 改成 `Grade`）
- Modify: `apps/api/internal/api/writing_grade_key_test.go`（注释改一句）
- Modify: `apps/web/src/console/ClassesView.tsx`（一条注释里的过期常量名）
- Modify: `apps/web/src/console/ClassDetailView.tsx`（年级下拉补 `disabled={busy}`）
- Modify: `apps/web/test/console/OverviewView.test.tsx`（一条会偶尔红的判据）
- Test: `gradeFromClasses` 现有的测试文件（用 Step 1 的 grep 找）

这三件都是 spec §8.6「带着走的几条小账」里记下的，本期正好碰这几个文件。

- [ ] **Step 1: 先数一遍调用方**

```
cd apps/api && grep -rn "gradeFromClasses" internal/
```

记下每一处。下一步改签名，**每一处都要跟着改**。

- [ ] **Step 2: 年级对不上时报出来**

`gradeFromClasses` 在一个学生分属两个年级不同的班时返回 `""`（「不知道」，绝不挑一个）。这条分支今天**一声不响**。本期之后老师真的会开始填年级，所以值得留一行能查的记录。

把 `apps/api/internal/api/writing_grade.go` 里的 `gradeFromClasses` 改成：

```go
// gradeFromClasses —— 她在哪个年级。
//
// 两个班的年级对不上时返回「不知道」（空串）而不是挑一个：猜错是整篇按
// 错误年级教，而屏幕上毫无异常；不猜只是少一条线索，落回不分年级的内容。
//
// 第二个返回值区分「对不上」和「本来就没填」—— 调用方据此记日志。没有它，
// 一个同时在两个班里的学生会永远收不到年级内容，而没有人知道为什么。
func gradeFromClasses(grades []string) (string, bool) {
	found := ""
	for _, g := range grades {
		if g == "" {
			continue
		}
		if found == "" {
			found = g
			continue
		}
		if found != g {
			return "", true // 对不上 —— 不猜
		}
	}
	return found, false
}
```

生产调用点在 `apps/api/internal/api/writing_plan.go`（约第 598 行，`grade = gradeFromClasses(gradeRows)` 那一行），改成：

```go
		var conflict bool
		grade, conflict = gradeFromClasses(gradeRows)
		if conflict {
			slog.Info("writing plan turn: 学生在年级对不上的两个班里，按不分年级办",
				"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
```

🚨 **日志里不写年级的具体取值，也不写班级名** —— 那是学生的归属信息，`atom_id` 和 `request_id` 足够查（AGENTS.md：密钥与个人数据不进日志，这条按同样的克制办）。

Step 1 找到的测试调用点里，把 `got := gradeFromClasses(...)` 改成 `got, _ := gradeFromClasses(...)`。

- [ ] **Step 3: 给第二个返回值加一条测试**

加在 `gradeFromClasses` 现有测试的同一个文件里：

```go
func TestGradeFromClassesReportsTheDisagreement(t *testing.T) {
	if _, conflict := gradeFromClasses([]string{"junior2", "senior1"}); !conflict {
		t.Error("两个班年级对不上，应该报出来")
	}
	if _, conflict := gradeFromClasses([]string{"junior2", "junior2", ""}); conflict {
		t.Error("年级一致（外加一个没填的班），不该报对不上")
	}
	if _, conflict := gradeFromClasses(nil); conflict {
		t.Error("一个班都没有，不该报对不上")
	}
}
```

- [ ] **Step 4: 测试名里的 `Stage` 改成 `Grade`**

二期 a 把 `guidance.Key.Stage` 改名成了 `Grade`，但测试名没跟着改。

```
cd apps/api && grep -rn "Stage\|stage" internal/guidance/
```

把**测试函数名、变量名、注释里的措辞**里的 `Stage` / `stage` / 「学段」用词按实际含义改成 `Grade` / `grades` / 「年级」（band 相关的仍叫学段，那个词是对的）。例如 `TestStageScopedRowNeverMatchesUnknownStage` → `TestGradeScopedRowNeverMatchesUnknownGrade`，`coverage_test.go` 里那个 `stages := []string{...}` → `grades`。

🚨 **只改名字，不改任何一条断言。** 这是一次纯改名；哪条测试的行为变了，就是改错了。

- [ ] **Step 5: 把那条验收测试的注释改准**

`apps/api/internal/api/writing_grade_key_test.go` 顶上写着「这一期不加任何一行年级专属的内容」。改成：

```go
// 🚨 到二期 b 为止，注册表里**一行按年级分的内容都没有**，所以填不填年级、
// 填哪个年级，结果必须一模一样。这条测试是二期 a 的验收，二期 b 复核过一次
// 仍然成立（见二期 b 计划里那条裁定：学段今天只用来记录，不用来分叉教学内容）。
//
// 第一个人登记 Grades 行的时候，这条测试会红 —— 那是计划内的报废，不是回归。
// 同时要做的是把 coverage_test.go 的 want 表从 lang/genre 两轴改成三轴，
// 否则那一行内容会输给已在的文体行（26 分对 28 分），一次都不出现而测试全绿。
```

- [ ] **Step 6: pro 那个年级下拉在存盘途中没有禁用**

Task 2 的评审发现：`apps/web/src/console/ClassDetailView.tsx` 里新加的那个 `<Select>` 没有 `disabled={busy}`，而同一个文件里的「确认轮换」「保存」都有，lite 那块（Task 3）也有。

给它补上 `disabled={busy}`：

```tsx
          <Select
            id="class-grade-picker"
            data-testid="class-grade-picker"
            value={c.grade}
            onChange={(v) => void doSetGrade(v)}
            options={CLASS_GRADE_OPTIONS}
            disabled={busy}
          />
```

**其余属性一个都不动** —— 尤其不要顺手加 `placeholder`。

- [ ] **Step 7: 一条真的会偶尔红的测试**

`apps/web/test/console/OverviewView.test.tsx` 第一条测试**判据站在数据到达之前**：

```tsx
    expect(await screen.findByText("概览")).toBeInTheDocument();   // 无条件渲染的字
    expect(screen.getByText("120")).toBeInTheDocument();           // 要等 getOverview 落地
```

「概览」在 `{data && ...}` **外面**，所以那个 `await` 可以在 mock 的 `getOverview()` 落地、`setData` 提交重渲染**之前**就满足；紧跟的同步 `getByText("120")` 于是撞上还没到的状态。Task 3 跑全量时它红过一次，单跑五次都绿 —— 机理是实的，并行跑满时会偶尔红。

改成先等**数据来了才有**的那个字：

```tsx
    expect(await screen.findByText("120")).toBeInTheDocument();
    expect(screen.getByText("概览")).toBeInTheDocument();
```

其余三条 `getByText` 不动 —— 它们和 `120` 在同一次渲染里。
**第二条测试不用动**，它等的 `暂无用量。` 本来就是数据依赖的。

这不是本期改坏的东西，是路过时看见的。AGENTS.md / memory：整个产品都归你，不只是你今天改的那块。

- [ ] **Step 8: 一条过期的注释**

`apps/web/src/console/ClassesView.tsx` 里那条解释「为什么不给 Select 传 placeholder」的注释，正文里还写着旧名 `GRADE_OPTIONS[0]`。Task 1 把那个常量改名成了 `CLASS_GRADE_OPTIONS` 并挪到了 `api/classes.ts`，注释没跟着改（Task 1 的 brief 明说别动那一块的其余部分，所以那时不改是对的）。

把注释里的 `GRADE_OPTIONS[0]` 改成 `CLASS_GRADE_OPTIONS[0]`。**只改这一处文字，代码一行不动。**

改完跑一次前端的门（这是本任务唯一碰前端的一步）：

```
cd apps/web && npm run typecheck
cd apps/web && npx vitest run
```

- [ ] **Step 9: 跑全后端的门**

```
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go build ./...
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./internal/guidance/... ./internal/prompts/... ./cmd/promptinspect/...
cd apps/api && CGO_ENABLED=0 ~/sdk/go1.26.0/bin/go test ./internal/api -run 'Writing|Grade|Class' -timeout 1800s
```

- [ ] **Step 10: 提交**

暂存改过的那几个文件（含 `apps/web/src/console/ClassesView.tsx`），信息写：`chore(grade): 年级对不上时记一行；测试名里的 Stage 改成 Grade`

---

## 收尾（控制者做，不派给实施者）

- [ ] 全门一遍：四条前端 + 三条后端。
- [ ] 🚨 **真的打开那两屏看一眼**：pro 的班级详情、lite 教师端的班级页。Task 3 没有测试挡着，这一条就是它的验证。看三件事：年级那一格在不在、空年级显示的是不是「未填写」、选一个之后有没有变。
      （AGENTS.md：UI 用真浏览器看；memory `look-at-the-image-not-the-assertions-2026-09-22`：绿是必要条件，不是结论。）
- [ ] 整支终审（whole-branch review），模型用最强的那一档。
- [ ] 🚨 `LIVE_LLM` 一次：新提示词上线前要跑一次真模型（memory `prompt-output-must-be-verifiable-2026-09-03`）。
      判据：拿一道 discuss-both-views 的英文题目走一轮立题，看印记有没有**先把题目拆成两项任务**、有没有**把整句开头塞给她**。
      key 不在环境里就跳过，并在收尾里写明「没跑」—— 不要写成跑过了。
- [ ] 把二期 b 留下的东西写进 spec §8.7。
