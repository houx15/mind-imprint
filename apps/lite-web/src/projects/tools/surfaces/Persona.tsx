import { Says, errorMarkdown } from "../../Says";
import { useEffect, useRef, useState } from "react";
import { toggleAudienceRole, getAudienceDraft, saveAudienceDraft, summarizeAudienceDraft, confirmAudienceDraft, type AudienceDraft, type AudienceDocument, type AudienceBoard } from "../../../api/audienceBoard";
import { apiErrorText } from "../../../api/errorText";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";
import { AudienceProfileCard } from "./AudienceProfileCard";
import { AudiencePortrait } from "./AudiencePortrait";

const roles = [
  { name: "父母", hints: ["我的成长", "学习成果", "作品集"], people: ["妈妈", "爸爸", "照顾我的家人"] },
  { name: "老师", hints: ["学习成果", "作品集", "尝试与进步"], people: ["班主任", "一位任课老师", "社团指导老师"] },
  { name: "同学", hints: ["酷炫的效果", "互动小游戏", "闯关式介绍"], people: ["同班同学", "社团同学", "刚认识的新同学"] },
  { name: "陌生人", hints: ["我是什么样的人", "我的故事", "我的独特之处"], people: ["第一次看到作品的人", "有相同兴趣的人", "想参加活动的人"] },
];
const titles = { roles: "请选择主要读者", person: "请想到一个具体的人", age: "请选择这个人的年龄段", interests: "这个人最感兴趣的内容", offerings: "我准备向这个人展示的内容", summary: "请核对内容关键词" };
const blank: AudienceDocument = { boards: [], activeBoardId: "", step: "roles" };
const chip = "rounded-mk-lg border px-4 py-3 text-left text-mk-body transition-colors aria-pressed:border-mk-accent-500 aria-pressed:bg-mk-accent-100 disabled:opacity-40";
const action = "rounded-mk-lg bg-mk-accent-500 px-5 py-2.5 text-mk-body font-semibold text-white disabled:opacity-40";
const complete = (b: AudienceBoard) => Boolean(b.person && b.ageRange && b.interests.length && b.offerings.length);

