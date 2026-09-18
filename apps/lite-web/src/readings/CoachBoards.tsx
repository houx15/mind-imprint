import { useRef, useState } from "react";

/**
 * CoachBoards —— 印记 递到她手上的两块**板**。
 *
 * # 为什么是板，不是表单
 *
 * 产品负责人 2026-09-10：
 *
 *   > the guiding, now only texts, or questions/choices/text input, is not
 *   > enough. we have great 透镜 interaction. and we can be richer.
 *   > we can make our ai be able to call out an interactive part and then back
 *
 * 结构没问题（印记 排步骤、领着走），缺的是**她手上能摆的东西**。透镜是唯一
 * 一件，而它一篇文章只用一两次。
 *
 * 🚨 而「交互」在这个产品里有确定的意思（2026-09-04）：**一块能用手摆的板**，
 * 拖是主要动词。所以这两块都是拖，不是勾选框、不是下拉菜单 —— 2026-09-01 被
 * 否掉的正是那种「form-like things」。
 *
 * # 两块板
 *
 *   标注板   文章里的几句话，各自拖进一个角色格子（主张/证据/限制/背景/对比）。
 *            这是一次**不问「你懂了吗」的理解检查**：贴不出来就是没读懂，
 *            而她一个字都不用写。
 *   生词板   这一段里的几个词，各自拖进「认识 / 不确定 / 不认识」。
 *            板上**没有释义** —— 她分完之后，印记 下一轮只讲后两格里的词。
 *            「讲哪几个词」这件事因此由她决定，不由模型猜。
 *
 * # 拖，而且不只能拖
 *
 * 拖用的是 pointer 事件（触屏和鼠标同一套代码，HTML5 的 dragstart 在手机上
 * 根本不触发）。但**点也能用**：先点一张卡片选中它，再点一个格子。
 * 理由不是无障碍的教条 —— 是一只手扶着手机的人拖不动一张卡片。
 * 两条路径的最终状态完全一样。
 *
 * # 板不判对错
 *
 * 和 CoachCard 同一条：没有 ✓、没有 ✗、没有分数。她摆完，摆的结果原样回灌给
 * 印记，由 印记 在下一轮里回应她 —— 「第 5 段那句我会标成限制而不是证据，
 * 因为……」。那是教，不是判分。所以这个文件里没有任何地方知道正确答案，
 * 服务端也不发。
 */

/**
 * 每个角色格子底下那一句白话。
 *
 * 🚨 写的是**这一句在文章里干什么**，不是给这个词下定义。「主张 = 作者要你
 * 接受的那句话」她拿着就能去比对；「主张：作者的核心论点」只是把一个生词换成
 * 另外两个。
 *
 * 🚨 **这一份是阅读室的，别当成全局的。** 它的每一句都站在「读别人写的东西」
 * 这一侧说话 —— 「**作者**要你接受的那句话」。
 *
 * 而 `CoachBoard` 是两个房间共用的：写作室的 RoleBoard 也从这里进来
 *（apps/lite-web/src/writings/RoleBoard.tsx），它的格子是
 * 主张 / 证据 / 解释 / 让步 / 背景。那一侧**作者就是她自己**，同一句话摆过去
 * 就是错的；而且「解释」「让步」两格在这张表里根本没有，板会半边有字半边没字。
 *
 * 所以这一份只当**默认值**，房间可以自己传一份（`binHints`）。
 * 两个房间的格子本来就不是同一套词，共用一份表是让它们迟早互相踩的唯一原因。
 */
