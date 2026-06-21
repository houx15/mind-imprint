import type { CSSProperties } from "react";
import type { ProcessNode, ProcessNodeType } from "./processTree";

// ─── Type → label + accent table ─────────────────────────────────────────────
// Lifted verbatim from docs/design/思维印记_工作区.dc.html TM table (line ~1017)

interface TypeDef {
  tag: string;
  color: string;
  bg: string;
  fill: boolean;
  dashed?: boolean;
}

const TYPE_MAP: Record<ProcessNodeType, TypeDef> = {
  task_root:     { tag: "任务根",    color: "#2A3B7A", bg: "#EDEFF9", fill: true },
  sub_question:  { tag: "子问题",    color: "#2A3B7A", bg: "#EDEFF9", fill: false },
  card_use:      { tag: "工具卡使用", color: "#5B6BB5", bg: "#EBEDFA", fill: true },
  attempt:       { tag: "尝试草稿",  color: "#9AA1B0", bg: "#F1F2F5", fill: false, dashed: true },
  key_knowledge: { tag: "关键知识",  color: "#D9A23D", bg: "#FBF1DC", fill: true },
  concession:    { tag: "让步修订",  color: "#4C9A82", bg: "#E7F3EE", fill: true },
  reflection:    { tag: "反身收口",  color: "#7C6BB5", bg: "#F0ECF8", fill: true },
};

// ─── nodeView ─────────────────────────────────────────────────────────────────

export interface NodeViewResult {
  tag: string;
  tagStyle: CSSProperties;
  markerStyle: CSSProperties;
  rowStyle: CSSProperties;
  indent: boolean;
}

/**
 * Maps a ProcessNode to the visual properties needed to render a tree row.
 * Style values lifted verbatim from the binding HTML (思维印记_工作区.dc.html).
 *
 * rowStyle:    { display:'flex', gap:'11px', padding:'7px 0', marginLeft:(depth*18)+'px' }
 * markerStyle: { width:'12px', height:'12px', borderRadius:'50%', border:'2px [dashed|solid] color', background: fill?color:'#fff' }
 * tagStyle:    { display:'inline-block', fontSize:'11px', fontWeight:700, padding:'2px 9px', borderRadius:'6px', background:bg, color:color }
 */
export function nodeView(node: ProcessNode): NodeViewResult {
  const t = TYPE_MAP[node.type];
  const indent = node.parent_id !== null;
  const depth = indent ? 1 : 0;

  const rowStyle: CSSProperties = {
    display: "flex",
    gap: "11px",
    padding: "7px 0",
    marginLeft: `${depth * 18}px`,
  };

  const markerStyle: CSSProperties = {
    width: "12px",
    height: "12px",
    borderRadius: "50%",
    border: `2px ${t.dashed ? "dashed" : "solid"} ${t.color}`,
    background: t.fill ? t.color : "#fff",
  };

  const tagStyle: CSSProperties = {
    display: "inline-block",
    fontSize: "11px",
    fontWeight: 700,
    padding: "2px 9px",
    borderRadius: "6px",
    background: t.bg,
    color: t.color,
  };

  return { tag: t.tag, tagStyle, markerStyle, rowStyle, indent };
}
