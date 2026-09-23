import { useState } from "react";
import { Button, Modal } from "@/ui";
import { putReadingSource, uploadReadingSourceFile } from "../api/readings";
import { generateReadingPlan } from "../api/readingRoom";
import { apiErrorText } from "../api/errorText";

/**
 * PasteFullTextModal —— 这一篇只拿到了摘要：先把标题和摘要摆给她，问她要不要
 * 读全文；要读，就下载原文上传，或者把全文粘进来。
 *
 * # 为什么是一个弹窗
 *
 * 产品负责人 2026-09-17 截图：一篇 Quanta 的文章，正文区只有一段导语和一行
 * 「Source」，而她已经在「先预测」那一步上开始了。「This paper only has abstract.
 * and we should guide students to paste full text with a modal.」正文下面那一条
 * 小字在两段话的文章里不起作用 —— 她读完两段就去跟印记说话了。
 *
 * 同一天他把弹窗的样子也定了：
 *
 *   > present this paper title and abstract. and ask if students want to read
 *   > further. if they want they can download that paper or paste text here.
 *   > and a small ? which hover can show why we cannot get full text.
 *
 * 所以是两屏：先是「这一篇 + 摘要 + 想读全文吗」，她说要读才出现两条路。
 * 不想读就关掉，摘要照样能读（正文下面那一条里还有一颗按钮能再打开这里）。
 *
 * # 保存之后
 *
 * 1. 上传（POST /source/file）或粘贴（PUT /source）—— 服务端据此把「只有摘要」
 *    收掉，清掉按摘要排的读法清单和导读，原网址留着（replaceReadingBody）。
 * 2. POST /plan 按全文重新排一份读法。没成也不要紧：她下一次开口时教练那一轮
 *    会按需重排。
 * 3. 让房间重新加载。
 */
