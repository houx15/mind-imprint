import { useEffect, useState } from "react";
import { listProjects, type Project } from "../api/projects";
import { apiErrorText } from "../api/errorText";
import { ProjectRoom } from "./ProjectRoom";
import { SiteStudio } from "./SiteStudio";

/**
 * 一个项目打开之后是哪个房间。
 *
 * 主页项目（`kind === "website"`，由 spec §4 强制）打开的是 `SiteStudio`；别的
 * 项目打开的是通用的 `ProjectRoom`。
 *
 * 为什么分派在这里而不是在 `ProjectRoom` 内部：`ProjectRoom` 的 hook 在函数体
 * 最前面，中间插一个「先取项目、再决定渲染谁」的早退会把 hook 顺序变成条件性
 * 的。分派抽出来一层，两个房间就都不用知道对方存在。
 */
export function ProjectSurface({ projectId }: { projectId: string }) {
  const [kind, setKind] = useState<"loading" | "website" | "other">("loading");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    listProjects()
      .then((rows: Project[]) => {
        if (cancelled) return;
        const p = rows.find((x) => x.id === projectId);
        setKind(p?.kind === "website" ? "website" : "other");
      })
      .catch((err) => {
        if (cancelled) return;
        // 取不到就按普通项目开——`ProjectRoom` 自己也会报它取不到的东西。
        // 这里静默换成主页工作面才是真的错：她会进到一个不属于这个项目的房间。
        setError(apiErrorText(err));
        setKind("other");
      });
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  if (kind === "loading") return <div className="min-h-full" />;
  if (kind === "website") return <SiteStudio projectId={projectId} />;
  return (
    <>
      {error ? (
        <p className="px-6 pt-4 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      ) : null}
      <ProjectRoom key={projectId} projectId={projectId} />
    </>
  );
}
