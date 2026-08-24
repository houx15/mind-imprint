import type { DigCandidate } from "@mind-imprint/contracts";

export type TourPlacement = "top" | "bottom" | "left" | "right" | "center";
/** Kinds of static real-UI mocks a step can pair its 印记 line with (§Task 2).
 *  Extend with a new string as more mocks are built — the registry lives in
 *  `tour/mocks/index.tsx`. */
export type DemoMockKind = "question-card" | "write-mode-choice";
export type NavTabKey = "home" | "projects" | "courses" | "me";
export type CoursesSub = "courses" | "history" | "gallery";
/** The five studio rooms (mirrors `BlockKey` in workspace/blocks/mockData —
 *  kept as a separate tour-facing type so `tour/` doesn't import `workspace/`). */
export type StudioRoom = "forming" | "plan" | "reading" | "writing" | "reflection";
/** The shared, world-readable demo project the projects tour opens into the
 *  studio (read-only — the backend 403s all writes for it). */
export const DEMO_PROJECT_ID = "00000000-0000-0000-0000-000000000200";
/** The demo project's seeded reading material (Chen et al. 2019, Nature
 *  Sustainability — the CRAAP source-check chain's target) and the reference
 *  row it hangs off, used by `openDemoReadingRoom` (P6, Task 5) to open the
 *  real immersive Reading Room in a read-only replay. IDs are pinned by
 *  migration `0082_seed_demo_project_finished.sql`. */
export const DEMO_READING_MATERIAL_ID = "00000000-0000-0000-0000-000000000271";
export const DEMO_READING_REFERENCE_ID = "00000000-0000-0000-0000-000000000260";
/** The freeform 我的笔记 note seeded on `DEMO_READING_REFERENCE_ID` by migration
 *  `0091_demo_reading_notes.sql` (`reference.reading_note` for reference
 *  …0260). `getMaterialSource` (used by `openDemoReadingRoom`) returns a
 *  `MaterialSource`, not the `Reference` row, so it carries no `readingNote`
 *  field — this constant mirrors the 0091 seed text verbatim rather than
 *  adding a second fetch (`getLibrary`) to the tour's read-only open path.
 *  Keep this in sync with 0091 if that migration's text ever changes. */
export const DEMO_READING_NOTE =
  "读到这里先记一笔：论文用 NASA MODIS 2000–2017 的数据说全球绿叶面积净增约 5%，中国和印度合计贡献了净增量的三分之一以上——但机制主要是农业集约化（约 32%）和人工造林（约 42%），不是森林自然恢复。这是个关键区分：论文只证明了「变绿」，从没有说这等于「更可持续」，更没有提碳排放。我得把这条数据和 Global Carbon Project 的排放数字放在一起看，才能判断「趋势变好」是不是等于「问题已解决」。";

/** Setters the tour uses to drive the app. Assembled in StudentApp (§Task 9).
 *  P1 only needs the courses-side setters; P3 extends this. */
