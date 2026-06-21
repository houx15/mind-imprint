import { useState } from "react";
import { createStore, useStore, type Store } from "../store";

const defaultStore: Store = createStore({ storage: window.localStorage });

export function StorePanel({ store = defaultStore }: { store?: Store }) {
  const snap = useStore(store);
  const [title, setTitle] = useState("");
  const [seed, setSeed] = useState("");
  const [selected, setSelected] = useState<string | null>(null);
  const [content, setContent] = useState("");

  return (
    <div className="space-y-6 p-6">
      <section className="space-y-2">
        <h2 className="text-sm font-semibold text-mk-ink">新建任务</h2>
        <input aria-label="任务标题" value={title} onChange={(e) => setTitle(e.target.value)}
          className="rounded border border-mk-border px-2 py-1 text-sm" />
        <input aria-label="任务种子" value={seed} onChange={(e) => setSeed(e.target.value)}
          className="ml-2 rounded border border-mk-border px-2 py-1 text-sm" />
        <button type="button" className="ml-2 rounded bg-mk-primary px-3 py-1 text-sm text-white"
          onClick={() => {
            if (!title.trim()) return;
            store.createTask({ title, seed: seed || null });
            setTitle("");
            setSeed("");
          }}>新建任务</button>
      </section>

      <section className="space-y-2">
        <h2 className="text-sm font-semibold text-mk-ink">任务（刷新后仍在 = 刷新不丢）</h2>
        <ul className="space-y-1">
          {snap.tasks.map((t) => (
            <li key={t.id} className="flex items-center gap-2 text-sm">
              <span>{t.title}</span>
              <button type="button" className="rounded border border-mk-border px-2 py-0.5 text-xs"
                onClick={() => setSelected(t.id)}>选择</button>
            </li>
          ))}
        </ul>
      </section>

      {selected && (
        <section className="space-y-2">
          <h2 className="text-sm font-semibold text-mk-ink">消息</h2>
          <input aria-label="消息内容" value={content} onChange={(e) => setContent(e.target.value)}
            className="rounded border border-mk-border px-2 py-1 text-sm" />
          <button type="button" className="ml-2 rounded bg-mk-primary px-3 py-1 text-sm text-white"
            onClick={() => {
              if (!content.trim()) return;
              store.appendMessage({ task_id: selected, role: "user", content });
              setContent("");
            }}>追加消息</button>
          <ul className="space-y-1">
            {store.listMessages(selected).map((m) => (
              <li key={m.id} className="text-sm text-mk-muted-2">{m.content}</li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}
