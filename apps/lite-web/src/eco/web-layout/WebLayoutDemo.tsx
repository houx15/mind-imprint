import { useEffect, useMemo, useState } from "react";
import { ArrowLeft, FileCode2, Image, Send, Sparkles, Trash2, Upload } from "lucide-react";
import { Icon, Pebble } from "@/ui";
import type { PatchSpecV1, ReferenceSpecV1, VisualDesignSpecV1, VisualNodeV1 } from "@mind-imprint/contracts";
import { checkGateway, gatewayAddress, requestVisualDesign, requestVisualPatch, requestVisionAnalysis, VisualDesignValidationError, type VisualDesignDiagnostic } from "./api";
import { WhiteboardStage } from "./WhiteboardStage";
import { htmlToReferenceSpec, imageToReferenceSpec, mergeVisionAnalysis, whiteboardToReferenceSpec } from "./referenceSpec";
import type { WireframeDocument } from "./types";
import { VisualDesignCanvas } from "./VisualDesignCanvas";
import { applyPatch, manualPatch, rebasePatch } from "./visualPatch";

const initialWireframe: WireframeDocument = { viewport: "desktop", width: 960, height: 720, nodes: [] };
type GatewayState = { state: "checking" | "ready" | "offline"; provider?: string; model?: string };
type Run = { id: string; prompt: string; references: string[]; design: VisualDesignSpecV1; createdAt: string };
type PatchRow = { id: string; patch: PatchSpecV1; status: "applied" | "rejected" };
type SavedDesignDiagnostic = VisualDesignDiagnostic & { id: string; createdAt: string };
const readDataUrl = (file: File) => new Promise<string>((resolve, reject) => { const reader = new FileReader(); reader.onload = () => resolve(String(reader.result)); reader.onerror = () => reject(reader.error); reader.readAsDataURL(file); });
const readText = (file: File) => new Promise<string>((resolve, reject) => { const reader = new FileReader(); reader.onload = () => resolve(String(reader.result)); reader.onerror = () => reject(reader.error); reader.readAsText(file); });

function nodeById(design: VisualDesignSpecV1 | null, id: string | null): VisualNodeV1 | null {
  return design && id ? design.pages.flatMap((page) => page.nodes).find((node) => node.id === id) ?? null : null;
}

function diagnosticPath(path: (string | number)[]) {
  return path.reduce<string>((result, part) => typeof part === "number" ? `${result}[${part}]` : result ? `${result}.${String(part)}` : String(part), "") || "root";
}

function DesignDiagnosticScreen({ diagnostics, activeId, onSelect, onClose }: { diagnostics: SavedDesignDiagnostic[]; activeId: string; onSelect: (id: string) => void; onClose: () => void }) {
  const diagnostic = diagnostics.find((item) => item.id === activeId) ?? diagnostics[0];
  if (!diagnostic) return null;
  return <div className="eco-diagnostic-screen" role="dialog" aria-modal="true" aria-label="AI 生成诊断">
    <header className="eco-diagnostic-topbar"><div><span>视觉 PBL</span><strong>AI 生成诊断</strong></div><button onClick={onClose}>返回设计工具</button></header>
    <div className="eco-diagnostic-layout">
      <aside><h2>失败记录</h2><p>当前页面会话共 {diagnostics.length} 次</p>{diagnostics.map((item, index) => <button key={item.id} className={item.id === diagnostic.id ? "is-active" : ""} onClick={() => onSelect(item.id)}><strong>失败 {diagnostics.length - index}</strong><span>{item.createdAt} · {item.source === "gateway" ? "Gateway" : "前端 Schema"}</span><small>{item.issues.length} 个问题 · {item.attempts.length || 1} 次输出</small></button>)}</aside>
      <main>
        <div className="eco-diagnostic-summary"><div><span>{diagnostic.source === "gateway" ? "Gateway 拒绝" : "前端 Schema 拒绝"}</span><h1>VisualDesignSpecV1 未通过校验</h1><p>失败阶段：{diagnostic.stage}</p></div><b>{diagnostic.issues.length} 个问题</b></div>
        <section className="eco-diagnostic-issues"><h2>不符合项</h2><ol>{diagnostic.issues.map((issue, index) => <li key={`${diagnosticPath(issue.path)}.${index}`}><code>{diagnosticPath(issue.path)}</code><span>{issue.message}</span><small>{issue.code}</small></li>)}</ol></section>
        {diagnostic.attempts.map((attempt) => <section className="eco-diagnostic-output" key={attempt.attempt}><div><h2>第 {attempt.attempt} 次 AI 原始输出</h2><span>{attempt.parseError || attempt.qualityError || attempt.schemaError || attempt.callError || "已返回"}</span></div>{attempt.rawOutput ? <pre>{attempt.rawOutput}</pre> : <p>模型没有返回可保存的文本。</p>}</section>)}
        {!diagnostic.attempts.length && <section className="eco-diagnostic-output"><div><h2>未经修改的 AI JSON</h2></div><pre>{JSON.stringify(diagnostic.raw, null, 2) ?? String(diagnostic.raw)}</pre></section>}
        {diagnostic.canonicalized !== undefined && <section className="eco-diagnostic-output"><div><h2>前端标准化后的 JSON</h2></div><pre>{JSON.stringify(diagnostic.canonicalized, null, 2) ?? String(diagnostic.canonicalized)}</pre></section>}
      </main>
    </div>
  </div>;
}

