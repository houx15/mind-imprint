import { useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import type { Material, Anchor } from "@mind-imprint/contracts";
import { api } from "../api";

type Phase = "loading" | "ready" | "paste";

export function MaterialPane({ taskId, seedUrl, anchors = [] }: { taskId: string; seedUrl: string | null; anchors?: Anchor[] }) {
  const [materials, setMaterials] = useState<Material[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [phase, setPhase] = useState<Phase>("loading");
  const [pasteTitle, setPasteTitle] = useState("");
  const [pasteText, setPasteText] = useState("");
  const started = useRef(false);

  useEffect(() => {
    if (started.current) return;
    started.current = true;
    void (async () => {
      try {
        const existing = await api.listMaterials(taskId);
        if (existing.length > 0) {
          setMaterials(existing);
          setActiveId(existing[0]!.id);
          setPhase("ready");
          return;
        }
        if (!seedUrl) {
          setPhase("paste");
          return;
        }
        try {
          const m = await api.fetchMaterialFromSeed(taskId);
          setMaterials([m]);
          setActiveId(m.id);
          setPhase("ready");
        } catch {
          setPhase("paste");
        }
      } catch {
        setPhase("paste");
      }
    })();
  }, [taskId, seedUrl]);

  async function submitPaste() {
    const text = pasteText.trim();
    if (!text) return;
    const kind: "article" | "draft" = seedUrl ? "article" : "draft";
    const m = await api.createMaterial(taskId, { kind, title: pasteTitle.trim() || "我的材料", text });
    setMaterials((prev) => [...prev, m]);
    setActiveId(m.id);
    setPasteTitle("");
    setPasteText("");
    setPhase("ready");
  }

  const active = materials.find((m) => m.id === activeId) ?? null;

  return (
    <div style={{ flex: 1, minHeight: 0, display: "flex", flexDirection: "column" }}>
      {materials.length > 0 && (
        <div style={{ flex: "none", padding: "12px 16px 10px", borderBottom: "1px solid #F2F3F7", display: "flex", gap: 8, overflowX: "auto" }}>
          {materials.map((m) => {
            const on = m.id === activeId;
            return (
              <button
                key={m.id}
                type="button"
                onClick={() => setActiveId(m.id)}
                style={{
                  flex: "none", padding: "7px 12px", borderRadius: 10, fontSize: 12, fontWeight: 600,
                  cursor: "pointer", whiteSpace: "nowrap", fontFamily: "inherit",
                  background: on ? "#EDEFF9" : "#F7F8FB", color: on ? "#2A3B7A" : "#8A92A3",
                  border: `1px solid ${on ? "#DFE3F4" : "#EEF0F4"}`,
                }}
              >
                {m.title}
              </button>
            );
          })}
          <button
            type="button"
            onClick={() => { setPasteTitle(""); setPasteText(""); setPhase("paste"); }}
            style={{ flex: "none", padding: "7px 11px", borderRadius: 10, fontSize: 12, fontWeight: 600, cursor: "pointer", whiteSpace: "nowrap", fontFamily: "inherit", color: "#9AA1B0", background: "#fff", border: "1px dashed #E1E4ED" }}
          >
            ＋ 加材料
          </button>
        </div>
      )}

      <div style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "14px 16px 24px" }}>
        {phase === "loading" && <div style={{ fontSize: 13, color: "#9AA1B0" }}>正在载入材料…</div>}

        {phase === "paste" && (
          <div>
            <div style={{ fontSize: 13.5, color: "#6B7384", lineHeight: 1.6, marginBottom: 12 }}>
              没能自动读取这个链接。把你正在读的材料贴进来，印记就能在上面陪你圈画。
            </div>
            <input
              value={pasteTitle}
              onChange={(e) => setPasteTitle(e.target.value)}
              placeholder="给材料起个名字（可留空）"
              style={{ width: "100%", border: "1px solid #E1E4ED", borderRadius: 10, padding: "10px 12px", fontSize: 13.5, color: "#1C2333", background: "#fff", outline: "none", marginBottom: 10 }}
            />
            <textarea
              value={pasteText}
              onChange={(e) => setPasteText(e.target.value)}
              rows={10}
              placeholder="把材料贴进来——文章正文，或你自己的草稿。"
              style={{ width: "100%", border: "1px solid #E1E4ED", borderRadius: 10, padding: "11px 13px", fontSize: 14, lineHeight: 1.7, color: "#1C2333", background: "#fff", outline: "none", resize: "vertical" }}
            />
            <button
              type="button"
              onClick={() => void submitPaste()}
              style={{ marginTop: 10, background: "#2A3B7A", color: "#fff", border: "none", padding: "10px 18px", borderRadius: 10, fontSize: 14, fontWeight: 700, cursor: "pointer", fontFamily: "inherit" }}
            >
              加入材料
            </button>
          </div>
        )}

        {phase === "ready" && active && <MaterialBody key={active.id} taskId={taskId} material={active} anchors={anchors} />}
      </div>
    </div>
  );
}

