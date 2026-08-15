# Course Runtime — Slice 1: Contract & Validation Foundation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create `packages/course-contract` — the TypeScript types, Zod schemas, and deterministic cross-reference/workflow validators for CourseDefinition 2.0 and CourseSession, with golden fixtures — the single source of truth every later course-runtime slice depends on.

**Architecture:** A framework-free TS package (like `packages/contracts`), Zod as the single source of truth for both runtime validation and inferred types. Three validation layers run in order: **structural** (Zod `safeParse`), **referential** (unique IDs + every ID reference resolves + layout completeness + video/interaction source identity), and **workflow** (step reachability, transition-target existence, terminal-navigation correctness, no cross-slice branching, action/event compatibility, bounded cycles). A single `validateCourseDefinition(doc)` entry point returns a typed result with a flat list of issues. Golden fixtures (one valid course exercising every block type + layout, plus targeted invalid fixtures) lock the validators.

**Tech Stack:** TypeScript 5.4 (ES2022, `moduleResolution: Bundler`, `strict`, `noUncheckedIndexedAccess`), Zod ^3.23, Vitest ^1.6 (node environment), pnpm workspace package.

**Authoritative spec:** `docs/2026-08-15-student-course-runtime-data-and-renderer-design.md`. Every type shape in this plan is defined there by section; where this plan says "per §N", transcribe the field list **exactly** from that section — do not invent or omit fields. Unknown properties must fail validation (§5 normative rules), so schemas are **strict** (`.strict()`), not `.passthrough()`.

## Global Constraints

- Package name `@mind-imprint/course-contract`; `"type": "module"`; `main`/`types`/`exports` point at `src/index.ts` (no build step), mirroring `packages/contracts/package.json`.
- Zod is the single source of truth. Every exported TS type is `z.infer<typeof Schema>` — never a hand-written `interface` that could drift from its schema.
- All object schemas that represent CourseDefinition content use `.strict()` so unknown properties fail (§5: "Unknown properties fail deterministic validation"). CourseSession schemas may use `.strict()` too; payloads typed `unknown` in the spec stay `z.unknown()`.
- IDs are lower-case hyphenated strings (§5). Provide one shared `idSchema = z.string().regex(/^[a-z0-9]+(-[a-z0-9]+)*$/)` and reuse it for every `*Id`.
- `schemaVersion` is the literal `"2.0"` for CourseDefinition, `"1.1"` for VideoInteraction, `"1.0"` for SliceWorkflow, `"2.0"` for CourseSession `courseSchemaVersion` — copy each literal exactly from the spec.
- Relative asset paths must be validated as safe: no leading `/`, no `..` segment, no scheme (`://`), no backslash. Provide `relativeAssetPathSchema` and reuse it everywhere the spec says `RelativeAssetPath`.
- Validators are pure and deterministic: same input → same ordered issue list. No `Date.now()`, no randomness, no I/O (fixtures are imported JSON/TS, not read from disk at validate time).
- Tests live under `test/`; the package `test` script is `vitest run`, `typecheck` is `tsc --noEmit`, matching `packages/contracts`.

---

### Task 1: Scaffold the package

**Files:**
- Create: `packages/course-contract/package.json`
- Create: `packages/course-contract/tsconfig.json`
- Create: `packages/course-contract/vitest.config.ts`
- Create: `packages/course-contract/src/index.ts`
- Test: `packages/course-contract/test/smoke.test.ts`

**Interfaces:**
- Produces: the package `@mind-imprint/course-contract` importable via `workspace:*`; `src/index.ts` as the single public entry.

- [ ] **Step 1: Write the smoke test**

```ts
// test/smoke.test.ts
import { describe, it, expect } from "vitest";
import { COURSE_CONTRACT_VERSION } from "../src/index";

describe("course-contract package", () => {
  it("exposes a version constant", () => {
    expect(COURSE_CONTRACT_VERSION).toBe("0.0.0");
  });
});
```

- [ ] **Step 2: Create package.json** (mirror `packages/contracts/package.json`)

```json
{
  "name": "@mind-imprint/course-contract",
  "version": "0.0.0",
  "type": "module",
  "main": "src/index.ts",
  "types": "src/index.ts",
  "exports": { ".": "./src/index.ts" },
  "scripts": {
    "test": "vitest run",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "zod": "^3.23.0"
  },
  "devDependencies": {
    "typescript": "^5.4.0",
    "vitest": "^1.6.0"
  }
}
```

- [ ] **Step 3: Create tsconfig.json and vitest.config.ts**

```json
// tsconfig.json
{
  "extends": "../../tsconfig.base.json",
  "compilerOptions": { "rootDir": "." },
  "include": ["src", "test"]
}
```

```ts
// vitest.config.ts
import { defineConfig } from "vitest/config";

export default defineConfig({
  test: { environment: "node", include: ["test/**/*.test.ts", "src/**/*.test.ts"] },
});
```

- [ ] **Step 4: Create src/index.ts stub**

```ts
export const COURSE_CONTRACT_VERSION = "0.0.0";
```

- [ ] **Step 5: Install and run**

Run: `pnpm install` (from repo root, registers the new workspace package), then `pnpm --filter @mind-imprint/course-contract test` and `pnpm --filter @mind-imprint/course-contract typecheck`.
Expected: both PASS.

- [ ] **Step 6: Commit**

```bash
git add packages/course-contract/package.json packages/course-contract/tsconfig.json packages/course-contract/vitest.config.ts packages/course-contract/src/index.ts packages/course-contract/test/smoke.test.ts pnpm-lock.yaml
git commit -m "feat(course-contract): scaffold package"
```

---

### Task 2: Shared primitives — IDs, asset paths, branded IDs

**Files:**
- Create: `packages/course-contract/src/primitives.ts`
- Test: `packages/course-contract/test/primitives.test.ts`

**Interfaces:**
- Produces: `idSchema`, `relativeAssetPathSchema`, and per-namespace id schemas (`courseIdSchema`, `objectiveIdSchema`, `partIdSchema`, `sliceIdSchema`, `blockIdSchema`, `narrationIdSchema`, `workflowStepIdSchema`), plus the inferred branded types. Every later task imports these instead of re-declaring `z.string()`.

- [ ] **Step 1: Write the failing test**

```ts
// test/primitives.test.ts
import { describe, it, expect } from "vitest";
import { idSchema, relativeAssetPathSchema } from "../src/primitives";

describe("idSchema", () => {
  it("accepts lower-case hyphenated ids", () => {
    expect(idSchema.safeParse("slice-observe-1").success).toBe(true);
  });
  it("rejects upper-case, spaces, leading/trailing/double hyphens", () => {
    for (const bad of ["Slice", "a b", "-a", "a-", "a--b", ""]) {
      expect(idSchema.safeParse(bad).success).toBe(false);
    }
  });
});

describe("relativeAssetPathSchema", () => {
  it("accepts safe relative paths", () => {
    expect(relativeAssetPathSchema.safeParse("assets/audio/intro.mp3").success).toBe(true);
  });
  it("rejects absolute, parent-traversal, scheme, and backslash paths", () => {
    for (const bad of ["/assets/x.png", "../secret", "a/../b", "https://x/y.png", "a\\b", ""]) {
      expect(relativeAssetPathSchema.safeParse(bad).success).toBe(false);
    }
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter @mind-imprint/course-contract test primitives`
Expected: FAIL (module not found).

- [ ] **Step 3: Implement primitives.ts**

