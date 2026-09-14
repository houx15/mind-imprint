import { studentArtwork } from "../../../learning/StudentArtwork";
import { apiErrorText } from "../../../api/errorText";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Plus, Timer, Combine } from "lucide-react";
import { Icon } from "@/ui";
import {
  archiveNote,
  clusterNotes,
  createNotes,
  listNotes,
  moveNote,
  pickIdea,
  type Note,
} from "../../../api/notes";
import { Stage } from "../board/Stage";
import { Sticky } from "../board/Sticky";
import { GroupLasso } from "../board/sketch";
import { useCanvasDrag } from "../board/useCanvasDrag";
import { NOTE_H, NOTE_W, boardSpot } from "../boardLayout";
import { DONE, tone, toneAt } from "../../../shared/tone";
import type { ToolSurfaceProps } from "../registry";

/**
 * Ideas —— 想法尽量多。一块自由的画布。
 *
 * 🚨 产品负责人 2026-09-03 的第二张图：黑板上一堆便签，两个手画的圈各贴着一条
 * 胶带（「减少拿多」「让剩下的有去处」），底下一行小字「拖到一起，就能合并」，
 * 右上角一个倒计时。上一版是一列可以打勾的行——同样能挑、能合并，但**没有一个
 * 动作是用手做的**，而这一步教的恰恰是"先把东西摊开，再看它们之间的关系"。
 *
 * 摊开这件事有物理性：一条办法放在哪儿、和哪几条挨着，是她在能说出"这两个其实
 * 是一回事"之前就已经做出的判断。所以位置落库（Board.tsx 的那条注释同理），
 * 圈出来的堆也落库（cluster）。
 *
 * 规矩仍然只有一条：想满四个才让挑。急着挑第一个想到的，是这一步最常见的失手。
 */

const ENOUGH = 4;
/** 一轮发散给三分钟。倒计时不拦任何事，它只是让"还在发散"这件事看得见。 */
const ROUND_SECONDS = 180;
/** 宽屏画布一行放得下几张。窄的那一档由 boardSpot 的默认值管。 */
const PER_ROW = 5;
const CANVAS_MIN_H = 340;

function mmss(total: number): string {
  const m = Math.floor(total / 60);
  const s = total % 60;
  return `${m}:${String(s).padStart(2, "0")}`;
}

