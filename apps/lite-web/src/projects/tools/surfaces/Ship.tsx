import { useCallback, useEffect, useState } from "react";
import { Check, Link2, Monitor, RefreshCw, Smartphone } from "lucide-react";
import { Icon } from "@/ui";
import { getSite, publishSite, revokeSite, type SiteState } from "../../../api/site";
import { apiErrorText } from "../../../api/errorText";
import { BuiltSite } from "../../../site/BuiltSite";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";

/**
 * Ship —— 第五关的最后一下：看这一页，然后上线。
 *
 * ## 这里没有一个输入框，这是设计
 *
 * 产品负责人 2026-09-03 否掉的就是那九个带 label 的输入框（"don't let students
 * enter forms"）。她的字不在这里敲——她在对话里回答印记的问题，印记把她的原话
 * 摆到页面上（`site_content` 产出 + `GroundSiteDraft` 那道闸）。这一屏只做三件
 * 她要**判断**的事：看这一页、看还缺什么、决定要不要放出去。
 *
 * ## 预览就是真页面
 *
 * `BuiltSite` 是公开页 `/p/:token` 用的同一个组件。做一份「示意图」等于又造一个
 * 迟早会和真页面走散的东西——她照着示意图点了上线，别人看到的却是另一页。
 *
 * ## 缺什么由服务端说
 *
 * `missing` 来自 `pbl.SiteMissing`：页面上没有她自己的字就发不出去。前端不自己
 * 判一遍——两处判断迟早会有一处说「可以发」而另一处拒绝，而她看到的是一个按了
 * 没反应的按钮。
 */
export function Ship({ tool, onFinish, onClose }: ToolSurfaceProps) {
  const [state, setState] = useState<SiteState | null>(null);
  const [narrow, setNarrow] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      setState(await getSite());
    } catch (err) {
      setError(apiErrorText(err));
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const act = useCallback(
    async (fn: () => Promise<unknown>) => {
      setBusy(true);
      setError(null);
      try {
        await fn();
        setState(await getSite());
      } catch (err) {
        setError(apiErrorText(err));
      } finally {
        setBusy(false);
      }
    },
    [],
  );

  if (!state) {
    return (
      <ToolFrame
        title="上线"
        task="请查看这一页，确认之后放出去"
        why={tool.reason}
        todo="处理中"
        onFinish={() => onFinish({}, "")}
        onClose={onClose}
      >
        <div className="p-4">
          {error ? (
            <p className="text-mk-small" style={{ color: "var(--mk-danger)" }}>
              {error}
            </p>
          ) : null}
        </div>
      </ToolFrame>
    );
  }

  const missing = state.missing ?? [];
  return (
    <ToolFrame
      title="上线"
      task={state.published ? "这一页已经在线上。请确认无误" : "请查看这一页，确认之后放出去"}
      why={tool.reason}
      // 还缺她自己的字的时候，「完成」不给按——服务端也拦着（发布会被拒），
      // 这里只是把话说在前面。
      todo={missing.length ? `还缺 ${missing.length} 处你自己的话` : ""}
      onFinish={() =>
        onFinish(
          { published: state.published, url: state.url },
          state.published ? `主页已上线：${state.url}` : "看过这一页了，还没放出去",
        )
      }
      onClose={onClose}
      busy={busy}
    >
      <div className="flex h-full min-h-0 flex-col">
        <div className="flex flex-wrap items-center gap-2 border-b border-mk-border px-4 py-2.5">
          <span className="text-mk-label text-mk-secondary">
            {state.published ? "已上线" : "待上线"}
          </span>
          <div className="flex-1" />
          <button
            type="button"
            onClick={() => setNarrow(false)}
            aria-label="宽屏预览"
            className="rounded-mk-full p-1.5"
            style={{ color: narrow ? "var(--mk-faint)" : "var(--mk-accent-500)" }}
          >
            <Icon icon={Monitor} size={16} />
          </button>
          <button
            type="button"
            onClick={() => setNarrow(true)}
            aria-label="手机预览"
            className="rounded-mk-full p-1.5"
            style={{ color: narrow ? "var(--mk-accent-500)" : "var(--mk-faint)" }}
          >
            <Icon icon={Smartphone} size={16} />
          </button>
          <button
            type="button"
            onClick={() => void load()}
            aria-label="刷新预览"
            className="rounded-mk-full p-1.5 text-mk-faint hover:text-mk-secondary"
          >
            <Icon icon={RefreshCw} size={16} />
          </button>
        </div>

        {error ? (
          <p className="px-4 pt-3 text-mk-small" style={{ color: "var(--mk-danger)" }}>
            {error}
          </p>
        ) : null}

        {missing.length ? (
          <div className="mx-4 mt-3 rounded-mk-lg border border-mk-border p-3">
            <h3 className="text-mk-label text-mk-secondary">还缺你自己的话</h3>
            <ul className="mt-1.5 space-y-1">
              {missing.map((m) => (
                <li key={m} className="text-mk-small text-mk-secondary">
                  · {m}
                </li>
              ))}
            </ul>
            {/* 🚨 不给输入框。缺的那几处回对话里说给印记，它摆上去——这一屏
                是判断的地方，不是填空的地方。 */}
            <p className="mt-2 text-mk-small text-mk-muted">
              请回到对话，把这几处讲给印记，它会放到页面上。
            </p>
          </div>
        ) : null}

        <div className="min-h-0 flex-1 overflow-auto p-4">
          <div
            className="mx-auto overflow-hidden rounded-mk-lg border border-mk-border"
            style={{ width: narrow ? 390 : "100%", maxWidth: "100%" }}
          >
            {/* 🚨 narrow 要传下去。它是 prop 而不是媒体查询：`md:` 读到的是
                真实视口，会把 390px 的手机框排成桌面版——恰好在她检查手机效果
                的那一刻排错。见 BuiltSite 的文件头。 */}
            <BuiltSite
              site={state.content}
              layout={state.layout}
              palette={state.palette}
              heroUrl={state.heroUrl}
              narrow={narrow}
              editing
            />
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-3 border-t border-mk-border px-4 py-3">
          {state.published ? (
            <>
              <a
                href={state.url}
                target="_blank"
                rel="noreferrer"
                className="flex items-center gap-1.5 text-mk-small text-mk-accent-600"
              >
                <Icon icon={Link2} size={14} />
                {state.url}
              </a>
              <div className="flex-1" />
              <button
                type="button"
                disabled={busy}
                onClick={() => void act(revokeSite)}
                className="rounded-mk-full border border-mk-border px-4 py-2 text-mk-body text-mk-secondary disabled:opacity-40"
              >
                撤回链接
              </button>
            </>
          ) : (
            <>
              <span className="text-mk-small text-mk-muted">
                链接不可索引，只有拿到它的人打得开。随时可以撤回。
              </span>
              <div className="flex-1" />
              <button
                type="button"
                disabled={busy || missing.length > 0}
                onClick={() => void act(publishSite)}
                className="flex items-center gap-1.5 rounded-mk-full px-4 py-2 text-mk-body font-semibold text-white disabled:opacity-40"
                style={{ background: "var(--mk-accent-500)" }}
              >
                <Icon icon={Check} size={15} />
                上线
              </button>
            </>
          )}
        </div>
      </div>
    </ToolFrame>
  );
}