const BIN_HINT: Record<string, string> = {
  // 现行两套（2026-09-17）。产品负责人：「maybe not every paragraph has this
  // thing… maybe we need to simplify it. key statement 关键主张 / key evidence
  // 证据 / or sometimes it is an argument: 驳斥观点 / 作者观点 / 证据」
  关键主张: "作者要你接受的那句话",
  证据: "拿来撑住主张的事实、数字或例子",
  作者观点: "作者自己站的那一边",
  驳斥观点: "作者要反对的那个说法",
  // 另外三种体裁各自的板（2026-09-17，同事的阅读模块 PRD；服务端闭表在
  // reading_genre.go 的 coachGenreBoards）。
  事实: "记者核实过、写成陈述的事",
  // 🚨「通常带引号」会把转述推出这一格：线上走查里「Some residents blamed the
  // council」（记者转述居民的说法）被她来回在事实和解释之间挪了二十步。
  引述: "某一方的说法，直接引用或转述都算",
  解释: "对事件的原因、影响所作的说明或推断",
  说明对象: "这篇要讲清楚的那样东西",
  原理与过程: "它怎么运作、分哪几步",
  例子与数据: "拿来说明它的具体事例或数字",
  动作描写: "人物做了什么",
  语言描写: "人物说了什么",
  心理描写: "人物想了什么、感受到什么",
  环境描写: "人物周围的景物和气氛",
  // 2026-09-17 换下来的三个。**不再发给她**，留在这里是因为她三天前摆过的
  // 那块板还在转写里，回看时格子底下那句话不该是空的。
  主张: "作者要你接受的那句话",
  限制: "作者自己承认的那一点「但是」",
  背景: "交代情况，不参与说服",
  对比: "拿来比的另一面",
};

/** 一张待分类的卡片：它显示什么，以及回灌时它是谁。 */
export type BoardItem = {
  /** 板内唯一。 */
  id: string;
  /** 卡片上显示的字。 */
  text: string;
  /** 它出自哪一段（用来在回灌里说「第几段」）。 */
  blockId: string;
  /**
   * 它出自第几段，她看得懂的那个写法（「第4段」）。服务端填的
   * （`coachCardOption.where`），这里只负责显示。
   *
   * 🚨 产品负责人 2026-09-17：「对整体拆分时，选择的都是单句，并未标注段落，
   * 有时候单独的句子拆出来很难看出属于什么部分。」四句话摘出上下文摆在一块板
   * 上，不说各自从哪儿来，「哪句在撑哪句」这件事她无从判断。
   */
  where?: string;
};

/** 她摆完之后的状态：itemId → 格子名。没摆的那些不在表里。 */
export type BoardPlacement = Record<string, string>;

// ---------------------------------------------------------------------------
// 拖 + 点，一套状态
// ---------------------------------------------------------------------------

/** 超过这么多像素才算「拖」，之内都算「点」。见 isDrag。 */
export const DRAG_SLOP = 6;

/**
 * 这一下是「拖」还是「点」。
 *
 * 🚨 它是一个独立的纯函数，不是写在事件处理里的一行，因为**它是这块板上唯一
 * 一处「读代码看不出对错」的逻辑**，而 jsdom 里没有 PointerEvent，事件那条路
 * 根本测不了。
 *
 * 原来那一行写的是「动了就算拖」（`> 0`）。手指按下去总会动一两个像素，鼠标
 * 也一样，于是绝大多数「点一下选中」都被当成一次拖动；而那次拖动的落点还在
 * 原地（未分类那一堆的容器 `data-board-bin=""`），于是卡片被「放回」原处，
 * 屏幕上什么都没发生。模拟学生走查里这块板出现了 52 步、她摆了 51 次，
 * 四张卡片一张都没进格子 —— 看上去像她不会用，其实是那一行。
 *
 * 距离从**按下的那个点**算，不累加每一帧的位移：累加的话，慢慢挪一圈再回到
 * 原处也会被算成拖了很远。
 */
export function isDrag(from: { x: number; y: number }, to: { x: number; y: number }): boolean {
  return Math.hypot(to.x - from.x, to.y - from.y) > DRAG_SLOP;
}

