import { useCallback, useEffect, useRef, useState } from "react";
import { ArrowLeft, Loader2, Sparkles } from "lucide-react";
import {
  finishQuiz,
  startQuiz,
  type QuizHook,
  type QuizResult,
} from "../../api/interestQuiz";
import {
  CHALLENGE_OPTIONS,
  CHALLENGE_PASS,
  CHALLENGE_PROMPT,
  CHALLENGE_RULE,
  HOOKS,
  MAX_REASON,
  MAX_WORK,
  MISSION_STEPS,
  NAVIGATORS,
  PRINCIPLES,
  PRINCIPLE_REPLY,
  REASON_HINT_AT,
  WORK_EXAMPLES,
  missionIndex,
  type Step,
} from "./content";
import "./quiz.css";

/**
 * 觉醒协议 —— 兴趣测试的七屏。
 *
 * # 它解决的问题
 *
 * 采集器只能从她**做完的事**里长词。一个刚注册的学生什么都还没做完，所以她的
 * 树是空的 —— 而一棵空树说不出「这就是你的模型」。这七屏是冷启动：用五分钟换
 * 第一批词。
 *
 * # 三条它必须守住的线
 *
 * 1. **开一次作答在第一屏就发**（POST），不是交卷时才发。她在第三屏关掉页面，
 *    库里也留着一行 finished_at 为空的作答 —— 那不算「做过了」（邀请还会再
 *    出现），但它记着她走到过这里。过程即数据。
 * 2. **交卷会慢**，因为服务端要同步跑一次采集调用，而结果页要显示的就是那几个
 *    词。所以这里有一个真的加载态，不是一个假的进度条。
 * 3. **它可能长出零个词，而结果页必须照实说。** 她只写了「很帅」，就没有可摘
 *    的原话。编一个「你关心角色的成长弧」挂上去，是她无从反驳的假话
 *    （memory: ai-errors-must-surface-never-fake）。
 *
 * # 为什么它长得和产品其他地方不一样
 *
 * 整个 lite 是暖纸 + 暖夜，这一屏是冷蓝科幻。这是有意的温差：她从外面的系统
 * 走回自己的树。见 quiz.css 顶部。
 */
