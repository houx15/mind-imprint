import { ImagePlus, MessageSquare, MoveHorizontal, MoveVertical, Plus, Send, Trash2 } from "lucide-react";
import { useState } from "react";
import type { CSSProperties, PointerEvent as ReactPointerEvent } from "react";
import { Icon } from "@/ui";
import { themeFor } from "../../site/themes";
import type { AnnotationRecord, AnnotationScope, ComponentKind, PageComponent, PageDocument } from "./types";

function componentText(component: PageComponent): string {
  return Array.isArray(component.text) ? component.text.join(" · ") : component.text;
}

export function EditableSitePreview({
  page,
  selected,
  annotations,
  draft,
  busy,
  onSelect,
  onDraftChange,
  onSubmit,
  onMove,
  onResize,
  onResizeEnd,
  onResizeHeight,
  onResizeHeightEnd,
  onDelete,
  onAdd,
  onImageChange,
  onTextChange,
}: {
  page: PageDocument;
  selected: AnnotationScope | null;
  annotations: AnnotationRecord[];
  draft: string;
  busy: boolean;
  onSelect: (scope: AnnotationScope) => void;
  onDraftChange: (value: string) => void;
  onSubmit: () => void;
  onMove: (componentId: string, targetBlockId: string, index: number) => void;
  onResize: (componentId: string, span: number) => void;
  onResizeEnd: (componentId: string, from: number, to: number) => void;
  onResizeHeight: (componentId: string, height: number) => void;
  onResizeHeightEnd: (componentId: string, from: number, to: number) => void;
  onDelete: (componentId: string) => void;
  onAdd: (kind: ComponentKind, blockId?: string) => void;
  onImageChange: (componentId: string, src: string) => void;
  onTextChange: (componentId: string, text: string | string[]) => void;
}) {
  const theme = themeFor(page.layout, page.palette);
  const [dragOver, setDragOver] = useState<string | null>(null);
  const [addingToBlock, setAddingToBlock] = useState<string | null>(null);
  const countFor = (id: string) => annotations.filter((item) => item.scopeId === id).length;

  function startResize(event: ReactPointerEvent<HTMLButtonElement>, component: PageComponent) {
    event.preventDefault();
    event.stopPropagation();
    const grid = event.currentTarget.closest(".eco-site-grid");
    if (!grid) return;
    const startX = event.clientX;
    const gridWidth = grid.getBoundingClientRect().width;
    const initial = component.span;
    let latest = initial;
    const move = (moveEvent: globalThis.PointerEvent) => {
      latest = Math.max(2, Math.min(12, Math.round(initial + ((moveEvent.clientX - startX) / gridWidth) * 12)));
      onResize(component.id, latest);
    };
    const finish = () => {
      document.removeEventListener("pointermove", move);
      document.removeEventListener("pointerup", finish);
      onResizeEnd(component.id, initial, latest);
    };
    document.addEventListener("pointermove", move);
    document.addEventListener("pointerup", finish, { once: true });
  }

  function startHeightResize(event: ReactPointerEvent<HTMLButtonElement>, component: PageComponent) {
    event.preventDefault();
    event.stopPropagation();
    const host = event.currentTarget.closest(".eco-site-component");
    if (!host) return;
    const startY = event.clientY;
    const initial = Math.round(host.getBoundingClientRect().height);
    let latest = initial;
    const move = (moveEvent: globalThis.PointerEvent) => {
      latest = Math.max(48, Math.min(640, Math.round((initial + moveEvent.clientY - startY) / 8) * 8));
      onResizeHeight(component.id, latest);
    };
    const finish = () => {
      document.removeEventListener("pointermove", move);
      document.removeEventListener("pointerup", finish);
      onResizeHeightEnd(component.id, initial, latest);
    };
    document.addEventListener("pointermove", move);
    document.addEventListener("pointerup", finish, { once: true });
  }

  return (
    <div
      className="eco-site-canvas"
      data-template={page.templateId}
      style={
        {
          "--st-paper": page.background ?? theme.paper,
          "--st-ink": theme.ink,
          "--st-accent": theme.accent,
          fontFamily: theme.font,
        } as CSSProperties
      }
    >
      <nav className="eco-site-nav">
        <b>{page.siteName ?? "我的网站"}</b>
        <span>{(page.navItems ?? ["首页", "内容", "联系"]).join("　")}</span>
      </nav>

      {page.blocks.map((block, blockIndex) => {
        const blockScope: AnnotationScope = { type: "block", id: block.id, label: `区块：${block.name}` };
        const blockSelected = selected?.id === block.id;
        return (
          <section
            key={block.id}
            role="button"
            tabIndex={0}
            aria-label={`批注${block.name}`}
            data-selected={blockSelected || undefined}
            className="eco-site-block"
            onClick={() => onSelect(blockScope)}
            onKeyDown={(event) => {
              if (event.target !== event.currentTarget) return;
              if (event.key === "Enter" || event.key === " ") onSelect(blockScope);
            }}
          >
            <span className="eco-boundary-label">
              {String(blockIndex + 1).padStart(2, "0")} · {block.name}
              {countFor(block.id) > 0 ? ` · ${countFor(block.id)} 条批注` : ""}
            </span>
            {blockSelected && (
              <InlineComposer draft={draft} busy={busy} label={blockScope.label} onChange={onDraftChange} onSubmit={onSubmit} />
            )}
            <div className="eco-site-grid">
              {block.items.map((component, componentIndex) => {
                const scope: AnnotationScope = {
                  type: "component",
                  id: component.id,
                  label: `组件：${component.type}`,
                };
                const active = selected?.id === component.id;
                const componentStyle = {
                  height: component.height,
                  ...(component.type === "heading" || component.type === "text" ? { color: component.color } : {}),
                  "--eco-component-color": component.color,
                } as CSSProperties;
                return (
                  <div
                    key={component.id}
                    className="eco-component-slot"
                    style={{ gridColumn: `span ${component.span}`, height: component.height }}
                    onDragOver={(event) => { event.preventDefault(); setDragOver(component.id); }}
                    onDragLeave={(event) => {
                      if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setDragOver(null);
                    }}
                    onDrop={(event) => {
                      event.preventDefault();
                      event.stopPropagation();
                      const draggedId = event.dataTransfer.getData("text/plain");
                      setDragOver(null);
                      if (draggedId && draggedId !== component.id) onMove(draggedId, block.id, componentIndex);
                    }}
                  >
                    <div
                      aria-hidden
                      data-active={dragOver === component.id || undefined}
                      className="eco-drop-target"
                      onDragOver={(event) => { event.preventDefault(); setDragOver(component.id); }}
                      onDragLeave={() => setDragOver(null)}
                      onDrop={(event) => {
                        event.preventDefault();
                        const draggedId = event.dataTransfer.getData("text/plain");
                        setDragOver(null);
                        if (draggedId && draggedId !== component.id) onMove(draggedId, block.id, componentIndex);
                      }}
                    />
                    <div
                      role="button"
                      tabIndex={0}
                      draggable
                      data-selected={active || undefined}
                      className={`eco-site-component eco-site-component-${component.type}`}
                      style={componentStyle}
                      onDragStart={(event) => {
                        if ((event.target as HTMLElement).closest(".eco-inline-comment, .eco-inline-text, .eco-resize-handle, .eco-component-controls")) {
                          event.preventDefault();
                          return;
                        }
                        event.dataTransfer.setData("text/plain", component.id);
                        event.dataTransfer.effectAllowed = "move";
                      }}
                      onDragEnd={() => setDragOver(null)}
                      onClick={(event) => {
                        event.stopPropagation();
                        onSelect(scope);
                      }}
                      onKeyDown={(event) => {
                        if (event.target !== event.currentTarget) return;
                        if (event.key === "Enter" || event.key === " ") onSelect(scope);
                      }}
                    >
                      {component.type === "image" ? (
                        component.src ? (
                          <img className="eco-site-image eco-site-uploaded-image" src={component.src} alt="用户插入的页面图片" />
                        ) : (
                          <span
                            className="eco-site-image"
                            aria-label="页面头图"
                            style={component.color ? { background: component.color } : undefined}
                          />
                        )
                      ) : component.type === "color" ? (
                        <span className="eco-site-color" style={{ background: component.color ?? "#cbd5e1" }} aria-label="纯色块" />
                      ) : component.type === "cards" && Array.isArray(component.text) ? (
                        <span className="eco-site-cards">
                          {component.text.map((item, itemIndex) => (
                            <span
                              key={`${component.id}-${itemIndex}`}
                              className="eco-inline-text"
                              contentEditable
                              suppressContentEditableWarning
                              spellCheck
                              title="直接编辑卡片文字"
                              style={component.color ? { borderColor: component.color } : undefined}
                              onPointerDown={(event) => event.stopPropagation()}
                              onFocus={() => onSelect(scope)}
                              onBlur={(event) => {
                                const next = [...component.text as string[]];
                                next[itemIndex] = event.currentTarget.innerText.trim();
                                onTextChange(component.id, next);
                              }}
                            >{item}</span>
                          ))}
                        </span>
                      ) : (
                        <span
                          className="eco-inline-text"
                          contentEditable
                          suppressContentEditableWarning
                          spellCheck
                          title="直接编辑文字"
                          onPointerDown={(event) => event.stopPropagation()}
                          onFocus={() => onSelect(scope)}
                          onKeyDown={(event) => {
                            if (event.key === "Enter" && component.type !== "text") {
                              event.preventDefault();
                              event.currentTarget.blur();
                            }
                          }}
                          onBlur={(event) => {
                            const value = event.currentTarget.innerText.trim();
                            onTextChange(component.id, component.type === "text" ? value : value.replace(/\s*\n\s*/g, " "));
                          }}
                        >{componentText(component)}</span>
                      )}
                      <span className="eco-component-note" aria-hidden>
                        <Icon icon={MessageSquare} size={12} />
                        {countFor(component.id) || "批注"}
                      </span>
                      {active && (
                        <>
                          <div className="eco-component-controls" onClick={(event) => event.stopPropagation()}>
                            {component.type === "image" && (
                              <label className="eco-component-control" title="选择本地图片">
                                <Icon icon={ImagePlus} size={13} />
                                <input
                                  type="file"
                                  accept="image/*"
                                  aria-label="选择本地图片"
                                  onChange={(event) => {
                                    const file = event.target.files?.[0];
                                    if (!file) return;
                                    const reader = new FileReader();
                                    reader.onload = () => typeof reader.result === "string" && onImageChange(component.id, reader.result);
                                    reader.readAsDataURL(file);
                                  }}
                                />
                              </label>
                            )}
                            <button type="button" className="eco-component-control" aria-label={`删除${scope.label}`} onClick={() => onDelete(component.id)}>
                              <Icon icon={Trash2} size={13} />
                            </button>
                          </div>
                          <button
                            type="button"
                            className="eco-resize-handle"
                            aria-label={`拖动调整${scope.label}宽度，当前 ${component.span}/12`}
                            onPointerDown={(event) => startResize(event, component)}
                            onClick={(event) => event.stopPropagation()}
                          >
                            <Icon icon={MoveHorizontal} size={14} />
                          </button>
                          <button
                            type="button"
                            className="eco-resize-handle eco-resize-handle-height"
                            aria-label={`拖动调整${scope.label}高度`}
                            onPointerDown={(event) => startHeightResize(event, component)}
                            onClick={(event) => event.stopPropagation()}
                          >
                            <Icon icon={MoveVertical} size={14} />
                          </button>
                          <InlineComposer draft={draft} busy={busy} label={scope.label} onChange={onDraftChange} onSubmit={onSubmit} />
                        </>
                      )}
                    </div>
                  </div>
                );
              })}
              <div
                aria-hidden
                data-active={dragOver === `${block.id}:end` || undefined}
                className="eco-drop-target eco-drop-target-end"
                onDragOver={(event) => { event.preventDefault(); setDragOver(`${block.id}:end`); }}
                onDragLeave={() => setDragOver(null)}
                onDrop={(event) => {
                  event.preventDefault();
                  const draggedId = event.dataTransfer.getData("text/plain");
                  setDragOver(null);
                  if (draggedId) onMove(draggedId, block.id, block.items.length);
                }}
              />
            </div>
            <div className="eco-canvas-add" onClick={(event) => event.stopPropagation()}>
              <button
                type="button"
                className="eco-canvas-add-trigger"
                aria-expanded={addingToBlock === block.id}
                onClick={() => setAddingToBlock((current) => current === block.id ? null : block.id)}
              >
                <Icon icon={Plus} size={14} />
                新增组件
              </button>
              {addingToBlock === block.id && (
                <div className="eco-component-palette" role="menu" aria-label={`在${block.name}新增组件`}>
                  {([
                    ["heading", "标题"],
                    ["text", "文本框"],
                    ["button", "按钮"],
                    ["color", "纯色块"],
                    ["image", "图片"],
                    ["cards", "卡片"],
                  ] as Array<[ComponentKind, string]>).map(([kind, label]) => (
                    <button
                      key={kind}
                      type="button"
                      role="menuitem"
                      onClick={() => {
                        onAdd(kind, block.id);
                        setAddingToBlock(null);
                      }}
                    >
                      {label}
                    </button>
                  ))}
                </div>
              )}
            </div>
          </section>
        );
      })}
    </div>
  );
}

function InlineComposer({
  draft,
  busy,
  label,
  onChange,
  onSubmit,
}: {
  draft: string;
  busy: boolean;
  label: string;
  onChange: (value: string) => void;
  onSubmit: () => void;
}) {
  return (
    <div
      role="dialog"
      aria-label={`${label}快捷批注`}
      className="eco-inline-comment"
      onClick={(event) => event.stopPropagation()}
      onPointerDown={(event) => event.stopPropagation()}
    >
      <textarea
        rows={2}
        value={draft}
        disabled={busy}
        aria-label="在选中位置批注"
        placeholder={`批注${label}`}
        onChange={(event) => onChange(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) onSubmit();
        }}
      />
      <button type="button" aria-label="发送快捷批注" disabled={busy || !draft.trim()} onClick={onSubmit}>
        <Icon icon={Send} size={14} />
      </button>
    </div>
  );
}