/**
 * 一块板的公共行为：选中一张卡、把它放进一个格子、以及拖动时的落点判定。
 *
 * 落点是用 `elementFromPoint` 找的，而不是靠每个格子各挂一个 pointerenter：
 * 拖动过程中指针被 `setPointerCapture` 捕获在卡片上，格子收不到任何
 * pointer 事件 —— 不捕获的话，手指一离开卡片这次拖动就断了。
 *
 * 🚨 捕获还有第三个后果，比上面两个隐蔽：**接下来的 click 事件也会被改派给
 * 捕获它的那个元素**，而不是手指底下那个。写作那边 2026-09-12 就是这么把
 * 「点标题改名」弄坏的 —— 加了拖拽之后，click 全被卡片接走了，而 644 个单元
 * 测试一路全绿（jsdom 没有真的指针捕获，它**不可能**看见这件事）。
 *
 * 这块板没被这一条咬到，但**不是因为运气**，是因为两条路各自不依赖 click：
 *   拖    松手时 `onItemPointerUp` 用 elementFromPoint 找落点，不看 click。
 *   点选  点卡片是一次完整的 down+up，捕获在 up 时就释放了；她接着点格子
 *         是**另一次**手势，那时没有任何捕获，格子的 onClick 照常响。
 * 所以那两条看着重复的路**都不能删**：删掉 pointerup 那条，拖拽就没有落点；
 * 删掉格子的 onClick，一只手扶着手机的人就没法用点选。
 */