```ts
import { z } from "zod";

/** Lower-case hyphenated identifier (§5): segments of [a-z0-9] joined by single hyphens. */
export const idSchema = z.string().regex(/^[a-z0-9]+(-[a-z0-9]+)*$/, "must be a lower-case hyphenated id");

/**
 * A safe relative asset path (§4, §20): no leading slash, no `..` segment, no
 * URL scheme, no backslash. The AssetResolver later maps this to a preview or
 * CDN URL; the package itself never resolves it.
 */
export const relativeAssetPathSchema = z
  .string()
  .min(1)
  .refine((p) => !p.startsWith("/"), "must not be absolute")
  .refine((p) => !p.includes("\\"), "must not contain backslashes")
  .refine((p) => !/^[a-z][a-z0-9+.-]*:\/\//i.test(p), "must not contain a URL scheme")
  .refine((p) => !p.split("/").includes(".."), "must not contain a parent-directory segment");

// Namespace ids all share idSchema's shape; distinct exports document intent and
// let later code read as the spec does. (Branding is documentation-only here.)
export const courseIdSchema = idSchema;
export const objectiveIdSchema = idSchema;
export const partIdSchema = idSchema;
export const sliceIdSchema = idSchema;
export const blockIdSchema = idSchema;
export const narrationIdSchema = idSchema;
export const workflowStepIdSchema = idSchema;

export type Id = z.infer<typeof idSchema>;
export type RelativeAssetPath = z.infer<typeof relativeAssetPathSchema>;
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter @mind-imprint/course-contract test primitives`
Expected: PASS.

- [ ] **Step 5: Export from index and commit**

Add `export * from "./primitives";` to `src/index.ts`.

```bash
git add packages/course-contract/src/primitives.ts packages/course-contract/test/primitives.test.ts packages/course-contract/src/index.ts
git commit -m "feat(course-contract): shared id and asset-path primitives"
```

---

### Task 3: Block schemas (all seven block types)

**Files:**
- Create: `packages/course-contract/src/blocks.ts`
- Test: `packages/course-contract/test/blocks.test.ts`

**Spec:** §8 (shared rules), §9.1–§9.7. Transcribe each block's fields **exactly** from those sections. Blocks are a discriminated union on `type`.

**Interfaces:**
- Produces: `BlockDefinition` (discriminated union), each member schema (`TextBlock`, `ImagesBlock`, `PdfBlock`, `VideoBlock`, `InteractiveHtmlBlock`, `FillBlankBlock`, `SingleChoiceBlock`), plus the shared assessment sub-schemas reused by video cues in a later slice: `ChoiceOption`, `SingleChoiceAssessment`, `FillBlankAssessment`, `SingleChoiceCompletionRule`, `FillBlankCompletionRule`. Export these sub-schemas — Task in Slice 5 (video cues, §14) reuses them.

- [ ] **Step 1: Write the failing test** (representative coverage; add the remaining block types following the same shape)

```ts
// test/blocks.test.ts
import { describe, it, expect } from "vitest";
import { BlockDefinition } from "../src/blocks";

const textBlock = { id: "explain", type: "text", content: "Hello." };
const singleChoice = {
  id: "q1",
  type: "singleChoice",
  prompt: "Comparable?",
  options: [ { id: "yes", label: "Yes" }, { id: "no", label: "No" } ],
  assessment: { mode: "graded", correctOptionId: "no" },
  completion: { rule: "submit-correct-or-exhausted", maxAttempts: 3 },
};

describe("BlockDefinition", () => {
  it("parses a text block", () => {
    expect(BlockDefinition.safeParse(textBlock).success).toBe(true);
  });
  it("parses a graded single-choice block", () => {
    expect(BlockDefinition.safeParse(singleChoice).success).toBe(true);
  });
  it("rejects unknown properties (strict)", () => {
    expect(BlockDefinition.safeParse({ ...textBlock, extra: 1 }).success).toBe(false);
  });
  it("rejects an unknown block type", () => {
    expect(BlockDefinition.safeParse({ id: "x", type: "audio" }).success).toBe(false);
  });
  it("rejects a graded single-choice whose completion is submit-any-only shape mismatch", () => {
    // maxAttempts is required by submit-correct-or-exhausted
    const bad = { ...singleChoice, completion: { rule: "submit-correct-or-exhausted" } };
    expect(BlockDefinition.safeParse(bad).success).toBe(false);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter @mind-imprint/course-contract test blocks`
Expected: FAIL (module not found).

- [ ] **Step 3: Implement blocks.ts**

Transcribe every block per §9. Full reference implementation of the shared assessment sub-schemas and the two assessment blocks (the fiddliest); transcribe the media/static blocks (`text`, `images`, `pdf`, `video`, `interactiveHtml`) from §9.1–§9.5 in the same `.strict()` style.

```ts
import { z } from "zod";
import { blockIdSchema, relativeAssetPathSchema } from "./primitives";

// ---- shared assessment sub-schemas (reused by video cues, §14) ----
export const ChoiceOption = z.object({ id: z.string().min(1), label: z.string().min(1) }).strict();

export const SingleChoiceAssessment = z.discriminatedUnion("mode", [
  z.object({
    mode: z.literal("graded"),
    correctOptionId: z.string().min(1),
    correctFeedback: z.string().optional(),
    incorrectFeedback: z.string().optional(),
  }).strict(),
  z.object({ mode: z.literal("survey") }).strict(),
]);

export const FillBlankAssessment = z.discriminatedUnion("mode", [
  z.object({
    mode: z.literal("graded"),
    acceptedAnswers: z.array(z.string().min(1)).min(1),
    caseSensitive: z.boolean().optional(),
    correctFeedback: z.string().optional(),
    incorrectFeedback: z.string().optional(),
  }).strict(),
  z.object({ mode: z.literal("reflection"), rubric: z.string().min(1) }).strict(),
]);

const submitAny = z.object({ rule: z.literal("submit-any") }).strict();
const submitCorrect = z.object({ rule: z.literal("submit-correct") }).strict();
const submitCorrectOrExhausted = z
  .object({ rule: z.literal("submit-correct-or-exhausted"), maxAttempts: z.number().int().positive() })
  .strict();

export const SingleChoiceCompletionRule = z.discriminatedUnion("rule", [submitAny, submitCorrect, submitCorrectOrExhausted]);
export const FillBlankCompletionRule = z.discriminatedUnion("rule", [submitAny, submitCorrect, submitCorrectOrExhausted]);

// ---- block members ----
export const TextBlock = z.object({ id: blockIdSchema, type: z.literal("text"), content: z.string() }).strict();

export const ImageItem = z
  .object({ id: z.string().min(1), source: relativeAssetPathSchema, alt: z.string().min(1), caption: z.string().optional() })
  .strict();
export const ImagesBlock = z
  .object({
    id: blockIdSchema,
    type: z.literal("images"),
    presentation: z.enum(["single", "side-by-side", "gallery"]),
    items: z.array(ImageItem).min(1),
  })
  .strict();

export const PdfBlock = z
  .object({
    id: blockIdSchema,
    type: z.literal("pdf"),
    title: z.string().min(1),
    source: relativeAssetPathSchema,
    initialPage: z.number().int().positive().optional(),
  })
  .strict();

export const VideoBlock = z
  .object({
    id: blockIdSchema,
    type: z.literal("video"),
    source: relativeAssetPathSchema,
    poster: relativeAssetPathSchema.optional(),
    captions: relativeAssetPathSchema.optional(),
    durationSeconds: z.number().positive().optional(),
    interaction: z.object({ source: relativeAssetPathSchema }).strict().optional(),
    completion: z
      .discriminatedUnion("rule", [
        z.object({ rule: z.literal("video-ended") }).strict(),
        z.object({ rule: z.literal("video-ended-and-interactions-completed") }).strict(),
      ])
      .optional(),
  })
  .strict();

export const InteractiveHtmlBlock = z
  .object({
    id: blockIdSchema,
    type: z.literal("interactiveHtml"),
    source: relativeAssetPathSchema,
    protocolVersion: z.literal("1.0"),
    aspectRatio: z.enum(["1:1", "4:3"]),
    completion: z.object({ rule: z.literal("interaction-complete") }).strict().optional(),
  })
  .strict();

export const FillBlankBlock = z
  .object({
    id: blockIdSchema,
    type: z.literal("fillBlank"),
    prompt: z.string().min(1),
    placeholder: z.string().optional(),
    assessment: FillBlankAssessment,
    completion: FillBlankCompletionRule,
  })
  .strict();

export const SingleChoiceBlock = z
  .object({
    id: blockIdSchema,
    type: z.literal("singleChoice"),
    prompt: z.string().min(1),
    options: z.array(ChoiceOption).min(2),
    assessment: SingleChoiceAssessment,
    completion: SingleChoiceCompletionRule,
  })
  .strict();

export const BlockDefinition = z.discriminatedUnion("type", [
  TextBlock,
  ImagesBlock,
  PdfBlock,
  VideoBlock,
  InteractiveHtmlBlock,
  FillBlankBlock,
  SingleChoiceBlock,
]);

export type BlockDefinition = z.infer<typeof BlockDefinition>;
export type BlockType = BlockDefinition["type"];
```

