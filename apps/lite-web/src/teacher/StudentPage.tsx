// StudentPage — placeholder. Task 9 fills in the body (roster row detail:
// the student's items list, sourced from `getStudentPage`). The props below
// are final — Task 9 implements against this signature without touching the
// shell that mounts it (`LiteTeacherShell.tsx`).
export function StudentPage({
  classId,
  userId,
  onBack,
  onOpenItem,
}: {
  classId: string;
  userId: string;
  onBack: () => void;
  onOpenItem: (atomId: string) => void;
}) {
  void classId;
  void userId;
  void onBack;
  void onOpenItem;
  return <div className="p-10">加载中…</div>;
}
