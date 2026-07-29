// Real export formats for the four workspace rooms (slice 6). Each fn is pure —
// it takes already-loaded room data + project meta, builds the school's real
// deliverable (.xlsx / .docx), triggers a download, and returns the Blob so it
// can be inspected/tested. The heavy libs (`xlsx`, `docx`) are **dynamically
// imported** inside each fn so they land in lazy chunks, never the main bundle.
import type { LogEntry, PlanItem, Proposal, Reference } from "@mind-imprint/contracts";

const XLSX_MIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet";
const DOCX_MIME = "application/vnd.openxmlformats-officedocument.wordprocessingml.document";

// Local enum→label maps (kept here so the export module owns no UI imports).
const TAG_LABEL: Record<PlanItem["tag"], string> = { read: "读", write: "写", review: "省" };
const COLUMN_LABEL: Record<PlanItem["column"], string> = { todo: "待办", doing: "进行中", done: "完成" };
// The annotated-bib "Should I use this resource?" verdict (English form-facing).
const USE_LABEL: Record<"use" | "maybe" | "drop", string> = { use: "YES", maybe: "MAYBE", drop: "NO" };

// Download a Blob under a filename. Guarded so it no-ops in environments
// without object-URL support (jsdom/tests), never throwing.
export function saveBlob(name: string, blob: Blob): void {
  if (typeof document === "undefined" || typeof URL === "undefined" || typeof URL.createObjectURL !== "function") return;
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  URL.revokeObjectURL(url);
}

// The cited parts of a source, as recorded in the Reading Room, flattened into
// the annotated-bib "Relevance" cell.
function relevanceCell(ref: Reference): string {
  const parts = ref.notes.map((n) => `「${n.quote}」→ ${n.finding}`);
  return parts.length > 0 ? parts.join("；") : "—";
}

/* ---------- 1 · Annotated Bibliography → .xlsx (ReadingBlock) ---------- */

export async function exportAnnotatedBib(refs: Reference[], project: { title: string }): Promise<Blob> {
  const XLSX = await import("xlsx");
  const rows = refs.filter((r) => !r.pending);
  const header = [
    "Resource",
    "Classification",
    "Authors",
    "Author Credentials",
    "Journal name / Website",
    "Relevance of resource (cited parts)",
    "Evaluation of this resource",
    "Should I use this resource?",
  ];
  const body = rows.map((r) => [
    r.title,
    r.classification || "",
    r.author || "",
    r.credentials || "",
    r.url || "",
    relevanceCell(r),
    r.evaluation || "",
    r.decision ? USE_LABEL[r.decision] : "待定",
  ]);
  const aoa: (string | number)[][] = [
    [`Research Title: ${project.title || "未命名项目"}`],
    [`How many articles/papers: ${rows.length}`],
    [],
    header,
    ...body,
  ];
  const ws = XLSX.utils.aoa_to_sheet(aoa);
  ws["!cols"] = [{ wch: 30 }, { wch: 16 }, { wch: 18 }, { wch: 24 }, { wch: 30 }, { wch: 40 }, { wch: 36 }, { wch: 22 }];
  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, ws, "Annotated Bibliography");
  const buf = XLSX.write(wb, { bookType: "xlsx", type: "array" }) as ArrayBuffer;
  const blob = new Blob([buf], { type: XLSX_MIME });
  saveBlob("注释书目_AnnotatedBibliography.xlsx", blob);
  return blob;
}

/* ---------- 2 · Timescale / Gantt → .xlsx (PlanBlock working phase) ---------- */

export async function exportTimescale(
  items: PlanItem[],
  project: { title: string },
  timelineDays: number,
): Promise<Blob> {
  const XLSX = await import("xlsx");
  const dayCols = Array.from({ length: timelineDays }, (_, i) => i + 1);
  const header = ["WBS NUMBER", "TASK TITLE", "类型", "状态", ...dayCols.map((d) => String(d))];

  // Distinct stages in first-appearance order (no dependency on mock STAGES).
  const stages: string[] = [];
  for (const it of items) if (!stages.includes(it.stage)) stages.push(it.stage);

  const aoa: (string | number)[][] = [
    ["TIMESCALE"],
    [`Project: ${project.title || "未命名项目"}`],
    ["Name / Date:"],
    [],
    header,
  ];

  stages.forEach((stage, si) => {
    aoa.push([`── ${stage} ──`]);
    const stageItems = items.filter((it) => it.stage === stage).slice().sort((a, b) => a.start - b.start);
    stageItems.forEach((it, ii) => {
      const marks = dayCols.map((d) => (d >= it.start + 1 && d <= it.start + it.days ? "█" : ""));
      aoa.push([`${si + 1}.${ii + 1}`, it.title, TAG_LABEL[it.tag], COLUMN_LABEL[it.column], ...marks]);
    });
  });

  const ws = XLSX.utils.aoa_to_sheet(aoa);
  ws["!cols"] = [{ wch: 12 }, { wch: 34 }, { wch: 6 }, { wch: 8 }, ...dayCols.map(() => ({ wch: 3 }))];
  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, ws, "Timescale");
  const buf = XLSX.write(wb, { bookType: "xlsx", type: "array" }) as ArrayBuffer;
  const blob = new Blob([buf], { type: XLSX_MIME });
  saveBlob("项目计划_Timescale.xlsx", blob);
  return blob;
}

/* ---------- 3 · Proposal → .docx (PlanBlock ProposalWriter) ---------- */

