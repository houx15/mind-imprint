import { useState } from "react";
import { AlertTriangle, Check, Copy, Link2, RotateCcw, Send, Trash2, Users } from "lucide-react";
import { useEco } from "../../store";
import type { ArtifactSpec, FormQuestion, Project } from "../../data/types";
import { Bold, Btn, Field, Sys, cx } from "../../ui";

/**
 * 问卷 — 印记 drafts it, she fixes it, and it goes to real people.
 *
 * ## Why this step is worth its weight
 * Everything else in a project can be undone. A questionnaire cannot: once it
 * has gone out, a defective question comes back as defective data, and no
 * amount of care afterwards recovers it. That makes it the one place where
 * *fix it before you send it* is not a lesson about diligence — it is the only
 * option. The panel says so, in `payoff`, above the work.
 *
 * ## The move that does the teaching
 * 🚨 **印记 marks its own bad questions.** It writes five and then says: *this
 * one asks two things at once; this one has already told the reader which
 * answer I want.* A draft that arrives pre-flagged converts the review from
 * politeness into work — she cannot hand it back with 「挺好的」 — and it
 * teaches the named failure modes (see `METHODS`, from Pew's question-wording
 * research) on questions she is about to send to people she knows.
 *
 * She cannot send until every flagged question is either rewritten or cut.
 * That is the only hard gate in this file, and it is the right one.
 *
 * ## The invitation is hers
 * 印记 drafts one and says outright that it should not go as written. A
 * message in the AI's voice, under her name, to her own neighbours, is the one
 * place in this project where 印记 writing for her would be a lie about who is
 * asking. So the draft is shown greyed, as an example of what not to send.
 */
export function FormSurface({ project, spec }: { project: Project; spec: ArtifactSpec }) {
  const { editFormQ, cutFormQ, setInvite, sendForm, settleArtifact } = useEco();
  const st = project.artifacts[spec.id];
  const form = spec.form;
  const mine = st?.form;
  const [read, setRead] = useState("");

  if (!form) return null;

  const cut = mine?.cut ?? [];
  const edits = mine?.edits ?? {};
  const kept = form.questions.filter((q) => !cut.includes(q.id));
  // A flagged question is handled when she has cut it or actually changed the
  // wording. Retyping it identically does not count.
  const unhandled = form.questions.filter(
    (q) => q.flag && !cut.includes(q.id) && (edits[q.id] ?? q.q).trim() === q.q.trim(),
  );
  const invite = mine?.invite ?? "";
  const canSend = unhandled.length === 0 && invite.trim().length >= 8 && kept.length >= 2;
  const sent = mine?.sent ?? false;

  if (sent) {
    return (
      <Replies project={project} spec={spec} read={read} setRead={setRead} onSettle={settleArtifact} />
    );
  }

  return (
    <>
      <p className="max-w-[66ch] whitespace-pre-wrap text-mk-body-lg leading-[1.9] text-mk-ink">
        <Bold text={spec.note} />
      </p>

      {/* ── who it goes to. Named before the questions, because who you are
             asking changes which questions are worth asking. ────────────── */}
      <div className="mt-4 flex items-center gap-2 text-mk-body text-mk-secondary">
        <Users size={15} strokeWidth={1.9} className="text-mk-muted" />
        发给：<span className="text-mk-ink">{form.audience}</span>
      </div>

      {/* ── the questions ─────────────────────────────────────────────── */}
      <Sys className="mb-2 mt-5 block">
        问卷草稿 · {kept.length} 道题
        {unhandled.length > 0 ? (
          <span className="ml-2 font-semibold text-mk-accent-700">
            还有 {unhandled.length} 道我标的题没处理
          </span>
        ) : null}
      </Sys>

      <ol className="space-y-3">
        {form.questions.map((q, i) => (
          <li key={q.id}>
            <Question
              q={q}
              index={i}
              value={edits[q.id] ?? q.q}
              cut={cut.includes(q.id)}
              touched={(edits[q.id] ?? q.q).trim() !== q.q.trim()}
              onEdit={(v) => editFormQ(project.id, spec.id, q.id, v)}
              onCut={() => cutFormQ(project.id, spec.id, q.id)}
            />
          </li>
        ))}
      </ol>

      {/* ── the invitation ────────────────────────────────────────────── */}
      <div className="mt-6 rounded-mk-lg border border-mk-border bg-mk-surface p-5">
        <Sys>邀请语 · 这段必须你写</Sys>
        <p className="mt-1.5 max-w-[64ch] text-mk-body leading-[1.85] text-mk-secondary">
          这条消息会用你的名字发进业主群，读它的人认识你。
          <span className="text-mk-ink">所以它不能是我写的。</span>
          下面是我拟的一版——它读起来像一封群发通知，你可以拿它当反例。
        </p>

        <div
          className="mt-3 rounded-mk-md border border-dashed p-3.5 text-mk-body leading-[1.85]"
          style={{ borderColor: "var(--mk-border)", color: "var(--mk-faint)" }}
        >
          <span className="eco-mono mb-1 block">印记拟的（别原样发）</span>
          {form.inviteDraft}
        </div>

        <div className="mt-3.5">
          <Field
            label="你自己写一句"
            hint="说清楚你是谁、你在做什么、要占他们多久。两三句就够。"
            value={invite}
            onChange={(v) => setInvite(project.id, spec.id, v)}
            rows={3}
            placeholder="例如：大家好，我是 8 号楼的知瑶，初二。我在做一个花园里的指路地图，想先搞清楚大家一般在哪儿会迷路。三个问题，一分钟。"
          />
        </div>
      </div>

      {/* ── send ──────────────────────────────────────────────────────── */}
      <div
        className="mt-5 rounded-mk-lg border p-5"
        style={{ borderColor: "var(--mk-accent-200)", background: "var(--mk-accent-50)" }}
      >
        <div className="flex items-center gap-2">
          <Link2 size={15} strokeWidth={1.9} className="text-mk-accent-700" />
          <span className="font-mono text-mk-small text-mk-secondary">
            mind.uni-robot.cn/f/{spec.id}
          </span>
          <button
            type="button"
            className="rounded-mk-sm p-1 text-mk-faint transition-colors hover:text-mk-ink
                       focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
            title="复制链接（原型：这个链接是假的）"
          >
            <Copy size={13} strokeWidth={1.9} />
          </button>
          <span
            className="eco-mono rounded-mk-full px-2 py-0.5 text-mk-faint"
            style={{ border: "1px solid var(--mk-border)" }}
          >
            原型数据
          </span>
        </div>

        <Btn
          className="mt-3.5"
          disabled={!canSend}
          iconStart={<Send size={16} strokeWidth={2} />}
          onClick={() => sendForm(project.id, spec.id)}
        >
          发出去
        </Btn>
        {!canSend ? (
          <p className="mt-2 text-mk-small leading-[1.75] text-mk-muted">
            {unhandled.length > 0
              ? `我标出来的那 ${unhandled.length} 道题，改掉或者删掉才能发。发出去就收不回来了。`
              : invite.trim().length < 8
                ? "还差你自己写的那句邀请。"
                : "至少留两道题。"}
          </p>
        ) : null}
      </div>
    </>
  );
}