function renderBlock(text: string, quotes: string[]): ReactNode {
  if (quotes.length === 0) return text;
  // Highlight the first occurrence of each distinct quote.
  const marks = Array.from(new Set(quotes.filter(Boolean)));
  type Seg = { text: string; hl: boolean };
  let segs: Seg[] = [{ text, hl: false }];
  for (const q of marks) {
    const next: Seg[] = [];
    for (const s of segs) {
      if (s.hl) { next.push(s); continue; }
      const idx = s.text.indexOf(q);
      if (idx < 0) { next.push(s); continue; }
      if (idx > 0) next.push({ text: s.text.slice(0, idx), hl: false });
      next.push({ text: q, hl: true });
      const rest = s.text.slice(idx + q.length);
      if (rest) next.push({ text: rest, hl: false });
    }
    segs = next;
  }
  return segs.map((s, i) =>
    s.hl ? (
      <mark key={i} style={{ background: "#F0ECF8", color: "#1C2333", borderBottom: "2px solid #7C6BB5", borderRadius: 3, padding: "1px 2px" }}>{s.text}</mark>
    ) : (
      <span key={i}>{s.text}</span>
    ),
  );
}

function MaterialBody({ taskId, material, anchors }: { taskId: string; material: Material; anchors: Anchor[] }) {
  const [scratch, setScratch] = useState(material.scratch);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  function onScratch(v: string) {
    setScratch(v);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      void api.saveScratch(taskId, material.id, v).catch(() => {});
    }, 600);
  }

  return (
    <div>
      <div style={{ background: "#fff", border: "1px solid #ECEEF3", borderRadius: 14, padding: "20px 22px" }}>
        <div style={{ fontSize: 17, fontWeight: 800, color: "#1C2333", lineHeight: 1.5 }}>{material.title}</div>
        <div style={{ marginTop: 14 }}>
          {material.blocks.map((b) => {
            const quotes = anchors.filter((a) => a.block_id === b.id).map((a) => a.quote);
            return (
              <p key={b.id} style={{ fontSize: 15, lineHeight: 2.1, color: "#2B3346", margin: "0 0 14px" }}>
                {renderBlock(b.text, quotes)}
              </p>
            );
          })}
        </div>
      </div>
      <div style={{ marginTop: 14, background: "#FBF7EF", border: "1px solid #F0E6D2", borderRadius: 12, padding: "13px 15px" }}>
        <div style={{ fontSize: 12, fontWeight: 700, color: "#8A6520", marginBottom: 8 }}>随手记</div>
        <textarea
          value={scratch}
          onChange={(e) => onScratch(e.target.value)}
          rows={3}
          placeholder="临时的想法、要去查的东西、一个反例……"
          style={{ width: "100%", border: "none", borderRadius: 8, padding: "9px 11px", fontSize: 13, lineHeight: 1.6, color: "#5C4A22", background: "#fff", outline: "none", resize: "vertical" }}
        />
      </div>
    </div>
  );
}
