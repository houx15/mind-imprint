import { useState } from "react";
import type { AddMaterialBody } from "../../api/materials";

export type AddSourceFormProps = {
  onSubmit: (body: AddMaterialBody) => Promise<void>;
  error?: string;
};

const TIERS = ["一手数据", "一手论文", "机构报告", "二手 · 需追源", "评论 / 观点"];

type Tab = "link" | "paste";

function tabStyle(active: boolean): React.CSSProperties {
  return {
    padding: "7px 16px",
    borderRadius: 9,
    fontSize: 13,
    fontWeight: 700,
    cursor: "pointer",
    border: "none",
    fontFamily: "inherit",
    background: active ? "var(--mk-surface)" : "transparent",
    color: active ? "var(--mk-accent-700)" : "var(--mk-muted)",
    boxShadow: active ? "0 1px 3px rgba(20,30,60,.10)" : "none",
  };
}

const INPUT_STYLE: React.CSSProperties = {
  width: "100%",
  border: "1px solid var(--mk-input-border)",
  borderRadius: 10,
  fontSize: 13.5,
  padding: "8px 11px",
  fontFamily: "inherit",
  color: "var(--mk-ink)",
  boxSizing: "border-box",
};

function PlusIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M12 5v14M5 12h14" stroke="var(--mk-accent-700)" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function AddSourceForm({ onSubmit, error }: AddSourceFormProps) {
  const [expanded, setExpanded] = useState(false);
  const [tab, setTab] = useState<Tab>("link");
  const [url, setUrl] = useState("");
  const [title, setTitle] = useState("");
  const [text, setText] = useState("");
  const [takeaway, setTakeaway] = useState("");
  const [tier, setTier] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const canSubmit =
    tab === "link"
      ? url.trim() !== "" && takeaway.trim() !== "" && tier !== ""
      : title.trim() !== "" && text.trim() !== "" && takeaway.trim() !== "" && tier !== "";

  const reset = () => {
    setUrl("");
    setTitle("");
    setText("");
    setTakeaway("");
    setTier("");
    setTab("link");
    setExpanded(false);
  };

  const handleSubmit = async () => {
    if (!canSubmit || submitting) return;
    setSubmitting(true);
    try {
      const body: AddMaterialBody =
        tab === "link"
          ? { url: url.trim(), takeaway: takeaway.trim(), tier }
          : { title: title.trim(), text: text.trim(), takeaway: takeaway.trim(), tier };
      await onSubmit(body);
      reset();
    } catch {
      // The parent surfaces the failure via the `error` prop (the server's
      // own honest Chinese message) — nothing to invent here.
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div style={{ marginBottom: 14 }}>
      {error && (
        <div
          role="alert"
          style={{
            display: "flex",
            alignItems: "flex-start",
            gap: 9,
            background: "var(--mk-danger-bg)",
            border: "1px solid var(--mk-danger)",
            borderRadius: 12,
            padding: "10px 13px",
            marginBottom: 10,
            fontSize: 12.5,
            lineHeight: 1.6,
            color: "var(--mk-danger)",
          }}
        >
          {error}
        </div>
      )}

      {!expanded && (
        <button
          type="button"
          onClick={() => setExpanded(true)}
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: 7,
            fontSize: 13,
            fontWeight: 700,
            color: "var(--mk-accent-700)",
            cursor: "pointer",
            background: "transparent",
            border: "none",
            padding: "4px 2px",
            fontFamily: "inherit",
          }}
        >
          <PlusIcon />
          添加信源
        </button>
      )}

      {expanded && (
        <div
          style={{
            background: "var(--mk-surface)",
            border: "1px solid var(--mk-border)",
            borderRadius: 12,
            padding: "13px 15px",
            display: "flex",
            flexDirection: "column",
            gap: 10,
          }}
        >
          <div style={{ display: "flex", gap: 3, background: "var(--mk-border)", borderRadius: 9, padding: 3, alignSelf: "flex-start" }}>
            <button type="button" onClick={() => setTab("link")} style={tabStyle(tab === "link")}>
              链接
            </button>
            <button type="button" onClick={() => setTab("paste")} style={tabStyle(tab === "paste")}>
              粘贴正文
            </button>
          </div>

          {tab === "link" ? (
            <input
              placeholder="粘贴链接，例如 https://..."
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              style={INPUT_STYLE}
            />
          ) : (
            <>
              <input
                placeholder="标题（必填）"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                style={INPUT_STYLE}
              />
              <textarea
                placeholder="把正文粘贴进来……"
                value={text}
                onChange={(e) => setText(e.target.value)}
                rows={4}
                style={{ ...INPUT_STYLE, resize: "vertical" }}
              />
            </>
          )}

          <textarea
            placeholder="一句话说说你从这条里读到了什么……"
            value={takeaway}
            onChange={(e) => setTakeaway(e.target.value)}
            rows={2}
            style={{ ...INPUT_STYLE, resize: "vertical" }}
          />

          <div>
            <div style={{ fontSize: 12, fontWeight: 700, color: "var(--mk-muted)", marginBottom: 6 }}>层级</div>
            <div style={{ display: "flex", flexWrap: "wrap", gap: "8px 14px" }}>
              {TIERS.map((t) => (
                <label
                  key={t}
                  style={{ display: "inline-flex", alignItems: "center", gap: 5, fontSize: 12.5, color: "var(--mk-secondary)", cursor: "pointer" }}
                >
                  <input type="radio" name="tier" value={t} checked={tier === t} onChange={() => setTier(t)} />
                  {t}
                </label>
              ))}
            </div>
          </div>

          <button
            type="button"
            onClick={handleSubmit}
            disabled={!canSubmit || submitting}
            style={{
              alignSelf: "flex-start",
              marginTop: 4,
              background: !canSubmit || submitting ? "#F0E9E1" : "var(--mk-accent)",
              color: !canSubmit || submitting ? "#B8ADA2" : "var(--mk-surface)",
              border: "none",
              borderRadius: 10,
              fontSize: 13,
              fontWeight: 700,
              padding: "8px 16px",
              cursor: !canSubmit || submitting ? "default" : "pointer",
              fontFamily: "inherit",
            }}
          >
            {submitting ? "正在取正文…" : "加入信源档案"}
          </button>
        </div>
      )}
    </div>
  );
}