export interface TourNavContext {
  setTab: (t: NavTabKey) => void;
  openCourse: (slug: string) => void;
  setCoursesSub: (s: CoursesSub) => void;
  /** P3: open the shared demo project into the studio. */
  openDemoProject: () => void;
  /** P3: switch the open project's studio to this room. */
  setStudioRoom: (room: StudioRoom) => void;
  /** P3: open the shared demo project's 过程评估报告 (评估报告 sub, focused on
   *  the demo). The report is world-readable and fetched by id, so a non-owner
   *  (the tour user) sees it. */
  openDemoReport: () => void;
  /** P5: switch the OPEN project's reading room to a specific inner view —
   *  列表 (Zotero-shaped library table) or 探索图谱 (the rabbit-hole graph). Sets
   *  `pendingReadingView` in WorkspaceContainer, threaded to ReadingBlock's
   *  `forceView` (a ref-guarded one-shot that never fights the student's own
   *  later toggling). Assumes the reading room is already open (drive there
   *  first with `setStudioRoom("reading")`). */
  setReadingView: (view: "list" | "graph") => void;
  /** P6 (Task 5): open the demo's seeded material (Chen et al. 2019) into the
   *  REAL immersive 精读 Reading Room, as a read-only replay — switches the
   *  open project's studio to `reading`, fetches its `MaterialSource` (GET
   *  `/materials/{mid}/source`), and opens `ReadingRoom` with the canned
   *  `demoReadingTranscript` + `demoMode` (composer/finalize/brief/notes all
   *  disabled, no writes fire). On a fetch failure, falls back to
   *  `setReadingView("list")` so the step never dead-ends. */
  openDemoReadingRoom: () => void;
  /** P6 (Task 9): drive the OPEN project's writing room to a specific document
   *  (提案/正文) + tab (大纲/片段/正文) so the tour can land on the PROPOSAL 片段
   *  tab where the 片段引导/写作卡 (`writing-aicard`) lives. Sets
   *  `pendingWritingView` in WorkspaceContainer, which switches the doc via its
   *  `docOverride` and hands the tab to WritingBlock as `forceTab` (a
   *  ref-guarded one-shot). Assumes the writing room is already open (drive
   *  there first with `setStudioRoom("writing")`). */
  setWritingView: (view: TourWritingView) => void;
  /** P7: select a tab on the OPEN writing room's left `ReferencePanel` — used
   *  to explicitly switch to AI批注 (`"anno"`) after the demo now defaults to
   *  阅读笔记 (`"notes"`). Sets `pendingRefPanelTab` in WorkspaceContainer,
   *  threaded to ReferencePanel as a ref-guarded one-shot `forceTab` (applies
   *  once, never fights the student's later tab clicks). Assumes the writing
   *  room is already open (drive there first with `setStudioRoom("writing")`). */
  selectRefPanelTab: (tab: TourRefPanelTab) => void;
  /** P7: force the OPEN project's 管理 (PlanBlock) room to a specific view
   *  (看板/甘特图/活动日志) — used to land on 活动日志. Sets `pendingPlanView` in
   *  WorkspaceContainer, threaded to PlanBlock/WorkingPhase as a ref-guarded
   *  one-shot `forceView` (applies once, never fights the student's later
   *  Segmented clicks). Assumes the 管理 room is already open (drive there
   *  first with `setStudioRoom("plan")`). */
  setPlanView: (view: TourPlanView) => void;
  /** P7: open the 检索卡 teaching modal in the exploration graph's controls
   *  column. Sets `pendingOpenSearchCard` (a bumped nonce — the action carries
   *  no payload) in WorkspaceContainer, threaded through ReadingBlock to
   *  ExplorationView as a ref-guarded one-shot that opens `SearchCardModal`
   *  once. Assumes the reading room is already open on the 探索图谱 view (drive
   *  there first with `setStudioRoom("reading")` + `setReadingView("graph")`). */
  openSearchCard: () => void;
  /** P7 (Task 4b): demo-badge one warren-map root as 已读, client-side only —
   * called when the tour returns from the read-only demo reading room
   * (`openDemoReadingRoom`). The demo project's tour-read reference
   * (`DEMO_READING_REFERENCE_ID`) is seeded a non-'done' status (migration
   * 0088) specifically so its containing roots start un-badged and this call
   * can show a real un-badged → badged transition — the demo project itself
   * is write-blocked (403), so there is no real write to badge it. Sets
   * `pendingMarkNodeRead` in WorkspaceContainer, which ACCUMULATES it into a
   * `demoReadRootIds` set (unlike every other one-shot above, never
   * retracted — a root once marked stays marked for the rest of the
   * session) and merges it into `readByRoot` before handing that to
   * WarrenMap. Assumes the reading room is already open on the 探索图谱 view. */
  markDemoNodeRead: (rootLeadId: string) => void;
  /** P8 (Task 6): demo-simulate ADOPTING a searched candidate into the OPEN
   *  project's exploration graph, client-side only — the demo project is
   *  write-blocked (403), so `ExplorationView`'s real POST /exploration/adopt
   *  is swapped for this call when the project isDemo. Mirrors
   *  `markDemoNodeRead`'s shape (accumulated, never retracted — every call
   *  this session adds one more synthetic node) but the state ends up in
   *  WorkspaceContainer's `demoAdoptedLeads`/`demoAdoptedRefs` instead of a
   *  root-id Set: a synthetic `ExplorationLead` (status "connected",
   *  `parentLeadId` = the node the candidate was dug/adopted from — same
   *  papers-never-roots invariant as the real adopt) + a synthetic
   *  `Reference` built from the `DigCandidate`'s bibliographic fields,
   *  merged into the exploration view before `WarrenMap`/`QuestionMindmap`
   *  and the "文献 x 篇" counts render. Reachable both from a tour step's
   *  `onEnter` (this hook) and directly from ExplorationView's own
   *  adopt/addFromDetail click handlers (threaded a callback down from
   *  WorkspaceContainer, same accumulator either way). */
  markDemoNodeAdopted: (candidate: DigCandidate, parentLeadId: string) => void;
}

/** A one-shot writing-room deep-link target (P6, Task 9): which document and
 *  which tab the guided tour wants the open project's writing room to show. */
export interface TourWritingView {
  doc: "proposal" | "essay";
  tab: "outline" | "snippets" | "draft";
}

/** The left `ReferencePanel`'s tab keys (P7) — mirrors the `tabs` list built
 *  in `ReferencePanel.tsx` (§93). `selectRefPanelTab` targets one of these. */
export type TourRefPanelTab = "notes" | "anno" | "snippets" | "proposal";

/** The 管理 room's (PlanBlock/WorkingPhase) view-toggle keys (P7) — mirrors the
 *  local `PlanView` type in `PlanBlock.tsx`. `setPlanView` targets one of
 *  these. */
export type TourPlanView = "kanban" | "gantt" | "log";

export interface TourStep {
  id: string;
  /** Drive app state before the step renders (navigate, open a course, switch sub-tab). */
  onEnter?: (ctx: TourNavContext) => void | Promise<void>;
  /** CSS selector, convention [data-tour="<id>"]. Absent → centered bubble. */
  anchor?: string;
  /** Cut a hole over the anchor (default true when anchor set). */
  spotlight?: boolean;
  /** Bubble placement relative to the anchor; "center" = modal-like, ignores anchor. */
  placement?: TourPlacement;
  title?: string;
  /** 印记's line, rendered as Markdown. */
  text: string;
  /** "next" = advance on the 下一步 button; "action" = advance when the user does the thing. */
  advance: "next" | "action";
  /** For advance:"action": which element + event advances the step. */
  actionEvent?: { selector: string; type: "click" | "input" };
  /** Pair this step's 印记 line with a static real-UI mock, rendered in a
   *  `@/ui` `Modal` instead of the normal spotlight+popover. The mock IS the
   *  focus — `anchor`/`spotlight` are ignored when this is set. */
  demoModal?: { kind: DemoMockKind; title?: string };
}

export interface TourSegment {
  id: string;
  name: string;
  steps: TourStep[];
}

export type TourJourney = TourSegment[];

export interface TourController {
  running: boolean;
  segment: TourSegment | null;
  step: TourStep | null;
  segmentIndex: number;
  stepIndex: number;
  /** 0-1 progress within the whole active journey. */
  progress: number;
  play: (target: TourSegment | TourJourney) => void;
  next: () => void;
  prev: () => void;
  skipSegment: () => void;
  stop: () => void;
}