function useBoard(initial: BoardPlacement = {}) {
  const [placed, setPlaced] = useState<BoardPlacement>(initial);
  const [picked, setPicked] = useState<string | null>(null);
  const [hoverBin, setHoverBin] = useState<string | null>(null);
  const [dragging, setDragging] = useState<string | null>(null);
  const [ghost, setGhost] = useState<{ x: number; y: number } | null>(null);
  // 按下去的那个点，用来判断这到底是一次「点」还是一次「拖」。
  const downAtRef = useRef<{ x: number; y: number } | null>(null);
  const movedRef = useRef(false);
  /**
   * 🚨 正在被按住的是哪一张 —— **ref，不是 state**。
   *
   * 这里原来用的是上面那个 `dragging` state 来判断「这次 pointerup 属不属于
   * 一次真的按下」。一次**快速点击**里 pointerdown 和 pointerup 落在同一个
   * React 批次里：pointerup 的处理函数读到的还是上一次渲染的闭包，`dragging`
   * 仍然是 null，于是它直接 return，这一下什么都没发生。
   *
   * 手指在触屏上的一次点按正正好就是这么快。所以「点一下选中」对真人基本上
   * 是坏的 —— 改完拖动阈值之后，模拟学生又摆了 82 次，屏幕上仍然是
   * 「现在一张都还没摆」。
   *
   * ref 在同一个事件循环里就是最新值，不等渲染。state 留着，它只负责画。
   */
  const draggingRef = useRef<string | null>(null);

  function binAt(x: number, y: number): string | null {
    const el = document.elementFromPoint(x, y);
    const bin = el?.closest?.("[data-board-bin]");
    return bin ? bin.getAttribute("data-board-bin") : null;
  }

  /**
   * 🚨 每摆一次留一份上一步，给「撤销」用。
   *
   * 产品负责人 2026-09-12：「为阅读模块的标注板卡片及各类选项增加回退功能，
   * 支持返回上一步重新选择。」摆错一张之前只能靠再摆一次盖过去，而她往往
   * 记不清它原来在哪一格 —— 摆错就等于丢了一个信息。
   */
  const historyRef = useRef<BoardPlacement[]>([]);

  function place(itemId: string, bin: string | null) {
    setPlaced((prev) => {
      historyRef.current.push(prev);
      const next = { ...prev };
      if (bin) next[itemId] = bin;
      else delete next[itemId];
      return next;
    });
  }

  /** 退回上一步。没有上一步就什么都不做。 */
  function undo() {
    const prev = historyRef.current.pop();
    if (!prev) return;
    setPlaced(prev);
    setPicked(null);
  }

  function canUndo() {
    return historyRef.current.length > 0;
  }

  function onItemPointerDown(itemId: string, e: React.PointerEvent<HTMLElement>) {
    // 只接主键/单指。右键和第二根手指不该开始一次拖动。
    if (e.button !== 0) return;
    e.currentTarget.setPointerCapture(e.pointerId);
    movedRef.current = false;
    downAtRef.current = { x: e.clientX, y: e.clientY };
    draggingRef.current = itemId;
    setDragging(itemId);
    setGhost({ x: e.clientX, y: e.clientY });
  }

  function onItemPointerMove(e: React.PointerEvent<HTMLElement>) {
    if (!draggingRef.current) return;
    // 抖动不算拖动，门槛见 isDrag。
    const from = downAtRef.current;
    if (from && isDrag(from, { x: e.clientX, y: e.clientY })) movedRef.current = true;
    setGhost({ x: e.clientX, y: e.clientY });
    setHoverBin(binAt(e.clientX, e.clientY));
  }

  function onItemPointerUp(itemId: string, e: React.PointerEvent<HTMLElement>) {
    if (draggingRef.current !== itemId) return;
    e.currentTarget.releasePointerCapture?.(e.pointerId);
    const bin = binAt(e.clientX, e.clientY);
    draggingRef.current = null;
    setDragging(null);
    setGhost(null);
    setHoverBin(null);
    if (movedRef.current) {
      // 拖到格子外面松手 = 把它拿回来，不是把它丢进最近的格子。
      place(itemId, bin);
      setPicked(null);
      return;
    }
    downAtRef.current = null;
    // 🚨 已经选中了**另一张**卡，而这一下点在某个**格子里**：
    // 这是「把选中的那张放进这个格子」，不是「改选这一张」。
    //
    // 2026-09-11 写作面走查抓到的第三个「点选路径断掉」的毛病，和上面那两个
    // （拖动阈值、dragging 读到上一次渲染）是各自独立的：
    // 格子一旦有了一张卡，那张卡就占住了格子的中心，于是「点格子」这一下实际
    // 点在卡片上，被 Chip 的 stopPropagation 吃掉，armed 的那张永远放不进去。
    // 也就是说**点选这条路在格子非空之后就断了** —— 而它正是一只手扶着手机的
    // 人唯一能用的那条路。拖那条路没事（binAt 用的是坐标），所以这个毛病在
    // 鼠标上很难发现：标注板上四张卡，前两张进得去，第三张起就不动了。
    if (picked && picked !== itemId && bin) {
      place(picked, bin);
      setPicked(null);
      return;
    }
    // 没动过 = 这是一次点击：选中 / 取消选中。
    setPicked((prev) => (prev === itemId ? null : itemId));
  }

  /** 点一个格子：把选中的那张放进去。没有选中的就什么也不做。 */
  function onBinClick(bin: string) {
    if (!picked) return;
    place(picked, bin);
    setPicked(null);
  }

  return { placed, picked, hoverBin, dragging, ghost, place, undo, canUndo, setPicked, onItemPointerDown, onItemPointerMove, onItemPointerUp, onBinClick };
}

// ---------------------------------------------------------------------------
// 板
// ---------------------------------------------------------------------------