export function Persona({ projectId, onFinish, onClose }: ToolSurfaceProps) {
  const [doc, setDoc] = useState<AudienceDocument>(blank);
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [remote, setRemote] = useState<AudienceDraft | null>(null);
  const [custom, setCustom] = useState("");
  const summary = doc.summary ?? null;
  const keywords = doc.keywords ?? {};
  const setKeywords = (update: (previous: Record<string, string[]>) => Record<string, string[]>) => setDoc(prev => ({ ...prev, keywords: update(prev.keywords ?? {}) }));
  const revision = useRef(0);
  const current = useRef(doc); current.current = doc;
  const saved = useRef("");
  const queue = useRef<Promise<void>>(Promise.resolve());
  const failed = useRef(false);

  useEffect(() => {
    let live = true;
    getAudienceDraft(projectId).then(data => {
      if (!live) return;
      revision.current = data.revision;
      saved.current = JSON.stringify(data.document);
      setDoc(data.document); setLoaded(true);
    }).catch(err => live && setError(apiErrorText(err)));
    return () => { live = false; };
  }, [projectId]);

  const persist = (next: AudienceDocument) => {
    const task = queue.current.catch(() => {}).then(async () => {
      if (failed.current) throw new Error("保存失败：请先处理当前保存错误");
      const encoded = JSON.stringify(next);
      if (saved.current === encoded) return;
      setSaving(true);
      try {
        const result = await saveAudienceDraft(projectId, { document: next, revision: revision.current });
        revision.current = result.revision; saved.current = encoded;
      } catch (err) { failed.current = true; setError(apiErrorText(err)); throw err; }
      finally { setSaving(false); }
    });
    queue.current = task;
    return task;
  };
  useEffect(() => {
    if (!loaded || failed.current) return;
    const timer = setTimeout(() => { void persist(doc).catch(() => {}); }, 400);
    return () => clearTimeout(timer);
  }, [doc, loaded]);

  const compareSaved = async () => {
    setBusy(true);
    // Drain all pending writes before loading the revision used for recovery.
    failed.current = true;
    try { await queue.current.catch(() => {}); setRemote(await getAudienceDraft(projectId)); }
    catch (err) { setError(apiErrorText(err)); }
    finally { setBusy(false); }
  };
  const resolveSave = async (keepLocal: boolean) => {
    if (!remote) return;
    setBusy(true);
    try {
      await queue.current.catch(() => {});
      revision.current = remote.revision;
      saved.current = JSON.stringify(remote.document);
      failed.current = false;
      if (keepLocal) await persist(current.current);
      else { current.current = remote.document; setDoc(remote.document); setLoaded(true); }
      setRemote(null); setError(null);
    } catch (err) { setRemote(null); setError(apiErrorText(err)); }
    finally { setBusy(false); }
  };

  const change = (next: AudienceDocument) => { setDoc(next); setCustom(""); };
  const active = doc.boards.find(b => b.id === doc.activeBoardId) ?? doc.boards[0];
  const updateBoard = (patch: Partial<AudienceBoard>) => change({ ...doc, summary: undefined, keywords: undefined, boards: doc.boards.map(b => b.id === active?.id ? { ...b, ...patch } : b) });
  const chooseRole = (role: string) => {
    const next = toggleAudienceRole(doc, role, () => crypto.randomUUID());
    if (next === doc) { setError("人物板数量已达上限，请恢复已有读者后再调整选择"); return; }
    change(next);
  };
  const go = async (next: AudienceDocument) => {
    setBusy(true); setError(null);
    try { await persist(next); change(next); } catch { /* Keep current input visible. */ } finally { setBusy(false); }
  };
  const summarize = async () => {
    setBusy(true); setError(null);
    try {
      await persist(current.current);
      const result = await summarizeAudienceDraft(projectId, revision.current);
      const next = { ...current.current, summary: result.summary, keywords: Object.fromEntries(result.summary.boards.map(b => [b.boardId, b.keywords.map(k => k.text)])) };
      change(next);
      await persist(next);
    } catch (err) { setError(apiErrorText(err)); } finally { setBusy(false); }
  };
  const finish = async () => {
    setBusy(true); setError(null);
    try {
      await persist(current.current);
      const result = await confirmAudienceDraft(projectId, revision.current, keywords);
      revision.current = result.revision;
      onFinish({ boards: doc.boards, keywords }, `已确定${doc.boards.length}类读者：${doc.boards.map(b => b.role + "（" + b.person + "）").join("、")}。人物板与内容关键词已确认，请据此进入自由风格、意象与第一幕构思；这些偏好是学生的判断，尚非访谈结果。`);
    } catch (err) { setError(apiErrorText(err)); setBusy(false); }
  };

  return <ToolFrame title="读者人物板" task={doc.step === "roles" ? titles.roles : doc.step === "summary" ? titles.summary : "请完善这位读者的人物卡"} why="读者关注的内容会影响作品的展示重点。请结合具体的人完善人物板。" todo={summary && doc.boards.every(b => keywords[b.id]?.length) ? "" : "请完成人物板并核对关键词"} onFinish={() => void finish()} finishLabel="确认人物板" busy={busy || !loaded || failed.current} onClose={() => {
    if (!loaded) { onClose(); return; }
    setBusy(true); void persist(current.current).then(onClose).catch(() => setBusy(false));
  }}>
    <div className="min-h-0 flex-1 overflow-auto p-4">
      <p className="mb-3 text-mk-small text-mk-muted" role="status">{!loaded ? "正在读取人物板…" : saving ? "正在保存…" : "人物板草稿"}</p>
      {error && <section role="alert" className="mb-4 rounded-mk-lg border border-mk-border p-4">
        <Says content={errorMarkdown(error)} />
        <button disabled={busy} className="mt-3 underline" onClick={() => void compareSaved()}>读取已保存版本进行比较</button>
        {remote && <>
          <p className="my-3 text-mk-small">请比较两份人物板。保留当前输入会替换已保存的草稿；使用已保存版本会放弃当前未保存的修改。</p>
          <div className="grid gap-4 md:grid-cols-2"><AudienceDraftReview title="当前输入" document={doc} /><AudienceDraftReview title="已保存版本" document={remote.document} /></div>
          <div className="mt-4 flex flex-wrap gap-3">
            <button disabled={busy} className={chip} onClick={() => void resolveSave(false)}>使用已保存版本</button>
            <button disabled={busy || !loaded} className={chip} onClick={() => void resolveSave(true)}>保留当前输入并保存</button>
          </div>
        </>}
      </section>}
      <fieldset disabled={busy || !loaded || failed.current} className="min-w-0">
        {doc.step === "roles" ? <>
          <p className="mb-4 text-mk-small text-mk-secondary">可选择多类读者。取消选择会保留人物板，重新选择可恢复。卡片中的关注点仅供参考。</p>
          <div className="grid grid-cols-2 gap-3">{roles.map(role => <button key={role.name} type="button" aria-pressed={doc.boards.some(b => b.role === role.name)} onClick={() => chooseRole(role.name)} className={chip} style={{ background: doc.boards.some(b => b.role === role.name) ? "var(--mk-accent-100)" : "var(--mk-surface)" }}>
            <AudiencePortrait role={role.name} className="mx-auto h-28" /><strong className="block mt-2">{role.name}</strong><span className="mt-2 block text-mk-small text-mk-secondary">{role.hints.join(" · ")}</span>
          </button>)}</div>
          {[...doc.boards, ...(doc.archivedBoards ?? [])].filter(b => !roles.some(r => r.name === b.role)).map(b => <button key={b.id} aria-pressed={doc.boards.some(selected => selected.id === b.id)} className={`${chip} mt-3`} onClick={() => chooseRole(b.role)}>{b.role} · {doc.boards.some(selected => selected.id === b.id) ? "已选择" : "未选择，草稿已保留"}</button>)}
          <form className="mt-4 flex gap-2" onSubmit={e => { e.preventDefault(); if (custom.trim() && doc.boards.length < 12) chooseRole(custom.trim()); }}><input aria-label="其他读者" placeholder="其他读者" maxLength={100} value={custom} onChange={e => setCustom(e.target.value)} className="min-w-0 flex-1 rounded-mk-lg border border-mk-border p-2" /><button disabled={!custom.trim() || doc.boards.length >= 12} className={chip}>添加</button></form>
        </> : <>
          <div className="mb-4"><button className="text-mk-small text-mk-secondary underline" onClick={() => void go({ ...doc, step: "roles" })}>修改读者选择</button>{doc.step === "summary" && <button className="ml-4 text-mk-small text-mk-secondary underline" onClick={() => void go({ ...doc, step: "person", activeBoardId: doc.boards[0]?.id ?? "" })}>返回人物卡</button>}</div>
          {doc.step !== "summary" && active ? <AudienceProfileCard key={active.id} board={active} index={doc.boards.findIndex(b => b.id === active.id)} total={doc.boards.length} onChange={updateBoard}
            onPrevious={() => { const index = doc.boards.findIndex(b => b.id === active.id); if (index > 0) void go({ ...doc, activeBoardId: doc.boards[index - 1]!.id, step: "person" }); }}
            onNext={() => { const index = doc.boards.findIndex(b => b.id === active.id); if (index < doc.boards.length - 1) void go({ ...doc, activeBoardId: doc.boards[index + 1]!.id, step: "person" }); }}
            onReview={() => { const pending = doc.boards.find(b => !complete(b)); if (pending) { void go({ ...doc, activeBoardId: pending.id, step: "person" }); } else { void go({ ...doc, step: "summary" }); } }}
          /> : <section>
            <p className="mb-3 text-mk-small text-mk-secondary">以下总结来自人物板。请核对每个关键词及其依据，删除不合适的词。</p>
            {!summary ? <button className={action} onClick={() => void summarize()} disabled={!doc.boards.every(complete)}>{busy ? "正在总结…" : "总结内容关键词"}</button> : summary.boards.map(board => {
              const source = doc.boards.find(b => b.id === board.boardId)!;
              return <div key={board.boardId} className="mb-4 rounded-mk-lg border border-mk-border p-4"><h3 className="font-semibold">{source.role} · {source.person}</h3><p className="my-2 text-mk-small">准备展示：{source.offerings.join("、")}</p>{board.keywords.map(k => <button key={k.text} className={`${chip} mb-2 mr-2`} aria-pressed={keywords[board.boardId]?.includes(k.text)} disabled={!keywords[board.boardId]?.includes(k.text) && (keywords[board.boardId]?.length ?? 0) >= 6} onClick={() => setKeywords(prev => ({ ...prev, [board.boardId]: prev[board.boardId]!.includes(k.text) ? prev[board.boardId]!.filter(w => w !== k.text) : [...prev[board.boardId]!, k.text] }))}><span className={keywords[board.boardId]?.includes(k.text) ? "" : "line-through opacity-40"}>{k.text}</span><span className="block text-mk-small text-mk-muted">依据：{source[k.sourceField][k.sourceIndex]}</span></button>)}{(keywords[board.boardId] ?? []).filter(word => !board.keywords.some(k => k.text === word)).map(word => <button key={word} className={`${chip} mb-2 mr-2`} aria-pressed="true" onClick={() => setKeywords(prev => ({ ...prev, [board.boardId]: prev[board.boardId]!.filter(w => w !== word) }))}>{word}<span className="block text-mk-small text-mk-muted">学生补充</span></button>)}<KeywordInput label={source.role} disabled={(keywords[board.boardId]?.length ?? 0) >= 6} onAdd={word => setKeywords(prev => ({ ...prev, [board.boardId]: Array.from(new Set([...(prev[board.boardId] ?? []), word])) }))} /></div>;
            })}
          </section>}
        </>}
        {doc.step === "roles" && <button className={`${action} mt-5`} disabled={!doc.boards.length} onClick={() => void go({ ...doc, step: "person", activeBoardId: doc.boards[0]!.id })}>完善人物板</button>}
      </fieldset>
    </div>
  </ToolFrame>;
}

