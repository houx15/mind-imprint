import { useCallback, useEffect, useState } from "react";
import { Check, ImageOff, Loader2, RefreshCw, Sparkles } from "lucide-react";
import { Icon } from "@/ui";
import {
  clearSiteHero,
  drawSiteHero,
  generatePalettes,
  getSite,
  putSiteLook,
  type SiteState,
} from "../../../api/site";
import { apiErrorText } from "../../../api/errorText";
import { BuiltSite } from "../../../site/BuiltSite";
import { SITE_LAYOUTS } from "../../../site/themes";
import type { SiteLayout, SitePalette } from "../../../site/types";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";

/**
 * Look —— 主页项目第三关：给网站定调子。
 *
 * 产品负责人 2026-09-03：「what is color palette and your choice? style? hero
 * image, do you need? can generate it here - your website is becoming real.」
 *
 * ## 配色不是一排色卡
 *
 * 「挑一个你喜欢的颜色」是一道和这个项目无关的题——她凭直觉点一个，页面就多了
 * 一个她说不出理由的决定。这里的三组配色是从她第一关留下的关键词派生的，每一组
 * 底下就写着「它为什么配那几个词」。她挑的时候，理由已经在屏幕上了。
 *
 * ## 她挑什么，就当场看见什么
 *
 * 每一次点击都直接改右边（窄屏是下面）那一页真实预览。预览用的是公开页
 * `/p/:token` 的同一个组件——做一份「示意图」等于又造一个迟早会和真页面走散的
 * 东西。
 *
 * ## 这一屏没有输入框
 *
 * 挑配色、挑风格、要不要头图，三件都是判断。
 */