Note: cross-field rules that Zod cannot express on a single object (e.g. `correctOptionId` must reference an existing option; `graded` fill-blank incompatible with `submit-any`) belong to **referential validation** (Task 8), not here — a comment in the file should say so.

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter @mind-imprint/course-contract test blocks`
Expected: PASS.

- [ ] **Step 5: Export from index and commit**

Add `export * from "./blocks";` to `src/index.ts`.

```bash
git add packages/course-contract/src/blocks.ts packages/course-contract/test/blocks.test.ts packages/course-contract/src/index.ts
git commit -m "feat(course-contract): block definition schemas"
```

---

### Task 4: Layout, Narration, Navigation schemas

**Files:**
- Create: `packages/course-contract/src/layout.ts`
- Create: `packages/course-contract/src/narration.ts`
- Create: `packages/course-contract/src/navigation.ts`
- Test: `packages/course-contract/test/layout.test.ts`

**Spec:** §10 (Layout), §11 (Narration), §13 (Navigation). The per-preset required-slot rule (`full`→`main`; `split-*`→two named slots + required `ratio`; `grid`→2–4 cells) is partly structural (this task) and partly referential (every block assigned to exactly one slot — Task 8).

**Interfaces:**
- Produces: `LayoutDefinition`, `LayoutSlot`, `LayoutPreset`, `SplitRatio`; `NarrationDefinition`; `NavigationDefinition`.

- [ ] **Step 1: Write the failing test**

```ts
// test/layout.test.ts
import { describe, it, expect } from "vitest";
import { LayoutDefinition } from "../src/layout";

const splitH = {
  preset: "split-horizontal",
  ratio: "2:1",
  slots: [ { id: "left", blockIds: ["v"] }, { id: "right", blockIds: ["q"] } ],
};