function KeywordInput({ label, disabled, onAdd }: { label: string; disabled: boolean; onAdd: (word: string) => void }) {
  const [value, setValue] = useState("");
  return <form className="mt-3 flex gap-2" onSubmit={event => { event.preventDefault(); if (value.trim() && !disabled) { onAdd(value.trim()); setValue(""); } }}>
    <input aria-label={`${label}补充关键词`} value={value} maxLength={40} onChange={event => setValue(event.target.value)} placeholder="补充关键词" className="min-w-0 flex-1 rounded-mk-lg border border-mk-border p-2" />
    <button className={chip} disabled={disabled || !value.trim()}>添加</button>
  </form>;
}

function AudienceDraftReview({ title, document }: { title: string; document: AudienceDocument }) {
  return <section className="min-w-0 rounded-xl border-2 border-mk-border bg-mk-surface p-4">
    <h3 className="mb-3 font-semibold">{title}</h3>
    {!document.boards.length && <p>未选择读者</p>}
    {[...document.boards, ...(document.archivedBoards ?? [])].map(board => <div key={board.id} className="mb-3 border-t border-mk-border pt-3 text-mk-small break-words">
      <h4 className="font-semibold">{board.role}{document.boards.some(b => b.id === board.id) ? " · 已选择" : " · 未选择"}</h4>
      <p>具体人物：{board.person || "未填写"}</p><p>大致年龄：{board.ageRange || "未填写"}</p>
      <p>兴趣爱好：{board.hobbies?.join("、") || "未填写"}</p>
      <p>关注内容：{board.interests.join("、") || "未填写"}</p>
      <p>展示内容：{board.offerings.join("、") || "未填写"}</p>
      <p>内容关键词：{document.keywords?.[board.id]?.join("、") || "未总结"}</p>
    </div>)}
  </section>;
}
