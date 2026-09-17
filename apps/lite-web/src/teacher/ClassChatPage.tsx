import { useEffect, useRef, useState } from "react";
import { ArrowRight, ClipboardList, Send, UserRoundSearch } from "lucide-react";
import { Button, Icon } from "@/ui";
import { api } from "@/api";
import { getRoster, type RosterRow } from "../api/teacher";
import { postWorkspaceTurn } from "../api/teacherWorkspace";
import { errorText, failText, writeLastClassId, writePendingRecipients } from "./assignmentLogic";
import { LearningSnapshot } from "./LearningSnapshot";
import { BackLink, SectionHead, StudioError, StudioHeading, StudioLoading } from "./StudioArtwork";
import type { TeacherRoute } from "./teacherRouting";
import { keepDataCards, navigateOf, navigateRoute, withNavigateCard, type NavigateTarget } from "./workspace/homeLogic";
import { useWorkspaceThread } from "./workspace/useWorkspaceThread";
import { AssignmentsCard, ClassSnapshotCard, StudentsCard } from "./workspace/WorkspaceCards";
import { WorkspacePanel } from "./workspace/WorkspacePanel";

// teacher/ClassChatPage.tsx — `/classes/:classId/chat`, the class
// conversation opened from the summary on a class card (§12.5). Same shell
// and thread as the homework card, surface "home". The canvas shows the
// class overview, then the tool result cards of the last turn, then the page
// offer as a 「前往：…」 button that only navigates when she clicks it.
//
// The shell mounts this page with `key={classId}`, so another class starts a
// new conversation.

/** The home surface has no artifact to edit; the thread only needs the scope. */
interface HomeArtifact {
  classId: string;
}

const scopeOf = (a: HomeArtifact) => a.classId;
const ignoreArtifact = () => {};