export function CoachBoard({
  items,
  bins,
  itemLabel,
  submitLabel,
  busy,
  onSubmit,
  binHints,
  prefill,
}: {
  items: BoardItem[];
  /** 格子的名字，按屏幕顺序。 */
  bins: string[];
  /** 未分类那一堆上面的一行说明。 */
  itemLabel: string;
  submitLabel: string;
  busy?: boolean;
  onSubmit: (placement: BoardPlacement) => void;
  /**
   * 她上一次在同一块板上的摆放，开局就摆好。
   *
   * 🚨 同事 2026-09-17：「像是这种填入句子的卡片，很多时候会出现两三次交互，
   * 对其中的具体句子进行替换的情况（比如 1、4 句正确，2 和 3 重新填写），
   * 需要在 2、3 次调用的时候进行复用，保存上一次的填写结果，不用每一次都要
   * 重新填。」印记 指出两句放错了，而她要重摆的是四句 —— 另外两句是她已经
   * 做对的功课，让她再做一遍是我们在收回她的成果。
   *
   * 只在**开局**读一次（useState 的初值）。她接下来怎么摆都以屏幕上为准。
   */
  prefill?: BoardPlacement;
  /**
   * 每个格子底下那一句白话。不给就用阅读室那一份（BIN_HINT）。
   *
   * 🚨 房间自己传，因为那句话是**站在谁的位置上说的**：阅读室说「作者要你
   * 接受的那句话」，写作室那一侧作者就是她自己。见 BIN_HINT 上面那段。
   */
  binHints?: Record<string, string>;
}) {
  const hints = binHints ?? BIN_HINT;
  const b = useBoard(prefill ?? {});
  const loose = items.filter((it) => !b.placed[it.id]);
  const done = loose.length === 0;

  return (
    <div className="mk-board">
      <div className="mk-board__loose" data-board-bin="">
        {/* 🚨 摆完之后要说下一步是什么。
            走查里她摆完四张卡片就停住了：「我摆完卡片了但屏幕没变化，不知道
            该点哪。」—— 那颗按钮此刻刚从禁用变成可点，但屏幕上没有任何东西
            把她指过去，而未分类那一格这时候是空的，看着像这块板已经交掉了。 */}
        <p className="mk-board__hint">
          {loose.length > 0 ? itemLabel : `都摆好了。请点下面的「${submitLabel}」。`}
        </p>
        {/* 格子里开局就有东西的时候，说清它们是哪儿来的。不说的话，这块板看着
            像是印记替她摆了一半 —— 而那正是这个产品绝不做的事。 */}
        {prefill && Object.keys(prefill).length > 0 && (
          <p className="mk-board__hint">已保留你上一次的摆放，请只调整需要改的那几张。</p>
        )}
        <div className="mk-board__chips">
          {loose.map((it) => (
            <Chip
              key={it.id}
              item={it}
              picked={b.picked === it.id}
              dragging={b.dragging === it.id}
              board={b}
            />
          ))}
        </div>
      </div>

      <div className="mk-board__bins">
        {bins.map((bin) => {
          const inside = items.filter((it) => b.placed[it.id] === bin);
          return (
            <div
              key={bin}
              data-board-bin={bin}
              onClick={() => b.onBinClick(bin)}
              className={`mk-board__bin${b.hoverBin === bin ? " is-over" : ""}${b.picked ? " is-armed" : ""}`}
            >
              <span className="mk-board__binname">{bin}</span>
              {/* 🚨 格子名底下要有一句她能照着做的话。
                  走查里同一条抱怨出现了十二次：「『主张』『证据』『限制』『对比』
                  这几个词到底怎么分啊，英语课上没这么讲过，我只能瞎猜。」
                  这五个词是她来这儿要学的（所以照说），但一个只有名字的格子
                  对她就是一个生词 —— 她只能猜，或者照着 印记 说漏的答案搬。
                  一句白话不是把题目做掉：她仍然得判断每一句在干什么。 */}
              {hints[bin] && <span className="mk-board__binhint">{hints[bin]}</span>}
              <div className="mk-board__chips">
                {inside.map((it) => (
                  <Chip
                    key={it.id}
                    item={it}
                    picked={b.picked === it.id}
                    dragging={b.dragging === it.id}
                    board={b}
                  />
                ))}
              </div>
            </div>
          );
        })}
      </div>

      <div className="mk-board__foot">
        {/* 退一步。摆错一张之前只能靠再摆一次盖过去，而她往往记不清它原来在
            哪一格 —— 摆错就等于丢了一个信息。 */}
        <button
          type="button"
          disabled={busy || !b.canUndo()}
          onClick={b.undo}
          className="mk-board__undo"
        >
          撤销上一步
        </button>
        <button
          type="button"
          disabled={busy || !done}
          onClick={() => onSubmit(b.placed)}
          className="mk-board__submit"
        >
          {submitLabel}
        </button>
      </div>

      {/* 跟着手指走的那一张。`position: fixed`，所以它不受任何祖先的
          overflow 裁剪 —— 板本身在一个会滚动的面板里。 */}
      {b.dragging && b.ghost && (
        <span
          aria-hidden="true"
          className="mk-board__ghost"
          style={{ left: b.ghost.x, top: b.ghost.y }}
        >
          {items.find((it) => it.id === b.dragging)?.text}
        </span>
      )}
    </div>
  );
}

