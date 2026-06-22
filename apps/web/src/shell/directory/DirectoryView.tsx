import { useSyncExternalStore, useState } from "react";
import type { Store } from "../../store/createStore";
import { taskCardView } from "./taskCardView";

export function DirectoryView({
  store,
  onOpenTask,
  now,
}: {
  store: Store;
  onOpenTask: (taskId: string) => void;
  now?: () => Date;
}) {
  const _now = now ?? (() => new Date());
  useSyncExternalStore(store.subscribe, store.getSnapshot);
  const tasks = store.listTasks();
  const taskCount = tasks.length;
  const [input, setInput] = useState("");

  function handleStart() {
    const trimmed = input.trim();
    if (!trimmed) return;
    const urlMatch = trimmed.match(/(https?:\/\/\S+)/);
    const seed = urlMatch ? urlMatch[1]! : null;
    const t = store.createTask({ title: trimmed, seed });
    store.appendMessage({ task_id: t.id, role: "user", content: trimmed });
    onOpenTask(t.id);
  }

  return (
    <div style={{ height: "100%", minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 980, margin: "0 auto", padding: "44px 40px 60px" }}>
        <div style={{ fontSize: 13, color: "#8A92A3", fontWeight: 600 }}>下午好，Phoebe</div>
        <div
          style={{
            fontSize: 28,
            fontWeight: 800,
            color: "#1C2333",
            marginTop: 6,
            letterSpacing: "-0.01em",
          }}
        >
          今天你在尝试什么？
        </div>
        <div style={{ fontSize: 15, color: "#6B7384", fontWeight: 500, marginTop: 8, lineHeight: 1.6 }}>
          你有任何想讨论的作业、课题、信息、资料，都可以来找我哦
        </div>

        {/* new task entry */}
        <div
          style={{
            marginTop: 22,
            background: "#fff",
            border: "1px solid #EAECF2",
            borderRadius: 18,
            padding: "20px 22px",
            display: "flex",
            alignItems: "center",
            gap: 16,
            boxShadow: "0 4px 18px rgba(20,30,60,.05)",
          }}
        >
          <svg viewBox="0 0 48 48" width="42" height="42" style={{ display: "block", flex: "none" }}>
            <rect x="5" y="6" width="38" height="36" rx="13" fill="#2A3B7A" />
            <rect x="5" y="6" width="38" height="17" rx="13" fill="#ffffff" opacity="0.10" />
            <ellipse cx="18.5" cy="24" rx="3.4" ry="3.9" fill="#fff" />
            <ellipse cx="29.5" cy="24" rx="3.4" ry="3.9" fill="#fff" />
            <circle cx="19.3" cy="25" r="1.5" fill="#1C2333" />
            <circle cx="30.3" cy="25" r="1.5" fill="#1C2333" />
            <path
              d="M19 31.5 Q24 35 29 31.5"
              stroke="#fff"
              strokeWidth="2.2"
              fill="none"
              strokeLinecap="round"
            />
            <circle cx="39" cy="9" r="4.5" fill="#E8A33D" />
          </svg>
          <input
            placeholder="把你正在纠结的问题写下来——带上你自己的东西（链接、草稿、本子上的话）。"
            style={{
              flex: 1,
              border: "none",
              outline: "none",
              fontSize: 15,
              color: "#1C2333",
              background: "transparent",
            }}
            value={input}
            onChange={(e) => setInput(e.target.value)}
          />
          <button
            onClick={handleStart}
            style={{
              flex: "none",
              display: "inline-flex",
              alignItems: "center",
              gap: 8,
              background: "#2A3B7A",
              color: "#fff",
              border: "none",
              padding: "12px 20px",
              borderRadius: 12,
              fontSize: 14,
              fontWeight: 700,
              cursor: "pointer",
              fontFamily: "inherit",
            }}
          >
            开始
            <svg
              width="15"
              height="15"
              viewBox="0 0 24 24"
              fill="none"
              stroke="#fff"
              strokeWidth="2.2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M5 12h14M13 6l6 6-6 6" />
            </svg>
          </button>
        </div>

        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            margin: "34px 0 16px",
          }}
        >
          <div style={{ fontSize: 15, fontWeight: 700, color: "#1C2333" }}>进行中的任务</div>
          <div style={{ fontSize: 13, color: "#9AA1B0", fontWeight: 600 }}>{taskCount} 个任务</div>
        </div>

        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(2, 1fr)",
            gap: 16,
          }}
        >
          {tasks.map((t) => {
            const view = taskCardView(t, store.listCards(t.id), _now());
            return (
              <div
                key={t.id}
                onClick={() => onOpenTask(t.id)}
                style={{
                  background: "#fff",
                  border: "1px solid #EAECF2",
                  borderRadius: 16,
                  padding: "18px 20px",
                  cursor: "pointer",
                  boxShadow: "0 1px 3px rgba(20,30,60,.04)",
                  transition: "box-shadow .18s, transform .18s",
                }}
              >
                <div
                  style={{
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "space-between",
                    marginBottom: 12,
                  }}
                >
                  <span style={Object.fromEntries(
                    view.statusStyle
                      .split(";")
                      .filter(Boolean)
                      .map((s) => {
                        const [k, ...rest] = s.split(":");
                        const key = k!.trim().replace(/-([a-z])/g, (_, c) => c.toUpperCase());
                        return [key, rest.join(":").trim()];
                      })
                  ) as React.CSSProperties}>
                    {view.status}
                  </span>
                  <span style={{ fontSize: 12, color: "#9AA1B0", fontWeight: 500 }}>{view.last}</span>
                </div>
                <div
                  style={{
                    fontSize: 16,
                    fontWeight: 700,
                    color: "#1C2333",
                    lineHeight: 1.45,
                    minHeight: 46,
                  }}
                >
                  {view.title}
                </div>
                <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 14 }}>
                  <div
                    style={{
                      flex: 1,
                      height: 6,
                      background: "#EEF0F4",
                      borderRadius: 999,
                      overflow: "hidden",
                    }}
                  >
                    <div
                      style={Object.fromEntries(
                        view.barStyle
                          .split(";")
                          .filter(Boolean)
                          .map((s) => {
                            const [k, ...rest] = s.split(":");
                            const key = k!.trim().replace(/-([a-z])/g, (_, c) => c.toUpperCase());
                            return [key, rest.join(":").trim()];
                          })
                      ) as React.CSSProperties}
                    />
                  </div>
                  <span style={{ fontSize: 12, color: "#8A92A3", fontWeight: 600, flex: "none" }}>
                    {view.cardsLabel}
                  </span>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