export function Look({ tool, onFinish, onClose }: ToolSurfaceProps) {
  const [site, setSite] = useState<SiteState | null>(null);
  const [palettes, setPalettes] = useState<SitePalette[]>([]);
  const [pickedPalette, setPickedPalette] = useState<SitePalette | null>(null);
  const [layout, setLayout] = useState<SiteLayout>("essay");
  const [busy, setBusy] = useState(false);
  const [drawing, setDrawing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    getSite()
      .then((s) => {
        if (cancelled) return;
        setSite(s);
        setLayout(s.layout);
        if (s.palette?.paper) setPickedPalette(s.palette);
      })
      .catch((err) => !cancelled && setError(apiErrorText(err)));
    return () => {
      cancelled = true;
    };
  }, []);

  const suggest = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      setPalettes(await generatePalettes());
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setBusy(false);
    }
  }, []);

  const hero = useCallback(
    async (want: boolean) => {
      setDrawing(true);
      setError(null);
      try {
        setSite(await (want ? drawSiteHero() : clearSiteHero()));
      } catch (err) {
        setError(apiErrorText(err));
      } finally {
        setDrawing(false);
      }
    },
    [],
  );

  const settled = Boolean(site?.palette?.paper);

  return (
    <ToolFrame
      title="视觉基调"
      task="请挑一组配色和一个风格"
      why={tool.reason}
      todo={pickedPalette ? "" : "还没有定下配色"}
      onFinish={() =>
        onFinish(
          { layout, palette: site?.palette ?? null, hasHero: Boolean(site?.heroUrl) },
          site?.palette?.label
            ? `她定下的调子是「${site.palette.label}」，风格是${
                SITE_LAYOUTS.find((l) => l.id === layout)?.name ?? layout
              }`
            : "她还没定调子",
        )
      }
      onClose={onClose}
      busy={busy}
    >
      <div className="flex h-full min-h-0 flex-col lg:flex-row">
        {/* 左：她做判断的地方 */}
        <div className="min-h-0 flex-1 overflow-auto px-4 py-3 lg:max-w-[420px]">
          <div className="flex items-center gap-2">
            <span className="text-mk-label text-mk-secondary">
              {settled ? "已确定" : "待确定"}
            </span>
            <div className="flex-1" />
            <button
              type="button"
              disabled={busy}
              onClick={() => void suggest()}
              className="flex items-center gap-1.5 rounded-mk-full border border-mk-border px-3 py-1.5 text-mk-small text-mk-secondary disabled:opacity-40"
            >
              <Icon icon={busy ? Loader2 : RefreshCw} size={14} />
              {palettes.length ? "换一批" : "看看配色"}
            </button>
          </div>

          {error ? (
            <p className="mt-3 text-mk-small" style={{ color: "var(--mk-danger)" }}>
              {error}
            </p>
          ) : null}

          {/* 配色。每一组底下写着它为什么配她那几个关键词。 */}
          <h3 className="mt-4 text-mk-label text-mk-secondary">配色</h3>
          {palettes.length === 0 ? (
            <p className="mt-1 text-mk-small text-mk-muted">
              配色从你在「受众画像」里留下的关键词派生。
            </p>
          ) : null}
          <div className="mt-2 space-y-2">
            {palettes.map((p) => {
              const on = pickedPalette?.label === p.label;
              return (
                <button
                  key={p.label}
                  type="button"
                  // 配色的名字是模型每次现起的（「旧纸」「工作台」…），按文字找
                  // 必然不稳。这一格是一个**位置**，和 DropField 的 testId 同理。
                  data-testid="palette-option"
                  onClick={() => setPickedPalette(p)}
                  className="w-full rounded-mk-lg border p-3 text-left"
                  style={{ borderColor: on ? "var(--mk-accent-500)" : "var(--mk-border)" }}
                >
                  <div className="flex items-center gap-2">
                    {[p.paper, p.ink, p.accent].map((c) => (
                      <span
                        key={c}
                        className="h-5 w-5 rounded-mk-full border border-mk-border"
                        style={{ background: c }}
                      />
                    ))}
                    <span className="text-mk-body font-semibold text-mk-ink">{p.label}</span>
                  </div>
                  {p.why ? (
                    <p className="mt-1.5 text-mk-small text-mk-secondary">{p.why}</p>
                  ) : null}
                </button>
              );
            })}
          </div>

          {/* 风格 = 版式。三个真正不一样的页面。 */}
          <h3 className="mt-5 text-mk-label text-mk-secondary">风格</h3>
          <div className="mt-2 space-y-2">
            {SITE_LAYOUTS.map((l) => {
              const on = layout === l.id;
              return (
                <button
                  key={l.id}
                  type="button"
                  onClick={() => setLayout(l.id)}
                  className="w-full rounded-mk-lg border p-3 text-left"
                  style={{ borderColor: on ? "var(--mk-accent-500)" : "var(--mk-border)" }}
                >
                  <div className="flex items-baseline justify-between gap-2">
                    <span className="text-mk-body font-semibold text-mk-ink">{l.name}</span>
                    <span className="text-mk-small text-mk-faint">{l.tag}</span>
                  </div>
                  <ul className="mt-1 space-y-0.5">
                    {l.bullets.map((b) => (
                      <li key={b} className="text-mk-small text-mk-secondary">
                        {b}
                      </li>
                    ))}
                  </ul>
                </button>
              );
            })}
          </div>

          {/* 头图。不要，也是一个完整的选择。 */}
          <h3 className="mt-5 text-mk-label text-mk-secondary">头图</h3>
          <p className="mt-1 text-mk-small text-mk-muted">
            照着你的关键词画一张。生成大约需要一分钟。
          </p>
          <div className="mt-2 flex flex-wrap gap-2">
            <button
              type="button"
              disabled={drawing}
              onClick={() => void hero(true)}
              className="flex items-center gap-1.5 rounded-mk-full px-4 py-2 text-mk-body font-semibold text-white disabled:opacity-40"
              style={{ background: "var(--mk-accent-500)" }}
            >
              <Icon icon={drawing ? Loader2 : Sparkles} size={15} />
              {drawing ? "生成中" : site?.heroUrl ? "换一张" : "生成头图"}
            </button>
            {site?.heroUrl ? (
              <button
                type="button"
                disabled={drawing}
                onClick={() => void hero(false)}
                className="flex items-center gap-1.5 rounded-mk-full border border-mk-border px-4 py-2 text-mk-body text-mk-secondary disabled:opacity-40"
              >
                <Icon icon={ImageOff} size={15} />
                不要头图
              </button>
            ) : null}
          </div>

          <button
            type="button"
            disabled={busy || !pickedPalette}
            onClick={async () => {
              if (!pickedPalette) return;
              setBusy(true);
              setError(null);
              try {
                setSite(await putSiteLook(layout, pickedPalette));
              } catch (err) {
                setError(apiErrorText(err));
              } finally {
                setBusy(false);
              }
            }}
            className="mt-5 flex items-center gap-1.5 rounded-mk-full px-4 py-2 text-mk-body font-semibold text-white disabled:opacity-40"
            style={{ background: "var(--mk-accent-500)" }}
          >
            <Icon icon={Check} size={15} />
            确认选择
          </button>
        </div>

        {/* 右：她挑什么就看见什么。预览用的是公开页那一个组件。 */}
        <div
          className="min-h-0 flex-1 overflow-auto border-t border-mk-border p-4 lg:border-l lg:border-t-0"
          style={{ background: "var(--mk-paper)" }}
        >
          {site ? (
            <div className="overflow-hidden rounded-mk-lg border border-mk-border">
              <BuiltSite
                site={site.content}
                layout={layout}
                palette={pickedPalette}
                heroUrl={site.heroUrl}
                editing
              />
            </div>
          ) : null}
        </div>
      </div>
    </ToolFrame>
  );
}