function Chip({
  item,
  picked,
  dragging,
  board,
}: {
  item: BoardItem;
  picked: boolean;
  dragging: boolean;
  board: ReturnType<typeof useBoard>;
}) {
  return (
    <button
      type="button"
      aria-pressed={picked}
      className={`mk-board__chip${picked ? " is-picked" : ""}${dragging ? " is-dragging" : ""}`}
      onPointerDown={(e) => board.onItemPointerDown(item.id, e)}
      onPointerMove={board.onItemPointerMove}
      onPointerUp={(e) => board.onItemPointerUp(item.id, e)}
      onPointerCancel={(e) => board.onItemPointerUp(item.id, e)}
      // 点击已经由 pointerup 处理了；再让浏览器合成一次 click，会连着
      // 冒泡到格子上，把刚选中的那张立刻放进去。
      onClick={(e) => e.stopPropagation()}
    >
      {/* 段号在句子前面，不在后面：她扫这块板的时候先要知道「这是哪儿的话」，
          再读那句话本身。没有段号（生词板、老数据）就整个不渲染。 */}
      {item.where && <span className="mk-board__chipwhere">{item.where}</span>}
      {item.text}
    </button>
  );
}

/**
 * 一块摆完了的板，只读。
 *
 * 🚨 产品负责人 2026-09-17：「阅读卡片选择以后无法看到其他选项（贴句子的卡片
 * 也是），无法回退。」她一交上来，这块板就从屏幕上整个消失，只剩一段
 * 「主张：……证据：……」的文字 —— 于是「我刚才把哪句放在哪儿」这件事，
 * 她只能靠读那段文字重建。
 *
 * 留下来的是**同一块板的样子**（同样的格子、同样的卡片、同样的位置），
 * 只是不能再动。能不能再动这件事是有意的：板一交上去，印记 那一轮就是照着这个
 * 摆法讲的，再改一次只会让屏幕和对话对不上（system prompt 里那条「不要再让她
 * 动那块板」说的是同一件事）。
 */
export function CoachBoardRecap({
  items,
  bins,
  placement,
  binHints,
}: {
  items: BoardItem[];
  bins: string[];
  placement: BoardPlacement;
  binHints?: Record<string, string>;
}) {
  const hints = binHints ?? BIN_HINT;
  return (
    <div className="mk-board is-done" aria-label="你摆好的板">
      <div className="mk-board__bins">
        {bins.map((bin) => {
          const inside = items.filter((it) => placement[it.id] === bin);
          return (
            <div key={bin} data-board-bin={bin} className="mk-board__bin is-readonly">
              <span className="mk-board__binname">{bin}</span>
              {hints[bin] && <span className="mk-board__binhint">{hints[bin]}</span>}
              <div className="mk-board__chips">
                {inside.map((it) => (
                  <span key={it.id} className="mk-board__chip is-readonly">
                    {it.where && <span className="mk-board__chipwhere">{it.where}</span>}
                    {it.text}
                  </span>
                ))}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