export function PasteFullTextModal({
  open,
  onClose,
  readingId,
  title,
  abstract,
  sourceUrl,
  onReplaced,
  mode = "abstract",
}: {
  open: boolean;
  onClose: () => void;
  readingId: string;
  title: string;
  /** 现在正文区里那几段（摘要那一档就是那段摘要；重新分段那一档是正文）。 */
  abstract: string[];
  sourceUrl: string;
  onReplaced: () => void;
  /**
   * 这个框这一次是来干什么的。
   *
   * - `"abstract"`（默认，原来的唯一一种）：这一篇只拿到了摘要，先问她要不要
   *   读全文，要读才给两条路。
   * - `"resplit"`：她已经有正文了，**分段分错了要重贴**
   *   （产品负责人 2026-09-23 第 3 条）。这一档不问「想读全文吗」——
   *   她已经在读了；也不查「粘进来的比原来长」—— 重新分段常常一个字都不多。
   *
   * 🚨 分成两档而不是复用同一套字：这个框整篇的措辞都是按摘要那一档写的
   *（「我们只获取到了这篇文章的摘要」「只读摘要，后面的通读、精读都做不完整」）。
   * 原样搬到重新分段那一档上，每一句都是假话。
   */
  mode?: "abstract" | "resplit";
}) {
  const resplit = mode === "resplit";
  // 重新分段那一档没有「先问要不要」这一屏：她按进来就是要改。
  const [wantsMore, setWantsMore] = useState(resplit);
  const [text, setText] = useState("");
  const [phase, setPhase] = useState<"idle" | "saving" | "planning">("idle");
  const [error, setError] = useState<string | null>(null);
  const busy = phase !== "idle";
  // 「Source」「来源」这种单独一行的尾巴不是摘要的一部分，摆出来只会让人困惑。
  const shown = abstract.filter((p) => !/^(source|来源|原文)\s*[:：]?$/i.test(p.trim()));
  const abstractLength = abstract.reduce((n, p) => n + Array.from(p).length, 0);

  async function finish(store: () => Promise<unknown>) {
    setError(null);
    setPhase("saving");
    try {
      await store();
    } catch (err) {
      setPhase("idle");
      setError(`保存失败：${apiErrorText(err)}`);
      return;
    }
    setPhase("planning");
    try {
      await generateReadingPlan(readingId);
    } catch {
      // 重排没成不挡她：下一次开口时教练那一轮会按需重排。
    }
    setPhase("idle");
    setText("");
    onReplaced();
  }

  function savePasted() {
    const body = text.trim();
    if (!body) return;
    // 粘进来的比摘要还短：多半只复制到了一部分。
    // 🚨 重新分段那一档不查这一条 —— 她要做的正是把同一篇重贴一遍，
    // 段落改对了，字数常常一个都不多。
    if (!resplit && Array.from(body).length <= abstractLength) {
      setError("粘贴的内容不比摘要长，请确认复制的是文章全文。");
      return;
    }
    void finish(() => putReadingSource(readingId, { title, text: body, url: sourceUrl }));
  }

  function saveFile(file: File | undefined) {
    if (!file) return;
    void finish(() => uploadReadingSourceFile(readingId, file));
  }

  const statusLabel = phase === "saving" ? "保存中" : phase === "planning" ? "正在按全文重新安排阅读步骤" : "";

  return (
    <Modal
      open={open}
      onClose={busy ? () => {} : onClose}
      title={title || "这篇文章"}
      className="max-w-[640px]"
      footer={
        wantsMore ? (
          <>
            <Button variant="ghost" onClick={onClose} disabled={busy}>
              {resplit ? "取消" : "先读摘要"}
            </Button>
            <Button onClick={savePasted} disabled={!text.trim() || busy} loading={busy}>
              {statusLabel || (resplit ? "保存并重新分段" : "保存全文")}
            </Button>
          </>
        ) : (
          <>
            <Button variant="ghost" onClick={onClose}>
              只读摘要
            </Button>
            <Button onClick={() => setWantsMore(true)}>继续读全文</Button>
          </>
        )
      }
    >
      <div className="flex flex-col gap-3">
        {resplit ? (
          <p className="text-mk-small text-mk-muted">
            系统按空行把正文分成段落。分得不对时，请重新粘贴一次，并在需要分段的地方留一个空行。
          </p>
        ) : (
          <p className="flex items-center gap-1.5 text-mk-small text-mk-muted">
            我们只获取到了这篇文章的摘要
            <WhyNoFullText />
          </p>
        )}

        <div className="mk-rp-source rounded-mk-md bg-mk-accent-50 px-3 py-2.5">
          <p className="mb-1 text-mk-label text-mk-accent-700">{resplit ? "现在的分段" : "摘要"}</p>
          <div className="mk-scroll flex max-h-48 flex-col gap-1.5 overflow-y-auto text-mk-body leading-relaxed text-mk-ink">
            {shown.map((p, i) => (
              <p key={i}>{p}</p>
            ))}
          </div>
        </div>

        {!wantsMore ? (
          <p className="text-mk-body text-mk-ink">
            只读摘要，后面的通读、精读和讨论都做不完整。想继续读全文吗？
          </p>
        ) : (
          <>
            <div className="flex flex-col gap-1.5 rounded-mk-md border border-mk-border p-3">
              <p className="text-mk-label text-mk-ink">方式一：下载原文后上传</p>
              <p className="text-mk-small text-mk-muted">
                {sourceUrl ? (
                  <>
                    请打开
                    <a
                      href={sourceUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="mx-1 text-mk-accent-700 underline underline-offset-2"
                    >
                      原文网页
                    </a>
                    ，下载 PDF（或另存为 Word 文档），再选择文件上传。
                  </>
                ) : (
                  "请下载这篇文章的 PDF 或 Word 文档，再选择文件上传。"
                )}
              </p>
              <label className="w-fit">
                <span className="sr-only">选择 PDF 或 Word 文件</span>
                <input
                  type="file"
                  accept=".pdf,.docx,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document"
                  disabled={busy}
                  onChange={(e) => saveFile(e.target.files?.[0])}
                  className="text-mk-small text-mk-muted file:mr-2 file:rounded-mk-full file:border file:border-mk-border file:bg-mk-surface file:px-3 file:py-1 file:text-mk-small file:text-mk-ink"
                />
              </label>
            </div>

            <div className="flex flex-col gap-1.5 rounded-mk-md border border-mk-border p-3">
              <p className="text-mk-label text-mk-ink">方式二：粘贴全文</p>
              <p className="text-mk-small text-mk-muted">
                {resplit
                  ? "把这一篇重新粘一次：段与段之间留一个空行，系统按空行分段。"
                  : "请在原文网页上选中正文并复制（只选文章正文，不要选到导航栏和广告），粘贴到下面，再点击「保存全文」。"}
              </p>
              <textarea
                aria-label="文章全文"
                value={text}
                onChange={(e) => setText(e.target.value)}
                disabled={busy}
                rows={6}
                placeholder="在这里粘贴文章全文"
                className="mk-scroll w-full resize-y rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2 text-mk-small leading-relaxed text-mk-ink placeholder:text-mk-faint focus:border-mk-accent-200 focus:outline-none disabled:opacity-60"
              />
            </div>

            <p className="text-mk-small text-mk-muted">
              {resplit
                ? "保存之后，印记会按新的分段重新安排阅读步骤；之前的对话会保留。"
                : "保存之后，印记会按全文重新安排阅读步骤；之前的对话会保留。"}
            </p>
            {statusLabel && <p className="text-mk-small text-mk-muted">{statusLabel}…</p>}
          </>
        )}

        {error && (
          <p role="alert" className="text-mk-small text-mk-danger">
            {error}
          </p>
        )}
      </div>
    </Modal>
  );
}

/**
 * 那个小「?」：悬停（或键盘聚焦）时说清楚为什么拿不到全文。
 *
 * 产品负责人 2026-09-17：「a small ? which hover can show why we cannot get full
 * text.」原因写的是真实的那几种（付费墙、登录、网站不允许程序读取），不是一句
 * 「技术原因」。
 */
function WhyNoFullText() {
  return (
    <span className="mk-why" tabIndex={0} aria-label="为什么拿不到全文">
      ?
      <span role="tooltip" className="mk-why__tip">
        很多网站把全文放在付费墙或登录之后，或者不允许程序读取页面，只对外公开摘要。我们的服务器打不开这些页面；你在自己的浏览器里打开原文，通常就能看到全文。
      </span>
    </span>
  );
}