export function AwakeningQuiz({ onExit }: { onExit: (grew: boolean) => void }) {
  const [step, setStep] = useState<Step>("boot");
  const [attemptId, setAttemptId] = useState("");
  const [principle, setPrinciple] = useState("");
  const [navigator, setNavigator] = useState("");
  const [work, setWork] = useState("");
  const [reason, setReason] = useState("");
  const [hook, setHook] = useState<QuizHook | "">("");
  const [picked, setPicked] = useState("");
  const [wrong, setWrong] = useState<string[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<QuizResult | null>(null);

  const nav = NAVIGATORS.find((n) => n.name === navigator) ?? NAVIGATORS[2]!;

  // 开一次作答。失败不挡她 —— 她照样能做完这七屏，只是这一次不会被记下来。
  // 一个因为后台开不了作答就打不开的兴趣测试，比一个记不全的兴趣测试糟得多。
  const begin = useCallback(() => {
    setStep("world");
    startQuiz()
      .then((a) => setAttemptId(a.id))
      .catch((e: unknown) => {
        setError(e instanceof Error ? e.message : String(e));
      });
  }, []);

  async function submit() {
    setSubmitting(true);
    setError("");
    try {
      const r = await finishQuiz(attemptId, {
        navigator,
        anchorWork: work,
        anchorReason: reason,
        hook,
        challengeChoice: picked,
        challengeAttempts: wrong.length,
      });
      setResult(r);
      setStep("result");
    } catch (e: unknown) {
      // 报错：动词+失败，再接后台原话（AGENTS.md §8）。绝不静默地当成功。
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="awk flex min-h-full flex-col">
      <TopBar step={step} onExit={() => onExit(false)} />

      <div className="flex min-h-0 flex-1 items-center justify-center px-6 pb-10">
        {step === "boot" ? <Boot onNext={begin} /> : null}
        {step === "world" ? (
          <World picked={principle} onPick={setPrinciple} onNext={() => setStep("navigator")} />
        ) : null}
        {step === "navigator" ? (
          <PickNavigator
            picked={navigator}
            onPick={setNavigator}
            onBack={() => setStep("world")}
            onNext={() => setStep("interest")}
          />
        ) : null}
        {step === "interest" ? (
          <Anchor
            nav={nav.name}
            work={work}
            reason={reason}
            onWork={setWork}
            onReason={setReason}
            onBack={() => setStep("navigator")}
            onNext={() => setStep("lens")}
          />
        ) : null}
        {step === "lens" ? (
          <Lens
            nav={nav.name}
            picked={hook}
            onPick={setHook}
            onBack={() => setStep("interest")}
            onNext={() => setStep("challenge")}
          />
        ) : null}
        {step === "challenge" ? (
          <Challenge
            nav={nav.name}
            picked={picked}
            wrong={wrong}
            submitting={submitting}
            error={error}
            onPick={(key, correct) => {
              setPicked(key);
              if (!correct) setWrong((w) => (w.includes(key) ? w : [...w, key]));
            }}
            onBack={() => setStep("lens")}
            onSubmit={submit}
          />
        ) : null}
        {step === "result" && result ? (
          <Result
            result={result}
            navigatorName={navigator}
            onExit={() => onExit(result.keywords.length > 0)}
          />
        ) : null}
      </div>
    </div>
  );
}

/* ── 顶栏 ───────────────────────────────────────────────────────────────── */

function TopBar({ step, onExit }: { step: Step; onExit: () => void }) {
  const mi = missionIndex(step);
  return (
    <header className="flex flex-wrap items-center justify-between gap-3 px-6 pb-4 pt-5">
      <div className="flex items-center gap-3">
        <button type="button" onClick={onExit} className="awk-ghost !px-4 !py-2 text-mk-small">
          <span className="flex items-center gap-1.5">
            <ArrowLeft size={15} strokeWidth={1.8} />
            回到我的树
          </span>
        </button>
        <span className="awk-eyebrow">The Awakening Protocol</span>
      </div>
      <div className="flex items-center gap-3">
        {mi >= 0 ? (
          <span className="awk-meter" aria-label={`任务进度 ${mi + 1} / ${MISSION_STEPS.length}`}>
            {MISSION_STEPS.map((s, i) => (
              <span key={s} data-on={i <= mi} />
            ))}
          </span>
        ) : null}
        <span className="awk-pill">
          <span className="awk-dot" />
          联盟中枢 · 在线
        </span>
      </div>
    </header>
  );
}

/* ── 00 建立连接 ────────────────────────────────────────────────────────── */

function Boot({ onNext }: { onNext: () => void }) {
  return (
    <div className="awk-in max-w-[640px] text-center">
      <div className="awk-orbit mb-8">
        <div className="awk-ring" />
        <div className="awk-ring" />
        <div className="awk-ring" />
        <div className="awk-core" />
      </div>
      <div className="awk-eyebrow">The Awakening Protocol</div>
      <h1 className="mt-2 text-[40px] font-bold leading-tight tracking-tight">觉醒协议</h1>
      <p className="mx-auto mt-4 max-w-[46ch] text-mk-body-lg leading-[1.9] text-[#c6d7e8]">
        2050 年，人类没有被 AI 消灭，却被分成了两种。有人把大脑交给系统，有人选择与 AI
        并肩思考。现在，轮到你留下自己的思维印记。
      </p>
      <button type="button" className="awk-primary mt-8" onClick={onNext}>
        建立神经连接
      </button>
      <p className="mt-4 text-mk-small awk-dim">大约五分钟。做完之后，你的兴趣树上会长出第一批词。</p>
    </div>
  );
}

/* ── 01 世界的暗面 ──────────────────────────────────────────────────────── */

function World({
  picked,
  onPick,
  onNext,
}: {
  picked: string;
  onPick: (k: string) => void;
  onNext: () => void;
}) {
  return (
    <div className="awk-in awk-panel w-full max-w-[760px] p-7">
      <div className="awk-eyebrow">序章 01 / 世界的暗面</div>
      <h2 className="mt-2 text-mk-h1">完成任务，不等于真正成长</h2>
      <p className="mt-3 leading-[1.9] text-[#c6d7e8]">
        90% 的人选择把任务完全交给 AI。他们复制、提交，逐步失去了提问、判断与负责的能力。
        系统称他们为「被托管者」。
      </p>

      <div className="awk-bubble mt-5">
        <div className="awk-eyebrow mb-1.5">联盟中枢</div>
        AI 擅长搜索、整理和初稿；而你必须掌控问题、证据、取舍与最终责任。
        <br />
        检测到新意识接入。请选择你的行动原则：
      </div>

      <div className="mt-4 grid gap-3">
        {PRINCIPLES.map((p, i) => (
          <button
            key={p.key}
            type="button"
            className="awk-card awk-in"
            style={{ ["--i" as string]: i }}
            data-on={picked === p.key}
            onClick={() => onPick(p.key)}
          >
            <span className="flex items-start gap-3">
              <span className="awk-index">{p.index}</span>
              <span>
                <strong className="block">{p.title}</strong>
                <span className="mt-1 block text-mk-small awk-dim">{p.body}</span>
              </span>
            </span>
          </button>
        ))}
      </div>

      {picked ? (
        <p className="awk-in mt-4 border-l-2 pl-3 leading-[1.85] text-[#c6d7e8]" style={{ borderColor: "var(--amber)" }}>
          {PRINCIPLE_REPLY[picked]}
        </p>
      ) : null}

      <div className="mt-6 flex justify-end">
        <button type="button" className="awk-primary" disabled={!picked} onClick={onNext}>
          继续
        </button>
      </div>
    </div>
  );
}

/* ── 02 选导航员 ────────────────────────────────────────────────────────── */

function PickNavigator({
  picked,
  onPick,
  onBack,
  onNext,
}: {
  picked: string;
  onPick: (n: string) => void;
  onBack: () => void;
  onNext: () => void;
}) {
  return (
    <div className="awk-in w-full max-w-[980px]">
      <div className="text-center">
        <div className="awk-eyebrow">Neural Companion Setup</div>
        <h2 className="mt-2 text-mk-h1">唤醒你的专属 AI 导航员</h2>
        <p className="mx-auto mt-2 max-w-[52ch] leading-[1.85] awk-dim">
          它不会替你走路，只会用不同方式递出线索。请选择最能让你保持思考状态的搭档。
        </p>
      </div>

      <div className="mt-6 grid gap-4 md:grid-cols-3">
        {NAVIGATORS.map((n, i) => (
          <button
            key={n.name}
            type="button"
            className="awk-card awk-in !p-5"
            style={{ ["--i" as string]: i, borderColor: picked === n.name ? n.accent : undefined }}
            data-on={picked === n.name}
            onClick={() => onPick(n.name)}
          >
            <span
              className="mb-3 block h-1 w-10 rounded-full"
              style={{ background: n.accent, boxShadow: `0 0 12px ${n.accent}` }}
            />
            <span className="awk-eyebrow" style={{ color: n.accent }}>
              {n.label}
            </span>
            <strong className="mt-1.5 block text-mk-h3">
              {n.name} · {n.codename}
            </strong>
            <span className="mt-2 block text-mk-small leading-[1.8] awk-dim">{n.body}</span>
            <span className="mt-3 block border-l-2 pl-2.5 text-mk-small italic leading-[1.75] text-[#c6d7e8]"
                  style={{ borderColor: n.accent }}>
              「{n.quote}」
            </span>
          </button>
        ))}
      </div>

      <div className="mt-6 flex items-center justify-between">
        <button type="button" className="awk-ghost" onClick={onBack}>
          返回
        </button>
        <button type="button" className="awk-primary" disabled={!picked} onClick={onNext}>
          确认连接
        </button>
      </div>
    </div>
  );
}

/* ── 03 兴趣锚点 ────────────────────────────────────────────────────────── */

function Anchor({
  nav,
  work,
  reason,
  onWork,
  onReason,
  onBack,
  onNext,
}: {
  nav: string;
  work: string;
  reason: string;
  onWork: (v: string) => void;
  onReason: (v: string) => void;
  onBack: () => void;
  onNext: () => void;
}) {
  const ref = useRef<HTMLInputElement>(null);
  useEffect(() => ref.current?.focus(), []);
  const thin = [...reason.trim()].length < REASON_HINT_AT;

  return (
    <div className="awk-in awk-panel w-full max-w-[760px] p-7">
      <div className="flex items-center justify-between">
        <div>
          <div className="awk-eyebrow">Mission 01</div>
          <h2 className="mt-1 text-mk-h1">破冰契约</h2>
        </div>
      </div>

      <div className="awk-bubble mt-4">
        <strong>{nav}</strong>：终于连接成功了。先对个暗号吧——你最喜欢的一部作品，或最喜欢的角色是谁？
        没有标准答案，我只是想找到你的兴趣锚点。
      </div>

      <label className="awk-eyebrow mt-5 block" htmlFor="awk-work">
        作品或角色
      </label>
      <input
        id="awk-work"
        ref={ref}
        className="awk-input mt-2"
        maxLength={MAX_WORK}
        value={work}
        onChange={(e) => onWork(e.target.value)}
        placeholder="例如：《进击的巨人》里的利威尔"
      />
      <div className="mt-2.5 flex flex-wrap gap-2">
        {WORK_EXAMPLES.map((w) => (
          <button
            key={w}
            type="button"
            onClick={() => onWork(w)}
            className="rounded-full border px-3 py-1 text-mk-small awk-dim transition-colors hover:text-[#e8f3ff]"
            style={{ borderColor: "var(--line)" }}
          >
            {w}
          </button>
        ))}
      </div>

      <label className="awk-eyebrow mt-5 block" htmlFor="awk-reason">
        它最吸引你的地方
      </label>
      <textarea
        id="awk-reason"
        className="awk-textarea mt-2"
        maxLength={MAX_REASON}
        value={reason}
        onChange={(e) => onReason(e.target.value)}
        placeholder="写下一个具体画面、行动、情节或感受……"
      />
      <div className="mt-2 flex items-start justify-between gap-4">
        {/* 这不是一道门 —— 她写多短都能继续。它只是说清楚这一栏为什么值得多写
            两句：树上的词就是从这段话里摘出来的。 */}
        <span className="text-mk-small leading-[1.75] awk-dim">
          {thin
            ? "具体的细节比「很帅」「很好看」更能找到你真正的兴趣。写太短的话，这次可能一个词都长不出来。"
            : "很好。树上的关键词会从这段话里摘出来，并且原样引用你写的句子。"}
        </span>
        <span className="shrink-0 font-mono text-mk-small awk-dim">
          {[...reason].length} / {MAX_REASON}
        </span>
      </div>

      <div className="mt-6 flex items-center justify-between">
        <button type="button" className="awk-ghost" onClick={onBack}>
          返回
        </button>
        <button type="button" className="awk-primary" disabled={!work.trim()} onClick={onNext}>
          提交兴趣信号
        </button>
      </div>
    </div>
  );
}

/* ── 04 兴趣钩子 ────────────────────────────────────────────────────────── */

function Lens({
  nav,
  picked,
  onPick,
  onBack,
  onNext,
}: {
  nav: string;
  picked: QuizHook | "";
  onPick: (h: QuizHook) => void;
  onBack: () => void;
  onNext: () => void;
}) {
  return (
    <div className="awk-in awk-panel w-full max-w-[760px] p-7">
      <div className="awk-eyebrow">Mission 02</div>
      <h2 className="mt-1 text-mk-h1">选择兴趣钩子</h2>

      <div className="awk-bubble mt-4">
        <strong>{nav}</strong>：你给了我一个不错的锚点。但「喜欢」还不够——真正重要的是，
        哪一种问题会让你愿意继续追下去？
      </div>

      <div className="mt-4 grid gap-3">
        {HOOKS.map((h, i) => (
          <button
            key={h.key}
            type="button"
            className="awk-card awk-in"
            style={{ ["--i" as string]: i }}
            data-on={picked === h.key}
            onClick={() => onPick(h.key)}
          >
            <span className="flex items-start gap-3">
              <span className="awk-index">{h.index}</span>
              <span>
                <strong className="block">{h.question}</strong>
                <span className="mt-1 block text-mk-small awk-dim">{h.body}</span>
              </span>
            </span>
          </button>
        ))}
      </div>

      <p className="mt-4 text-mk-small awk-dim">选择当前最想追问的一条，不是给自己永久定型。</p>

      <div className="mt-5 flex items-center justify-between">
        <button type="button" className="awk-ghost" onClick={onBack}>
          返回
        </button>
        <button type="button" className="awk-primary" disabled={!picked} onClick={onNext}>
          装备学科透镜
        </button>
      </div>
    </div>
  );
}

/* ── 05 反方压力测试 ────────────────────────────────────────────────────── */

function Challenge({
  nav,
  picked,
  wrong,
  submitting,
  error,
  onPick,
  onBack,
  onSubmit,
}: {
  nav: string;
  picked: string;
  wrong: string[];
  submitting: boolean;
  error: string;
  onPick: (key: string, correct: boolean) => void;
  onBack: () => void;
  onSubmit: () => void;
}) {
  const chosen = CHALLENGE_OPTIONS.find((o) => o.key === picked);
  const passed = chosen?.correct === true;

  return (
    <div className="awk-in awk-panel w-full max-w-[760px] p-7">
      <div className="awk-eyebrow">Mission 03</div>
      <h2 className="mt-1 text-mk-h1">反方压力测试</h2>

      <div className="awk-bubble mt-4">
        <strong>{nav}</strong>：{CHALLENGE_PROMPT}
      </div>

      <div className="awk-soft mt-4 p-4">
        <div className="awk-eyebrow" style={{ color: "var(--amber)" }}>
          联盟提示 · 主张需要证据
        </div>
        <p className="mt-1.5 text-mk-small leading-[1.85] text-[#c6d7e8]">{CHALLENGE_RULE}</p>
      </div>

      <div className="mt-4 grid gap-3">
        {CHALLENGE_OPTIONS.map((o, i) => (
          <button
            key={o.key}
            type="button"
            className="awk-card awk-in"
            style={{ ["--i" as string]: i }}
            data-on={picked === o.key && o.correct}
            data-wrong={wrong.includes(o.key)}
            onClick={() => onPick(o.key, o.correct)}
          >
            <span className="flex items-start gap-3">
              <span className="awk-index">{o.index}</span>
              <span>
                <strong className="block">{o.title}</strong>
                <span className="mt-1 block text-mk-small awk-dim">{o.body}</span>
              </span>
            </span>
          </button>
        ))}
      </div>

      {chosen ? (
        <p
          className="awk-in mt-4 border-l-2 pl-3 leading-[1.85] text-[#c6d7e8]"
          style={{ borderColor: passed ? "var(--green)" : "var(--danger)" }}
        >
          {passed ? CHALLENGE_PASS : chosen.feedback}
        </p>
      ) : null}

      {/* 报错：动词+失败，再接后台原话。绝不把一次失败的交卷显示成成功。 */}
      {error ? (
        <p className="mt-4 rounded-lg p-3 text-mk-small leading-[1.8]"
           style={{ background: "rgba(255,113,137,.12)", border: "1px solid rgba(255,113,137,.4)" }}>
          提交失败：{error}
        </p>
      ) : null}

      <div className="mt-6 flex items-center justify-between">
        <button type="button" className="awk-ghost" onClick={onBack} disabled={submitting}>
          返回
        </button>
        <button type="button" className="awk-primary" disabled={!passed || submitting} onClick={onSubmit}>
          {submitting ? (
            <span className="flex items-center gap-2">
              <Loader2 size={16} className="animate-spin" />
              正在生成你的兴趣画像
            </span>
          ) : (
            "完成压力测试"
          )}
        </button>
      </div>
      {wrong.length > 0 && !passed ? (
        <p className="mt-3 text-right text-mk-small awk-dim">
          换一条再试。答错不扣分，这一步记录的是你的思考过程。
        </p>
      ) : null}
    </div>
  );
}

/* ── 06 结果 ────────────────────────────────────────────────────────────── */

function Result({
  result,
  navigatorName,
  onExit,
}: {
  result: QuizResult;
  navigatorName: string;
  onExit: () => void;
}) {
  const grew = result.keywords.length > 0;

  return (
    <div className="awk-in w-full max-w-[900px] py-4">
      <div className="text-center">
        <div className="awk-eyebrow">Awakener Identified</div>
        <h2 className="mt-2 text-[32px] font-bold">你的第一枚思维印记</h2>
        <p className="mx-auto mt-3 max-w-[54ch] leading-[1.9] text-[#c6d7e8]">
          你没有把兴趣交给算法定义，而是自己走完了从兴趣锚点、学科透镜到证据检验的第一次探索。
        </p>
      </div>

      <div className="awk-panel mt-6 p-6">
        <div className="grid gap-3 sm:grid-cols-3">
          <Fact label="AI 导航员" value={navigatorName || "—"} />
          <Fact label="兴趣锚点" value={result.attempt.anchorWork || "—"} />
          {/* 标签是名词（AGENTS.md 界面文案 §1），而这一格报的是**过程**不是
              结论：她试了几次才把「我喜欢」变成一个可检验的说法。答错不扣分，
              但也不隐藏 —— 摩擦是信号（铁律④）。 */}
          <Fact
            label="压力测试"
            value={
              result.attempt.challengeAttempts === 0
                ? "一次命中"
                : `第 ${result.attempt.challengeAttempts + 1} 次命中`
            }
          />
        </div>
      </div>

      {/* ── 学科透镜 ── 这一节是原型给不出的东西：真学科 + 考纲投影。 */}
      {result.lenses.length > 0 ? (
        <div className="mt-6">
          <div className="awk-eyebrow">Discipline Lenses</div>
          <h3 className="mt-1 text-mk-h2">你装上的学科透镜</h3>
          <p className="mt-1.5 text-mk-small awk-dim">
            把朴素的「喜欢」，连接到可以研究、验证和创造的知识方法。
          </p>
          <div className="mt-4 grid gap-3 md:grid-cols-3">
            {result.lenses.map((l, i) => (
              <div key={l.id} className="awk-soft awk-in p-4" style={{ ["--i" as string]: i }}>
                <strong className="block text-mk-h3">{l.zh}</strong>
                <span className="mt-0.5 block font-mono text-mk-small awk-dim">{l.en}</span>
                <p className="mt-2.5 text-mk-small leading-[1.8] text-[#c6d7e8]">{l.asks}</p>
                <p className="mt-2 text-mk-small leading-[1.75] awk-dim">{l.method}</p>
                {l.syllabus.length > 0 ? (
                  <div className="mt-3 flex flex-wrap gap-1.5">
                    {l.syllabus.slice(0, 3).map((s) => (
                      <span
                        key={s.code}
                        className="rounded-full px-2 py-0.5 font-mono text-[10px]"
                        style={{ border: "1px solid var(--line)", color: "var(--amber)" }}
                        title={s.label}
                      >
                        {s.board}
                        {s.level ? ` ${s.level}` : ""}
                      </span>
                    ))}
                  </div>
                ) : null}
              </div>
            ))}
          </div>
        </div>
      ) : null}

      {/* ── 树上长出来的词 ── */}
      <div className="mt-7">
        <div className="awk-eyebrow">Planted</div>
        <h3 className="mt-1 text-mk-h2">这次种到你树上的词</h3>

        {grew ? (
          <div className="mt-4 grid gap-3">
            {result.keywords.map((k, i) => (
              <div key={k.textZh} className="awk-soft awk-in p-4" style={{ ["--i" as string]: i }}>
                <div className="flex items-center gap-2">
                  <Sparkles size={15} strokeWidth={2} color="var(--amber)" />
                  <strong className="text-mk-h3">{k.textZh}</strong>
                  <span className="font-mono text-mk-small awk-dim">{k.textEn}</span>
                </div>
                {k.note ? (
                  <p className="mt-2 leading-[1.85] text-[#c6d7e8]">{k.note}</p>
                ) : null}
                <p
                  className="mt-2.5 border-l-2 pl-3 text-mk-small italic leading-[1.8] awk-dim"
                  style={{ borderColor: "var(--cyan)" }}
                >
                  「{k.evidence}」
                  <span className="mt-1 block not-italic">你自己写的</span>
                </p>
              </div>
            ))}
          </div>
        ) : (
          // 🚨 两种「没长出词」，说的话不一样。绝不编一个像样的词填进来。
          <div className="awk-soft mt-4 p-5">
            <p className="leading-[1.9] text-[#c6d7e8]">
              {result.harvested
                ? "这次没有长出关键词——你写的那段话里，还没有能作为根据的原话。"
                : "这次没有采集：你写的理由太短，里面还没有能摘出来的句子。"}
            </p>
            <p className="mt-2 text-mk-small leading-[1.85] awk-dim">
              可以再做一次，在「它最吸引你的地方」里写一个具体的画面或情节。重做是往树上再加几个词，
              不会清空已有的。
            </p>
          </div>
        )}
      </div>

      <div className="mt-8 flex items-center justify-center">
        <button type="button" className="awk-primary" onClick={onExit}>
          {grew ? "回到我的树，看看它长成什么样" : "回到我的树"}
        </button>
      </div>
    </div>
  );
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="awk-soft p-3.5">
      <div className="awk-eyebrow">{label}</div>
      <strong className="mt-1 block text-mk-body-lg">{value}</strong>
    </div>
  );
}