export function WebLayoutDemo() {
  const [requirement, setRequirement] = useState("为我的个人研究项目制作一个有作品集感的主页，重点展示研究过程、阶段成果和联系方式。");
  const [wireframe, setWireframe] = useState(initialWireframe);
  const [references, setReferences] = useState<ReferenceSpecV1[]>([]);
  const [design, setDesign] = useState<VisualDesignSpecV1 | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [gateway, setGateway] = useState<GatewayState>({ state: "checking" });
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [designDiagnostics, setDesignDiagnostics] = useState<SavedDesignDiagnostic[]>([]);
  const [activeDiagnosticId, setActiveDiagnosticId] = useState<string | null>(null);
  const [instruction, setInstruction] = useState("");
  const [pendingPatch, setPendingPatch] = useState<PatchSpecV1 | null>(null);
  const [patches, setPatches] = useState<PatchRow[]>([]);
  const [runs, setRuns] = useState<Run[]>([]);
  const [showSpecs, setShowSpecs] = useState(true);
  const selected = nodeById(design, selectedId);
  const whiteboardSpec = useMemo(() => whiteboardToReferenceSpec(wireframe), [wireframe]);
  const allReferences = useMemo(() => [...references, ...(whiteboardSpec ? [whiteboardSpec] : [])], [references, whiteboardSpec]);

  useEffect(() => { checkGateway().then((info) => setGateway({ state: "ready", ...info })).catch(() => setGateway({ state: "offline" })); }, []);

  async function upload(files: FileList | null) {
    if (!files?.length) return;
    setBusy("正在提取参考资料…"); setError("");
    try {
      const parsed: ReferenceSpecV1[] = [];
      for (const file of Array.from(files)) {
        if (/html?/i.test(file.type) || /\.html?$/i.test(file.name)) parsed.push(htmlToReferenceSpec(await readText(file), file.name));
        else if (["image/png", "image/jpeg", "image/webp"].includes(file.type)) {
          const dataUrl = await readDataUrl(file);
          const local = await imageToReferenceSpec(dataUrl, file.name, file.type as "image/png" | "image/jpeg" | "image/webp");
          if (gateway.state === "ready") {
            try { parsed.push(mergeVisionAnalysis(local, await requestVisionAnalysis(file.name, file.type, dataUrl))); }
            catch (cause) { parsed.push({ ...local, limitations: [...local.limitations, `视觉语义分析未合并：${cause instanceof Error ? cause.message : "视觉模型暂不可用"}`] }); }
          } else parsed.push(local);
        }
        else throw new Error(`${file.name} 不是支持的 HTML、PNG、JPG 或 WebP 文件`);
      }
      setReferences((items) => [...items, ...parsed]);
    } catch (cause) { setError(cause instanceof Error ? cause.message : "参考资料解析失败"); }
    finally { setBusy(null); }
  }

  async function generate() {
    if (gateway.state !== "ready") { setError("真实 AI Gateway 尚未连接，请先启动 8787 服务"); return; }
    if (requirement.trim().length < 8) { setError("请先用一句完整的话说明设计目标"); return; }
    setBusy("真实 AI 正在生成 VisualDesignSpecV1…"); setError(""); setActiveDiagnosticId(null); setPendingPatch(null);
    try {
      const next = await requestVisualDesign(requirement, allReferences);
      setDesign(next); setSelectedId(null);
      setRuns((items) => [{ id: crypto.randomUUID(), prompt: requirement, references: allReferences.map((item) => item.summary), design: next, createdAt: new Date().toLocaleTimeString() }, ...items].slice(0, 8));
    } catch (cause) {
      if (cause instanceof VisualDesignValidationError) {
        const saved = { ...cause.diagnostic, id: crypto.randomUUID(), createdAt: new Date().toLocaleTimeString() };
        setDesignDiagnostics((items) => [saved, ...items].slice(0, 20));
        setActiveDiagnosticId(saved.id);
      }
      setError(cause instanceof Error ? cause.message : "设计生成失败");
    }
    finally { setBusy(null); }
  }

  function commitPatch(patch: PatchSpecV1) {
    if (!design) return;
    try {
      const ready = patch.baseRevision === design.revision ? patch : rebasePatch(patch, design);
      setDesign(applyPatch(design, ready));
      setPatches((items) => [{ id: ready.patchId, patch: ready, status: "applied" }, ...items]);
      setPendingPatch(null);
    } catch (cause) { setError(cause instanceof Error ? cause.message : "补丁应用失败"); }
  }

  async function askAI() {
    if (!design || instruction.trim().length < 2) return;
    setBusy("AI 正在生成 PatchSpecV1…"); setError("");
    try { setPendingPatch(await requestVisualPatch(instruction, design, selectedId ?? undefined)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "AI 修改失败"); }
    finally { setBusy(null); }
  }

  function selectedStyle(background?: string, color?: string) {
    if (!design || !selected) return;
    commitPatch(manualPatch(design, selected.id, "使用调色板修改组件", { operationId: `op.style.${crypto.randomUUID()}`, action: "set-style", nodeId: selected.id, style: { ...(background ? { background } : {}), ...(color ? { color } : {}) } }));
  }

  const gatewayLabel = gateway.state === "ready"
    ? `${gateway.provider}/${gateway.model}`
    : gateway.state === "checking" ? "检查中" : "未连接";

  return <main className="eco-web-app eco-visual-demo">
    <header className="eco-web-topbar eco-visual-topbar">
      <div className="eco-visual-title">
        <span className="eco-visual-mark"><Sparkles size={15} /></span>
        <div><strong>网页视觉设计</strong><span>视觉 PBL 工具</span></div>
      </div>
      <div className={`eco-visual-gateway is-${gateway.state}`}>
        <i />
        <span>{gateway.state === "ready" ? "AI 已连接" : gateway.state === "checking" ? "正在连接 AI" : "AI 未连接"}</span>
        <b>{gatewayLabel}</b>
      </div>
      {designDiagnostics.length > 0 && <button className="eco-diagnostic-entry" onClick={() => setActiveDiagnosticId(designDiagnostics[0]!.id)}>生成诊断 <b>{designDiagnostics.length}</b></button>}
    </header>

    {!design ? <div className="eco-room eco-visual-room">
      <section className="eco-chat-pane">
        <header className="eco-pane-heading">
          <div><span>主题确认</span><strong>说明设计目标</strong></div>
          <span className="eco-pane-count">1 / 3</span>
        </header>
        <div className="eco-chat-log">
          <div className="eco-agent-row">
            <Pebble state="idle" size={25} />
            <div className="eco-ai-bubble">请说明要制作的网页、使用对象和希望重点呈现的内容。右侧可以绘制结构，也可以上传参考网页。</div>
          </div>
          <div className="eco-intake-note">
            <span>设计目标</span>
            <p>AI 会同时读取文字、白板和参考文件，并据此生成可编辑的页面。</p>
          </div>
          {references.length > 0 && <div className="eco-intake-note is-complete">
            <span>参考资料</span>
            <p>已解析 {references.length} 个文件；白板{whiteboardSpec ? "已加入" : "未绘制"}。</p>
          </div>}
          {error && <p className="eco-inline-error" role="alert">{error}</p>}
          {designDiagnostics.length > 0 && <button className="eco-open-diagnostic" onClick={() => setActiveDiagnosticId(designDiagnostics[0]!.id)}>查看 AI 原始返回与全部错误</button>}
        </div>
        <footer className="eco-composer eco-goal-composer">
          <label htmlFor="eco-design-goal">设计要求</label>
          <textarea id="eco-design-goal" value={requirement} onChange={(event) => setRequirement(event.target.value)} rows={6} placeholder="请输入页面类型、受众、目标和视觉偏好" />
          <p>输入内容将与右侧参考资料一并交给 AI。</p>
        </footer>
      </section>

      <section className="eco-work-pane">
        <nav className="eco-project-journey" aria-label="网页设计进度">
          <span className="eco-project-journey-title">网页设计</span>
          <span className="is-current"><b>1</b>确认主题</span>
          <span><b>2</b>生成设计</span>
          <span><b>3</b>修改页面</span>
        </nav>
        <div className="eco-stage-host">
          <div className="eco-intake-stage">
            <div className="eco-stage-intro">
              <div><span className="eco-stage-eyebrow">结构与参考</span><h1>把想法放到画布上</h1><p>绘制页面结构，或上传 HTML、网页截图作为视觉参考。两种方式可以同时使用。</p></div>
              <button className="eco-run-button" disabled={Boolean(busy) || gateway.state !== "ready"} onClick={() => void generate()}>
                <Icon icon={Sparkles} size={16} />{busy || "生成设计稿"}
              </button>
            </div>

            <div className="eco-intake-grid">
              <section className="eco-tool-card eco-board-card">
                <div className="eco-tool-card-heading"><div><strong>页面白板</strong><span>拖动组件表达区块位置和大小</span></div><span className="eco-soft-badge">可选</span></div>
                <WhiteboardStage value={wireframe} onChange={setWireframe} embedded />
              </section>

              <aside className="eco-reference-card">
                <div className="eco-tool-card-heading"><div><strong>参考网页</strong><span>上传源文件或完整页面截图</span></div><span className="eco-soft-badge">{references.length} 项</span></div>
                <label className="eco-reference-drop">
                  <Icon icon={Upload} size={20} />
                  <strong>上传 HTML 或截图</strong>
                  <span>HTML、PNG、JPG、WebP</span>
                  <input type="file" multiple accept=".html,.htm,image/png,image/jpeg,image/webp" onChange={(event) => void upload(event.target.files)} />
                </label>
                <div className="eco-reference-items">
                  {references.map((reference) => <article key={reference.referenceId}>
                    <span className="eco-reference-icon"><Icon icon={reference.source.type === "html" ? FileCode2 : Image} size={16} /></span>
                    <div><strong>{reference.source.name}</strong><p>{reference.summary}</p><span>{reference.pages[0]?.nodes.length ?? 0} 个节点 · {reference.interactions.length} 个交互 · {Math.round(reference.confidence * 100)}%</span><div className="eco-color-row">{reference.palette.accent.slice(0, 5).map((color) => <i key={color} title={color} style={{ background: color }} />)}</div></div>
                    <button aria-label={`移除 ${reference.source.name}`} onClick={() => setReferences((items) => items.filter((item) => item.referenceId !== reference.referenceId))}><Icon icon={Trash2} size={14} /></button>
                  </article>)}
                  {!references.length && <div className="eco-reference-empty"><p>尚未上传参考资料</p><span>只输入设计目标或绘制白板也可以生成。</span></div>}
                </div>
                <details className="eco-spec-details" open={showSpecs} onToggle={(event) => setShowSpecs(event.currentTarget.open)}>
                  <summary>解析结果 · ReferenceSpecV1（{allReferences.length}）</summary>
                  <div>{allReferences.map((reference) => <details key={reference.referenceId}><summary>{reference.source.name}</summary><pre>{JSON.stringify(reference, null, 2)}</pre></details>)}</div>
                </details>
                <p className="eco-gateway-address">Gateway：{gatewayAddress()}</p>
              </aside>
            </div>
          </div>
        </div>
      </section>
    </div> : <div className="eco-room eco-visual-room eco-design-room">
      <section className="eco-chat-pane eco-design-sidebar">
        <header className="eco-pane-heading">
          <button className="eco-back-button" onClick={() => { setDesign(null); setPendingPatch(null); }}><Icon icon={ArrowLeft} size={17} />参考输入</button>
          <span className="eco-pane-count">2 / 3</span>
        </header>
        <div className="eco-chat-log eco-design-chat">
          <div className="eco-agent-row">
            <Pebble state="idle" size={25} />
            <div className="eco-ai-bubble"><strong>{design.title}</strong><p>{design.rationale}</p></div>
          </div>
          <section className="eco-reference-influence">
            <strong>参考资料的应用</strong>
            {design.referenceInfluence.length ? design.referenceInfluence.map((item) => <div key={item.referenceId}><span>{item.referenceId}</span><p>{item.borrowedPatterns.join("、") || "未声明借鉴内容"}</p></div>) : <p>AI 未声明参考资料的应用方式。</p>}
          </section>
          <section className="eco-selection-card">
            <div><span>当前选择</span><strong>{selected?.name ?? "请在画布中选择组件"}</strong></div>
            {selected && <>
              <code>{selected.id}</code>
              <div className="eco-color-fields"><label>背景<input type="color" value={selected.style.background?.startsWith("#") ? selected.style.background.slice(0, 7) : "#ffffff"} onChange={(event) => selectedStyle(event.target.value)} /></label><label>文字<input type="color" value={selected.style.color?.startsWith("#") ? selected.style.color.slice(0, 7) : "#1e293b"} onChange={(event) => selectedStyle(undefined, event.target.value)} /></label></div>
              <button className="eco-delete-button" onClick={() => commitPatch(manualPatch(design, selected.id, "删除组件", { operationId: `op.remove.${crypto.randomUUID()}`, action: "remove-node", nodeId: selected.id, expectedParentId: selected.parentId }))}><Icon icon={Trash2} size={14} />删除组件</button>
            </>}
          </section>
          {pendingPatch && <section className="eco-patch-review">
            <span>AI 修改建议</span><strong>{pendingPatch.summary}</strong>
            <details><summary>查看 PatchSpecV1</summary><pre>{JSON.stringify(pendingPatch.operations, null, 2)}</pre></details>
            <div><button onClick={() => commitPatch(pendingPatch)}>接受修改</button><button onClick={() => { setPatches((items) => [{ id: pendingPatch.patchId, patch: pendingPatch, status: "rejected" }, ...items]); setPendingPatch(null); }}>拒绝</button></div>
          </section>}
          <details className="eco-patch-history"><summary>修改记录（{patches.length}）</summary>{patches.map((row) => <div key={row.id}><b>{row.status === "applied" ? "已应用" : "已拒绝"}</b><span>{row.patch.summary}</span><small>{row.patch.operations.map((op) => op.action).join(" · ")}</small></div>)}</details>
          {error && <p className="eco-inline-error" role="alert">{error}</p>}
        </div>
        <footer className="eco-composer eco-ai-composer">
          <textarea value={instruction} onChange={(event) => setInstruction(event.target.value)} rows={3} placeholder={selected ? `请输入对“${selected.name}”的修改意见` : "请输入对页面的修改意见"} />
          <button disabled={Boolean(busy) || !instruction.trim()} onClick={() => void askAI()} aria-label="生成修改建议"><Icon icon={Send} size={16} /></button>
          <p>{busy || "AI 将生成可确认的局部修改"}</p>
        </footer>
      </section>

      <section className="eco-work-pane eco-design-work">
        <nav className="eco-project-journey" aria-label="网页设计进度">
          <span className="eco-project-journey-title">网页设计</span>
          <span className="is-done"><b>✓</b>确认主题</span>
          <span className="is-done"><b>✓</b>生成设计</span>
          <span className="is-current"><b>3</b>修改页面</span>
        </nav>
        <div className="eco-design-toolbar">
          <div><strong>{design.title}</strong><span>版本 {design.revision} · 拖动、缩放或直接编辑文字</span></div>
          <button onClick={() => setShowSpecs((value) => !value)}>{showSpecs ? "关闭设计数据" : "查看设计数据"}</button>
        </div>
        <div className="eco-design-surface">
          <VisualDesignCanvas design={design} selectedId={selectedId} onSelect={setSelectedId} onPatch={commitPatch} />
          {showSpecs && <aside className="eco-design-inspector">
            <header><div><span>设计数据</span><strong>VisualDesignSpecV1</strong></div><button onClick={() => setShowSpecs(false)}>×</button></header>
            <details open><summary>当前设计规范</summary><pre>{JSON.stringify(design, null, 2)}</pre></details>
            <h3>生成记录（{runs.length}）</h3>
            {runs.map((run, index) => <button key={run.id} className={run.design.designId === design.designId && run.design.revision === design.revision ? "is-active" : ""} onClick={() => { setDesign(run.design); setSelectedId(null); }}><strong>{index === 0 ? "当前" : `历史 ${index}`} · {run.design.title}</strong><span>{run.createdAt} · {run.design.theme.palette.accent.join(" / ")}</span><small>{run.references.length} 个参考 · {run.design.pages[0]?.nodes.length ?? 0} 个节点</small></button>)}
          </aside>}
        </div>
      </section>
    </div>}
    {activeDiagnosticId && <DesignDiagnosticScreen diagnostics={designDiagnostics} activeId={activeDiagnosticId} onSelect={setActiveDiagnosticId} onClose={() => setActiveDiagnosticId(null)} />}
  </main>;
}
