import { useState } from "react";
import { api } from "../api";
import { Drawer, Button, Select, Radio, Textarea, IconButton, Icon, X, ArrowRight } from "../ui";

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

  function reset() {
    setPrompt("");
    setProjectType(PROJECT_TYPES[0] ?? "");
    setWritingLang("en");
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

        {error && <p className="text-mk-small text-mk-danger">{error}</p>}

        <Button onClick={() => void handleCreate()} disabled={!prompt.trim()} loading={creating} iconEnd={!creating && <Icon icon={ArrowRight} size={16} />} className="self-end">
          开始
        </Button>
      </div>
    </Drawer>
  );
}
