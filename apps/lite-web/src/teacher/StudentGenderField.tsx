import { useEffect, useRef, useState } from "react";
import { ApiError } from "../api/client";
import { setStudentGender, type StudentGender } from "../api/teacher";
import { Segmented } from "./formParts";

const GENDER_OPTIONS: { value: StudentGender; label: string }[] = [
  { value: "", label: "未设置" },
  { value: "female", label: "女" },
  { value: "male", label: "男" },
];

/**
 * StudentGenderField — the teacher sets a student's gender on the student
 * page. The AI on the teacher end (class summary, class chat, parent report)
 * reads it to choose 她 or 他; with 未设置 it uses neither.
 *
 * A click saves at once. Only the latest click's answer is applied, so two
 * quick clicks cannot land out of order; a failed save puts the stored value
 * back and says why.
 */
export function StudentGenderField({
  classId,
  userId,
  initial,
}: {
  classId: string;
  userId: string;
  initial: StudentGender;
}) {
  const [value, setValue] = useState<StudentGender>(initial);
  const [status, setStatus] = useState<"idle" | "saving" | "saved" | "error">("idle");
  const [error, setError] = useState("");
  const stored = useRef<StudentGender>(initial);
  const seq = useRef(0);

  useEffect(() => {
    setValue(initial);
    stored.current = initial;
    setStatus("idle");
    seq.current++;
  }, [classId, userId, initial]);

  const change = (next: StudentGender) => {
    if (next === value) return;
    const mine = ++seq.current;
    setValue(next);
    setStatus("saving");
    setStudentGender(classId, userId, next)
      .then((saved) => {
        if (mine !== seq.current) return;
        stored.current = saved;
        setValue(saved);
        setStatus("saved");
      })
      .catch((e: unknown) => {
        if (mine !== seq.current) return;
        setValue(stored.current);
        setError(e instanceof ApiError ? e.message : String(e));
        setStatus("error");
      });
  };

  return (
    <div className="mt-4 flex flex-wrap items-end gap-x-4 gap-y-2">
      <Segmented label="性别" options={GENDER_OPTIONS} value={value} onChange={change} />
      <p className="pb-2 text-mk-small text-mk-muted">
        AI 撰写摘要和家长报告时按此使用「他」或「她」；未设置时不使用这两个代词。
        {status === "saving" && <span className="ml-2">保存中</span>}
        {status === "saved" && <span className="ml-2 text-mk-accent-700">已保存</span>}
        {status === "error" && <span className="ml-2 font-semibold text-mk-danger">保存失败：{error}</span>}
      </p>
    </div>
  );
}