export async function exportProposalDocx(
  proposal: Proposal,
  project: { title: string; qualification?: string },
): Promise<Blob> {
  const docx = await import("docx");
  const { Document, Packer, Paragraph, TextRun, HeadingLevel } = docx;

  // The student's free text → paragraphs (blank-line-tolerant); a placeholder
  // when a section is empty.
  const textParagraphs = (raw: string): InstanceType<typeof Paragraph>[] => {
    const value = (raw || "").trim();
    if (!value) return [new Paragraph({ children: [new TextRun({ text: "（未填写）", italics: true })] })];
    return value.split(/\n+/).map((line) => new Paragraph({ children: [new TextRun(line)] }));
  };

  const sections: { n: string; title: string; text: string }[] = [
    { n: "§1", title: "题目、目标与职责", text: proposal.objective },
    { n: "§2", title: "选题理由", text: proposal.reason },
    { n: "§3", title: "活动与时间安排", text: proposal.activities },
    { n: "§4", title: "资源", text: proposal.resources },
  ];

  const children: InstanceType<typeof Paragraph>[] = [
    new Paragraph({ heading: HeadingLevel.TITLE, children: [new TextRun(project.title || "未命名项目")] }),
  ];
  if (project.qualification) {
    children.push(new Paragraph({ children: [new TextRun({ text: project.qualification, italics: true })] }));
  }
  for (const s of sections) {
    children.push(new Paragraph({ heading: HeadingLevel.HEADING_1, children: [new TextRun(`${s.n} ${s.title}`)] }));
    children.push(...textParagraphs(s.text));
  }

  const doc = new Document({ sections: [{ children }] });
  const blob = await Packer.toBlob(doc);
  // Packer.toBlob may not stamp the OOXML mime; normalise it for callers/tests.
  const typed = blob.type === DOCX_MIME ? blob : new Blob([await blob.arrayBuffer()], { type: DOCX_MIME });
  saveBlob("开题报告_Proposal.docx", typed);
  return typed;
}

/* ---------- 3b · Draft body → .docx (WritingBlock, WB) ---------- */

// The student's written body → .docx, so 完成→带走 is one motion. Light markdown:
// lines starting with #/##/### become headings, blank lines split paragraphs.
// Pure student text — 铁律: we export what the student wrote, we don't author or
// submit it.
export async function exportDraftDocx(content: string, project: { title: string; qualification?: string }): Promise<Blob> {
  const docx = await import("docx");
  const { Document, Packer, Paragraph, TextRun, HeadingLevel } = docx;

  const children: InstanceType<typeof Paragraph>[] = [
    new Paragraph({ heading: HeadingLevel.TITLE, children: [new TextRun(project.title || "未命名项目")] }),
  ];
  if (project.qualification) {
    children.push(new Paragraph({ children: [new TextRun({ text: project.qualification, italics: true })] }));
  }
  const body = (content || "").trim();
  if (!body) {
    children.push(new Paragraph({ children: [new TextRun({ text: "（正文还没有写）", italics: true })] }));
  } else {
    for (const block of body.split(/\n{2,}/)) {
      for (const line of block.split(/\n/)) {
        const h = /^(#{1,3})\s+(.*)$/.exec(line);
        if (h) {
          const hashes = h[1] ?? "#";
          const heading = h[2] ?? "";
          const level = hashes.length === 1 ? HeadingLevel.HEADING_1 : hashes.length === 2 ? HeadingLevel.HEADING_2 : HeadingLevel.HEADING_3;
          children.push(new Paragraph({ heading: level, children: [new TextRun(heading)] }));
        } else {
          children.push(new Paragraph({ children: [new TextRun(line)] }));
        }
      }
    }
  }

  const doc = new Document({ sections: [{ children }] });
  const blob = await Packer.toBlob(doc);
  const typed = blob.type === DOCX_MIME ? blob : new Blob([await blob.arrayBuffer()], { type: DOCX_MIME });
  saveBlob("成品正文_Draft.docx", typed);
  return typed;
}

/* ---------- 4 · Activity Log → .docx (PlanBlock log view) ---------- */

export async function exportActivityLog(entries: LogEntry[], project: { title: string }): Promise<Blob> {
  const docx = await import("docx");
  const { Document, Packer, Paragraph, TextRun, HeadingLevel, Table, TableRow, TableCell, WidthType } = docx;

  const cell = (text: string, opts?: { bold?: boolean; width?: number }) =>
    new TableCell({
      width: opts?.width ? { size: opts.width, type: WidthType.PERCENTAGE } : undefined,
      children: [new Paragraph({ children: [new TextRun({ text, bold: opts?.bold })] })],
    });

  const headerRow = new TableRow({
    children: [cell("Date", { bold: true, width: 15 }), cell("记录", { bold: true, width: 70 }), cell("来源", { bold: true, width: 15 })],
  });
  const bodyRows = entries.map(
    (e) =>
      new TableRow({
        children: [cell(e.date), cell(e.text), cell(e.source === "auto" ? "自动" : "我记的")],
      }),
  );

  const table = new Table({
    width: { size: 100, type: WidthType.PERCENTAGE },
    rows: [headerRow, ...bodyRows],
  });

  const doc = new Document({
    sections: [
      {
        children: [
          new Paragraph({
            heading: HeadingLevel.HEADING_1,
            children: [new TextRun(`活动日志 / Production Log — ${project.title || "未命名项目"}`)],
          }),
          table,
        ],
      },
    ],
  });
  const blob = await Packer.toBlob(doc);
  const typed = blob.type === DOCX_MIME ? blob : new Blob([await blob.arrayBuffer()], { type: DOCX_MIME });
  saveBlob("活动日志_ProductionLog.docx", typed);
  return typed;
}
