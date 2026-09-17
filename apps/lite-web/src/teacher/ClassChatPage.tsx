import { useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { Button, Icon } from "@/ui";
import { api } from "@/api";
import { getRoster, type RosterRow } from "../api/teacher";
import { postWorkspaceTurn } from "../api/teacherWorkspace";
import { errorText, failText, writeLastClassId } from "./assignmentLogic";
import { LearningSnapshot } from "./LearningSnapshot";
import type { TeacherRoute } from "./teacherRouting";
import { navigateOf, navigateRoute, withNavigateCard } from "./workspace/homeLogic";
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

const BACK_CLS =
  "flex items-center gap-1.5 rounded-mk-sm text-mk-small text-mk-muted transition-colors duration-[120ms] ease-mk hover:text-mk-accent-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200";

export function ClassChatPage({
  classId,
  onBack,
  go,
}: {
  classId: string;
  /** 返回班级: the class page. */
  onBack: () => void;
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
        ...("text" in input ? { text: input.text } : { choiceId: input.choiceId, choiceSlug: input.slug }),
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

  const target = navigateRoute(navigateOf(cards), classId);
  function open(route: TeacherRoute) {
    // The assignment form and the parent report list open on the remembered
    // class; make that this class.
    writeLastClassId(classId);
    go(route);
  }

  return (
    <WorkspacePanel
      turns={thread.turns}
      busy={thread.busy}
      error={thread.error}
      choices={choices}
      onSend={(text) => thread.run({ text })}
      onChoose={(choiceId) => {
        const choice = choices.find((c) => c.id === choiceId);
        return thread.run({ choiceId, label: choice?.label ?? choiceId, slug: choice?.slug });
      }}
      composer={thread.composer}
      onComposerChange={thread.setComposer}
      onRetry={thread.retry}
      canRetry={thread.failed !== null}
      intro="AI 根据本班这周的数据回答问题，需要时在回复下方给出前往相关页面的按钮。请输入问题，或选择下面的示例。"
      suggestions={["这周谁还没开始学习？", "哪些作业有学生逾期？", "本周整体情况如何？"]}
      header={
        <>
          <button type="button" onClick={onBack} className={BACK_CLS}>
            <Icon icon={ArrowLeft} size={15} />
            返回班级
          </button>
          <h1 className="teacher-page-title mt-4">{name ?? "班级对话"}</h1>
          {nameError && (
            <p className="mt-1 text-mk-small font-semibold text-mk-danger" role="alert">
              班级加载失败：{nameError}
            </p>
          )}
          <p className="mt-2 text-mk-small text-mk-muted">可询问本班的学习情况和作业进度；印记会在回复下方给出前往相关页面的按钮。</p>
        </>
      }
    >
      <div className="flex flex-col gap-4">

        <section className="rounded-mk-lg border border-mk-border bg-mk-surface p-4">
          <p className="text-mk-label font-bold text-mk-muted">班级概况</p>
          {rosterError ? (
            <p className="mt-2 text-mk-small font-semibold text-mk-danger">
              概况加载失败：{rosterError}{" "}
              <button type="button" onClick={() => setNonce((n) => n + 1)} className="cursor-pointer underline">
                重试
              </button>
            </p>
          ) : roster === null ? (
            <p className="mt-2 text-mk-small text-mk-muted">学习概况加载中…</p>
          ) : (
            <LearningSnapshot rows={roster} compact />
          )}
        </section>

        {/* The page offer rides in `cards` too (`withNavigateCard`); its kind
            matches none of these and renders as the button below. */}
        {cards.map((c, i) =>
          c.kind === "classSnapshot" ? (
            <ClassSnapshotCard key={i} card={c} onOpenStudent={(userId) => open({ view: "student", classId, userId })} />
          ) : c.kind === "students" ? (
            <StudentsCard key={i} card={c} />
          ) : c.kind === "assignments" ? (
            <AssignmentsCard key={i} card={c} onOpenAssignment={(assignmentId) => open({ view: "assignment", assignmentId })} />
          ) : null,
        )}

        {target && (
          <div>
            <Button variant="primary" size="sm" onClick={() => open(target.route)}>
              前往：{target.label}
            </Button>
          </div>
        )}
      </div>
    </WorkspacePanel>
  );
}
