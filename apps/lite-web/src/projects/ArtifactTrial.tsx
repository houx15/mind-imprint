import { Says, errorMarkdown } from "./Says";
import { useEffect, useRef, useState } from "react";
import type { Artifact } from "../api/artifacts";
import type { ArtifactTrial as Trial } from "../api/artifactTrial";
import { useArtifactTrialDraft } from "./useArtifactTrialDraft";
import { listKeepEntries, openKeepSession, type KeepEntry } from "../api/lookback";
import { apiErrorText } from "../api/errorText";
import { GrowingTextarea } from "../shared/GrowingTextarea";

export function ArtifactTrial({ projectId, artifact, registerFlush, onOpenSession, disabled = false }: { projectId: string; artifact: Artifact; registerFlush: (flush: () => Promise<void>) => () => void; disabled?: boolean; onOpenSession: (id: string) => void }) {
  const draft = useArtifactTrialDraft(projectId, artifact.id);
  const trial = draft.document;
  const setTrial = draft.change;
  useEffect(() => registerFlush(draft.flush), [registerFlush, draft.flush]);
  const [saved, setSaved] = useState<KeepEntry | null>(null);
  const [history, setHistory] = useState<KeepEntry[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const lock = useRef(false);
  const [focus, setFocus] = useState<"mode" | "version" | "task" | "expected" | "actual" | "next" | "review">("mode");
  const fields = ["mode", "version", "task", "expected", ...(trial.mode === "not_tested" ? [] : ["actual"]), "next", "review"] as const;
  const fieldInfo = {
    version: ["试用版本", "记录版本能让下一次修改有明确的比较对象。", "请填写版本名称或预览地址"],
    task: ["操作任务", "一次试用先检验一个具体任务，便于判断哪里需要修改。", "请描述一次能实际执行的操作，例如：用提纲试问一个问题，或按说明折一次小册子"],
    expected: ["预期结果", trial.mode === "not_tested" ? "请先明确怎样的结果符合你的设计。" : "请记录试用前的预期。如果当时没有设定预期，请如实注明。", "请写能观察到的判断标准，例如：对方能理解这个问题，或能按照说明完成操作"],
    actual: ["实际观察", "请记录发生了什么，再解释原因。停顿、误解、失败和与你预期相反的结果也有价值。", "请记录操作、反应或原话。谁操作、用了什么设备或材料，也可以一并注明；姓名可用代号"],
    next: ["保留或修改的判断", trial.mode === "not_tested" ? "还没有实际观察，请把修改想法作为待验证的猜想。" : "请比较预期与实际观察，说明准备保留什么或改动什么。", "请说明你的判断与理由；尚未决定可以留空"],
  } as const;
  useEffect(() => { if (trial.mode === "not_tested" && focus === "actual") setFocus("next"); }, [trial.mode, focus]);
  const index = fields.indexOf(focus);
  const move = (offset: number) => setFocus(fields[Math.max(0, Math.min(fields.length - 1, index + offset))] as typeof focus);
  const canContinue = focus === "mode" || focus === "next" || focus === "review" || Boolean(trial[focus].trim());
  const field = focus !== "mode" && focus !== "review" ? focus : null;
  useEffect(() => {
    let live = true;
    listKeepEntries(projectId).then(entries => {
      if (live) setHistory(current => [...current, ...entries.filter(entry => entry.body.split("\n")[0]?.endsWith(`（${artifact.id}）`) && !current.some(savedEntry => savedEntry.id === entry.id))]);
    }).catch(err => { if (live) setError(apiErrorText(err)); });
    return () => { live = false; };
  }, [projectId, artifact.id]);
  const save = async () => {
    if (lock.current || disabled) return;
    lock.current = true; setBusy(true); setError("");
    try {
      const entry = await draft.submit();
      setSaved(entry);
      setFocus("mode");
      setHistory(entries => [entry, ...entries]);
    } catch (err) { setError(apiErrorText(err)); }
    finally { lock.current = false; setBusy(false); }
  };
  const discuss = async () => {
    if (!saved || lock.current || disabled) return;
    lock.current = true; setBusy(true); setError("");
    try { const session = await openKeepSession(projectId, saved.id); onOpenSession(session.sessionId); }
    catch (err) { setError(apiErrorText(err)); }
    finally { lock.current = false; setBusy(false); }
  };
  return <section className="rounded-xl border border-mk-border p-4 space-y-3">
    <h4 className="font-semibold">试用与修改</h4>
    <p className="text-mk-small text-mk-secondary">具体操作的结果能帮助你判断下一次修改。请记录一次操作，并区分预期与实际发生的情况。</p>
    {error && <div role="alert" className="text-mk-danger"><Says content={errorMarkdown(error)} /></div>}
    {!saved && <p role="status" className="text-mk-small text-mk-muted">{!draft.loaded ? draft.error ? "草稿读取失败" : "正在读取试用草稿…" : draft.saving ? "草稿保存中…" : draft.dirty ? "草稿待保存" : "草稿已保存，尚未提交为试用记录"}</p>}
    {draft.error && <section role="alert" className="space-y-3 rounded border border-mk-border p-3">
      <div><Says content={errorMarkdown(draft.error)} /></div><button disabled={busy} className="underline" onClick={() => void draft.inspect()}>读取已保存草稿进行比较</button>
      {draft.remote && <><div className="grid gap-3 md:grid-cols-2">{[["当前输入", trial], ["已保存草稿", draft.remote.document]].map(([title, value]) => <div key={String(title)} className="rounded border border-mk-border p-3"><h5 className="font-semibold">{String(title)}</h5><dl className="text-mk-small">{Object.entries(value as Trial).map(([key, text]) => <div key={key}><dt>{({mode:"试用方式", version:"版本", task:"操作任务", expected:"预期结果", actual:"实际结果", next:"修改想法"} as Record<string,string>)[key]}</dt><dd className="whitespace-pre-wrap break-words">{key === "mode" ? ({self:"自己操作",other:"他人试用",not_tested:"尚未测试"} as Record<string,string>)[text] : text || "未填写"}</dd></div>)}</dl></div>)}</div>
      <p className="text-mk-small">保留当前输入会替换已保存草稿；使用已保存草稿会放弃当前未保存的修改。</p>
      <button className="mr-3 underline" disabled={busy} onClick={() => void draft.resolve(false)}>使用已保存草稿</button><button className="underline" disabled={busy || !draft.loaded} onClick={() => void draft.resolve(true)}>保留当前输入并保存</button></>}
    </section>}
    {history.length > 0 && <label className="block text-mk-small">已有试用记录<select disabled={busy || disabled} value={saved?.id ?? ""} onChange={e => setSaved(history.find(entry => entry.id === e.target.value) ?? null)} className="mt-1 block w-full rounded border border-mk-border bg-mk-surface p-2"><option value="">新记录</option>{history.map(entry => <option key={entry.id} value={entry.id}>{new Date(entry.createdAt).toLocaleString()} · {entry.kind === "thought" ? "未测试计划" : "试用记录"}</option>)}</select></label>}
    {saved ? <>
      <p role="status">{saved.kind === "thought" ? "试用计划已保存，尚未获得实际结果" : "试用记录已保存"}</p>
      <p className="whitespace-pre-wrap text-mk-small">{saved.body}</p>
      <button disabled={busy || disabled} onClick={() => void discuss()} className="rounded-full bg-mk-accent-500 px-4 py-2 text-white disabled:opacity-40">{busy ? "处理中…" : "讨论这条记录"}</button>
    </> : <fieldset disabled={busy || disabled || !draft.loaded || Boolean(draft.error)} className="space-y-3 min-w-0">
      <div className="flex items-center justify-between gap-3"><p className="text-mk-small text-mk-muted">{index + 1} / {fields.length} · {focus === "mode" ? "试用情况" : focus === "review" ? "核对记录" : fieldInfo[focus][0]}</p>{focus !== "mode" && <button type="button" className="underline text-mk-small" onClick={() => move(-1)}>上一步</button>}</div>
      {focus === "mode" ? <div className="space-y-3"><h5 className="font-semibold">这次已经实际试过了吗？</h5><div className="grid gap-3 sm:grid-cols-3">{([
        ["not_tested", "尚未测试", "先准备试用计划，不记录成实际结果"],
        ["self", "我实际试过了", "记录自己的操作，不代表其他人的体验"],
        ["other", "其他人试过了", "记录你观察到的反应，不替对方猜测"],
      ] as const).map(([mode,title,hint]) => <button key={mode} type="button" aria-pressed={trial.mode === mode} onClick={() => {setTrial({...trial,mode});setFocus("version");}} className="rounded-xl border border-mk-border p-4 text-left aria-pressed:border-mk-accent-500"><strong className="block text-mk-body">{title}</strong><span className="mt-2 block text-mk-small text-mk-secondary">{hint}</span></button>)}</div></div> : field ? <div className="space-y-3">
        <label className="block font-semibold">{fieldInfo[field][0]}<span className="my-2 block text-mk-small font-normal text-mk-secondary">{fieldInfo[field][1]}</span><GrowingTextarea aria-label={fieldInfo[field][0]} value={trial[field]} maxLength={1500} onChange={e => setTrial({...trial,[field]:e.target.value})} placeholder={fieldInfo[field][2]} className="mt-2 block w-full rounded-xl border border-mk-border bg-transparent p-3 font-normal"/></label>
        {field === "version" && <button type="button" className="text-mk-small underline" onClick={() => setTrial({...trial,version:`${artifact.title} · ${new Date(artifact.createdAt).toLocaleString()}`})}>使用当前成果版本</button>}
        {field === "expected" && trial.mode !== "not_tested" && <button type="button" className="text-mk-small underline" onClick={() => setTrial({...trial,expected:"试用前没有设定预期，本次仅记录实际观察。"})}>当时没有设定预期</button>}
        <button type="button" disabled={!canContinue} onClick={() => move(1)} className="rounded-full border border-mk-border px-4 py-2 disabled:opacity-40">{field === "next" ? "核对记录" : "下一步"}</button>
      </div> : <div className="space-y-3">
        <p className="text-mk-small text-mk-secondary">{trial.mode === "not_tested" ? "这份内容将保存为试用计划，不会算作反馈或实际测试结果。" : "请核对预期与实际观察是否分开，判断是否有观察依据。保存不会自动代表成果已通过审核。"}</p>
        <button type="button" className="text-mk-small underline" onClick={() => setFocus("mode")}>修改试用情况</button>
        <dl className="space-y-3">{fields.filter((key): key is keyof typeof fieldInfo => key !== "mode" && key !== "review").map(key => <div key={key} className="rounded-xl border border-mk-border p-3"><dt className="flex justify-between gap-3 text-mk-small font-semibold">{fieldInfo[key][0]}<button type="button" className="font-normal underline" onClick={() => setFocus(key)}>修改{fieldInfo[key][0]}</button></dt><dd className="mt-2 whitespace-pre-wrap break-words text-mk-small">{trial[key] || "尚未决定"}</dd></div>)}</dl>
        <button type="button" onClick={() => void save()} className="rounded-full bg-mk-accent-500 px-4 py-2 text-white disabled:opacity-40">{busy ? "保存中…" : trial.mode === "not_tested" ? "保存试用计划" : "保存试用记录"}</button>
      </div>}

    </fieldset>}
  </section>;
}
