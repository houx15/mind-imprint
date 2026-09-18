import { useState } from "react";
import { Lightbulb } from "lucide-react";
import { Button, Icon, Modal } from "@/ui";
import { apiErrorText } from "../api/errorText";

/**
 * NamePieceModal — 给这篇起个名字, asked once, at 完成这篇.
 *
 * ## Why this moment and not another
 *
 * A writing's title starts life as her raw 「我想写：…」 sentence, up to 200
 * characters of it (createWriting, writings.go). That is fine while the piece
 * is hers alone — `EditableTitle` sits in the room header the whole time, so
 * she can change it whenever she likes. It stops being fine the instant she
 * finishes: the title goes into display type at the top of the report, onto
 * the exported poster, and out through the share link to someone with no
 * account, who reads her private note-to-self where the piece's name should
 * be.
 *
 * 完成这篇 is therefore the one right moment to ask — the piece is written, so
 * there is something real to name, and it is the last moment before the title
 * becomes public.
 *
 * ## Asked once, and never to someone who already answered
 *
 * The server decides (`suggestWritingTitles` → `needsName`) by comparing the
 * title against her stored opening turn, character for character. Anyone who
 * used `EditableTitle` never sees this dialog at all, and no model call is
 * made for them.
 *
 * ## 铁律②: this is not a gate
 *
 * 用原来的 finishes with the title untouched, and it is a real button of equal
 * standing, not a greyed-out link in the corner. She can also close the dialog
 * outright. Nothing here blocks 完成这篇, because a piece she chose not to
 * rename is a legitimate outcome — the failure this fixes is never being
 * ASKED, not "having the wrong title".
 *
 * ## She names it; 印记 only hands her keywords, and only when asked
 *
 * 🚨 2026-09-18 产品负责人：「ai不要直接生成题目文本，我觉得可以在学生点击
 * 『需要提示』之后，给出一些关键词，但是不能直接给取名字。」
 *
 * The first version opened with four finished titles, the first one already
 * in the box — she pressed 确认 and 印记 had named her piece. Now the box
 * starts empty, nothing is suggested until she presses 「需要提示」, and what
 * comes back is keywords that the server has checked appear verbatim in her
 * draft (writing_title.go validateTitleKeywords). The chips are NOT buttons:
 * tapping one does not fill the box. Composing the name is hers.
 */
export function NamePieceModal({
  onHint,
  onName,
  onKeep,
  onClose,
  saving,
  error,
}: {
  /** 「需要提示」：取几个摘自她正文的关键词（POST /title-keywords）。 */
  onHint: () => Promise<string[]>;
  /** Save this title, then finish. */
  onName: (title: string) => void;
  /** Finish with the title as it stands. */
  onKeep: () => void;
  onClose: () => void;
  saving: boolean;
  error: string | null;
}) {
  const [value, setValue] = useState("");
  const trimmed = value.trim();
  const [keywords, setKeywords] = useState<string[] | null>(null);
  const [hinting, setHinting] = useState(false);
  const [hintError, setHintError] = useState<string | null>(null);

  async function hint() {
    setHinting(true);
    setHintError(null);
    try {
      setKeywords(await onHint());
    } catch (err) {
      setHintError(`获取提示失败：${apiErrorText(err)}`);
    } finally {
      setHinting(false);
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="给这篇起个名字"
      footer={
        <>
          {/* Equal standing, not a get-out link: see 铁律② in this file's
              header. Left of the primary button so it reads as a choice. */}
          <Button variant="ghost" onClick={onKeep} disabled={saving}>
            用原来的
          </Button>
          <Button onClick={() => onName(trimmed)} loading={saving} disabled={trimmed === ""}>
            确认并完成
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-5">
        <p className="text-mk-body text-mk-muted">
          现在这一栏里放的还是你最开始写的那句「我想写…」。报告、导出的图片和你发出去的链接上，写的都是它。
        </p>

        {/* 🚨 **这个弹窗从头到尾在说「起名字」，一个字没说它还会把这一篇合上。**
            2026-09-11 第七轮线上走查，一个学生在 448 字的时候走完了这个弹窗，
            下一屏就懵了：

              「我刚才明明写了448字了，怎么又回到这个页面了？我的文章去哪了？」
              「那篇文章标着『已完成』但我觉得我没写完，不知道能不能继续编辑」

            她不能。完成之后这一篇的每个写入口都被 loadOwnedWritingAtom 挡掉
            —— 这是有意的（完成才有过程评估，「过程即数据」），但**代价没有在
            按下去之前说过**。按钮上那个「完成」两个字不够：整个弹窗的语境是
            给文章起名，她读到的是「确认这个名字」。

            所以在这里把后果说清楚。不改「完成是单向的」这件事本身 —— 那关系到
            评估记录，是产品的判断，不是我在走查里顺手能定的。 */}
        {/* 2026-09-18：完成之后可以「修改」出新的一版（0153），原来那句
            「完成之后这一篇就不再改了」已经不是真的。后果仍然要说清楚：
            完成就是提交一版，老师和报告看到的是这一版。 */}
        <p className="text-mk-body text-mk-secondary">
          完成即提交这一版，印记会据此生成过程报告；老师看到的也是这一版。之后仍可点「修改」提交新的一版。
        </p>

        <div className="flex flex-col gap-2">
          <label htmlFor="mk-piece-name" className="text-mk-small text-mk-secondary">
            这篇文章叫
          </label>
          <input
            id="mk-piece-name"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder="写一个你想让别人看到的名字"
            autoFocus
            className="w-full rounded-mk-sm border border-mk-border bg-mk-paper px-3 py-2 text-mk-body-lg text-mk-ink focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
          />
        </div>

        <div className="flex flex-col gap-2">
          {keywords === null ? (
            <Button
              variant="secondary"
              size="sm"
              className="w-fit"
              onClick={() => void hint()}
              loading={hinting}
              iconStart={<Icon icon={Lightbulb} size={14} />}
            >
              需要提示
            </Button>
          ) : keywords.length === 0 ? (
            <span className="text-mk-small text-mk-muted">正文里暂时没有可摘的关键词，请直接写一个名字。</span>
          ) : (
            <>
              <span className="text-mk-small text-mk-secondary">关键词（摘自你的正文）</span>
              <div className="flex flex-wrap gap-2">
                {keywords.map((k) => (
                  <span
                    key={k}
                    className="rounded-mk-full border px-3 py-1 text-mk-body"
                    style={{ borderColor: "var(--mk-border)", background: "var(--mk-paper)", color: "var(--mk-ink)" }}
                  >
                    {k}
                  </span>
                ))}
              </div>
              <span className="text-mk-caption text-mk-faint">请用这些词组合出你自己的标题。</span>
            </>
          )}
          {hintError && (
            <p role="alert" className="text-mk-small text-mk-danger">
              {hintError}
            </p>
          )}
        </div>

        {error && (
          <p role="alert" className="text-mk-small text-mk-danger">
            {error}
          </p>
        )}
      </div>
    </Modal>
  );
}