export function ClassChatPage({
  classId,
  onBack,
  onOpenWeekly,
  go,
}: {
  classId: string;
  /** 返回班级: the class page. */
  onBack: () => void;
  /** 查看周报 in the left pane. */
  onOpenWeekly?: () => void;
  go: (route: TeacherRoute) => void;
}) {
  const [name, setName] = useState<string | null>(null);
  const [nameError, setNameError] = useState<string | null>(null);
  const [roster, setRoster] = useState<RosterRow[] | null>(null);
  const [rosterError, setRosterError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setNameError(null);
    setRosterError(null);
    api
      .getClass(classId)
      .then((d) => { if (!cancelled) setName(d.class.name); })
      .catch((e: unknown) => { if (!cancelled) setNameError(errorText(e)); });
    getRoster(classId)
      .then((rows) => { if (!cancelled) setRoster(rows); })
      .catch((e: unknown) => { if (!cancelled) setRosterError(errorText(e)); });
    return () => { cancelled = true; };
  }, [classId, nonce]);

  const [artifact] = useState<HomeArtifact>(() => ({ classId }));
  const thread = useWorkspaceThread<HomeArtifact>({
    artifact,
    setArtifact: ignoreArtifact,
    scopeOf,
    post: ({ artifact: a, turns, input }) =>
      postWorkspaceTurn({
        surface: "home",
        classId: a.classId,
        artifact: {},
        turns,
        ...("text" in input ? { text: input.text } : { choiceId: input.choiceId, choiceSlug: input.slug, choiceLabel: input.label }),
      }).then((res) => ({
        reply: res.reply,
        choices: res.choices,
        patch: {},
        cards: withNavigateCard(res.cards, res.navigate),
      })),
    // The server already prefixes its message with 「对话失败：」; `failText`
    // does not double it.
    describeError: (e) => failText("对话", e),
  });
  const { choices, cards } = thread;

  // The canvas keeps the last list a turn produced: a reply that only offers
  // a page (「请点击下方按钮前往布置作业」) does not blank it.
  const [shownCards, setShownCards] = useState(cards);
  const [cardsSeen, setCardsSeen] = useState(cards);
  if (cards !== cardsSeen) {
    setCardsSeen(cards);
    setShownCards((prev) => keepDataCards(prev, cards));
  }

  const target = navigateRoute(navigateOf(cards), classId);
  function open(route: TeacherRoute) {
    // The assignment form and the parent report list open on the remembered
    // class; make that this class.
    writeLastClassId(classId);
    go(route);
  }
  function openTarget(t: NavigateTarget) {
    if (t.userIds) writePendingRecipients(classId, t.userIds);
    open(t.route);
  }

  // She picked an option (「给这些学生布置作业」) and the reply offers the
  // page for it: that pick was her click, so the page opens without asking
  // for a second one. A typed request still gets the button.
  // Holds the cards that were on screen when she picked, until that turn
  // settles. A failed turn leaves `cards` as they were, and must not follow
  // the previous reply's offer.
  const choseRef = useRef<typeof cards | null>(null);
  useEffect(() => {
    const before = choseRef.current;
    if (before === null || thread.busy) return;
    choseRef.current = null;
    if (cards !== before && target) openTarget(target);
    // Runs when a picked turn settles; `target` is derived from `cards`.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [cards, thread.busy]);

  return (
    <WorkspacePanel
      turns={thread.turns}
      busy={thread.busy}
      error={thread.error}
      choices={choices}
      onSend={(text) => {
        const ok = thread.run({ text });
        if (ok) choseRef.current = null;
        return ok;
      }}
      onChoose={(choiceId) => {
        const choice = choices.find((c) => c.id === choiceId);
        const ok = thread.run({ choiceId, label: choice?.label ?? choiceId, slug: choice?.slug });
        if (ok) choseRef.current = cards;
        return ok;
      }}
      composer={thread.composer}
      onComposerChange={thread.setComposer}
      onRetry={thread.retry}
      canRetry={thread.failed !== null}
      intro="印记读取本班本周的数据后回答，需要时给出前往相关页面的按钮。请输入问题，或选择一个示例。"
      suggestions={["这周谁还没开始学习？", "哪些作业有学生逾期？", "本周整体情况如何？"]}
      replyAction={
        target && (
          <Button variant="primary" size="sm" onClick={() => openTarget(target)}>
            前往：{target.label}
          </Button>
        )
      }
      header={
        <>
          <BackLink label="返回班级" onClick={onBack} />
          <StudioHeading
            kicker="班级对话"
            title={name ?? "班级对话"}
            description="印记根据本班本周的学习数据回答问题。请在右侧输入问题。"
            kind="discovery"
          />
          {nameError && <StudioError verb="班级加载" message={nameError} />}
        </>
      }
    >
      <div className="teacher-chat-canvas">
        <section className="teacher-panel">
          <h2 className="teacher-panel-title">班级概况</h2>
          {rosterError ? (
            <StudioError verb="概况加载" message={rosterError} onRetry={() => setNonce((n) => n + 1)} />
          ) : roster === null ? (
            <StudioLoading>学习概况加载中…</StudioLoading>
          ) : (
            <LearningSnapshot rows={roster} compact />
          )}
        </section>

        <section className="teacher-companion">
          <div>
            <h2>可以询问的内容</h2>
            <p>回答只使用本班的数据。需要操作时，回复下方会给出前往的按钮。</p>
            <ul>
              <li>
                <Icon icon={UserRoundSearch} size={16} />
                谁还没有开始学习，谁需要关注
              </li>
              <li>
                <Icon icon={ClipboardList} size={16} />
                各份作业的完成与逾期情况
              </li>
              <li>
                <Icon icon={Send} size={16} />
                给指定的学生布置作业：印记会带着名单打开布置作业
              </li>
            </ul>
            {onOpenWeekly && (
              <div className="teacher-companion-links">
                <button type="button" className="teacher-link" onClick={onOpenWeekly}>
                  查看周报
                  <Icon icon={ArrowRight} size={14} />
                </button>
              </div>
            )}
          </div>
        </section>
      </div>

      {/* The page offer rides in `cards` too (`withNavigateCard`);
          `keepDataCards` leaves it out, and it renders under the reply. */}
      {shownCards.some((c) => c.kind === "classSnapshot" || c.kind === "students" || c.kind === "assignments") && (
        <SectionHead title="印记给出的数据" aside="来自最近一次回复" />
      )}
      <div className="flex flex-col gap-4">
        {shownCards.map((c, i) =>
          c.kind === "classSnapshot" ? (
            <ClassSnapshotCard key={i} card={c} onOpenStudent={(userId) => open({ view: "student", classId, userId })} />
          ) : c.kind === "students" ? (
            <StudentsCard key={i} card={c} />
          ) : c.kind === "assignments" ? (
            <AssignmentsCard key={i} card={c} onOpenAssignment={(assignmentId) => open({ view: "assignment", assignmentId })} />
          ) : null,
        )}
      </div>
    </WorkspacePanel>
  );
}
