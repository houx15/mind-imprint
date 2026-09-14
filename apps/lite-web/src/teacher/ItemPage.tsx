// ItemPage — placeholder. Task 9 fills in the body (one item's detail:
// reading/writing/project payload from `getItem`). The props below are
// final — Task 9 implements against this signature without touching the
// shell that mounts it (`LiteTeacherShell.tsx`).
export function ItemPage({
  classId,
  userId,
  atomId,
  onBack,
}: {
  classId: string;
  userId: string;
  atomId: string;
  onBack: () => void;
}) {
  void classId;
  void userId;
  void atomId;
  void onBack;
  return <div className="p-10">加载中…</div>;
}