function Question({
  q,
  index,
  value,
  cut,
  touched,
  onEdit,
  onCut,
}: {
  q: FormQuestion;
  index: number;
  value: string;
  cut: boolean;
  touched: boolean;
  onEdit: (v: string) => void;
  onCut: () => void;
}) {
  const bad = Boolean(q.flag) && !cut && !touched;
  return (
    <div
      className="rounded-mk-lg border p-4 transition-colors duration-[140ms]"
      style={{
        borderColor: cut
          ? "var(--mk-border)"
          : bad
            ? "var(--mk-accent)"
            : touched
              ? "var(--mk-success)"
              : "var(--mk-border)",
        background: cut ? "transparent" : "var(--mk-surface)",
        opacity: cut ? 0.5 : 1,
      }}
    >
      <div className="mb-2 flex items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
          <span className="eco-mono text-mk-faint">Q{index + 1}</span>
          <span className="eco-mono text-mk-muted">
            {q.kind === "choice" ? "选择题" : q.kind === "scale" ? "打分" : "填空"}
          </span>
          {cut ? (
            <span className="eco-mono text-mk-faint" style={{ letterSpacing: 0 }}>
              你删掉了
            </span>
          ) : touched ? (
            <span className="eco-mono text-mk-success" style={{ letterSpacing: 0 }}>
              你改过
            </span>
          ) : null}
        </div>
        <button
          type="button"
          onClick={onCut}
          aria-label={cut ? "加回来" : "删掉这道题"}
          title={cut ? "加回来" : "删掉这道题"}
          className="flex h-7 w-7 items-center justify-center rounded-mk-sm text-mk-faint
                     transition-colors hover:bg-mk-paper hover:text-mk-ink
                     focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
        >
          {cut ? <RotateCcw size={14} strokeWidth={1.9} /> : <Trash2 size={14} strokeWidth={1.9} />}
        </button>
      </div>

      <textarea
        value={value}
        rows={2}
        disabled={cut}
        onChange={(e) => onEdit(e.target.value)}
        className={cx(
          "w-full resize-y rounded-mk-md border bg-mk-paper p-3 text-mk-prose text-mk-ink",
          "outline-none transition-colors duration-[120ms] focus:border-mk-accent-300",
          "focus:ring-2 focus:ring-mk-accent-100 disabled:opacity-60",
          "border-mk-input-border",
        )}
      />

      {q.options && !cut ? (
        <div className="mt-2 flex flex-wrap gap-1.5">
          {q.options.map((o) => (
            <span
              key={o}
              className="rounded-mk-full border border-mk-border px-2.5 py-0.5 text-mk-small text-mk-muted"
            >
              {o}
            </span>
          ))}
        </div>
      ) : null}

      {/* 印记 marking its own draft. This is the whole lesson. */}
      {q.flag && !cut ? (
        <div
          className="mt-3 rounded-mk-md border-l-2 p-3"
          style={{
            borderColor: touched ? "var(--mk-success)" : "var(--mk-accent)",
            background: "var(--mk-paper)",
          }}
        >
          <div className="flex items-center gap-1.5">
            <AlertTriangle
              size={13}
              strokeWidth={2}
              color={touched ? "var(--mk-success)" : "var(--mk-accent)"}
            />
            <Sys>{touched ? "你已经处理了" : `我自己标的 · ${q.fault ?? "有毛病"}`}</Sys>
          </div>
          <p className="mt-1.5 text-mk-small leading-[1.8] text-mk-secondary">{q.flag}</p>
        </div>
      ) : null}
    </div>
  );
}