describe("LayoutDefinition", () => {
  it("parses a split-horizontal with a ratio and canonical slots", () => {
    expect(LayoutDefinition.safeParse(splitH).success).toBe(true);
  });
  it("rejects split-horizontal without a ratio", () => {
    const { ratio, ...noRatio } = splitH;
    expect(LayoutDefinition.safeParse(noRatio).success).toBe(false);
  });
  it("rejects split-horizontal with wrong slot ids", () => {
    const bad = { ...splitH, slots: [ { id: "top", blockIds: ["v"] }, { id: "bottom", blockIds: ["q"] } ] };
    expect(LayoutDefinition.safeParse(bad).success).toBe(false);
  });
  it("accepts a full layout with the single main slot", () => {
    expect(LayoutDefinition.safeParse({ preset: "full", slots: [ { id: "main", blockIds: ["t"] } ] }).success).toBe(true);
  });
  it("accepts grid with 2–4 cells, rejects 1 or 5", () => {
    const cells = (n: number) => ({ preset: "grid", slots: Array.from({ length: n }, (_, i) => ({ id: `cell-${i + 1}`, blockIds: [] })) });
    expect(LayoutDefinition.safeParse(cells(2)).success).toBe(true);
    expect(LayoutDefinition.safeParse(cells(4)).success).toBe(true);
    expect(LayoutDefinition.safeParse(cells(1)).success).toBe(false);
    expect(LayoutDefinition.safeParse(cells(5)).success).toBe(false);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter @mind-imprint/course-contract test layout`
Expected: FAIL.

- [ ] **Step 3: Implement the three files**

`layout.ts` — enforce the per-preset slot rule with a `superRefine`, since it spans preset + slots:

```ts
import { z } from "zod";
import { blockIdSchema } from "./primitives";

export const LayoutPreset = z.enum(["full", "split-horizontal", "split-vertical", "grid"]);
export const SplitRatio = z.enum(["1:1", "2:1", "1:2"]);

export const LayoutSlot = z.object({ id: z.string().min(1), blockIds: z.array(blockIdSchema) }).strict();

const GRID_CELL_IDS = ["cell-1", "cell-2", "cell-3", "cell-4"] as const;

export const LayoutDefinition = z
  .object({ preset: LayoutPreset, ratio: SplitRatio.optional(), slots: z.array(LayoutSlot).min(1) })
  .strict()
  .superRefine((layout, ctx) => {
    const ids = layout.slots.map((s) => s.id);
    const has = (want: string[]) => want.length === ids.length && want.every((w) => ids.includes(w));
    switch (layout.preset) {
      case "full":
        if (!has(["main"])) ctx.addIssue({ code: "custom", message: "full layout requires exactly one slot 'main'" });
        break;
      case "split-horizontal":
        if (!layout.ratio) ctx.addIssue({ code: "custom", message: "split-horizontal requires a ratio" });
        if (!has(["left", "right"])) ctx.addIssue({ code: "custom", message: "split-horizontal requires slots 'left' and 'right'" });
        break;
      case "split-vertical":
        if (!layout.ratio) ctx.addIssue({ code: "custom", message: "split-vertical requires a ratio" });
        if (!has(["top", "bottom"])) ctx.addIssue({ code: "custom", message: "split-vertical requires slots 'top' and 'bottom'" });
        break;
      case "grid": {
        const n = ids.length;
        const canonical = n >= 2 && n <= 4 && ids.every((id, i) => id === GRID_CELL_IDS[i]);
        if (!canonical) ctx.addIssue({ code: "custom", message: "grid requires 2–4 slots named cell-1..cell-N in order" });
        break;
      }
    }
  });

export type LayoutDefinition = z.infer<typeof LayoutDefinition>;
```

`narration.ts` (§11) and `navigation.ts` (§13) — transcribe exactly:

```ts
// narration.ts
import { z } from "zod";
import { narrationIdSchema, relativeAssetPathSchema } from "./primitives";
export const NarrationDefinition = z
  .object({ id: narrationIdSchema, text: z.string().min(1), audio: relativeAssetPathSchema, durationSeconds: z.number().positive().optional() })
  .strict();
export type NarrationDefinition = z.infer<typeof NarrationDefinition>;
```

```ts
// navigation.ts
import { z } from "zod";
export const NavigationDefinition = z
  .object({
    previous: z.literal("allowed"),
    manualNext: z.enum(["after-completion", "allowed"]),
    autoNext: z.boolean(),
    revisit: z.literal("restore-completed-state"),
  })
  .strict();
export type NavigationDefinition = z.infer<typeof NavigationDefinition>;
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter @mind-imprint/course-contract test layout`
Expected: PASS.

- [ ] **Step 5: Export from index and commit**

Add exports for the three files to `src/index.ts`.

```bash
git add packages/course-contract/src/layout.ts packages/course-contract/src/narration.ts packages/course-contract/src/navigation.ts packages/course-contract/test/layout.test.ts packages/course-contract/src/index.ts
git commit -m "feat(course-contract): layout, narration, navigation schemas"
```

---

### Task 5: Slice Workflow schema

**Files:**
- Create: `packages/course-contract/src/workflow.ts`
- Test: `packages/course-contract/test/workflow.test.ts`

**Spec:** §12.1–§12.4. Transcribe `SliceWorkflow`, `SliceInitialState`, `WorkflowStep`, `WorkflowTransition`, `WorkflowEventMatcher`, `TargetRef`, and the full `WorkflowAction` union (§12.3) and `WorkflowEventType` vocabulary (§12.4 table) exactly. Structural only here — graph properties (reachability, terminal navigation, no cross-slice) are Task 9.

**Interfaces:**
- Produces: `SliceWorkflow`, `WorkflowAction`, `WorkflowStep`, `WorkflowTransition`, `WorkflowEventMatcher`, `WorkflowEventType`, `TargetRef`, `SliceInitialState`.

- [ ] **Step 1: Write the failing test**

```ts
// test/workflow.test.ts
import { describe, it, expect } from "vitest";
import { SliceWorkflow, WorkflowAction } from "../src/workflow";

describe("WorkflowAction", () => {
  it("parses each action variant shape", () => {
    const actions = [
      { type: "show", targetId: "q" },
      { type: "focus", target: { blockId: "img", itemId: "a" } },
      { type: "clearFocus" },
      { type: "playNarration", narrationId: "intro" },
      { type: "startTimer", timerId: "t1", durationSeconds: 5 },
      { type: "completeSlice" },
      { type: "navigate", target: "nextSlice" },
    ];
    for (const a of actions) expect(WorkflowAction.safeParse(a).success).toBe(true);
  });
  it("rejects navigate to anything but nextSlice", () => {
    expect(WorkflowAction.safeParse({ type: "navigate", target: "prevSlice" }).success).toBe(false);
  });
  it("rejects an action carrying an arbitrary payload (strict)", () => {
    expect(WorkflowAction.safeParse({ type: "show", targetId: "q", url: "http://x" }).success).toBe(false);
  });
});

describe("SliceWorkflow", () => {
  it("parses a minimal one-step workflow", () => {
    const wf = { version: "1.0", initialStepId: "s1", steps: [ { id: "s1", enterActions: [], transitions: [] } ] };
    expect(SliceWorkflow.safeParse(wf).success).toBe(true);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter @mind-imprint/course-contract test workflow`
Expected: FAIL.

- [ ] **Step 3: Implement workflow.ts** (transcribe §12.3 action union and §12.4 event vocabulary in full)

```ts
import { z } from "zod";
import { blockIdSchema, narrationIdSchema, workflowStepIdSchema } from "./primitives";

export const TargetRef = z.union([
  z.object({ blockId: blockIdSchema }).strict(),
  z.object({ blockId: blockIdSchema, itemId: z.string().min(1) }).strict(),
]);

export const WorkflowAction = z.discriminatedUnion("type", [
  z.object({ type: z.literal("show"), targetId: blockIdSchema }).strict(),
  z.object({ type: z.literal("hide"), targetId: blockIdSchema }).strict(),
  z.object({ type: z.literal("focus"), target: TargetRef }).strict(),
  z.object({ type: z.literal("clearFocus") }).strict(),
  z.object({ type: z.literal("enable"), targetId: blockIdSchema }).strict(),
  z.object({ type: z.literal("disable"), targetId: blockIdSchema }).strict(),
  z.object({ type: z.literal("playNarration"), narrationId: narrationIdSchema }).strict(),
  z.object({ type: z.literal("pauseNarration"), narrationId: narrationIdSchema }).strict(),
  z.object({ type: z.literal("stopNarration"), narrationId: narrationIdSchema }).strict(),
  z.object({ type: z.literal("playBlock"), targetId: blockIdSchema }).strict(),
  z.object({ type: z.literal("pauseBlock"), targetId: blockIdSchema }).strict(),
  z.object({ type: z.literal("resetBlock"), targetId: blockIdSchema }).strict(),
  z.object({ type: z.literal("startTimer"), timerId: z.string().min(1), durationSeconds: z.number().positive() }).strict(),
  z.object({ type: z.literal("cancelTimer"), timerId: z.string().min(1) }).strict(),
  z.object({ type: z.literal("completeSlice") }).strict(),
  z.object({ type: z.literal("navigate"), target: z.literal("nextSlice") }).strict(),
]);

export const WorkflowEventType = z.enum([
  "narration.ended",
  "video.started",
  "video.paused",
  "video.ended",
  "video.interaction.shown",
  "video.interaction.completed",
  "pdf.opened",
  "pdf.pageChanged",
  "interaction.completed",
  "answer.submitted",
  "answer.correct",
  "answer.incorrect",
  "answer.attemptsExhausted",
  "block.completed",
  "student.continue",
  "timer.elapsed",
]);

export const WorkflowEventMatcher = z
  .object({
    type: WorkflowEventType,
    sourceId: z.string().min(1).optional(),
    interactionId: z.string().min(1).optional(),
    timerId: z.string().min(1).optional(),
  })
  .strict();

export const WorkflowTransition = z.object({ on: WorkflowEventMatcher, to: workflowStepIdSchema }).strict();

export const WorkflowStep = z
  .object({ id: workflowStepIdSchema, enterActions: z.array(WorkflowAction), transitions: z.array(WorkflowTransition) })
  .strict();

export const SliceInitialState = z
  .object({
    visibleBlockIds: z.array(blockIdSchema).optional(),
    enabledBlockIds: z.array(blockIdSchema).optional(),
    focusedTarget: TargetRef.optional(),
  })
  .strict();

export const SliceWorkflow = z
  .object({
    version: z.literal("1.0"),
    initialStepId: workflowStepIdSchema,
    initialState: SliceInitialState.optional(),
    steps: z.array(WorkflowStep).min(1),
  })
  .strict();

export type WorkflowAction = z.infer<typeof WorkflowAction>;
export type WorkflowEventType = z.infer<typeof WorkflowEventType>;
export type WorkflowStep = z.infer<typeof WorkflowStep>;
export type SliceWorkflow = z.infer<typeof SliceWorkflow>;
export type TargetRef = z.infer<typeof TargetRef>;
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter @mind-imprint/course-contract test workflow`
Expected: PASS.

- [ ] **Step 5: Export from index and commit**

```bash
git add packages/course-contract/src/workflow.ts packages/course-contract/test/workflow.test.ts packages/course-contract/src/index.ts
git commit -m "feat(course-contract): slice workflow schema"
```

---

### Task 6: Course-level schemas — Objective, Part, Slice, Opening, Closing, CourseDefinition

**Files:**
- Create: `packages/course-contract/src/course.ts`
- Test: `packages/course-contract/test/course.test.ts`

**Spec:** §5 (top-level), §6.1–§6.2 (Opening/Closing), §7 (Part/Slice). Transcribe exactly. `CourseDefinitionDocument` wraps `{ schemaVersion: "2.0", course }`.

**Interfaces:**
- Consumes: blocks, layout, narration, navigation, workflow schemas from Tasks 3–5.
- Produces: `CourseObjective`, `PartDefinition`, `SliceDefinition`, `OpeningDefinition`, `ClosingDefinition`, `CourseDefinition`, `CourseDefinitionDocument`.

- [ ] **Step 1: Write the failing test** (parse the golden §15 example built in Task 10; for now, a minimal inline course)

```ts
// test/course.test.ts
import { describe, it, expect } from "vitest";
import { CourseDefinitionDocument } from "../src/course";

const minimal = {
  schemaVersion: "2.0",
  course: {
    id: "demo",
    title: "Demo",
    language: "en",
    estimatedMinutes: 1,
    objectives: [ { id: "o1", text: "Learn.", evidenceBlockIds: ["q"] } ],
    opening: { learningPreview: ["a"], personalization: { enabled: false, allowedSignals: [] }, fallback: { text: "Welcome." } },
    parts: [ {
      id: "p1", title: "Part", objectiveIds: ["o1"],
      slices: [ {
        id: "s1", title: "Slice", objectiveIds: ["o1"], estimatedSeconds: 30,
        blocks: [ { id: "q", type: "text", content: "hi" } ],
        layout: { preset: "full", slots: [ { id: "main", blockIds: ["q"] } ] },
        narrations: [], 
        workflow: { version: "1.0", initialStepId: "only", steps: [ { id: "only", enterActions: [ { type: "completeSlice" }, { type: "navigate", target: "nextSlice" } ], transitions: [] } ] },
        navigation: { previous: "allowed", manualNext: "after-completion", autoNext: true, revisit: "restore-completed-state" },
      } ],
    } ],
    closing: { preparedSummary: "s", takeaways: ["t"], transferApplications: ["x"], personalization: { enabled: false, allowedSignals: [] }, fallback: { text: "Done." } },
  },
};

describe("CourseDefinitionDocument", () => {
  it("parses a minimal 2.0 course", () => {
    const r = CourseDefinitionDocument.safeParse(minimal);
    expect(r.success).toBe(true);
  });
  it("rejects a wrong schemaVersion", () => {
    expect(CourseDefinitionDocument.safeParse({ ...minimal, schemaVersion: "1.0" }).success).toBe(false);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter @mind-imprint/course-contract test course`
Expected: FAIL.

- [ ] **Step 3: Implement course.ts** (transcribe §5, §6, §7 exactly)

```ts
import { z } from "zod";
import { courseIdSchema, objectiveIdSchema, partIdSchema, sliceIdSchema, blockIdSchema, relativeAssetPathSchema } from "./primitives";
import { BlockDefinition } from "./blocks";
import { LayoutDefinition } from "./layout";
import { NarrationDefinition } from "./narration";
import { NavigationDefinition } from "./navigation";
import { SliceWorkflow } from "./workflow";

export const CourseObjective = z
  .object({ id: objectiveIdSchema, text: z.string().min(1), evidenceBlockIds: z.array(blockIdSchema).min(1) })
  .strict();

const OpeningSignal = z.enum(["recent-course-topics", "prior-objective-performance"]);
export const OpeningDefinition = z
  .object({
    learningPreview: z.array(z.string().min(1)),
    personalization: z.object({ enabled: z.boolean(), allowedSignals: z.array(OpeningSignal) }).strict(),
    fallback: z.object({ text: z.string().min(1), audio: relativeAssetPathSchema.optional() }).strict(),
  })
  .strict();

const ClosingSignal = z.enum(["answers", "attempts", "time-on-slice", "interaction-results"]);
export const ClosingDefinition = z
  .object({
    preparedSummary: z.string().min(1),
    takeaways: z.array(z.string().min(1)),
    transferApplications: z.array(z.string().min(1)),
    personalization: z.object({ enabled: z.boolean(), allowedSignals: z.array(ClosingSignal) }).strict(),
    fallback: z.object({ text: z.string().min(1), audio: relativeAssetPathSchema.optional() }).strict(),
  })
  .strict();

export const SliceDefinition = z
  .object({
    id: sliceIdSchema,
    title: z.string().min(1),
    objectiveIds: z.array(objectiveIdSchema),
    estimatedSeconds: z.number().positive(),
    blocks: z.array(BlockDefinition).min(1),
    layout: LayoutDefinition,
    narrations: z.array(NarrationDefinition),
    workflow: SliceWorkflow,
    navigation: NavigationDefinition,
  })
  .strict();

export const PartDefinition = z
  .object({ id: partIdSchema, title: z.string().min(1), objectiveIds: z.array(objectiveIdSchema), slices: z.array(SliceDefinition).min(1) })
  .strict();

export const CourseDefinition = z
  .object({
    id: courseIdSchema,
    title: z.string().min(1),
    language: z.string().min(2), // BCP-47; deep validation deferred
    estimatedMinutes: z.number().positive(),
    objectives: z.array(CourseObjective).min(1),
    opening: OpeningDefinition,
    parts: z.array(PartDefinition).min(1),
    closing: ClosingDefinition,
  })
  .strict();

export const CourseDefinitionDocument = z.object({ schemaVersion: z.literal("2.0"), course: CourseDefinition }).strict();

export type CourseObjective = z.infer<typeof CourseObjective>;
export type SliceDefinition = z.infer<typeof SliceDefinition>;
export type PartDefinition = z.infer<typeof PartDefinition>;
export type CourseDefinition = z.infer<typeof CourseDefinition>;
export type CourseDefinitionDocument = z.infer<typeof CourseDefinitionDocument>;
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter @mind-imprint/course-contract test course`
Expected: PASS.

- [ ] **Step 5: Export from index and commit**

```bash
git add packages/course-contract/src/course.ts packages/course-contract/test/course.test.ts packages/course-contract/src/index.ts
git commit -m "feat(course-contract): course-level definition schemas"
```

---

### Task 7: CourseSession schemas

**Files:**
- Create: `packages/course-contract/src/session.ts`
- Test: `packages/course-contract/test/session.test.ts`

**Spec:** §16. Transcribe `CourseSession`, `SliceSessionState`, `BlockSessionState`, `CourseRuntimeEvent`, `RuntimeSceneResult` (§6.3). Payload fields the spec types `unknown` stay `z.unknown()`.

**Interfaces:**
- Produces: `CourseSession`, `SliceSessionState`, `BlockSessionState`, `CourseRuntimeEvent`, `RuntimeSceneResult`.

- [ ] **Step 1: Write the failing test**

```ts
// test/session.test.ts
import { describe, it, expect } from "vitest";
import { CourseSession, RuntimeSceneResult } from "../src/session";

describe("CourseSession", () => {
  it("parses a created session with no progress", () => {
    const s = { id: "sess-1", courseId: "demo", courseSchemaVersion: "2.0", studentId: "stu-1", status: "created", sliceStates: {}, events: [] };
    expect(CourseSession.safeParse(s).success).toBe(true);
  });
  it("rejects an unknown status", () => {
    const s = { id: "sess-1", courseId: "demo", courseSchemaVersion: "2.0", studentId: "stu-1", status: "paused", sliceStates: {}, events: [] };
    expect(CourseSession.safeParse(s).success).toBe(false);
  });
});

describe("RuntimeSceneResult", () => {
  it("parses a generated scene result", () => {
    const r = { text: "hi", generatedAt: "2026-08-16T00:00:00Z", usedSignalTypes: [], fallbackUsed: false };
    expect(RuntimeSceneResult.safeParse(r).success).toBe(true);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter @mind-imprint/course-contract test session`
Expected: FAIL.

- [ ] **Step 3: Implement session.ts** (transcribe §16 + §6.3)

```ts
import { z } from "zod";
import { courseIdSchema, partIdSchema, sliceIdSchema, blockIdSchema, workflowStepIdSchema } from "./primitives";
import { WorkflowEventType } from "./workflow";

export const RuntimeSceneResult = z
  .object({
    text: z.string(),
    audioUrl: z.string().optional(),
    generatedAt: z.string(),
    usedSignalTypes: z.array(z.string()),
    fallbackUsed: z.boolean(),
  })
  .strict();

export const BlockSessionState = z
  .object({
    visible: z.boolean(),
    enabled: z.boolean(),
    completed: z.boolean(),
    attempts: z.number().int().nonnegative().optional(),
    answer: z.unknown().optional(),
    mediaPositionSeconds: z.number().nonnegative().optional(),
    interactionResult: z.unknown().optional(),
  })
  .strict();

export const SliceSessionState = z
  .object({
    status: z.enum(["not-started", "in-progress", "completed"]),
    currentWorkflowStepId: workflowStepIdSchema.optional(),
    startedAt: z.string().optional(),
    completedAt: z.string().optional(),
    elapsedSeconds: z.number().nonnegative(),
    blockStates: z.record(blockIdSchema, BlockSessionState),
  })
  .strict();

export const CourseRuntimeEvent = z
  .object({
    id: z.string(),
    sessionId: z.string(),
    courseId: courseIdSchema,
    partId: partIdSchema.optional(),
    sliceId: sliceIdSchema.optional(),
    sourceId: z.string(),
    type: z.union([WorkflowEventType, z.string()]),
    occurredAt: z.string(),
    payload: z.unknown(),
  })
  .strict();

export const CourseSession = z
  .object({
    id: z.string(),
    courseId: courseIdSchema,
    courseSchemaVersion: z.literal("2.0"),
    studentId: z.string(),
    status: z.enum(["created", "opening", "in-progress", "closing", "completed"]),
    startedAt: z.string().optional(),
    completedAt: z.string().optional(),
    current: z.object({ partId: partIdSchema, sliceId: sliceIdSchema, workflowStepId: workflowStepIdSchema }).strict().optional(),
    opening: RuntimeSceneResult.optional(),
    sliceStates: z.record(sliceIdSchema, SliceSessionState),
    events: z.array(CourseRuntimeEvent),
    closing: RuntimeSceneResult.optional(),
  })
  .strict();

export type CourseSession = z.infer<typeof CourseSession>;
export type SliceSessionState = z.infer<typeof SliceSessionState>;
export type BlockSessionState = z.infer<typeof BlockSessionState>;
export type CourseRuntimeEvent = z.infer<typeof CourseRuntimeEvent>;
export type RuntimeSceneResult = z.infer<typeof RuntimeSceneResult>;
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter @mind-imprint/course-contract test session`
Expected: PASS.

- [ ] **Step 5: Export from index and commit**

```bash
git add packages/course-contract/src/session.ts packages/course-contract/test/session.test.ts packages/course-contract/src/index.ts
git commit -m "feat(course-contract): course session schemas"
```

---

### Task 8: Referential validation

**Files:**
- Create: `packages/course-contract/src/validate/types.ts`
- Create: `packages/course-contract/src/validate/referential.ts`
- Test: `packages/course-contract/test/referential.test.ts`

**Spec:** §5 (unique IDs; every objective references a real block), §8 (block belongs to exactly one slice, assigned to exactly one slot), §9.6/§9.7 (`correctOptionId` references an option; `graded` fill-blank incompatible with `submit-any`), §10 (every slice block in exactly one slot), §14/§19.2 (video block ↔ interaction `source` identity — the block's `interaction.source` is asserted, the interaction document itself is validated in a later slice). Operates on an **already structurally-valid** `CourseDefinition` (post-Zod).

**Interfaces:**
- Consumes: `CourseDefinition` from Task 6.
- Produces: `ValidationIssue` type (`{ path: string; message: string; layer: "structural" | "referential" | "workflow" }`), and `validateReferential(course: CourseDefinition): ValidationIssue[]`.

- [ ] **Step 1: Write the failing tests** (one per rule; build minimal courses that violate exactly one)

```ts
// test/referential.test.ts
import { describe, it, expect } from "vitest";
import { validateReferential } from "../src/validate/referential";
import { validCourse } from "./fixtures";

const clone = () => structuredClone(validCourse);
const messages = (issues: { message: string }[]) => issues.map((i) => i.message).join(" | ");

describe("validateReferential", () => {
  it("passes on the golden course", () => {
    expect(validateReferential(validCourse)).toEqual([]);
  });
  it("flags a duplicate block id across slices", () => {
    const c = clone();
    c.parts[0].slices[0].blocks[0].id = "dup";
    c.parts[0].slices[1].blocks[0].id = "dup"; // fixture must have ≥2 slices
    expect(messages(validateReferential(c))).toMatch(/duplicate block id/i);
  });
  it("flags a block not assigned to any slot", () => {
    const c = clone();
    c.parts[0].slices[0].layout.slots[0].blockIds = [];
    expect(messages(validateReferential(c))).toMatch(/not assigned to a slot/i);
  });
  it("flags a slot referencing a non-existent block", () => {
    const c = clone();
    c.parts[0].slices[0].layout.slots[0].blockIds.push("ghost");
    expect(messages(validateReferential(c))).toMatch(/unknown block/i);
  });
  it("flags an objective whose evidence block does not exist", () => {
    const c = clone();
    c.objectives[0].evidenceBlockIds = ["nope"];
    expect(messages(validateReferential(c))).toMatch(/evidence block/i);
  });
  it("flags a graded single-choice whose correctOptionId is not an option", () => {
    const c = clone();
    const sc = c.parts[0].slices[0].blocks.find((b: any) => b.type === "singleChoice");
    sc.assessment.correctOptionId = "missing";
    expect(messages(validateReferential(c))).toMatch(/correctOptionId/i);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter @mind-imprint/course-contract test referential`
Expected: FAIL.

- [ ] **Step 3: Implement `validate/types.ts` and `validate/referential.ts`**

`types.ts`:

```ts
export type ValidationLayer = "structural" | "referential" | "workflow";
export interface ValidationIssue {
  path: string;
  message: string;
  layer: ValidationLayer;
}
```

`referential.ts` — walk the course, collect issues. Cover, at minimum: duplicate ids in each namespace (objective/part/slice/block/narration; step ids are per-slice); every slice block assigned to exactly one slot; every slot blockId references a real block in that slice; every objective `evidenceBlockIds` references a real block; every part/slice `objectiveIds` references a real objective; graded single-choice `correctOptionId` ∈ options; graded fill-blank not paired with `submit-any` (spec §9.6: graded is compatible only with submit-correct / submit-correct-or-exhausted); reflection fill-blank uses `submit-any`; `video` block with required completion `video-ended-and-interactions-completed` must declare `interaction`. Each issue carries a `path` like `parts[0].slices[1].blocks[2]`.

```ts
import type { CourseDefinition } from "../course";
import type { ValidationIssue } from "./types";

export function validateReferential(course: CourseDefinition): ValidationIssue[] {
  const issues: ValidationIssue[] = [];
  const add = (path: string, message: string) => issues.push({ path, message, layer: "referential" });

  // --- global id uniqueness ---
  const seen = <T>(items: T[], key: (t: T) => string, kind: string, path: (i: number) => string) => {
    const counts = new Map<string, number>();
    items.forEach((it) => counts.set(key(it), (counts.get(key(it)) ?? 0) + 1));
    items.forEach((it, i) => { if ((counts.get(key(it)) ?? 0) > 1) add(path(i), `duplicate ${kind} id '${key(it)}'`); });
  };

  const allBlocks: { block: CourseDefinition["parts"][number]["slices"][number]["blocks"][number]; path: string }[] = [];
  const objectiveIds = new Set(course.objectives.map((o) => o.id));

  seen(course.objectives, (o) => o.id, "objective", (i) => `objectives[${i}]`);
  seen(course.parts, (p) => p.id, "part", (i) => `parts[${i}]`);

  const sliceEntries: { slice: any; path: string }[] = [];
  course.parts.forEach((part, pi) => {
    part.objectiveIds.forEach((oid, oi) => { if (!objectiveIds.has(oid)) add(`parts[${pi}].objectiveIds[${oi}]`, `unknown objective '${oid}'`); });
    part.slices.forEach((slice, si) => sliceEntries.push({ slice, path: `parts[${pi}].slices[${si}]` }));
  });
  seen(sliceEntries.map((e) => e.slice), (s) => s.id, "slice", (i) => sliceEntries[i]!.path);

  // per-slice checks
  sliceEntries.forEach(({ slice, path }) => {
    const blockIds = new Set<string>();
    slice.blocks.forEach((b: any, bi: number) => {
      if (blockIds.has(b.id)) add(`${path}.blocks[${bi}]`, `duplicate block id '${b.id}' within slice`);
      blockIds.add(b.id);
      allBlocks.push({ block: b, path: `${path}.blocks[${bi}]` });
    });
    slice.objectiveIds.forEach((oid: string, oi: number) => { if (!objectiveIds.has(oid)) add(`${path}.objectiveIds[${oi}]`, `unknown objective '${oid}'`); });

    // narration id uniqueness within slice
    seen(slice.narrations, (n: any) => n.id, "narration", (i) => `${path}.narrations[${i}]`);

    // layout slot ↔ block assignment: each block in exactly one slot, each slot id real
    const assignment = new Map<string, number>();
    slice.layout.slots.forEach((slot: any, sli: number) => {
      slot.blockIds.forEach((bid: string) => {
        if (!blockIds.has(bid)) add(`${path}.layout.slots[${sli}]`, `slot references unknown block '${bid}'`);
        assignment.set(bid, (assignment.get(bid) ?? 0) + 1);
      });
    });
    slice.blocks.forEach((b: any, bi: number) => {
      const n = assignment.get(b.id) ?? 0;
      if (n === 0) add(`${path}.blocks[${bi}]`, `block '${b.id}' is not assigned to a slot`);
      if (n > 1) add(`${path}.blocks[${bi}]`, `block '${b.id}' is assigned to more than one slot`);
    });

    // assessment cross-field rules
    slice.blocks.forEach((b: any, bi: number) => {
      if (b.type === "singleChoice" && b.assessment.mode === "graded") {
        const opts = new Set(b.options.map((o: any) => o.id));
        if (!opts.has(b.assessment.correctOptionId)) add(`${path}.blocks[${bi}]`, `correctOptionId '${b.assessment.correctOptionId}' is not an option`);
      }
      if (b.type === "fillBlank" && b.assessment.mode === "graded" && b.completion.rule === "submit-any") {
        add(`${path}.blocks[${bi}]`, `graded fill-blank is incompatible with completion 'submit-any'`);
      }
      if (b.type === "video" && b.completion?.rule === "video-ended-and-interactions-completed" && !b.interaction) {
        add(`${path}.blocks[${bi}]`, `video completion requires an interaction reference`);
      }
    });
  });

  // objective evidence references (global block set)
  const globalBlockIds = new Set(allBlocks.map((e) => e.block.id));
  course.objectives.forEach((o, oi) => {
    o.evidenceBlockIds.forEach((bid, ei) => { if (!globalBlockIds.has(bid)) add(`objectives[${oi}].evidenceBlockIds[${ei}]`, `evidence block '${bid}' does not exist`); });
  });

  return issues;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter @mind-imprint/course-contract test referential`
Expected: PASS. (Fixtures come from Task 10; if executing in order, write a minimal inline fixture first and swap to `./fixtures` when Task 10 lands. The task reviewer will confirm the final wiring.)

- [ ] **Step 5: Export and commit**

Export `validateReferential` and `ValidationIssue` from `src/index.ts`.

```bash
git add packages/course-contract/src/validate/types.ts packages/course-contract/src/validate/referential.ts packages/course-contract/test/referential.test.ts packages/course-contract/src/index.ts
git commit -m "feat(course-contract): referential validation"
```

---

### Task 9: Workflow graph validation

**Files:**
- Create: `packages/course-contract/src/validate/workflow.ts`
- Test: `packages/course-contract/test/workflowValidate.test.ts`

**Spec:** §12.6 (the full validation list) + §12.3 action rules + §12.5 (ambiguous overlapping transitions rejected). Operates per slice on a structurally-valid workflow.

**Rules to enforce (§12.6):**
- `initialStepId` exists; step ids unique within the slice.
- every transition `to` targets an existing step.
- every action target (block, narration, timer created by `startTimer`, focus TargetRef block/item) exists in the slice. `cancelTimer` references a timer started somewhere on a path (structural check: timer id appears in some `startTimer`).
- every step reachable from `initialStepId` (BFS over transitions **and** any step targeted by a terminal `navigate`? no — only transitions form edges; a step reached solely by falling through is not a thing here).
- every reachable branch can reach a step that runs `completeSlice`/`navigate` (terminal). Detect: from each reachable step, can we reach a terminal step?
- only terminal steps use `completeSlice` and `navigate`; a step with `navigate` must have no outgoing transitions (spec §12: final step navigates; example's `next` step has empty transitions).
- no transition `to` crosses into another slice — structurally impossible here since ids are local, but assert every `to` is within this slice's step set (already covered by target existence).
- no unsupported Action/Event combination (e.g. `playBlock` only targets a block type that supports playback — video; `pauseNarration` targets a narration). Enforce the subset the spec calls out: `playBlock`/`pauseBlock`/`resetBlock` target a `video` block; narration actions target a declared narration id.
- ambiguous transitions: two transitions on the same step whose matchers can both match the same event (same `type` and overlapping `sourceId`, i.e. equal or one omitted) → reject (§12.5).
- bounded cycles: a cycle in the transition graph is allowed only if every step on the cycle is "gated" by an assessment attempt or an explicit `student.continue` (spec §12.6: "simple cycles require a bounded assessment attempt or an explicit learner action"). Implement: detect cycles; for each cycle, require at least one step on it whose incoming transition matcher type ∈ {`answer.incorrect`, `answer.attemptsExhausted`, `student.continue`, `timer.elapsed`}.

**Interfaces:**
- Consumes: `SliceDefinition` (needs blocks + narrations to check action targets).
- Produces: `validateSliceWorkflow(slice: SliceDefinition, path: string): ValidationIssue[]`.

- [ ] **Step 1: Write the failing tests**

```ts
// test/workflowValidate.test.ts
import { describe, it, expect } from "vitest";
import { validateSliceWorkflow } from "../src/validate/workflow";
import { validCourse } from "./fixtures";

const golden = () => structuredClone(validCourse.parts[0].slices[0]);
const msgs = (xs: { message: string }[]) => xs.map((x) => x.message).join(" | ");

describe("validateSliceWorkflow", () => {
  it("passes on the golden slice", () => {
    expect(validateSliceWorkflow(validCourse.parts[0].slices[0], "s")).toEqual([]);
  });
  it("flags a transition to a missing step", () => {
    const s = golden();
    s.workflow.steps[0].transitions.push({ on: { type: "student.continue" }, to: "nowhere" });
    expect(msgs(validateSliceWorkflow(s, "s"))).toMatch(/unknown step 'nowhere'/i);
  });
  it("flags an unreachable step", () => {
    const s = golden();
    s.workflow.steps.push({ id: "orphan", enterActions: [], transitions: [] });
    expect(msgs(validateSliceWorkflow(s, "s"))).toMatch(/unreachable/i);
  });
  it("flags a playNarration referencing an undeclared narration", () => {
    const s = golden();
    s.workflow.steps[0].enterActions.push({ type: "playNarration", narrationId: "ghost" });
    expect(msgs(validateSliceWorkflow(s, "s"))).toMatch(/narration 'ghost'/i);
  });
  it("flags ambiguous overlapping transitions", () => {
    const s = golden();
    const step = s.workflow.steps[0];
    step.transitions = [
      { on: { type: "student.continue" }, to: step.id === s.workflow.initialStepId ? s.workflow.steps[1].id : s.workflow.initialStepId },
      { on: { type: "student.continue" }, to: s.workflow.steps[1].id },
    ];
    expect(msgs(validateSliceWorkflow(s, "s"))).toMatch(/ambiguous/i);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter @mind-imprint/course-contract test workflowValidate`
Expected: FAIL.

- [ ] **Step 3: Implement `validate/workflow.ts`**

Build the step map, a `startTimer` timer-id set, narration-id set, block-id + video-block-id sets. Then run each rule, pushing `{ layer: "workflow" }` issues. Reachability = BFS from `initialStepId` over `transitions[].to`. Terminal = a step whose `enterActions` include `completeSlice` or `navigate`. Terminal reachability = reverse-reachability from terminal steps must cover every reachable step. Ambiguity = for each step, compare transition pairs: same `type` and (`sourceId` equal OR either omitted) ⇒ ambiguous. Cycle-gating = Tarjan/DFS to find cycles; for each non-trivial SCC, require an incoming matcher type in the gated set. Keep it a single exported function; helper functions private to the file.

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter @mind-imprint/course-contract test workflowValidate`
Expected: PASS.

- [ ] **Step 5: Export and commit**

```bash
git add packages/course-contract/src/validate/workflow.ts packages/course-contract/test/workflowValidate.test.ts packages/course-contract/src/index.ts
git commit -m "feat(course-contract): workflow graph validation"
```

---

### Task 10: Golden fixtures

**Files:**
- Create: `packages/course-contract/test/fixtures.ts`
- Create: `packages/course-contract/test/fixtures/coverage-course.json` (a course exercising every block type + all four layouts across several slices)
- Test: `packages/course-contract/test/fixtures.test.ts`

**Interfaces:**
- Produces: `validCourse: CourseDefinition` (parsed from the §15 golden example **extended** so it has ≥2 slices and at least one of every block type + every layout preset across its slices — Tasks 8/9 tests import `validCourse`), and a set of typed invalid documents for negative tests.

- [ ] **Step 1: Write the fixture test**

```ts
// test/fixtures.test.ts
import { describe, it, expect } from "vitest";
import { CourseDefinitionDocument } from "../src/course";
import { validateReferential } from "../src/validate/referential";
import { validateSliceWorkflow } from "../src/validate/workflow";
import { validCourse } from "./fixtures";

describe("golden coverage course", () => {
  it("is structurally valid", () => {
    expect(CourseDefinitionDocument.safeParse({ schemaVersion: "2.0", course: validCourse }).success).toBe(true);
  });
  it("is referentially clean", () => {
    expect(validateReferential(validCourse)).toEqual([]);
  });
  it("has clean workflows on every slice", () => {
    for (const part of validCourse.parts) for (const slice of part.slices) {
      expect(validateSliceWorkflow(slice, slice.id)).toEqual([]);
    }
  });
  it("exercises every block type", () => {
    const types = new Set(validCourse.parts.flatMap((p) => p.slices.flatMap((s) => s.blocks.map((b) => b.type))));
    for (const t of ["text","images","pdf","video","interactiveHtml","fillBlank","singleChoice"]) expect(types.has(t as any)).toBe(true);
  });
  it("exercises every layout preset", () => {
    const presets = new Set(validCourse.parts.flatMap((p) => p.slices.map((s) => s.layout.preset)));
    for (const p of ["full","split-horizontal","split-vertical","grid"]) expect(presets.has(p as any)).toBe(true);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter @mind-imprint/course-contract test fixtures`
Expected: FAIL (no fixture yet).

- [ ] **Step 3: Author the fixture**

Build `coverage-course.json` starting from the §15 example (which already covers `video` + `singleChoice` + `split-horizontal`) and add slices so all seven block types and all four layout presets appear, each slice with a valid workflow (terminal `navigate`). `fixtures.ts` imports the JSON, parses it through `CourseDefinition`, and re-exports the parsed object as `validCourse` (so it is typed and guaranteed structurally valid at test time).

```ts
// test/fixtures.ts
import coverage from "./fixtures/coverage-course.json";
import { CourseDefinition } from "../src/course";
export const validCourse = CourseDefinition.parse((coverage as any).course);
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter @mind-imprint/course-contract test fixtures`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/course-contract/test/fixtures.ts packages/course-contract/test/fixtures/coverage-course.json packages/course-contract/test/fixtures.test.ts
git commit -m "test(course-contract): golden coverage fixture"
```

---

### Task 11: Unified `validateCourseDefinition` entry point

**Files:**
- Create: `packages/course-contract/src/validate/index.ts`
- Test: `packages/course-contract/test/validate.test.ts`

**Interfaces:**
- Consumes: Zod `CourseDefinitionDocument`, `validateReferential`, `validateSliceWorkflow`.
- Produces: `validateCourseDefinition(input: unknown): { ok: true; course: CourseDefinition } | { ok: false; issues: ValidationIssue[] }`. Runs structural first; if structural fails, returns those issues and **stops** (referential/workflow assume a well-formed shape). Otherwise runs referential + every slice's workflow validation and returns the combined list (ok only if empty).

- [ ] **Step 1: Write the failing test**

```ts
// test/validate.test.ts
import { describe, it, expect } from "vitest";
import { validateCourseDefinition } from "../src/validate";
import { validCourse } from "./fixtures";

describe("validateCourseDefinition", () => {
  it("accepts the golden document", () => {
    const r = validateCourseDefinition({ schemaVersion: "2.0", course: validCourse });
    expect(r.ok).toBe(true);
  });
  it("returns structural issues and stops when the shape is wrong", () => {
    const r = validateCourseDefinition({ schemaVersion: "2.0", course: { id: "x" } });
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.issues.every((i) => i.layer === "structural")).toBe(true);
  });
  it("returns referential issues on a structurally-valid but broken course", () => {
    const broken = structuredClone(validCourse);
    broken.objectives[0].evidenceBlockIds = ["ghost"];
    const r = validateCourseDefinition({ schemaVersion: "2.0", course: broken });
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.issues.some((i) => i.layer === "referential")).toBe(true);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter @mind-imprint/course-contract test validate`
Expected: FAIL.

- [ ] **Step 3: Implement `validate/index.ts`**

```ts
import { CourseDefinitionDocument, type CourseDefinition } from "../course";
import { validateReferential } from "./referential";
import { validateSliceWorkflow } from "./workflow";
import type { ValidationIssue } from "./types";

export type ValidateResult =
  | { ok: true; course: CourseDefinition }
  | { ok: false; issues: ValidationIssue[] };

export function validateCourseDefinition(input: unknown): ValidateResult {
  const parsed = CourseDefinitionDocument.safeParse(input);
  if (!parsed.success) {
    return {
      ok: false,
      issues: parsed.error.issues.map((i) => ({ path: i.path.join("."), message: i.message, layer: "structural" as const })),
    };
  }
  const course = parsed.data.course;
  const issues: ValidationIssue[] = [...validateReferential(course)];
  course.parts.forEach((part, pi) =>
    part.slices.forEach((slice, si) => issues.push(...validateSliceWorkflow(slice, `parts[${pi}].slices[${si}]`))),
  );
  return issues.length === 0 ? { ok: true, course } : { ok: false, issues };
}

export * from "./types";
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter @mind-imprint/course-contract test validate`
Expected: PASS.

- [ ] **Step 5: Finalize public API, typecheck, full test, commit**

Ensure `src/index.ts` re-exports everything consumers need: all schemas + inferred types + `validateCourseDefinition` + `ValidationIssue`. Run the whole package suite and typecheck.

Run: `pnpm --filter @mind-imprint/course-contract test` and `pnpm --filter @mind-imprint/course-contract typecheck`
Expected: both PASS.

```bash
git add packages/course-contract/src/validate/index.ts packages/course-contract/test/validate.test.ts packages/course-contract/src/index.ts
git commit -m "feat(course-contract): unified validateCourseDefinition entry point"
```

---

## Self-Review Notes

- **Spec coverage:** §4 (safe paths → Task 2), §5 (top-level + unique-id/evidence rules → Tasks 6, 8), §6 (Opening/Closing + RuntimeSceneResult → Tasks 6, 7), §7 (Part/Slice → Task 6), §8–§9 (blocks → Task 3), §10 (layout → Task 4), §11 (narration → Task 4), §12 (workflow schema + graph validation → Tasks 5, 9), §13 (navigation → Task 4), §16 (session → Task 7), §19.1–§19.3 (structural/referential/workflow validation → Tasks 8, 9, 11). §14 (VideoInteraction document) and §19.4 (asset-file validation) are **deferred**: the block's `interaction.source` string is validated here, but the interaction document schema and on-disk asset checks belong to Slice 5 (media) — noted so the reviewer does not flag them as gaps.
- **Deferred by design:** deep BCP-47 language validation (§5) — a `.min(2)` placeholder is used; `estimatedMinutes` vs sum-of-slice-estimates reconciliation (§5) is a soft authoring check, not a load-time hard gate, and is left to the authoring side.
- **Type consistency:** every `*Id` flows from `primitives.ts`; assessment sub-schemas (`ChoiceOption`, `*Assessment`, `*CompletionRule`) are declared once in `blocks.ts` and exported for Slice 5's video cues.
