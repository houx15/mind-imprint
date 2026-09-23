import { useEffect, useState, type ReactNode } from "react";
import type { ApiClient, ClassSummary, Teacher } from "../api";
import { ApiError, CLASS_GRADE_OPTIONS } from "../api";
import { shortDate } from "./time";
import { Button, Input, Select, Surface } from "@/ui";

type Client = Pick<ApiClient, "listClasses" | "createClass" | "listTeachers">;

export function ClassesView({
  client,
  role,
  onOpenClass,
  studioArtwork,
  renderClassPreview,
}: {
  /** Optional Lite studio presentation; Pro keeps its existing layout. */
  studioArtwork?: string;
  renderClassPreview?: (classId: string) => ReactNode;
  client: Client;
  role: string;
  onOpenClass: (id: string) => void;
}) {
  const [classes, setClasses] = useState<ClassSummary[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [lastCreated, setLastCreated] = useState<ClassSummary | null>(null);
  const [teachers, setTeachers] = useState<Teacher[] | null>(null);
  const [teacherId, setTeacherId] = useState("");
  const [grade, setGrade] = useState("");

  const isTeacher = role === "teacher";
  const isAdmin = role === "admin";
  const canCreate = isTeacher || isAdmin;

  function load() {
    setError(null);
    client.listClasses().then(setClasses).catch((e) =>
      setError(e instanceof ApiError ? e.message : "加载失败"),
    );
  }
  useEffect(load, [client]);

  function openCreate() {
    setCreating(true);
    setLastCreated(null);
    if (isAdmin && teachers == null) {
      client.listTeachers().then(setTeachers).catch(() => setTeachers([]));
    }
  }

  async function submit() {
    const trimmed = name.trim();
    if (!trimmed) return;
    if (isAdmin && !teacherId) return;
    setBusy(true);
    setError(null);
    try {
      const input = isAdmin
        ? { name: trimmed, teacher_user_id: teacherId, grade }
        : { name: trimmed, grade };
      const c = await client.createClass(input);
      setLastCreated(c);
      setName("");
      setTeacherId("");
      setGrade("");
      setCreating(false);
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "创建失败");
    } finally {
      setBusy(false);
    }
  }

  const adminNoTeachers = isAdmin && teachers != null && teachers.length === 0;

  return (
    <div className={studioArtwork ? "teacher-classes" : undefined} style={{ flex: 1, minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 980, margin: "0 auto", padding: "44px 40px 60px" }}>
        {studioArtwork && (
          <header className="teacher-heading">
            <div className="teacher-heading-copy">
              <p className="teacher-heading-kicker">教学工作室</p>
              <h1>了解学生的学习过程</h1>
              <p className="teacher-heading-desc">班级卡片汇总本周的学习情况。请进入班级查看名单、布置作业。</p>
            </div>
            <img src={studioArtwork} alt="" />
          </header>
        )}
        <div className={studioArtwork ? "teacher-section-heading" : undefined} style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <div style={{ fontSize: 26, fontWeight: 800, color: "var(--mk-ink)", letterSpacing: "-.01em" }}>
            {isTeacher ? "我的班级" : "全校班级"}{studioArtwork && classes && <small className="teacher-count">{classes.length}</small>}
          </div>
          {canCreate && !creating && (
            <Button onClick={openCreate}>{studioArtwork ? "新建班级" : "+ 新建班级"}</Button>
          )}
        </div>

        {canCreate && creating && (
          <Surface level="md" radius="md" style={{ marginTop: 18, padding: "18px 20px", display: "flex", gap: 12, alignItems: "center", flexWrap: "wrap" }}>
            <div style={{ flex: 1, minWidth: 220 }}>
              <Input
                autoFocus
                placeholder="班级名称，如「11 年级 A · TOK」"
                value={name}
                onChange={setName}
                onKeyDown={(e) => { if (e.key === "Enter") void submit(); }}
              />
            </div>
            {isAdmin && (
              <div style={{ minWidth: 180 }}>
                <Select
                  data-testid="teacher-picker"
                  value={teacherId}
                  onChange={setTeacherId}
                  placeholder="选择教师…"
                  options={(teachers ?? []).map((t) => ({ value: t.id, label: `${t.display_name}（${t.email}）` }))}
                />
              </div>
            )}
            <div style={{ minWidth: 140 }}>
              {/* 不用 Select 的 placeholder：它会在 GRADE_OPTIONS[0]（未填写）之外
                  再插一个 disabled 的 value="" 选项，两个空值互相打架，关闭态永远
                  显示占位文字而不是真正选中的「未填写」。改用可见 label 标出字段。 */}
              <label style={{ display: "block", marginBottom: 4, fontSize: 12, color: "var(--mk-muted)", fontWeight: 600 }}>年级</label>
              <Select
                data-testid="grade-picker"
                value={grade}
                onChange={setGrade}
                options={CLASS_GRADE_OPTIONS}
              />
            </div>
            <Button onClick={() => void submit()} disabled={busy || (isAdmin && !teacherId)}>创建</Button>
            <Button variant="ghost" onClick={() => { setCreating(false); setName(""); setTeacherId(""); setGrade(""); }}>取消</Button>
            {adminNoTeachers && (
              <div style={{ flexBasis: "100%", color: "var(--mk-danger)", fontSize: 13, fontWeight: 600 }}>请先在「教师」生成邀请码，邀请教师注册后再建班。</div>
            )}
          </Surface>
        )}

        {lastCreated && (
          <div style={{ marginTop: 14, background: "var(--mk-accent-50)", border: "1px solid var(--mk-accent-100)", borderRadius: "var(--mk-radius-lg)", padding: "12px 16px", fontSize: 13.5, color: "var(--mk-accent-600)", fontWeight: 600 }}>
            已创建「{lastCreated.name}」· 邀请码 {lastCreated.join_code}（分享给学生加入）
          </div>
        )}

        {error && studioArtwork && (
          <div className="teacher-error" role="alert"><p>加载失败：{error}</p><button type="button" onClick={load}>重新加载</button></div>
        )}
        {error && !studioArtwork && (
          <div style={{ marginTop: 14, color: "var(--mk-danger)", fontSize: 13.5, fontWeight: 600 }}>{error} · <span onClick={load} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span></div>
        )}

        {classes && classes.length === 0 && !creating && studioArtwork && (
          <div className="teacher-empty-state" style={{ marginTop: 20 }}>
            <img src={studioArtwork} alt="" />
            <div>
              <p className="teacher-empty-title">暂无班级</p>
              <p className="teacher-empty-body">{canCreate ? "学生凭邀请码加入班级。请新建班级，并把邀请码发给学生。" : "请联系管理员为你分配班级。"}</p>
              {canCreate && <Button size="sm" onClick={openCreate}>新建班级</Button>}
            </div>
          </div>
        )}
        {classes && classes.length === 0 && !creating && !studioArtwork && (
          <div style={{ marginTop: 28, color: "var(--mk-muted)", fontSize: 14.5, lineHeight: 1.7 }}>
            {isTeacher ? "还没有班级，点「+ 新建班级」创建第一个。" : "本校暂无班级。"}
          </div>
        )}

        {!classes && !error && <p role="status" style={{ marginTop: 24, color: "var(--mk-muted)" }}>加载中…</p>}
        {classes && classes.length > 0 && (
          <div className={studioArtwork ? "teacher-class-grid" : undefined} style={{ display: "grid", gridTemplateColumns: "repeat(2, 1fr)", gap: 16, marginTop: 24 }}>
            {classes.map((c, index) => (
              <Surface
                className={studioArtwork ? "teacher-class-card" : undefined}
                key={c.id}
                as="div"
                level="md"
                radius="md"
                role="button"
                tabIndex={0}
                onClick={() => onOpenClass(c.id)}
                onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onOpenClass(c.id); } }}
                style={{ padding: "18px 20px", cursor: "pointer" }}
              >
                {studioArtwork && <div className="teacher-card-index"><span>班级 {String(index + 1).padStart(2, "0")}</span><span className="teacher-card-open">进入班级 <span aria-hidden="true">→</span></span></div>}
                <div style={{ fontSize: 16, fontWeight: 700, color: "var(--mk-ink)", lineHeight: 1.45 }}>
                  {c.name}
                  {c.grade_label && (
                    <span style={{ marginLeft: 8, fontSize: 12.5, fontWeight: 600, color: "var(--mk-muted)" }}>{c.grade_label}</span>
                  )}
                </div>
                {renderClassPreview?.(c.id)}
                <div className={studioArtwork ? "teacher-class-meta" : undefined} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginTop: 14 }}>
                  <span style={{ fontSize: 12.5, color: "var(--mk-muted)", fontWeight: 600 }}>邀请码 {c.join_code}</span>
                  <span style={{ fontSize: 12, color: "var(--mk-faint)", fontWeight: 500 }}>创建于 {shortDate(c.created_at)}</span>
                </div>
              </Surface>
            ))}
            {studioArtwork && canCreate && !creating && (
              <button type="button" className="teacher-task-new" onClick={openCreate}>
                <strong>新建班级</strong>
                <small>新建后会生成邀请码，学生凭邀请码加入。</small>
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