/* ── what came back ───────────────────────────────────────────────────── */

function Replies({
  project,
  spec,
  read,
  setRead,
  onSettle,
}: {
  project: Project;
  spec: ArtifactSpec;
  read: string;
  setRead: (v: string) => void;
  onSettle: (projectId: string, artifactId: string, summary: string) => void;
}) {
  const form = spec.form!;
  const cut = project.artifacts[spec.id]?.form?.cut ?? [];
  const edits = project.artifacts[spec.id]?.form?.edits ?? {};
  const label = (qId: string) =>
    edits[qId] ?? form.questions.find((q) => q.id === qId)?.q ?? qId;

  return (
    <>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <Sys>回来了 · {form.replies.length} 份</Sys>
        <span
          className="eco-mono rounded-mk-full px-2 py-0.5 text-mk-faint"
          style={{ border: "1px solid var(--mk-border)" }}
          title="这些回答是为原型写的示例内容"
        >
          原型数据
        </span>
      </div>
      <p className="mt-2 max-w-[66ch] text-mk-body-lg leading-[1.9] text-mk-ink">
        你发出去的那 {form.questions.length - cut.length} 道题，收回来
        {form.replies.length} 份。下面是原话，我一个字没动。
      </p>

      {/* 🚨 The mock replies only cover three of the five questions, and a
          student who counts will notice. Saying so is cheaper than letting her
          wonder whether the product dropped her answers — and it is the same
          rule the news items follow. */}
      <p className="mt-2 text-mk-small leading-[1.75] text-mk-faint">
        原型说明：这些回答是示例内容，只覆盖了其中三道题。真实的问卷每道题都会有回答。
      </p>

      <ul className="mt-4 space-y-3">
        {form.replies.map((r) => (
          <li key={r.id} className="rounded-mk-lg border border-mk-border bg-mk-surface p-4">
            <Sys>{r.who}</Sys>
            <dl className="mt-2 space-y-2">
              {r.answers
                .filter((a) => !cut.includes(a.qId))
                .map((a) => (
                  <div key={a.qId}>
                    <dt className="text-mk-small text-mk-muted">{label(a.qId)}</dt>
                    <dd className="mt-0.5 text-mk-body leading-[1.8] text-mk-ink">{a.a}</dd>
                  </div>
                ))}
            </dl>
          </li>
        ))}
      </ul>

      {/* 印记 reports counts and surprises. The reading of them is hers. */}
      <div
        className="mt-5 rounded-mk-lg border p-4"
        style={{ borderColor: "var(--mk-butter)", background: "var(--mk-butter-bg)" }}
      >
        <Sys className="!text-[#8A6320]">我数出来的</Sys>
        <ul className="mt-2 space-y-2">
          {form.readout.map((line) => (
            <li key={line} className="flex gap-2 text-mk-body leading-[1.85] text-[#6B4D14]">
              <span>·</span>
              <Bold text={line} />
            </li>
          ))}
        </ul>
      </div>

      <div className="mt-5 rounded-mk-lg border border-mk-border bg-mk-surface p-5">
        <p className="text-mk-body-lg leading-[1.9] text-mk-ink">
          我只会数。<span className="font-semibold">这些答案说明了什么，是你的活。</span>
        </p>
        <div className="mt-3.5">
          <Field
            label="你从这些回答里看到了什么"
            hint="写你打算拿它做什么。哪一条改变了你原来的想法，也写下来。"
            value={read}
            onChange={setRead}
            rows={4}
            placeholder="例如：三个人都提到东三门第二个岔路口，和我标的一样，那个点必须有。门卫说的「走到中心亭又回来问第二遍」是我没想到的——中间那一段也得有点。四份太少，不能写成比例。"
          />
        </div>
        <Btn
          className="mt-4"
          disabled={read.trim().length < 10}
          iconStart={<Check size={16} strokeWidth={2.4} />}
          onClick={() =>
            onSettle(
              project.id,
              spec.id,
              `收下了。你写的是「${read.trim().slice(0, 44)}」。\n\n这份判断是你的，不是我的——我只报了数字。接下来定点数的时候，我会按你这段来算。`,
            )
          }
        >
          我读完了，继续
        </Btn>
        {read.trim().length < 10 ? (
          <p className="mt-2 text-mk-small text-mk-muted">
            收了数据不读，等于没收。写一句再往下走。
          </p>
        ) : null}
      </div>
    </>
  );
}
