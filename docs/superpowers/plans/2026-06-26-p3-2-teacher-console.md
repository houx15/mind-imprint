# P3.2 · Teacher/Admin Class Console Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A frontend console that teachers and admins land on after login (students keep the existing workspace app), built on the P3.1 `teacherOrAdmin` class endpoints — classes list, create (teacher-only), and a class-detail roster of aggregate-only activity signals.

**Architecture:** Add a typed API client for the class endpoints. Refactor `AppShell` to route on role: `student` → an extracted `StudentApp` (today's UI, behavior-preserving), `teacher`/`admin` → a new `ConsoleShell` mirroring the student left-rail (班级 / 设置). The console composes leaf views (`ConsoleRail`, `ClassesView`, `ClassDetailView`) and reuses the existing `SettingsView`. Build leaves first, integrate `AppShell` last.

**Tech Stack:** React 18 + TypeScript, Vite, Vitest + @testing-library/react, the existing `apiFetch` client (cookie session, shared error envelope). Inline-style visual language reused from the student `.dc.html`.

## Global Constraints

- **Aggregate-only (iron law):** the roster shows activity *signals* (counts, timestamps) only — never student work contents. Roster rows are **non-clickable**; there is no student-work detail page. A caption states this.
- **Create is teacher-only:** the backend `POST /classes` requires `teacher_user_id` when the caller is an admin (returns `400 需要指定班级教师`). The `[+ 新建班级]` action is rendered for `role === "teacher"` only. Admin class-creation defers to P3.3.
- **DTO field names match `apps/api/internal/api/classes_dto.go` verbatim.** `classDTO`: `{ id, name, join_code, school_id, created_at }`. `rosterEntryDTO`: `{ id, display_name, email, last_active_at (string|null), task_count, evaluation_count, card_count }`. The list payload carries **no** student count or last-active.
- **Reuse the student palette and `LeftRail` idiom** — no new design system, no CSS framework changes. Inline styles with the existing hex tokens (`#2A3B7A`, `#1C2333`, `#8A92A3`, `#EAECF2`, `#F3F4F8`, `#E8A33D`, `#3B5BDB`, `#C76B6B`).
- **Client never holds secrets nor calls models** (project iron law).
- **Tests:** Vitest + @testing-library/react; inject a fake `client` (no network) for component tests; stub `fetch` only for the api-client unit test. Run from `apps/web` with `pnpm test`.
- **Never stage the repo-root `package.json`** (pre-existing `M`). Use explicit `git add` paths.
- **Commit trailer:** end every commit message with `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`.

---

## File Structure

- `apps/web/src/api/classes.ts` (new) — typed client functions + `ClassSummary` / `RosterStudent` / `ClassDetail` types.
- `apps/web/src/api/index.ts` (modify) — extend `ApiClient` interface + `api` object with the six class methods.
- `apps/web/src/console/time.ts` (new) — `shortDate` + `relativeTime` pure helpers.
- `apps/web/src/console/ConsoleRail.tsx` (new) — left rail (班级 / 设置), styled like `LeftRail`.
- `apps/web/src/console/ClassesView.tsx` (new) — classes list, role-aware header, empty state, teacher-only create flow.
- `apps/web/src/console/ClassDetailView.tsx` (new) — roster signal table, join-code chip + copy, back, empty roster (Task 4); rename / regenerate / remove mutations (Task 5).
- `apps/web/src/console/ConsoleShell.tsx` (new) — composes rail + views + `SettingsView`; owns tab + view state.
- `apps/web/src/shell/StudentApp.tsx` (new) — the current student tab UI extracted from `AppShell` (behavior-preserving).
- `apps/web/src/shell/AppShell.tsx` (modify) — boot + auth gate, then role-route to `StudentApp` or `ConsoleShell`.
- `docs/遗留项追踪_Carryforward.md` (modify) — append the P3.2 carry-forward section (Task 7).

Test files sit beside each module (`*.test.ts` / `*.test.tsx`).

---

### Task 1: Class API client

**Files:**
- Create: `apps/web/src/api/classes.ts`
- Create: `apps/web/src/api/classes.test.ts`
- Modify: `apps/web/src/api/index.ts`

**Interfaces:**
- Consumes: `apiFetch<T>(path, init?)` from `./client`.
- Produces:
  - `interface ClassSummary { id: string; name: string; join_code: string; school_id: string; created_at: string }`
  - `interface RosterStudent { id: string; display_name: string; email: string; last_active_at: string | null; task_count: number; evaluation_count: number; card_count: number }`
  - `interface ClassDetail { class: ClassSummary; roster: RosterStudent[] }`
  - `listClasses(): Promise<ClassSummary[]>`
  - `createClass(input: { name: string }): Promise<ClassSummary>`
  - `getClass(id: string): Promise<ClassDetail>`
  - `renameClass(id: string, name: string): Promise<ClassSummary>`
  - `regenerateJoinCode(id: string): Promise<ClassSummary>`
  - `removeEnrollment(id: string, userId: string): Promise<void>`

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/api/classes.test.ts` (mirrors `api/tasks.test.ts` — stub `fetch` with a `Response`):

```ts
import { describe, it, expect, vi } from "vitest";
import {
  listClasses, createClass, getClass, renameClass, regenerateJoinCode, removeEnrollment,
} from "./classes";

const ok = (body: unknown, status = 200) =>
  vi.fn(async () => new Response(status === 204 ? null : JSON.stringify(body), { status }));
const callOf = (spy: ReturnType<typeof vi.fn>, i = 0) =>
  spy.mock.calls[i] as unknown as [string, RequestInit];

describe("classes api", () => {
  it("listClasses unwraps {classes}", async () => {
    vi.stubGlobal("fetch", ok({ classes: [{ id: "c1", name: "11A", join_code: "AB-CD", school_id: "s1", created_at: "2026-06-26T00:00:00Z" }] }));
    const out = await listClasses();
    expect(out).toHaveLength(1);
    expect(out[0]!.id).toBe("c1");
  });

  it("createClass POSTs {name} and unwraps {class}", async () => {
    const spy = ok({ class: { id: "c2", name: "New", join_code: "EF-GH", school_id: "s1", created_at: "z" } });
    vi.stubGlobal("fetch", spy);
    const c = await createClass({ name: "New" });
    expect(c.join_code).toBe("EF-GH");
    expect(callOf(spy)[0]).toContain("/api/v1/classes");
    expect(callOf(spy)[1].method).toBe("POST");
    expect(callOf(spy)[1].body).toBe(JSON.stringify({ name: "New" }));
  });

  it("getClass returns {class, roster}", async () => {
    vi.stubGlobal("fetch", ok({
      class: { id: "c1", name: "11A", join_code: "AB-CD", school_id: "s1", created_at: "z" },
      roster: [{ id: "u1", display_name: "Phoebe", email: "p@d", last_active_at: null, task_count: 0, evaluation_count: 0, card_count: 0 }],
    }));
    const d = await getClass("c1");
    expect(d.roster[0]!.last_active_at).toBeNull();
  });

  it("renameClass PATCHes {name}", async () => {
    const spy = ok({ class: { id: "c1", name: "Renamed", join_code: "AB-CD", school_id: "s1", created_at: "z" } });
    vi.stubGlobal("fetch", spy);
    const c = await renameClass("c1", "Renamed");
    expect(c.name).toBe("Renamed");
    expect(callOf(spy)[1].method).toBe("PATCH");
    expect(callOf(spy)[1].body).toBe(JSON.stringify({ name: "Renamed" }));
  });

  it("regenerateJoinCode PATCHes {regenerate_join_code:true}", async () => {
    const spy = ok({ class: { id: "c1", name: "11A", join_code: "ZZ-ZZ", school_id: "s1", created_at: "z" } });
    vi.stubGlobal("fetch", spy);
    const c = await regenerateJoinCode("c1");
    expect(c.join_code).toBe("ZZ-ZZ");
    expect(callOf(spy)[1].body).toBe(JSON.stringify({ regenerate_join_code: true }));
  });

  it("removeEnrollment DELETEs the enrollment path", async () => {
    const spy = ok(null, 204);
    vi.stubGlobal("fetch", spy);
    await removeEnrollment("c1", "u1");
    expect(callOf(spy)[0]).toContain("/api/v1/classes/c1/enrollments/u1");
    expect(callOf(spy)[1].method).toBe("DELETE");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/web && pnpm test -- src/api/classes.test.ts`
Expected: FAIL — `Cannot find module './classes'`.

- [ ] **Step 3: Create the client module**

Create `apps/web/src/api/classes.ts`:

```ts
import { apiFetch } from "./client";

export interface ClassSummary {
  id: string;
  name: string;
  join_code: string;
  school_id: string;
  created_at: string;
}

export interface RosterStudent {
  id: string;
  display_name: string;
  email: string;
  last_active_at: string | null;
  task_count: number;
  evaluation_count: number;
  card_count: number;
}

export interface ClassDetail {
  class: ClassSummary;
  roster: RosterStudent[];
}

export async function listClasses(): Promise<ClassSummary[]> {
  const r = await apiFetch<{ classes: ClassSummary[] }>("/api/v1/classes");
  return r.classes;
}

export async function createClass(input: { name: string }): Promise<ClassSummary> {
  const r = await apiFetch<{ class: ClassSummary }>("/api/v1/classes", {
    method: "POST",
    body: JSON.stringify({ name: input.name }),
  });
  return r.class;
}

export async function getClass(id: string): Promise<ClassDetail> {
  return apiFetch<ClassDetail>(`/api/v1/classes/${id}`);
}

export async function renameClass(id: string, name: string): Promise<ClassSummary> {
  const r = await apiFetch<{ class: ClassSummary }>(`/api/v1/classes/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ name }),
  });
  return r.class;
}

export async function regenerateJoinCode(id: string): Promise<ClassSummary> {
  const r = await apiFetch<{ class: ClassSummary }>(`/api/v1/classes/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ regenerate_join_code: true }),
  });
  return r.class;
}

export async function removeEnrollment(id: string, userId: string): Promise<void> {
  await apiFetch<void>(`/api/v1/classes/${id}/enrollments/${userId}`, { method: "DELETE" });
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/web && pnpm test -- src/api/classes.test.ts`
Expected: PASS (6 tests).

- [ ] **Step 5: Wire into the `ApiClient` interface and `api` object**

Modify `apps/web/src/api/index.ts`. Add the import after the `auth` import (line 6):

```ts
import {
  listClasses, createClass, getClass, renameClass, regenerateJoinCode, removeEnrollment,
  type ClassSummary, type RosterStudent, type ClassDetail,
} from "./classes";
```

Add to the `export type { … }` line (line 8):

```ts
export type { TaskDetail, TurnEvent, MeUser, ClassSummary, RosterStudent, ClassDetail };
```

Add these methods to the `ApiClient` interface (after `getMe(): Promise<MeUser>;`):

```ts
  listClasses(): Promise<ClassSummary[]>;
  createClass(input: { name: string }): Promise<ClassSummary>;
  getClass(id: string): Promise<ClassDetail>;
  renameClass(id: string, name: string): Promise<ClassSummary>;
  regenerateJoinCode(id: string): Promise<ClassSummary>;
  removeEnrollment(id: string, userId: string): Promise<void>;
```

Add them to the `api` object literal (after `getMe`):

```ts
  listClasses, createClass, getClass, renameClass, regenerateJoinCode, removeEnrollment,
```

- [ ] **Step 6: Typecheck + full test run**

Run: `cd apps/web && pnpm typecheck && pnpm test`
Expected: typecheck clean; all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/api/classes.ts apps/web/src/api/classes.test.ts apps/web/src/api/index.ts
git commit -m "feat(p3): web class API client (list/create/get/rename/regen/remove)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 2: ConsoleRail (console left nav)

**Files:**
- Create: `apps/web/src/console/ConsoleRail.tsx`
- Create: `apps/web/src/console/ConsoleRail.test.tsx`

**Interfaces:**
- Produces: `type ConsoleTab = "classes" | "settings";` and
  `function ConsoleRail({ tab, onTab }: { tab: ConsoleTab; onTab: (t: ConsoleTab) => void }): JSX.Element`

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/console/ConsoleRail.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConsoleRail } from "./ConsoleRail";

describe("ConsoleRail", () => {
  it("renders 班级 and 设置 tabs and marks the active one", () => {
    render(<ConsoleRail tab="classes" onTab={() => {}} />);
    const tabs = screen.getAllByRole("tab");
    expect(tabs).toHaveLength(2);
    expect(screen.getByText("班级")).toBeInTheDocument();
    expect(screen.getByText("设置")).toBeInTheDocument();
    expect(screen.getByText("班级").closest('[role="tab"]')).toHaveAttribute("aria-selected", "true");
  });

  it("calls onTab when a tab is clicked", async () => {
    const onTab = vi.fn();
    render(<ConsoleRail tab="classes" onTab={onTab} />);
    await userEvent.click(screen.getByText("设置"));
    expect(onTab).toHaveBeenCalledWith("settings");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/web && pnpm test -- src/console/ConsoleRail.test.tsx`
Expected: FAIL — `Cannot find module './ConsoleRail'`.

- [ ] **Step 3: Implement ConsoleRail**

Create `apps/web/src/console/ConsoleRail.tsx` (lifts the rail container, logo, nav-item, and avatar-dot styling from `shell/LeftRail.tsx`; only the nav items differ):

```tsx
// No React import needed — `React.CSSProperties` / `React.ReactNode` resolve via the
// global namespace from @types/react, matching shell/LeftRail.tsx.

export type ConsoleTab = "classes" | "settings";

interface NavItem {
  key: ConsoleTab;
  label: string;
  icon: (stroke: string) => React.ReactNode;
}

const NAV_ITEMS: NavItem[] = [
  {
    key: "classes",
    label: "班级",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2" />
        <circle cx="9" cy="7" r="4" />
        <path d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75" />
      </svg>
    ),
  },
  {
    key: "settings",
    label: "设置",
    icon: (stroke) => (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="3" />
        <path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 11-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 11-2.83-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 112.83-2.83l.06.06a1.65 1.65 0 001.82.33H9a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 112.83 2.83l-.06.06a1.65 1.65 0 00-.33 1.82V9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z" />
      </svg>
    ),
  },
];

const BOX_BASE: React.CSSProperties = { width: 42, height: 42, borderRadius: 12, display: "flex", alignItems: "center", justifyContent: "center" };
const ACTIVE_BOX: React.CSSProperties = { ...BOX_BASE, background: "#EDEFF9" };
const INACTIVE_BOX: React.CSSProperties = { ...BOX_BASE, background: "transparent" };
const ACTIVE_ICON_STROKE = "#2A3B7A";
const INACTIVE_ICON_STROKE = "#9AA1B0";
const ACTIVE_LABEL: React.CSSProperties = { color: "#2A3B7A", fontWeight: 700, fontSize: 10 };
const INACTIVE_LABEL: React.CSSProperties = { color: "#9AA1B0", fontSize: 10 };

export function ConsoleRail({ tab, onTab }: { tab: ConsoleTab; onTab: (t: ConsoleTab) => void }) {
  return (
    <div style={{ width: 74, flexShrink: 0, background: "#FFFFFF", borderRight: "1px solid #EAECF2", display: "flex", flexDirection: "column", alignItems: "center", padding: "16px 0", gap: 4 }}>
      <svg viewBox="0 0 48 48" width="38" height="38" style={{ display: "block", marginBottom: 16 }}>
        <rect x="5" y="6" width="38" height="36" rx="13" fill="#2A3B7A" />
        <rect x="5" y="6" width="38" height="17" rx="13" fill="#ffffff" opacity="0.10" />
        <ellipse cx="18.5" cy="24" rx="3.4" ry="3.9" fill="#fff" />
        <ellipse cx="29.5" cy="24" rx="3.4" ry="3.9" fill="#fff" />
        <circle cx="19.3" cy="25" r="1.5" fill="#1C2333" />
        <circle cx="30.3" cy="25" r="1.5" fill="#1C2333" />
        <path d="M19 31.5 Q24 35 29 31.5" stroke="#fff" strokeWidth="2.2" fill="none" strokeLinecap="round" />
        <circle cx="39" cy="9" r="4.5" fill="#E8A33D" />
      </svg>

      {NAV_ITEMS.map(({ key, label, icon }) => {
        const isActive = tab === key;
        return (
          <div
            key={key}
            role="tab"
            aria-selected={isActive}
            onClick={() => onTab(key)}
            style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 5, cursor: "pointer", padding: "4px 0" }}
          >
            <div style={isActive ? ACTIVE_BOX : INACTIVE_BOX}>{icon(isActive ? ACTIVE_ICON_STROKE : INACTIVE_ICON_STROKE)}</div>
            <span style={isActive ? ACTIVE_LABEL : INACTIVE_LABEL}>{label}</span>
          </div>
        );
      })}
    </div>
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/web && pnpm test -- src/console/ConsoleRail.test.tsx`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/console/ConsoleRail.tsx apps/web/src/console/ConsoleRail.test.tsx
git commit -m "feat(p3): ConsoleRail nav (班级/设置)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 3: ClassesView (list + role header + empty + create)

**Files:**
- Create: `apps/web/src/console/time.ts`
- Create: `apps/web/src/console/time.test.ts`
- Create: `apps/web/src/console/ClassesView.tsx`
- Create: `apps/web/src/console/ClassesView.test.tsx`

**Interfaces:**
- Consumes: `ClassSummary`, and a client slice `Pick<ApiClient, "listClasses" | "createClass">`.
- Produces:
  - `shortDate(iso: string): string` (in `time.ts`)
  - `function ClassesView({ client, role, onOpenClass }: { client: Pick<ApiClient, "listClasses" | "createClass">; role: string; onOpenClass: (id: string) => void }): JSX.Element`

- [ ] **Step 1: Write the failing test for `shortDate`**

Create `apps/web/src/console/time.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { shortDate } from "./time";

describe("shortDate", () => {
  it("slices the date portion of an ISO timestamp", () => {
    expect(shortDate("2026-06-26T13:45:00Z")).toBe("2026-06-26");
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && pnpm test -- src/console/time.test.ts`
Expected: FAIL — `Cannot find module './time'`.

- [ ] **Step 3: Implement `shortDate`**

Create `apps/web/src/console/time.ts`:

```ts
// shortDate: ISO timestamp → YYYY-MM-DD. Locale-free and deterministic.
export function shortDate(iso: string): string {
  return iso.slice(0, 10);
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/web && pnpm test -- src/console/time.test.ts`
Expected: PASS.

- [ ] **Step 5: Write the failing test for ClassesView**

Create `apps/web/src/console/ClassesView.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ClassesView } from "./ClassesView";
import type { ClassSummary } from "../api";

const cls = (over: Partial<ClassSummary> = {}): ClassSummary => ({
  id: "c1", name: "11 年级 A · TOK", join_code: "AB-CD", school_id: "s1", created_at: "2026-06-20T00:00:00Z", ...over,
});

describe("ClassesView", () => {
  it("teacher: shows 我的班级, lists classes, shows the join code and create button", async () => {
    const client = { listClasses: vi.fn(async () => [cls()]), createClass: vi.fn() };
    render(<ClassesView client={client} role="teacher" onOpenClass={() => {}} />);
    expect(await screen.findByText("我的班级")).toBeInTheDocument();
    expect(screen.getByText("11 年级 A · TOK")).toBeInTheDocument();
    expect(screen.getByText(/AB-CD/)).toBeInTheDocument();
    expect(screen.getByText("+ 新建班级")).toBeInTheDocument();
  });

  it("admin: shows 全校班级 and NO create button", async () => {
    const client = { listClasses: vi.fn(async () => [cls()]), createClass: vi.fn() };
    render(<ClassesView client={client} role="admin" onOpenClass={() => {}} />);
    expect(await screen.findByText("全校班级")).toBeInTheDocument();
    expect(screen.queryByText("+ 新建班级")).not.toBeInTheDocument();
  });

  it("teacher empty state prompts to create the first class", async () => {
    const client = { listClasses: vi.fn(async () => []), createClass: vi.fn() };
    render(<ClassesView client={client} role="teacher" onOpenClass={() => {}} />);
    expect(await screen.findByText(/还没有班级/)).toBeInTheDocument();
  });

  it("clicking a class card calls onOpenClass with its id", async () => {
    const client = { listClasses: vi.fn(async () => [cls()]), createClass: vi.fn() };
    const onOpenClass = vi.fn();
    render(<ClassesView client={client} role="teacher" onOpenClass={onOpenClass} />);
    await userEvent.click(await screen.findByText("11 年级 A · TOK"));
    expect(onOpenClass).toHaveBeenCalledWith("c1");
  });

  it("create flow calls createClass and surfaces the new join code", async () => {
    const created = cls({ id: "c2", name: "新班", join_code: "EF-GH" });
    const client = { listClasses: vi.fn(async () => []), createClass: vi.fn(async () => created) };
    render(<ClassesView client={client} role="teacher" onOpenClass={() => {}} />);
    await userEvent.click(await screen.findByText("+ 新建班级"));
    await userEvent.type(screen.getByPlaceholderText(/班级名称/), "新班");
    await userEvent.click(screen.getByText("创建"));
    await waitFor(() => expect(client.createClass).toHaveBeenCalledWith({ name: "新班" }));
    expect(await screen.findByText(/EF-GH/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 6: Run to verify it fails**

Run: `cd apps/web && pnpm test -- src/console/ClassesView.test.tsx`
Expected: FAIL — `Cannot find module './ClassesView'`.

- [ ] **Step 7: Implement ClassesView**

Create `apps/web/src/console/ClassesView.tsx`:

```tsx
import { useEffect, useState } from "react";
import type { ApiClient, ClassSummary } from "../api";
import { ApiError } from "../api";
import { shortDate } from "./time";

type Client = Pick<ApiClient, "listClasses" | "createClass">;

export function ClassesView({
  client,
  role,
  onOpenClass,
}: {
  client: Client;
  role: string;
  onOpenClass: (id: string) => void;
}) {
  const [classes, setClasses] = useState<ClassSummary[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [lastCreated, setLastCreated] = useState<ClassSummary | null>(null);

  const isTeacher = role === "teacher";

  function load() {
    setError(null);
    client.listClasses().then(setClasses).catch((e) =>
      setError(e instanceof ApiError ? e.message : "加载失败"),
    );
  }
  useEffect(load, [client]);

  async function submit() {
    const trimmed = name.trim();
    if (!trimmed) return;
    setBusy(true);
    setError(null);
    try {
      const c = await client.createClass({ name: trimmed });
      setLastCreated(c);
      setName("");
      setCreating(false);
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "创建失败");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 980, margin: "0 auto", padding: "44px 40px 60px" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <div style={{ fontSize: 26, fontWeight: 800, color: "#1C2333", letterSpacing: "-.01em" }}>
            {isTeacher ? "我的班级" : "全校班级"}
          </div>
          {isTeacher && !creating && (
            <button
              onClick={() => { setCreating(true); setLastCreated(null); }}
              style={{ background: "#2A3B7A", color: "#fff", border: "none", padding: "10px 18px", borderRadius: 12, fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}
            >
              + 新建班级
            </button>
          )}
        </div>

        {isTeacher && creating && (
          <div style={{ marginTop: 18, background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "18px 20px", display: "flex", gap: 12, alignItems: "center" }}>
            <input
              autoFocus
              placeholder="班级名称，如「11 年级 A · TOK」"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") void submit(); }}
              style={{ flex: 1, border: "1px solid #E1E4ED", borderRadius: 11, padding: "11px 13px", fontSize: 14, color: "#1C2333", outline: "none", boxSizing: "border-box" }}
            />
            <button onClick={() => void submit()} disabled={busy} style={{ background: "#2A3B7A", color: "#fff", border: "none", padding: "11px 18px", borderRadius: 11, fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}>创建</button>
            <button onClick={() => { setCreating(false); setName(""); }} style={{ background: "transparent", color: "#8A92A3", border: "none", fontSize: 14, fontWeight: 600, cursor: "pointer", fontFamily: "inherit" }}>取消</button>
          </div>
        )}

        {lastCreated && (
          <div style={{ marginTop: 14, background: "#EDEFF9", border: "1px solid #D7DCF2", borderRadius: 12, padding: "12px 16px", fontSize: 13.5, color: "#2A3B7A", fontWeight: 600 }}>
            已创建「{lastCreated.name}」· 邀请码 {lastCreated.join_code}（分享给学生加入）
          </div>
        )}

        {error && (
          <div style={{ marginTop: 14, color: "#C76B6B", fontSize: 13.5, fontWeight: 600 }}>{error} · <span onClick={load} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span></div>
        )}

        {classes && classes.length === 0 && !creating && (
          <div style={{ marginTop: 28, color: "#8A92A3", fontSize: 14.5, lineHeight: 1.7 }}>
            {isTeacher ? "还没有班级，点「+ 新建班级」创建第一个。" : "本校暂无班级。"}
          </div>
        )}

        {classes && classes.length > 0 && (
          <div style={{ display: "grid", gridTemplateColumns: "repeat(2, 1fr)", gap: 16, marginTop: 24 }}>
            {classes.map((c) => (
              <div
                key={c.id}
                onClick={() => onOpenClass(c.id)}
                style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "18px 20px", cursor: "pointer", boxShadow: "0 1px 3px rgba(20,30,60,.04)" }}
              >
                <div style={{ fontSize: 16, fontWeight: 700, color: "#1C2333", lineHeight: 1.45 }}>{c.name}</div>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginTop: 14 }}>
                  <span style={{ fontSize: 12.5, color: "#8A92A3", fontWeight: 600 }}>邀请码 {c.join_code}</span>
                  <span style={{ fontSize: 12, color: "#9AA1B0", fontWeight: 500 }}>创建于 {shortDate(c.created_at)}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 8: Run to verify it passes**

Run: `cd apps/web && pnpm test -- src/console/ClassesView.test.tsx src/console/time.test.ts`
Expected: PASS (1 + 5 tests).

- [ ] **Step 9: Commit**

```bash
git add apps/web/src/console/time.ts apps/web/src/console/time.test.ts apps/web/src/console/ClassesView.tsx apps/web/src/console/ClassesView.test.tsx
git commit -m "feat(p3): ClassesView — list, role-aware header, teacher-only create

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 4: ClassDetailView — roster table (read parts)

**Files:**
- Modify: `apps/web/src/console/time.ts` (add `relativeTime`)
- Create: `apps/web/src/console/ClassDetailView.tsx`
- Create: `apps/web/src/console/ClassDetailView.test.tsx`
- Modify: `apps/web/src/console/time.test.ts` (add `relativeTime` tests)

**Interfaces:**
- Consumes: `ClassDetail`, `RosterStudent`, a client slice `Pick<ApiClient, "getClass" | "renameClass" | "regenerateJoinCode" | "removeEnrollment">`.
- Produces:
  - `relativeTime(iso: string | null, now: number): string` (in `time.ts`)
  - `function ClassDetailView({ client, classId, onBack, now }: { client: Pick<ApiClient, "getClass" | "renameClass" | "regenerateJoinCode" | "removeEnrollment">; classId: string; onBack: () => void; now?: number }): JSX.Element`
- Note: this task ships the **read** surface (roster table + join-code chip + copy + back + empty state). Task 5 adds rename/regenerate/remove using the same client slice (already declared here so the prop type is stable across both tasks).

- [ ] **Step 1: Write the failing test for `relativeTime`**

Append to `apps/web/src/console/time.test.ts`:

```ts
import { relativeTime } from "./time";

describe("relativeTime", () => {
  const now = Date.parse("2026-06-26T12:00:00Z");
  it("returns 从未 for null", () => {
    expect(relativeTime(null, now)).toBe("从未");
  });
  it("returns 刚刚 under a minute", () => {
    expect(relativeTime("2026-06-26T11:59:30Z", now)).toBe("刚刚");
  });
  it("returns minutes", () => {
    expect(relativeTime("2026-06-26T11:30:00Z", now)).toBe("30 分钟前");
  });
  it("returns hours", () => {
    expect(relativeTime("2026-06-26T10:00:00Z", now)).toBe("2 小时前");
  });
  it("returns days", () => {
    expect(relativeTime("2026-06-24T12:00:00Z", now)).toBe("2 天前");
  });
  it("falls back to a date for older than a week", () => {
    expect(relativeTime("2026-06-01T12:00:00Z", now)).toBe("2026-06-01");
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && pnpm test -- src/console/time.test.ts`
Expected: FAIL — `relativeTime` is not exported.

- [ ] **Step 3: Implement `relativeTime`**

Append to `apps/web/src/console/time.ts`:

```ts
// relativeTime: humanized activity relative to `now` (ms epoch). null → 从未.
// Caller passes Date.now(); the helper stays pure for deterministic tests.
export function relativeTime(iso: string | null, now: number): string {
  if (iso == null) return "从未";
  const then = Date.parse(iso);
  const diff = now - then;
  const MIN = 60_000, HOUR = 60 * MIN, DAY = 24 * HOUR;
  if (diff < MIN) return "刚刚";
  if (diff < HOUR) return `${Math.floor(diff / MIN)} 分钟前`;
  if (diff < DAY) return `${Math.floor(diff / HOUR)} 小时前`;
  if (diff < 7 * DAY) return `${Math.floor(diff / DAY)} 天前`;
  return iso.slice(0, 10);
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/web && pnpm test -- src/console/time.test.ts`
Expected: PASS (1 `shortDate` + 6 `relativeTime`).

- [ ] **Step 5: Write the failing test for the roster table**

Create `apps/web/src/console/ClassDetailView.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { ClassDetailView } from "./ClassDetailView";
import type { ClassDetail } from "../api";

const NOW = Date.parse("2026-06-26T12:00:00Z");

const detail = (over: Partial<ClassDetail> = {}): ClassDetail => ({
  class: { id: "c1", name: "11 年级 A", join_code: "AB-CD", school_id: "s1", created_at: "2026-06-20T00:00:00Z" },
  roster: [
    { id: "u1", display_name: "Phoebe", email: "p@d", last_active_at: "2026-06-26T10:00:00Z", task_count: 3, evaluation_count: 1, card_count: 7 },
    { id: "u2", display_name: "Mia", email: "m@d", last_active_at: null, task_count: 0, evaluation_count: 0, card_count: 0 },
  ],
  ...over,
});

function makeClient(d: ClassDetail) {
  return {
    getClass: vi.fn(async () => d),
    renameClass: vi.fn(),
    regenerateJoinCode: vi.fn(),
    removeEnrollment: vi.fn(),
  };
}

describe("ClassDetailView roster", () => {
  it("renders the class name, join code, and roster rows with signals", async () => {
    const client = makeClient(detail());
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} now={NOW} />);
    expect(await screen.findByText("11 年级 A")).toBeInTheDocument();
    expect(screen.getByText(/AB-CD/)).toBeInTheDocument();
    expect(screen.getByText("Phoebe")).toBeInTheDocument();
    expect(screen.getByText("2 小时前")).toBeInTheDocument();
    expect(screen.getByText("从未")).toBeInTheDocument();
  });

  it("shows the aggregate-only caption and column headers", async () => {
    const client = makeClient(detail());
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} now={NOW} />);
    expect(await screen.findByText("姓名")).toBeInTheDocument();
    expect(screen.getByText("任务")).toBeInTheDocument();
    expect(screen.getByText(/暂无学生作品详情/)).toBeInTheDocument();
  });

  it("renders an empty-roster note with the join code", async () => {
    const client = makeClient(detail({ roster: [] }));
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} now={NOW} />);
    expect(await screen.findByText(/还没有学生加入/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 6: Run to verify it fails**

Run: `cd apps/web && pnpm test -- src/console/ClassDetailView.test.tsx`
Expected: FAIL — `Cannot find module './ClassDetailView'`.

- [ ] **Step 7: Implement the read surface of ClassDetailView**

Create `apps/web/src/console/ClassDetailView.tsx`:

```tsx
import { useEffect, useState } from "react";
import type { ApiClient, ClassDetail } from "../api";
import { ApiError } from "../api";
import { relativeTime } from "./time";

type Client = Pick<ApiClient, "getClass" | "renameClass" | "regenerateJoinCode" | "removeEnrollment">;

const TH: React.CSSProperties = { textAlign: "left", fontSize: 12, fontWeight: 700, color: "#8A92A3", padding: "10px 12px", borderBottom: "1px solid #EAECF2" };
const TD: React.CSSProperties = { fontSize: 13.5, color: "#1C2333", padding: "12px", borderBottom: "1px solid #F2F3F7" };

export function ClassDetailView({
  client,
  classId,
  onBack,
  now,
}: {
  client: Client;
  classId: string;
  onBack: () => void;
  now?: number;
}) {
  const _now = now ?? Date.now();
  const [detail, setDetail] = useState<ClassDetail | null>(null);
  const [error, setError] = useState<string | null>(null);

  function load() {
    setError(null);
    client.getClass(classId).then(setDetail).catch((e) =>
      setError(e instanceof ApiError ? e.message : "加载失败"),
    );
  }
  useEffect(load, [client, classId]);

  if (error) {
    return (
      <div style={{ flex: 1, padding: 40 }}>
        <button onClick={onBack} style={backBtn}>← 返回</button>
        <div style={{ marginTop: 20, color: "#C76B6B", fontSize: 14, fontWeight: 600 }}>{error} · <span onClick={load} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span></div>
      </div>
    );
  }
  if (!detail) {
    return <div style={{ flex: 1 }} />;
  }

  const c = detail.class;
  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 980, margin: "0 auto", padding: "32px 40px 60px" }}>
        <button onClick={onBack} style={backBtn}>← 返回</button>

        <div style={{ display: "flex", alignItems: "center", gap: 14, marginTop: 18 }}>
          <div style={{ fontSize: 24, fontWeight: 800, color: "#1C2333", letterSpacing: "-.01em" }}>{c.name}</div>
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 14 }}>
          <span style={{ background: "#EDEFF9", color: "#2A3B7A", fontWeight: 700, fontSize: 13, padding: "6px 12px", borderRadius: 10 }}>邀请码 {c.join_code}</span>
          <button onClick={() => void navigator.clipboard?.writeText(c.join_code)} style={chipBtn}>复制</button>
        </div>

        {detail.roster.length === 0 ? (
          <div style={{ marginTop: 30, color: "#8A92A3", fontSize: 14.5, lineHeight: 1.7 }}>
            还没有学生加入。分享邀请码 {c.join_code} 让学生加入。
          </div>
        ) : (
          <>
            <table style={{ width: "100%", borderCollapse: "collapse", marginTop: 26, background: "#fff", border: "1px solid #EAECF2", borderRadius: 12, overflow: "hidden" }}>
              <thead>
                <tr>
                  <th style={TH}>姓名</th>
                  <th style={TH}>邮箱</th>
                  <th style={TH}>最近活跃</th>
                  <th style={TH}>任务</th>
                  <th style={TH}>评估</th>
                  <th style={TH}>卡片</th>
                  <th style={TH} aria-label="操作" />
                </tr>
              </thead>
              <tbody>
                {detail.roster.map((s) => (
                  <tr key={s.id}>
                    <td style={{ ...TD, fontWeight: 600 }}>{s.display_name}</td>
                    <td style={{ ...TD, color: "#6B7384" }}>{s.email}</td>
                    <td style={TD}>{relativeTime(s.last_active_at, _now)}</td>
                    <td style={TD}>{s.task_count}</td>
                    <td style={TD}>{s.evaluation_count}</td>
                    <td style={TD}>{s.card_count}</td>
                    <td style={TD} />
                  </tr>
                ))}
              </tbody>
            </table>
            <div style={{ marginTop: 12, fontSize: 12, color: "#9AA1B0" }}>行不可点入 — 暂无学生作品详情页。</div>
          </>
        )}
      </div>
    </div>
  );
}

const backBtn: React.CSSProperties = { background: "transparent", border: "none", color: "#8A92A3", fontSize: 14, fontWeight: 600, cursor: "pointer", fontFamily: "inherit", padding: 0 };
const chipBtn: React.CSSProperties = { background: "transparent", border: "1px solid #D7DCF2", color: "#2A3B7A", fontSize: 13, fontWeight: 600, cursor: "pointer", fontFamily: "inherit", padding: "6px 12px", borderRadius: 10 };
```

- [ ] **Step 8: Run to verify it passes**

Run: `cd apps/web && pnpm test -- src/console/ClassDetailView.test.tsx`
Expected: PASS (3 tests).

- [ ] **Step 9: Commit**

```bash
git add apps/web/src/console/time.ts apps/web/src/console/time.test.ts apps/web/src/console/ClassDetailView.tsx apps/web/src/console/ClassDetailView.test.tsx
git commit -m "feat(p3): ClassDetailView roster — aggregate signal table (read)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 5: ClassDetailView — mutations (rename / regenerate / remove)

**Files:**
- Modify: `apps/web/src/console/ClassDetailView.tsx`
- Modify: `apps/web/src/console/ClassDetailView.test.tsx`

**Interfaces:**
- Consumes: the `renameClass`, `regenerateJoinCode`, `removeEnrollment` methods already in the `Client` slice from Task 4.
- Produces: no new exports — adds in-component rename, regenerate (with inline confirm), and per-row remove (with inline confirm) to the existing `ClassDetailView`.

- [ ] **Step 1: Write the failing tests**

Append to `apps/web/src/console/ClassDetailView.test.tsx` (reuse `detail`, `makeClient`, `NOW` from Task 4):

```tsx
import userEvent from "@testing-library/user-event";

describe("ClassDetailView mutations", () => {
  it("renames the class", async () => {
    const client = makeClient(detail());
    client.renameClass.mockResolvedValue({ ...detail().class, name: "新名字" });
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} now={NOW} />);
    await userEvent.click(await screen.findByText("改名"));
    const input = screen.getByDisplayValue("11 年级 A");
    await userEvent.clear(input);
    await userEvent.type(input, "新名字");
    await userEvent.click(screen.getByText("保存"));
    await waitFor(() => expect(client.renameClass).toHaveBeenCalledWith("c1", "新名字"));
  });

  it("regenerates the join code only after confirming", async () => {
    const client = makeClient(detail());
    client.regenerateJoinCode.mockResolvedValue({ ...detail().class, join_code: "ZZ-ZZ" });
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} now={NOW} />);
    await userEvent.click(await screen.findByText("轮换"));
    expect(client.regenerateJoinCode).not.toHaveBeenCalled();
    await userEvent.click(screen.getByText("确认轮换"));
    await waitFor(() => expect(client.regenerateJoinCode).toHaveBeenCalledWith("c1"));
    expect(await screen.findByText(/ZZ-ZZ/)).toBeInTheDocument();
  });

  it("removes a student only after confirming, then drops the row", async () => {
    const client = makeClient(detail());
    client.removeEnrollment.mockResolvedValue(undefined);
    render(<ClassDetailView client={client} classId="c1" onBack={() => {}} now={NOW} />);
    await userEvent.click(await screen.findByLabelText("移除 Phoebe"));
    expect(client.removeEnrollment).not.toHaveBeenCalled();
    await userEvent.click(screen.getByText("确认移除"));
    await waitFor(() => expect(client.removeEnrollment).toHaveBeenCalledWith("c1", "u1"));
    await waitFor(() => expect(screen.queryByText("Phoebe")).not.toBeInTheDocument());
  });
});
```

Also add `waitFor` to the existing testing-library import at the top of the file:
`import { render, screen, waitFor } from "@testing-library/react";`

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && pnpm test -- src/console/ClassDetailView.test.tsx`
Expected: FAIL — no "改名" / "轮换" / "移除 …" controls yet.

- [ ] **Step 3: Add mutation state + handlers**

In `apps/web/src/console/ClassDetailView.tsx`, add state inside the component (after the `detail`/`error` state):

```tsx
  const [renaming, setRenaming] = useState(false);
  const [draftName, setDraftName] = useState("");
  const [confirmRegen, setConfirmRegen] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState<string | null>(null); // student id
  const [busy, setBusy] = useState(false);
```

Add handlers (after `load`):

```tsx
  async function doRename() {
    const trimmed = draftName.trim();
    if (!trimmed) return;
    setBusy(true);
    try {
      const updated = await client.renameClass(classId, trimmed);
      setDetail((d) => (d ? { ...d, class: updated } : d));
      setRenaming(false);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "改名失败");
    } finally {
      setBusy(false);
    }
  }

  async function doRegen() {
    setBusy(true);
    try {
      const updated = await client.regenerateJoinCode(classId);
      setDetail((d) => (d ? { ...d, class: updated } : d));
      setConfirmRegen(false);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "轮换失败");
    } finally {
      setBusy(false);
    }
  }

  async function doRemove(studentId: string) {
    setBusy(true);
    try {
      await client.removeEnrollment(classId, studentId);
      setDetail((d) => (d ? { ...d, roster: d.roster.filter((s) => s.id !== studentId) } : d));
      setConfirmRemove(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "移除失败");
    } finally {
      setBusy(false);
    }
  }
```

- [ ] **Step 4: Render rename + regenerate controls**

Replace the class-name block and the join-code row with versions that include the controls. The name block:

```tsx
        <div style={{ display: "flex", alignItems: "center", gap: 14, marginTop: 18 }}>
          {renaming ? (
            <>
              <input
                autoFocus
                value={draftName}
                onChange={(e) => setDraftName(e.target.value)}
                style={{ fontSize: 18, fontWeight: 700, color: "#1C2333", border: "1px solid #E1E4ED", borderRadius: 10, padding: "8px 12px", outline: "none" }}
              />
              <button onClick={() => void doRename()} disabled={busy} style={chipBtn}>保存</button>
              <button onClick={() => setRenaming(false)} style={backBtn}>取消</button>
            </>
          ) : (
            <>
              <div style={{ fontSize: 24, fontWeight: 800, color: "#1C2333", letterSpacing: "-.01em" }}>{c.name}</div>
              <button onClick={() => { setDraftName(c.name); setRenaming(true); }} style={chipBtn}>改名</button>
            </>
          )}
        </div>
```

The join-code row (add the 轮换 button + inline confirm):

```tsx
        <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 14, flexWrap: "wrap" }}>
          <span style={{ background: "#EDEFF9", color: "#2A3B7A", fontWeight: 700, fontSize: 13, padding: "6px 12px", borderRadius: 10 }}>邀请码 {c.join_code}</span>
          <button onClick={() => void navigator.clipboard?.writeText(c.join_code)} style={chipBtn}>复制</button>
          <button onClick={() => setConfirmRegen(true)} style={chipBtn}>轮换</button>
          {confirmRegen && (
            <span style={{ display: "inline-flex", alignItems: "center", gap: 8, fontSize: 13, color: "#C76B6B", fontWeight: 600 }}>
              轮换后旧邀请码立即失效，确定？
              <button onClick={() => void doRegen()} disabled={busy} style={dangerBtn}>确认轮换</button>
              <button onClick={() => setConfirmRegen(false)} style={backBtn}>取消</button>
            </span>
          )}
        </div>
```

- [ ] **Step 5: Render the per-row remove control + confirm**

Replace the empty action `<td style={TD} />` in the roster row with:

```tsx
                    <td style={{ ...TD, textAlign: "right" }}>
                      {confirmRemove === s.id ? (
                        <span style={{ display: "inline-flex", alignItems: "center", gap: 8, fontSize: 12.5, color: "#C76B6B", fontWeight: 600 }}>
                          将 {s.display_name} 移出班级？仅解除关联，不删除其账号或作品。
                          <button onClick={() => void doRemove(s.id)} disabled={busy} style={dangerBtn}>确认移除</button>
                          <button onClick={() => setConfirmRemove(null)} style={backBtn}>取消</button>
                        </span>
                      ) : (
                        <button
                          aria-label={`移除 ${s.display_name}`}
                          onClick={() => setConfirmRemove(s.id)}
                          style={{ background: "transparent", border: "none", color: "#B7BECC", fontSize: 16, cursor: "pointer", fontFamily: "inherit", lineHeight: 1 }}
                        >
                          ✕
                        </button>
                      )}
                    </td>
```

Add the `dangerBtn` style next to `backBtn`/`chipBtn` at the bottom of the file:

```tsx
const dangerBtn: React.CSSProperties = { background: "#C76B6B", border: "none", color: "#fff", fontSize: 12.5, fontWeight: 700, cursor: "pointer", fontFamily: "inherit", padding: "5px 11px", borderRadius: 9 };
```

- [ ] **Step 6: Run to verify it passes**

Run: `cd apps/web && pnpm test -- src/console/ClassDetailView.test.tsx`
Expected: PASS (3 read + 3 mutation tests).

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/console/ClassDetailView.tsx apps/web/src/console/ClassDetailView.test.tsx
git commit -m "feat(p3): ClassDetailView mutations — rename, regenerate, remove

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 6: ConsoleShell (compose rail + views + settings)

**Files:**
- Create: `apps/web/src/console/ConsoleShell.tsx`
- Create: `apps/web/src/console/ConsoleShell.test.tsx`

**Interfaces:**
- Consumes: `ConsoleRail`, `ClassesView`, `ClassDetailView`, `SettingsView` (`../shell/settings/SettingsView`), `SessionStore` (`../shell/session`), `MeUser`, and a client slice of the six class methods.
- Produces: `type ConsoleClient = Pick<ApiClient, "listClasses" | "createClass" | "getClass" | "renameClass" | "regenerateJoinCode" | "removeEnrollment">;` and
  `function ConsoleShell({ session, client, onLogout }: { session: SessionStore; client: ConsoleClient; onLogout: () => void }): JSX.Element`

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/console/ConsoleShell.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConsoleShell } from "./ConsoleShell";
import { createSession } from "../shell/session";
import type { ClassDetail, ClassSummary, MeUser } from "../api";

const mem = () => { let s = "{}"; return { getItem: () => s, setItem: (_: string, v: string) => { s = v; } }; };
const TEACHER: MeUser = { id: "u1", email: "t@d", display_name: "Teacher", role: "teacher", avatar_color: "#2A3B7A", school: { id: "s1", name: "Demo" }, classes: [] };

const summary: ClassSummary = { id: "c1", name: "11A", join_code: "AB-CD", school_id: "s1", created_at: "2026-06-20T00:00:00Z" };
const detail: ClassDetail = { class: summary, roster: [] };

function client() {
  return {
    listClasses: vi.fn(async () => [summary]),
    createClass: vi.fn(),
    getClass: vi.fn(async () => detail),
    renameClass: vi.fn(),
    regenerateJoinCode: vi.fn(),
    removeEnrollment: vi.fn(),
  };
}

function mount() {
  const session = createSession({ storage: mem() });
  session.setUser(TEACHER);
  const onLogout = vi.fn();
  render(<ConsoleShell session={session} client={client()} onLogout={onLogout} />);
  return { onLogout };
}

describe("ConsoleShell", () => {
  it("lands on the classes list", async () => {
    mount();
    expect(await screen.findByText("我的班级")).toBeInTheDocument();
  });

  it("opens a class detail when a card is clicked, and 返回 goes back", async () => {
    mount();
    await userEvent.click(await screen.findByText("11A"));
    // The fixture roster is empty, so the detail shows the empty-roster note.
    expect(await screen.findByText(/还没有学生加入/)).toBeInTheDocument();
    await userEvent.click(screen.getByText("← 返回"));
    expect(await screen.findByText("我的班级")).toBeInTheDocument();
  });

  it("switches to the 设置 tab and can log out", async () => {
    const { onLogout } = mount();
    await userEvent.click(screen.getByText("设置"));
    expect(await screen.findByText("退出登录")).toBeInTheDocument();
    await userEvent.click(screen.getByText("退出登录"));
    expect(onLogout).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && pnpm test -- src/console/ConsoleShell.test.tsx`
Expected: FAIL — `Cannot find module './ConsoleShell'`.

- [ ] **Step 3: Implement ConsoleShell**

Create `apps/web/src/console/ConsoleShell.tsx`:

```tsx
import { useState } from "react";
import type { ApiClient } from "../api";
import type { SessionStore } from "../shell/session";
import { SettingsView } from "../shell/settings/SettingsView";
import { ConsoleRail, type ConsoleTab } from "./ConsoleRail";
import { ClassesView } from "./ClassesView";
import { ClassDetailView } from "./ClassDetailView";

export type ConsoleClient = Pick<
  ApiClient,
  "listClasses" | "createClass" | "getClass" | "renameClass" | "regenerateJoinCode" | "removeEnrollment"
>;

export function ConsoleShell({
  session,
  client,
  onLogout,
}: {
  session: SessionStore;
  client: ConsoleClient;
  onLogout: () => void;
}) {
  const [tab, setTab] = useState<ConsoleTab>("classes");
  const [openClassId, setOpenClassId] = useState<string | null>(null);
  const user = session.getUser();
  const role = user?.role ?? "teacher";

  return (
    <div style={{ display: "flex", height: "100%", width: "100%", background: "#F3F4F8", overflow: "hidden" }}>
      <ConsoleRail tab={tab} onTab={(t) => { setTab(t); if (t === "classes") setOpenClassId(null); }} />
      <div style={{ flex: 1, overflow: "hidden", position: "relative", display: "flex" }}>
        {tab === "classes" && openClassId == null && (
          <ClassesView client={client} role={role} onOpenClass={setOpenClassId} />
        )}
        {tab === "classes" && openClassId != null && (
          <ClassDetailView client={client} classId={openClassId} onBack={() => setOpenClassId(null)} />
        )}
        {tab === "settings" && (
          <SettingsView session={session} user={user} onLogout={onLogout} />
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/web && pnpm test -- src/console/ConsoleShell.test.tsx`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/console/ConsoleShell.tsx apps/web/src/console/ConsoleShell.test.tsx
git commit -m "feat(p3): ConsoleShell — rail + classes/detail + settings

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 7: AppShell role-routing + StudentApp extraction

**Files:**
- Create: `apps/web/src/shell/StudentApp.tsx`
- Modify: `apps/web/src/shell/AppShell.tsx`
- Modify: `apps/web/src/shell/AppShell.test.tsx`
- Modify: `docs/遗留项追踪_Carryforward.md`

**Interfaces:**
- Consumes: `ConsoleShell` + `ConsoleClient` (Task 6); `Store`, `SessionStore`, `ApiClient`.
- Produces:
  - `function StudentApp({ store, session, onLogout }: { store: Store; session: SessionStore; onLogout: () => void }): JSX.Element` — today's tasks/records/settings UI.
  - `AppShell` routes on `role`: `teacher`/`admin` → `ConsoleShell`, otherwise → `StudentApp`.

- [ ] **Step 1: Extract StudentApp (behavior-preserving)**

Create `apps/web/src/shell/StudentApp.tsx` by lifting the current student UI out of `AppShell` (the tab state + the `<div style=…><LeftRail/>…</div>` block). It takes `onLogout` instead of building it inline:

```tsx
import { useState } from "react";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { Store } from "../store/createStore";
import type { SessionStore } from "./session";
import { LeftRail } from "./LeftRail";
import { DirectoryView } from "./directory/DirectoryView";
import { WorkspaceContainer } from "./WorkspaceContainer";
import { RecordsView } from "./records/RecordsView";
import { SettingsView } from "./settings/SettingsView";

type Tab = "tasks" | "records" | "settings";
type TaskView = "directory" | "workspace";

export function StudentApp({
  store,
  session,
  onLogout,
}: {
  store: Store;
  session: SessionStore;
  onLogout: () => void;
}) {
  const [tab, setTab] = useState<Tab>("tasks");
  const [taskView, setTaskView] = useState<TaskView>("directory");
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);
  const [openingMessage, setOpeningMessage] = useState<string | undefined>(undefined);

  return (
    <div style={{ display: "flex", height: "100%", width: "100%", background: "#F3F4F8", overflow: "hidden" }}>
      <LeftRail tab={tab} onTab={setTab} />
      <div style={{ flex: 1, overflow: "hidden", position: "relative" }}>
        {tab === "tasks" && taskView === "directory" && (
          <DirectoryView
            store={store}
            onOpenTask={(id, opening) => {
              setActiveTaskId(id);
              setOpeningMessage(opening);
              setTaskView("workspace");
            }}
          />
        )}
        {tab === "tasks" && taskView === "workspace" && (
          <WorkspaceContainer
            store={store}
            taskId={activeTaskId!}
            openingMessage={openingMessage}
            onBack={() => { setTaskView("directory"); setOpeningMessage(undefined); }}
          />
        )}
        {tab === "records" && <RecordsView store={store} registry={CARD_REGISTRY} />}
        {tab === "settings" && (
          <SettingsView session={session} user={session.getUser()} onLogout={onLogout} />
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Rewrite AppShell to route on role**

Replace `apps/web/src/shell/AppShell.tsx` with the boot/auth gate + role router. The `client` prop widens to include the class methods so it can pass them to `ConsoleShell`:

```tsx
import { useEffect, useState } from "react";
import { createStore } from "../store";
import { createSession, useSession, type SessionStore } from "./session";
import type { Store } from "../store/createStore";
import { api as defaultApi, type ApiClient, type MeUser } from "../api";
import { AuthScreen } from "./auth/AuthScreen";
import { StudentApp } from "./StudentApp";
import { ConsoleShell } from "../console/ConsoleShell";

const defaultStore = createStore({});
const defaultSession = createSession({ storage: window.localStorage });

type ShellClient = Pick<
  ApiClient,
  "getMe" | "signout" | "listClasses" | "createClass" | "getClass" | "renameClass" | "regenerateJoinCode" | "removeEnrollment"
>;

export function AppShell({
  store = defaultStore,
  session = defaultSession,
  client = defaultApi,
}: {
  store?: Store;
  session?: SessionStore;
  client?: ShellClient;
}) {
  const sess = useSession(session);
  const [booted, setBooted] = useState(false);

  useEffect(() => {
    let cancelled = false;
    client.getMe()
      .then((u: MeUser) => { if (!cancelled) { session.setUser(u); session.setAuthed(true); } })
      .catch(() => { if (!cancelled) session.setAuthed(false); })
      .finally(() => { if (!cancelled) setBooted(true); });
    return () => { cancelled = true; };
  }, [client, session]);

  if (!booted) {
    return <div style={{ width: "100%", height: "100%", background: "#F3F4F8" }} />;
  }
  if (!sess.authed) {
    return <AuthScreen onAuthed={(u) => { session.setUser(u); session.setAuthed(true); }} />;
  }

  const onLogout = () => { void client.signout().finally(() => { session.setUser(null); session.setAuthed(false); }); };
  const role = session.getUser()?.role;

  if (role === "teacher" || role === "admin") {
    return <ConsoleShell session={session} client={client} onLogout={onLogout} />;
  }
  return <StudentApp store={store} session={session} onLogout={onLogout} />;
}
```

- [ ] **Step 3: Add role-routing tests**

Append to `apps/web/src/shell/AppShell.test.tsx` (the existing two tests still pass — student role still shows the directory). Add a `teacher` fixture and two cases:

```tsx
  it("routes a teacher to the console (我的班级)", async () => {
    const store = createStore({});
    const session = createSession({ storage: mem() });
    const TEACHER = { ...ME, role: "teacher" };
    const client = {
      getMe: vi.fn(async () => TEACHER),
      signout: vi.fn(),
      listClasses: vi.fn(async () => []),
      createClass: vi.fn(), getClass: vi.fn(), renameClass: vi.fn(), regenerateJoinCode: vi.fn(), removeEnrollment: vi.fn(),
    };
    render(<AppShell store={store} session={session} client={client as never} />);
    await waitFor(() => expect(screen.getByText("我的班级")).toBeInTheDocument());
  });

  it("routes an admin to the console (全校班级)", async () => {
    const store = createStore({});
    const session = createSession({ storage: mem() });
    const ADMIN = { ...ME, role: "admin" };
    const client = {
      getMe: vi.fn(async () => ADMIN),
      signout: vi.fn(),
      listClasses: vi.fn(async () => []),
      createClass: vi.fn(), getClass: vi.fn(), renameClass: vi.fn(), regenerateJoinCode: vi.fn(), removeEnrollment: vi.fn(),
    };
    render(<AppShell store={store} session={session} client={client as never} />);
    await waitFor(() => expect(screen.getByText("全校班级")).toBeInTheDocument());
  });
```

- [ ] **Step 4: Run the full web suite + typecheck**

Run: `cd apps/web && pnpm typecheck && pnpm test`
Expected: typecheck clean; ALL tests PASS (existing student tests + new console + role-routing).

- [ ] **Step 5: Append the P3.2 carry-forward section**

Add to `docs/遗留项追踪_Carryforward.md` a P3.2 section recording: (1) admin class-creation + teacher-assignment deferred (backend requires `teacher_user_id`, needs a teacher-picker endpoint) → P3.3; (2) per-class student count on the classes-list card deferred (needs a backend `COUNT(enrollments)` on `GET /classes`); (3) admin-only screens (teacher-invites / import / overview) → P3.3; (4) per-student work-detail visibility still waits on the evaluation-data-model brainstorm. Match the file's existing section formatting.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/shell/StudentApp.tsx apps/web/src/shell/AppShell.tsx apps/web/src/shell/AppShell.test.tsx "docs/遗留项追踪_Carryforward.md"
git commit -m "feat(p3): AppShell role routing — student vs teacher/admin console

Extract StudentApp (behavior-preserving) and route teacher/admin to
ConsoleShell. Record P3.2 carry-forward.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Final verification (after all tasks)

- [ ] Run the whole web suite + typecheck: `cd apps/web && pnpm typecheck && pnpm test` — all green.
- [ ] Sanity-build: `cd apps/web && pnpm build` — succeeds.
- [ ] Manual smoke (optional, needs the API running): sign in as the seeded admin (`admin@demo.mindimprint.local`) → see 全校班级 (no create button); sign in as a teacher (created via an invite) → see 我的班级, create a class, open it, copy/rotate the join code, view the roster, remove a student.