export function Ideas({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [ideas, setIdeas] = useState<Note[]>([]);
  const [draft, setDraft] = useState("");
  const [picked, setPicked] = useState<string | null>(null);
  const [why, setWhy] = useState("");
  /** 点中的几张，用来圈成一堆。 */
  const [chosen, setChosen] = useState<string[]>([]);
  /** 拖到一起之后等她点头的那一对。 */
  const [pair, setPair] = useState<{ a: string; b: string } | null>(null);
  const [naming, setNaming] = useState(false);
  const [groupName, setGroupName] = useState("");
  const [left, setLeft] = useState(ROUND_SECONDS);
  const [error, setError] = useState<string | null>(null);
  const boardRef = useRef<HTMLDivElement>(null);
  // 🚨 座位号用 ref 同步地取。她连着敲三次回车，三次 add 拿到的是同一个
  // ideas.length，三张纸会摞在同一个位置上——Board.tsx 在 e2e 里撞出来过。
  const seatRef = useRef(0);

  useEffect(() => {
    if (left <= 0) return;
    const t = window.setInterval(() => setLeft((s) => Math.max(0, s - 1)), 1000);
    return () => window.clearInterval(t);
  }, [left]);

  const reload = useCallback(async () => {
    const all = await listNotes(projectId);
    const mine = all.filter((n) => n.kind === "idea");
    seatRef.current = Math.max(seatRef.current, mine.length);
    // 🚨 老数据没有位置（上一版是一列清单，从没写过 x/y）。全是 0 的话它们会
    // 叠成一张纸，看起来像"只想出了一个办法"。给它们排一次座位并落库，之后
    // 就是她自己的位置了。
    const homeless = mine.filter((n) => n.x === 0 && n.y === 0);
    if (homeless.length) {
      const placed = await Promise.all(
        homeless.map((n, i) => {
          const spot = boardSpot(i, PER_ROW);
          return moveNote(projectId, n.id, spot.x, spot.y);
        }),
      );
      const by = new Map(placed.map((n) => [n.id, n]));
      setIdeas(mine.map((n) => by.get(n.id) ?? n));
      return;
    }
    setIdeas(mine);
  }, [projectId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const canvasH = useMemo(
    () => ideas.reduce((h, n) => Math.max(h, n.y + NOTE_H + 16), CANVAS_MIN_H),
    [ideas],
  );

  async function add() {
    const body = draft.trim();
    if (!body) return;
    setDraft("");
    try {
      const seat = seatRef.current++;
      const [made] = await createNotes(projectId, [{ kind: "idea", body }]);
      if (!made) return;
      const spot = boardSpot(seat, PER_ROW);
      const placed = await moveNote(projectId, made.id, spot.x, spot.y);
      setIdeas((prev) => [...prev, placed]);
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  const drag = useCanvasDrag({
    boardRef,
    width: NOTE_W,
    height: NOTE_H,
    boardHeight: canvasH,
    onPreview: (id, x, y) => setIdeas((prev) => prev.map((n) => (n.id === id ? { ...n, x, y } : n))),
    onDrop: (id, x, y) => {
      // 🚨 dragged=true 只在这一处传：她真的用手把这张纸挪到了那儿。排座位和
      // 补位置走的是同一个函数，但那两次不是她的判断。
      void moveNote(projectId, id, x, y, true).catch((err) => setError(apiErrorText(err)));
    },
    // 🚨 拖到一起不当场合并，先问一句。合并会归档两张纸——一次手滑就没了，
    // 而她刚刚想出来的东西是这块板上最不该弄丢的。
    //
    // 🚨 同时把这张纸放回原位。不放回去它就停在目标那张底下被完全盖住，
    // 她按了「取消」之后屏幕上少一条办法——看起来像刚才那一条被合掉了。
    onDropOn: (a, b, from) => {
      setIdeas((prev) => prev.map((n) => (n.id === a ? { ...n, x: from.x, y: from.y } : n)));
      setPair({ a, b });
    },
    onTap: (id) =>
      setChosen((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id])),
  });

  /** 合成一个：两个办法里各有一半对的，拼起来往往比两个都强。 */
  async function merge() {
    if (!pair) return;
    const a = ideas.find((i) => i.id === pair.a);
    const b = ideas.find((i) => i.id === pair.b);
    setPair(null);
    if (!a || !b) return;
    try {
      const [made] = await createNotes(projectId, [
        { kind: "idea", body: `${a.body}，同时${b.body}` },
      ]);
      if (!made) return;
      // 合出来的那张落在被砸中的那张的位置上——她的手指刚才就在那儿。
      const placed = await moveNote(projectId, made.id, b.x, b.y);
      await Promise.all([a, b].map((p) => archiveNote(projectId, p.id)));
      setIdeas((prev) => [...prev.filter((i) => i.id !== a.id && i.id !== b.id), placed]);
      setChosen((prev) => prev.filter((x) => x !== a.id && x !== b.id));
      if (picked === a.id || picked === b.id) setPicked(null);
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  /** 圈成一堆，并给这堆起个名字。名字是她的判断，不能省。 */
  async function group() {
    const name = groupName.trim();
    if (!name || chosen.length < 2) return;
    try {
      const got = await clusterNotes(projectId, chosen, name);
      const by = new Map(got.map((n) => [n.id, n]));
      setIdeas((prev) => prev.map((n) => by.get(n.id) ?? n));
      setChosen([]);
      setGroupName("");
      setNaming(false);
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  /** 每一堆的外接矩形，用来画那个圈。 */
  const lassos = useMemo(() => {
    const by = new Map<string, Note[]>();
    for (const n of ideas) {
      const c = n.cluster.trim();
      if (!c) continue;
      by.set(c, [...(by.get(c) ?? []), n]);
    }
    const pad = 14;
    return [...by.entries()]
      .filter(([, members]) => members.length > 1)
      .map(([label, members], i) => {
        const xs = members.map((n) => n.x);
        const ys = members.map((n) => n.y);
        return {
          label,
          color: toneAt(i).solid,
          box: {
            x: Math.min(...xs) - pad,
            y: Math.min(...ys) - pad,
            w: Math.max(...xs) + NOTE_W - Math.min(...xs) + pad * 2,
            h: Math.max(...ys) + NOTE_H - Math.min(...ys) + pad * 2,
          },
        };
      });
  }, [ideas]);

  const enough = ideas.length >= ENOUGH;
  const todo = !enough
    ? `再想 ${ENOUGH - ideas.length} 个`
    : !picked
      ? "挑一个先试"
      : why.trim()
        ? ""
        : "一句为什么先试它";

  async function finish() {
    const one = ideas.find((i) => i.id === picked);
    if (!one) return;
    // 🚨 先落库，再收工。印记从来看不到 onFinish 的 payload——回灌是回头读表的，
    // 所以「她挑了哪一条」不写进表里就等于没发生。线上就是这么错的：她挑第三条，
    // 印记照着列表第一条说「你那条点子说……」。
    try {
      await pickIdea(projectId, one.id, why.trim());
    } catch (err) {
      setError(apiErrorText(err));
      return;
    }
    onFinish({ count: ideas.length, picked: one.id, idea: one.body }, why.trim());
  }

  return (
    <Stage
      title={tool.label}
      task="先别判断好不好。把可能的办法都放上来，再看看哪几条其实是一回事。"
      why={tool.reason}
      badge={
        <span
          className="flex items-center gap-1 rounded-mk-full px-2.5 py-1 text-mk-small tabular-nums"
          style={{
            background: left > 0 ? "var(--mk-surface)" : DONE.bg,
            color: left > 0 ? "var(--mk-secondary)" : DONE.solid,
          }}
        >
          <Icon icon={Timer} size={13} />
          {left > 0 ? mmss(left) : "时间到"}
        </span>
      }
      todo={todo}
      insight={enough ? `${ideas.length} 个办法。挑一个先试。` : undefined}
      finishLabel="确认选择"
      onFinish={() => void finish()}
      onClose={onClose}
    >
      {error && (
        <p className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      )}

      <div className="flex items-end gap-2">
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") void add();
          }}
          placeholder="一个办法，回车记下"
          className="flex-1 rounded-mk-md border border-mk-input-border bg-mk-surface px-3 py-2 text-mk-body text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
        />
        <button
          type="button"
          onClick={() => void add()}
          disabled={!draft.trim()}
          aria-label="添加"
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-mk-full text-white disabled:opacity-40"
          style={{ background: "var(--mk-accent-500)" }}
        >
          <Icon icon={Plus} size={16} />
        </button>
      </div>

      {/* 🚨 「想满四个才让挑」原来只是一行字，她看不见自己还差几个——按钮是灰的，
          原因写在别处。四个格子亮起来，规矩就成了看得见的东西。 */}
      <div className="mt-3 flex items-center gap-2">
        <div className="flex gap-1">
          {Array.from({ length: Math.max(ENOUGH, ideas.length) }, (_, i) => (
            <span
              key={i}
              className="h-2 w-6 rounded-mk-full"
              style={{ background: i < ideas.length ? DONE.solid : DONE.bg }}
            />
          ))}
        </div>
        <p className="text-mk-small text-mk-muted">
          {enough
            ? `${ideas.length} 个办法。挑一个先试。`
            : `还差 ${ENOUGH - ideas.length} 个。别急着挑第一个。`}
        </p>
      </div>

      <div
        ref={boardRef}
        className="relative mt-3 overflow-auto rounded-mk-lg"
        style={{
          height: canvasH,
          background: "var(--mk-surface)",
          backgroundImage:
            "radial-gradient(color-mix(in srgb, var(--mk-border) 60%, transparent) 1px, transparent 1px)",
          backgroundSize: "16px 16px",
          touchAction: "none",
        }}
      >
        {lassos.map((l) => (
          <GroupLasso key={l.label} box={l.box} label={l.label} color={l.color} />
        ))}

        {ideas.length === 0 && (
          <div className="student-tool-empty absolute inset-0"><img src={studentArtwork.project} alt="" /><p>暂无想法。请在上方记录一个解决办法。</p></div>
        )}

        {ideas.map((n) => (
          <div
            key={n.id}
            ref={drag.itemRef(n.id)}
            data-idea-id={n.id}
            style={{ position: "absolute", left: n.x, top: n.y, width: NOTE_W }}
          >
            <Sticky
              tone={picked === n.id ? DONE : tone("butter")}
              byYinji={n.author === "yinji"}
              dragging={drag.dragging === n.id}
              selected={chosen.includes(n.id) || drag.over === n.id}
              onPointerDown={(e) => drag.start(n.id, { x: n.x, y: n.y }, e)}
              style={{ minHeight: NOTE_H }}
              right={
                enough ? (
                  <button
                    type="button"
                    onPointerDown={(e) => e.stopPropagation()}
                    onClick={() => setPicked(picked === n.id ? null : n.id)}
                    title="先试这个"
                    className="shrink-0 rounded-mk-full px-1.5 py-0.5 text-mk-small"
                    style={{
                      background: picked === n.id ? DONE.solid : "var(--mk-surface)",
                      color: picked === n.id ? "#fff" : "var(--mk-secondary)",
                    }}
                  >
                    {picked === n.id ? "先试这个" : "先试"}
                  </button>
                ) : undefined
              }
            >
              {n.body}
            </Sticky>
          </div>
        ))}
      </div>

      {ideas.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-2" aria-label="定位想法">
          {ideas.map((idea, i) => <button type="button" key={idea.id}
            className="max-w-full truncate rounded-mk-full border border-mk-border bg-mk-surface px-3 py-1.5 text-mk-small text-mk-secondary hover:border-mk-accent-300"
            title={idea.body}
            onClick={() => {
              const card = Array.from(boardRef.current?.querySelectorAll<HTMLElement>("[data-idea-id]") ?? []).find(el => el.dataset.ideaId === idea.id);
              card?.scrollIntoView({ block: "nearest", inline: "center", behavior: "auto" });
            }}>
            {i + 1} · {idea.body.length > 22 ? idea.body.slice(0, 22) + "…" : idea.body} ↗
          </button>)}
        </div>
      )}
      <p className="mt-2 text-center text-mk-small text-mk-faint">
        拖到一起，就能合并 · 点两张以上，可以圈成一堆
      </p>

      {pair && (
        <div
          className="mt-2 flex flex-wrap items-center justify-between gap-2 rounded-mk-md px-3 py-2"
          style={{ background: tone("peach").bg }}
        >
          <p className="flex items-center gap-1.5 text-mk-small" style={{ color: tone("peach").fg }}>
            <Icon icon={Combine} size={14} />
            把这两条合成一条？
          </p>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => setPair(null)}
              className="rounded-mk-full border border-mk-border bg-mk-surface px-3 py-1 text-mk-small text-mk-secondary"
            >
              取消
            </button>
            <button
              type="button"
              onClick={() => void merge()}
              className="rounded-mk-full px-3 py-1 text-mk-small text-white"
              style={{ background: tone("peach").solid }}
            >
              合并想法
            </button>
          </div>
        </div>
      )}

      {chosen.length >= 2 && (
        <div
          className="mt-2 flex flex-wrap items-center justify-between gap-2 rounded-mk-md px-3 py-2"
          style={{ background: "var(--mk-surface)" }}
        >
          <p className="text-mk-small text-mk-secondary">选了 {chosen.length} 张</p>
          {naming ? (
            <div className="flex flex-1 items-center justify-end gap-2">
              <input
                autoFocus
                value={groupName}
                onChange={(e) => setGroupName(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && void group()}
                placeholder="这几张是一回事，因为……"
                className="min-w-0 flex-1 rounded-mk-md border border-mk-input-border bg-mk-paper px-2.5 py-1 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint"
              />
              <button
                type="button"
                onClick={() => void group()}
                disabled={!groupName.trim()}
                className="rounded-mk-full px-3 py-1 text-mk-small text-white disabled:opacity-40"
                style={{ background: "var(--mk-accent-500)" }}
              >
                确认
              </button>
            </div>
          ) : (
            <button
              type="button"
              onClick={() => setNaming(true)}
              className="rounded-mk-full border border-mk-border px-3 py-1 text-mk-small text-mk-secondary"
            >
              圈成一堆
            </button>
          )}
        </div>
      )}

      {picked && (
        <div className="mt-3 border-t border-mk-border pt-3">
          <label className="text-mk-small text-mk-secondary">为什么先试它？</label>
          <textarea
            value={why}
            onChange={(e) => setWhy(e.target.value)}
            rows={2}
            placeholder="请阐述原因"
            className="mt-1 w-full resize-none rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-2 text-mk-body text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
          />
        </div>
      )}
    </Stage>
  );
}
