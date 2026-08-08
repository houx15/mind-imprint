import { useState, type CSSProperties, type ReactNode } from "react";
import { LogOut } from "lucide-react";
import type { SessionStore } from "../session";
import type { MeUser } from "../../api";
import { useAccent, ACCENT_PRESETS } from "../../ui/accent";
import { Card, Toggle, Pebble, Icon, Check } from "../../ui";

/**
 * SettingsView — restyle + server-persisted accent picker (platform shell
 * rebuild, Task 9).
 *
 * Replaces the old local-only "AI 形象" 4-hex swatch picker (which wrote to
 * `session.aiAvatar`/`setAvatar` in localStorage only) with the real 8-preset
 * accent picker from `ui/accent`: selection lives in `useAccent()`, which is
 * provided above this view by `StudentApp`'s `<AccentProvider>` — clicking a
 * swatch there both retints the whole app live and fires `onPersist` (wired
 * to `api.setAccent`) so the choice survives a reload on another device.
 *
 * The 个人 section now shows the real signed-in `MeUser` (display name,
 * email, school, classes) instead of the hardcoded "Phoebe Chen" demo copy;
 * with no user it shows a plain placeholder rather than fake data.
 */

const TOGGLES_DEFAULT = [
  { label: "自动触发工具卡", desc: "分析你输入的内容，在合适时机弹出对应工具卡。", on: true },
  { label: "过程记录", desc: "将每次工具卡填写和对话节点保存到过程树。", on: true },
  { label: "使用统计", desc: "帮助改进工具推荐与陪练策略（数据不出设备）。", on: false },
];

function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

function SectionLabel({ children }: { children: ReactNode }) {
  return <div className="mb-3 mt-8 text-mk-label text-mk-faint first:mt-0">{children}</div>;
}

export function SettingsView({
  session: _session,
  onLogout,
  user = null,
}: {
  session: SessionStore;
  onLogout: () => void;
  user?: MeUser | null;
}) {
  const { id: accentId, setAccent } = useAccent();
  const [toggles, setToggles] = useState(TOGGLES_DEFAULT);

  function flipToggle(index: number) {
    setToggles((prev) => prev.map((t, i) => (i === index ? { ...t, on: !t.on } : t)));
  }

  const orgLabel = user
    ? [user.school.name, user.classes.map((c) => c.name).join(" · ") || null].filter(Boolean).join(" · ")
    : null;

  return (
    <div className="h-full w-full overflow-y-auto bg-mk-paper">
      <div className="mx-auto max-w-[680px] px-10 py-10 pb-16">
        <h1 className="text-mk-h1 text-mk-ink">设置</h1>

        {/* === 个人 === */}
        <SectionLabel>个人</SectionLabel>
        <Card className="p-6">
          <div className="mb-5 flex items-center gap-4">
            <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-mk-md bg-mk-accent text-mk-h2 font-semibold text-white">
              {user ? user.display_name.slice(0, 1) : "?"}
            </div>
            <div className="min-w-0">
              <div className="truncate text-mk-h3 text-mk-ink">{user?.display_name ?? "未登录"}</div>
              <div className="mt-0.5 truncate text-mk-small text-mk-muted">
                {orgLabel ?? "登录后在这里看到你的学校与班级"}
              </div>
            </div>
          </div>

          <div className="mb-1.5 text-mk-caption text-mk-secondary">姓名</div>
          <input
            value={user?.display_name ?? ""}
            placeholder="—"
            readOnly
            className="mb-3.5 w-full rounded-mk-sm border border-mk-input-border bg-mk-paper px-3.5 py-2.5 text-mk-body text-mk-ink outline-none"
          />
          <div className="mb-1.5 text-mk-caption text-mk-secondary">邮箱</div>
          <input
            value={user?.email ?? ""}
            placeholder="—"
            readOnly
            className="w-full rounded-mk-sm border border-mk-input-border bg-mk-paper px-3.5 py-2.5 text-mk-body text-mk-ink outline-none"
          />
        </Card>

        {/* === 主题色 (accent picker, replaces the old local-only AI 形象 4-hex swatches) === */}
        <SectionLabel>主题色</SectionLabel>
        <Card className="p-6">
          <div className="flex items-center gap-4">
            <Pebble size={56} />
            <div className="min-w-0 flex-1">
              <div className="text-mk-h3 text-mk-ink">你的陪练 · 印记</div>
              <div className="mt-1 text-mk-body text-mk-muted">
                它克制、安静，一次只问你一个问题。选一个你看着舒服的颜色。
              </div>
            </div>
          </div>

          <div className="mt-5 flex flex-wrap gap-3">
            {ACCENT_PRESETS.map((preset) => {
              const selected = preset.id === accentId;
              return (
                <button
                  key={preset.id}
                  type="button"
                  data-testid="accent-swatch"
                  aria-pressed={selected}
                  aria-label={preset.name}
                  onClick={() => setAccent(preset.id)}
                  className={cx(
                    "flex flex-col items-center gap-1.5 rounded-mk-sm p-2 transition-colors duration-[120ms] ease-mk",
                    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                    selected ? "bg-mk-accent-50" : "bg-transparent",
                  )}
                >
                  <span
                    className="relative flex h-11 w-11 items-center justify-center"
                    style={{ "--mk-accent-500": preset.scale[500] } as CSSProperties}
                  >
                    <Pebble size={40} />
                    {selected && (
                      <span className="absolute -bottom-0.5 -right-0.5 flex h-4 w-4 items-center justify-center rounded-mk-full bg-mk-ink text-white">
                        <Icon icon={Check} size={11} />
                      </span>
                    )}
                  </span>
                  <span className="text-mk-small text-mk-muted">{preset.name}</span>
                </button>
              );
            })}
          </div>
        </Card>

        {/* === 其他 === */}
        <SectionLabel>其他</SectionLabel>
        <Card className="overflow-hidden">
          {toggles.map((t, i) => (
            <div
              key={t.label}
              className={cx(
                "flex items-center justify-between gap-4 px-5 py-4",
                i < toggles.length - 1 && "border-b border-mk-border",
              )}
            >
              <div className="min-w-0">
                <div className="text-mk-body font-medium text-mk-ink">{t.label}</div>
                <div className="mt-0.5 text-mk-small text-mk-faint">{t.desc}</div>
              </div>
              <Toggle checked={t.on} onChange={() => flipToggle(i)} label={t.label} />
            </div>
          ))}
        </Card>

        {/* === 退出登录 === */}
        <button
          type="button"
          onClick={onLogout}
          className="mt-8 inline-flex items-center gap-2 rounded-mk-sm px-3 py-2 text-mk-body font-medium text-mk-danger hover:bg-mk-danger-bg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
        >
          <Icon icon={LogOut} size={17} />
          退出登录
        </button>
      </div>
    </div>
  );
}
