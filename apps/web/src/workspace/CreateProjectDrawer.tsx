import { useEffect, useState } from "react";
import { api } from "../api";
import { Drawer, Button, Select, Radio, Textarea, IconButton, Icon, X, ArrowRight, MACARONS, coverGradientStyle, type MacaronName } from "../ui";

/**
 * CreateProjectDrawer — the "新建项目" flow, extracted from Directory's old
 * inline form (Task 6) into a right `Drawer` built from `ui/` form controls.
 * Directory's first grid tile (the dashed ＋ tile) opens this; on a
 * successful create it hands the new id up via `onCreated` (which opens the
 * project) and closes itself.
 */

// #1: the project TYPE is picked from a selector (not typed). The value is a
// display label shown as the project pill; it does not change the board.
const PROJECT_TYPES = ["拓展论文 EE", "TOK 论文", "内部评估 IA", "EPQ", "个人项目", "其他"];

// #4: the essay's target writing language. Students on the international
// track ultimately write in English, so it leads and is the default.
const WRITING_LANGS: { value: "en" | "zh" | "bilingual"; label: string }[] = [
  { value: "en", label: "English" },
  { value: "zh", label: "中文" },
  { value: "bilingual", label: "双语" },
];

// Task 3: the 7 gradient swatch options, alongside the 15 fetched photo
// covers — same macaron palette `coverGradientStyle` deterministically hashes
// project ids to elsewhere (Directory/HomePage cards, Task 4).
const MACARON_NAMES = Object.keys(MACARONS) as MacaronName[];

// Derive a readable project title from the first line of the assignment
// prompt — the refined research question is sharpened later, in forming.
function titleFromPrompt(prompt: string): string {
  const firstLine = prompt.split("\n").map((s) => s.trim()).find((s) => s.length > 0) ?? "";
  return [...firstLine].slice(0, 60).join("");
}

export interface CreateProjectDrawerProps {
  open: boolean;
  onClose: () => void;
  /** A project was created; hands the new id up (the caller opens it). */
  onCreated: (id: string) => void;
}

export function CreateProjectDrawer({ open, onClose, onCreated }: CreateProjectDrawerProps) {
  const [prompt, setPrompt] = useState("");
  const [projectType, setProjectType] = useState(PROJECT_TYPES[0] ?? "");
  const [writingLang, setWritingLang] = useState<"en" | "zh" | "bilingual">("en");
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Task 3: the cover picker. Covers are fetched fresh each time the drawer
  // opens; once they land, a random photo cover is preselected so the
  // default project card never looks unstyled — but the student can still
  // pick any of the 15 photos or 7 gradients before creating.
  const [covers, setCovers] = useState<{ key: string; url: string }[]>([]);
  const [selectedCover, setSelectedCover] = useState("");

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    api.getProjectCovers().then((list) => {
      if (cancelled) return;
      setCovers(list);
      if (list.length > 0) {
        setSelectedCover(list[Math.floor(Math.random() * list.length)]!.key);
      }
    }).catch(() => { /* picker is optional polish; creation still works without it */ });
    return () => { cancelled = true; };
  }, [open]);

  function reset() {
    setPrompt("");
    setProjectType(PROJECT_TYPES[0] ?? "");
    setWritingLang("en");
    setSelectedCover("");
    setError(null);
    setCreating(false);
  }

  function handleClose() {
    if (creating) return;
    reset();
    onClose();
  }

  async function handleCreate() {
    const p = prompt.trim();
    if (!p || creating) return;
    setCreating(true);
    setError(null);
    try {
      // Primary captured field is the assignment prompt (#1); the title is
      // derived from it for the list, and the research question is
      // sharpened later in forming. Type + writing language travel too
      // (#1, #4).
      const { id } = await api.createProject({
        title: titleFromPrompt(p),
        prompt: p,
        projectType,
        writingLanguage: writingLang,
        cover: selectedCover || undefined,
      });
      reset();
      onCreated(id);
    } catch {
      setError("创建失败，请重试");
      setCreating(false);
    }
  }

  return (
    <Drawer open={open} onClose={handleClose} side="right">
      <div className="flex items-center justify-between gap-3 pb-4">
        <div className="flex items-center gap-2 text-mk-accent-700">
          <Icon icon={ArrowRight} size={16} />
          <span className="text-mk-h3 text-mk-ink">新建项目</span>
        </div>
        <IconButton icon={X} label="关闭" size="sm" onClick={handleClose} />
      </div>

      <div className="flex flex-col gap-4">
        <div>
          <label className="mb-1 block text-mk-small text-mk-muted">项目类型</label>
          <Select
            value={projectType}
            onChange={setProjectType}
            options={PROJECT_TYPES.map((t) => ({ value: t, label: t }))}
          />
        </div>

        <div>
          <label className="mb-1 block text-mk-small text-mk-muted">写作语言</label>
          <Radio name="writing-language" value={writingLang} onChange={(v) => setWritingLang(v as "en" | "zh" | "bilingual")} options={WRITING_LANGS} />
        </div>

        <div>
          <label className="mb-1 block text-mk-small text-mk-muted">作业题目 / 提示</label>
          <Textarea
            value={prompt}
            onChange={setPrompt}
            rows={5}
            // Enter+Cmd/Ctrl submits; plain Enter is a newline so long,
            // multi-line prompts paste cleanly. IME-safe.
            onKeyDown={(e) => {
              if (e.key === "Enter" && (e.metaKey || e.ctrlKey) && !e.nativeEvent.isComposing) {
                e.preventDefault();
                void handleCreate();
              }
            }}
            placeholder="贴上你的作业题目或提示，比如「Can we only understand something to the extent that we understand its context? Discuss with reference to two areas of knowledge.」"
          />
          <p className="mt-1 text-mk-small text-mk-muted">研究问题不用现在就想好——进立题房间我陪你一部分一部分磨。</p>
        </div>

        <div>
          <label className="mb-1 block text-mk-body text-mk-muted">封面</label>
          <div className="grid max-h-48 grid-cols-4 gap-2 overflow-y-auto">
            {covers.map((c) => (
              <button
                key={c.key}
                type="button"
                onClick={() => setSelectedCover(c.key)}
                className={
                  selectedCover === c.key
                    ? "h-16 w-full overflow-hidden rounded-mk-sm ring-2 ring-mk-accent"
                    : "h-16 w-full overflow-hidden rounded-mk-sm"
                }
              >
                <img src={c.url} alt="" className="h-full w-full object-cover" />
              </button>
            ))}
            {MACARON_NAMES.map((name) => (
              <button
                key={name}
                type="button"
                onClick={() => setSelectedCover(`grad:${name}`)}
                style={coverGradientStyle(name)}
                className={
                  selectedCover === `grad:${name}`
                    ? "h-16 w-full rounded-mk-sm ring-2 ring-mk-accent"
                    : "h-16 w-full rounded-mk-sm"
                }
              />
            ))}
          </div>
        </div>

        {error && <p className="text-mk-small text-mk-danger">{error}</p>}

        <Button onClick={() => void handleCreate()} disabled={!prompt.trim()} loading={creating} iconEnd={!creating && <Icon icon={ArrowRight} size={16} />} className="self-end">
          开始
        </Button>
      </div>
    </Drawer>
  );
}
