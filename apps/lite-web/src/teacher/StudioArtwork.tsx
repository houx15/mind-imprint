import type { ReactNode } from "react";
import { ArrowLeft } from "lucide-react";
import { Button, Icon } from "@/ui";
import { studentArtwork } from "../learning/StudentArtwork";

export type StudioArtKind = keyof typeof studentArtwork;

/**
 * 教师端的空状态，照学生端的 `.learning-empty` / `.student-materials-empty`：
 * 插图 + 一句标题 + 一句原因 + 可选的一颗按钮。每个调用处传自己的字，
 * 这里不放任何一页的文案。
 *
 * `compact`：嵌在一个区块里（学生页的作业、家长报告），插图缩小，排成一行。
 */
export function StudioEmpty({
  title,
  children,
  kind = "ideas",
  action,
  compact = false,
}: {
  title: string;
  /** 原因或下一步，一句话。 */
  children?: ReactNode;
  kind?: StudioArtKind;
  action?: { label: string; onClick: () => void };
  compact?: boolean;
}) {
  return (
    <div className={`teacher-empty-state${compact ? " is-compact" : ""}`}>
      <img src={studentArtwork[kind]} alt="" />
      <div>
        <p className="teacher-empty-title">{title}</p>
        {children && <p className="teacher-empty-body">{children}</p>}
        {action && (
          <Button variant="primary" size="sm" onClick={action.onClick}>
            {action.label}
          </Button>
        )}
      </div>
    </div>
  );
}

/**
 * 每个教师页的页头，照学生端的 `LandingHeader`：中文眉题 + 标题 + 一句用途
 * + 插图。`kind` 省略时不放插图（批改、家长报告编辑这类工作台页面）。
 * `actions` 放在说明下面，`children` 放在标题和说明之间（状态标签一类）。
 */
export function StudioHeading({
  kicker,
  title,
  description,
  kind,
  actions,
  children,
}: {
  kicker: string;
  title: ReactNode;
  description?: ReactNode;
  kind?: StudioArtKind;
  actions?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <header className={`teacher-heading${kind ? "" : " is-plain"}`}>
      <div className="teacher-heading-copy">
        <p className="teacher-heading-kicker">{kicker}</p>
        <h1>{title}</h1>
        {children && <div className="teacher-heading-meta">{children}</div>}
        {description && <p className="teacher-heading-desc">{description}</p>}
        {actions && <div className="teacher-heading-actions">{actions}</div>}
      </div>
      {kind && <img src={studentArtwork[kind]} alt="" />}
    </header>
  );
}

/** 返回链接。`label` 写明回到哪里（返回班级 / 返回学生 / 返回作业）。 */
export function BackLink({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <button type="button" onClick={onClick} className="teacher-back">
      <Icon icon={ArrowLeft} size={15} />
      {label}
    </button>
  );
}

/** 加载中，照学生端的 `.learning-loading`。 */
export function StudioLoading({ children = "加载中…" }: { children?: ReactNode }) {
  return (
    <p className="teacher-loading" role="status">
      {children}
    </p>
  );
}

/** 加载失败：「{what}失败：{后台原话}」+ 重新加载，照学生端的 `.learning-error`。 */
export function StudioError({ message, onRetry, verb = "加载" }: { message: string; onRetry?: () => void; verb?: string }) {
  return (
    <div className="teacher-error" role="alert">
      <p>
        {verb}失败：{message}
      </p>
      {onRetry && (
        <button type="button" onClick={onRetry}>
          重新加载
        </button>
      )}
    </div>
  );
}

export type StepState = "done" | "current" | "todo";

/**
 * 一条编号的步骤路径，照学生端阅读室的 `.reading-quest__path`：圆点 + 连线，
 * 已完成 / 当前 / 未到三种状态。只展示，不可点。
 */
export function StepPath({ steps, label }: { steps: { label: string; state: StepState; note?: string }[]; label: string }) {
  return (
    <ol className="teacher-steps" aria-label={label}>
      {steps.map((s, i) => (
        <li key={s.label} data-state={s.state} aria-current={s.state === "current" ? "step" : undefined}>
          <span className="teacher-steps-dot">{s.state === "done" ? "✓" : i + 1}</span>
          <span className="teacher-steps-copy">
            <strong>{s.label}</strong>
            {s.note && <small>{s.note}</small>}
          </span>
        </li>
      ))}
    </ol>
  );
}

/** 表单里的一个编号分组：「01 类型与班级」。 */
export function FormStep({ index, title, note, children }: { index: number; title: string; note?: string; children: ReactNode }) {
  return (
    <section className="teacher-form-step">
      <header>
        <span>{String(index).padStart(2, "0")}</span>
        <div>
          <h2>{title}</h2>
          {note && <p>{note}</p>}
        </div>
      </header>
      <div className="teacher-form-step-body">{children}</div>
    </section>
  );
}

/** 区块标题：左边 h2，右边一句说明或一个链接，照学生端的 `.learning-section-head`。 */
export function SectionHead({ title, aside, id }: { title: string; aside?: ReactNode; id?: string }) {
  return (
    <div className="teacher-section-head">
      <h2 id={id}>{title}</h2>
      {aside !== undefined && <div className="teacher-section-aside">{aside}</div>}
    </div>
  );
}
