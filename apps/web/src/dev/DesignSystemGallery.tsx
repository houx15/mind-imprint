import { useState, type ReactNode } from "react";
import {
  AccentProvider,
  useAccent,
  ACCENT_PRESETS,
  MACARONS,
  SEMANTIC,
  Icon,
  Search,
  Settings,
  MenuIcon,
  Plus,
  Button,
  IconButton,
  Surface,
  Card,
  CompactRow,
  Input,
  Textarea,
  Select,
  Toggle,
  Radio,
  Checkbox,
  Chip,
  Rating,
  Badge,
  CountBadge,
  Tabs,
  Segmented,
  Progress,
  Stepper,
  Tooltip,
  Modal,
  Drawer,
  Menu,
  toast,
  ToastHost,
  Skeleton,
  SkeletonText,
  SkeletonCard,
  SkeletonRow,
  Pebble,
  PebbleProgress,
  PebbleInlineSpinner,
  RabbitHoleLoader,
  Illustration,
  EmptyState,
  type BadgeTone,
  type IllustrationName,
  type PebbleState,
} from "@/ui";

/**
 * Design-system gallery (design-system foundation, Part 3 Task 13, spec
 * §1-§16). A dev/QA-only surface — reached via `?ds` (see Root.tsx) — that
 * renders every primitive in `ui/` at once so a reviewer can eyeball the
 * whole library and, via the accent picker, watch it retint live. Not part
 * of the product surface: clarity over polish, but nothing here is a stub —
 * every control below is wired to real local state.
 */

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-4">
      <h2 className="text-mk-h2 text-mk-ink">{title}</h2>
      <Surface level="sm" radius="md" className="flex flex-col gap-4 p-5">
        {children}
      </Surface>
    </section>
  );
}

function Swatch({ hex, label, sub }: { hex: string; label: string; sub?: string }) {
  return (
    <div className="flex flex-col gap-1">
      <div
        className="h-14 w-20 rounded-mk-sm border border-mk-border"
        style={{ background: hex }}
      />
      <div className="text-mk-caption text-mk-ink">{label}</div>
      {sub && <div className="text-mk-small text-mk-faint">{sub}</div>}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Accent picker
// ---------------------------------------------------------------------------

function AccentPickerSection() {
  const { id, setAccent } = useAccent();
  const current = ACCENT_PRESETS.find((p) => p.id === id) ?? ACCENT_PRESETS[0];
  return (
    <Section title="Accent 主题选择">
      <p className="text-mk-body text-mk-muted">
        当前主题：<span className="font-medium text-mk-ink">{current.name}</span>
        （点击下方色块切换 — 全页所有 accent 元素会实时跟随重新着色）
      </p>
      <div className="flex flex-wrap gap-3">
        {ACCENT_PRESETS.map((preset) => (
          <button
            key={preset.id}
            type="button"
            data-testid="accent-swatch"
            aria-pressed={preset.id === id}
            aria-label={preset.name}
            onClick={() => setAccent(preset.id)}
            className="flex flex-col items-center gap-1 rounded-mk-sm p-1 focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-mk-accent/15"
          >
            <span
              className="h-10 w-10 rounded-mk-full border-2"
              style={{
                background: preset.scale[500],
                borderColor: preset.id === id ? preset.scale[700] : "transparent",
              }}
            />
            <span className="text-mk-small text-mk-muted">{preset.name}</span>
          </button>
        ))}
      </div>
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Color swatches
// ---------------------------------------------------------------------------

const ACCENT_STEPS = [50, 100, 200, 300, 400, 500, 600, 700, 800] as const;

function ColorSwatchesSection() {
  const { id } = useAccent();
  const current = ACCENT_PRESETS.find((p) => p.id === id) ?? ACCENT_PRESETS[0];
  return (
    <Section title="色彩">
      <div>
        <h3 className="mb-2 text-mk-h3 text-mk-ink">Accent 色阶（{current.name} 50→800）</h3>
        <div className="flex flex-wrap gap-3">
          {ACCENT_STEPS.map((step) => (
            <Swatch key={step} hex={current.scale[step]} label={String(step)} sub={current.scale[step]} />
          ))}
        </div>
      </div>
      <div>
        <h3 className="mb-2 text-mk-h3 text-mk-ink">7 马卡龙辅助色</h3>
        <div className="flex flex-wrap gap-3">
          {Object.entries(MACARONS).map(([name, c]) => (
            <div key={name} className="flex gap-2">
              <Swatch hex={c.base} label={`${name} base`} sub={c.base} />
              <Swatch hex={c.bg} label={`${name} bg`} sub={c.bg} />
            </div>
          ))}
        </div>
      </div>
      <div>
        <h3 className="mb-2 text-mk-h3 text-mk-ink">语义色</h3>
        <div className="flex flex-wrap gap-3">
          {Object.entries(SEMANTIC).map(([name, c]) => (
            <Swatch key={name} hex={c.base} label={name} sub={c.base} />
          ))}
        </div>
      </div>
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Typography
// ---------------------------------------------------------------------------

const TYPE_SCALE: Array<{ label: string; className: string; sample: string }> = [
  { label: "Display", className: "text-mk-display", sample: "思维印记" },
  { label: "H1", className: "text-mk-h1", sample: "中国是否让地球变得更可持续？" },
  { label: "H2", className: "text-mk-h2", sample: "过程评估：你的思维印记" },
  { label: "H3", className: "text-mk-h3", sample: "CRAAP 溯源体检" },
  {
    label: "Body Large",
    className: "text-mk-body-lg",
    sample: "陪练的职责不是给答案，而是在对的时刻把思考塞回给学生。",
  },
  {
    label: "Body",
    className: "text-mk-body",
    sample: "你引用的这篇文章追到了 NASA 的原始数据，这是个好起点。",
  },
  { label: "Small", className: "text-mk-small", sample: "来源：Nature Sustainability, 2023" },
  { label: "Caption", className: "text-mk-caption", sample: "已保存草稿 · 12:03" },
  { label: "Label", className: "text-mk-label", sample: "让步段" },
];

function TypographySection() {
  return (
    <Section title="字体尺度">
      <div className="flex flex-col gap-3">
        {TYPE_SCALE.map((t) => (
          <div key={t.label} className="flex items-baseline gap-4">
            <span className="w-24 shrink-0 text-mk-small text-mk-faint">{t.label}</span>
            <span className={`${t.className} text-mk-ink`}>{t.sample}</span>
          </div>
        ))}
      </div>
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Buttons
// ---------------------------------------------------------------------------

const BUTTON_VARIANTS = ["primary", "secondary", "ghost", "link", "danger"] as const;

function ButtonsSection() {
  return (
    <Section title="按钮">
      <p className="text-mk-small text-mk-muted">
        （鼠标悬停在任意「default」按钮上可看到真实 hover 态 — 颜色加深，这是活的 CSS，不是单独渲染的状态）
      </p>
      <div className="flex flex-col gap-4">
        {BUTTON_VARIANTS.map((variant) => (
          <div key={variant} className="flex flex-wrap items-center gap-3">
            <span className="w-20 shrink-0 text-mk-small text-mk-faint">{variant}</span>
            <Button variant={variant} size="md">
              default md
            </Button>
            <Button variant={variant} size="sm">
              default sm
            </Button>
            <Button variant={variant} size="md" disabled>
              disabled
            </Button>
            <Button variant={variant} size="md" loading>
              loading
            </Button>
          </div>
        ))}
      </div>
      <div>
        <h3 className="mb-2 text-mk-h3 text-mk-ink">IconButton</h3>
        <div className="flex items-center gap-3">
          <IconButton icon={Search} label="搜索" />
          <IconButton icon={Settings} label="设置" size="sm" />
          <IconButton icon={Plus} label="新建" variant="primary" />
          <IconButton icon={MenuIcon} label="菜单" variant="secondary" disabled />
        </div>
      </div>
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Cards / Surfaces
// ---------------------------------------------------------------------------

function CardsSection() {
  return (
    <Section title="卡片 / Surface">
      <div className="flex flex-wrap gap-4">
        <Surface level="hairline" radius="sm" className="p-4 text-mk-body">
          hairline
        </Surface>
        <Surface level="sm" radius="sm" className="p-4 text-mk-body">
          sm
        </Surface>
        <Surface level="md" radius="md" className="p-4 text-mk-body">
          md
        </Surface>
        <Surface level="lg" radius="md" className="p-4 text-mk-body">
          lg
        </Surface>
      </div>
      <Card className="max-w-sm p-4">
        <h3 className="text-mk-h3 text-mk-ink">中国是否让地球变得更可持续？</h3>
        <p className="mt-1 text-mk-body text-mk-muted">Phoebe · 论证写作 · 更新于 2 小时前</p>
      </Card>
      <div className="flex max-w-sm flex-col gap-2">
        <CompactRow title="CRAAP 溯源体检" meta="工具卡 · 已使用 3 次" trailing={<Badge tone="done">完成</Badge>} onClick={() => {}} />
        <CompactRow title="让步段落" meta="工具卡 · 待打开" trailing={<Badge tone="pending">待处理</Badge>} onClick={() => {}} />
      </div>
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Forms
// ---------------------------------------------------------------------------

function FormsSection() {
  const [name, setName] = useState("Phoebe");
  const [emailError, setEmailError] = useState("student@");
  const [notes, setNotes] = useState("这篇文章的核心论点是……");
  const [lens, setLens] = useState("craap");
  const [notifyOn, setNotifyOn] = useState(true);
  const [stance, setStance] = useState("agree");
  const [agreed, setAgreed] = useState(false);
  const [lenses, setLenses] = useState<Set<string>>(new Set(["craap"]));
  const [rating, setRating] = useState(4);

  function toggleLens(key: string, selected: boolean) {
    setLenses((prev) => {
      const next = new Set(prev);
      if (selected) next.add(key);
      else next.delete(key);
      return next;
    });
  }

  return (
    <Section title="表单控件">
      <div className="grid max-w-xl gap-4">
        <div>
          <label className="mb-1 block text-mk-small text-mk-muted">姓名</label>
          <Input value={name} onChange={setName} placeholder="你的名字" />
        </div>
        <div>
          <label className="mb-1 block text-mk-small text-mk-muted">邮箱（示例错误态）</label>
          <Input value={emailError} onChange={setEmailError} error="邮箱格式不完整" />
        </div>
        <div>
          <label className="mb-1 block text-mk-small text-mk-muted">笔记</label>
          <Textarea value={notes} onChange={setNotes} />
        </div>
        <div>
          <label className="mb-1 block text-mk-small text-mk-muted">阅读透镜</label>
          <Select
            value={lens}
            onChange={setLens}
            options={[
              { value: "craap", label: "CRAAP 溯源" },
              { value: "toulmin", label: "Toulmin 论证" },
              { value: "concession", label: "让步段" },
            ]}
          />
        </div>
        <div className="flex items-center gap-2">
          <Toggle checked={notifyOn} onChange={setNotifyOn} label="开启提醒" />
          <span className="text-mk-body text-mk-ink">开启提醒</span>
        </div>
        <div>
          <label className="mb-1 block text-mk-small text-mk-muted">你的立场</label>
          <Radio
            name="stance"
            value={stance}
            onChange={setStance}
            options={[
              { value: "agree", label: "中国正在让地球更可持续" },
              { value: "disagree", label: "中国的碳排放仍是全球第一" },
              { value: "mixed", label: "两者都要承认（让步段）" },
            ]}
          />
        </div>
        <Checkbox checked={agreed} onChange={setAgreed} label="我已核对来源出处" />
        <div>
          <label className="mb-1 block text-mk-small text-mk-muted">多选透镜（Chip）</label>
          <div className="flex flex-wrap gap-2">
            {["craap", "toulmin", "concession", "sift"].map((key) => (
              <Chip
                key={key}
                label={key}
                selected={lenses.has(key)}
                onChange={(selected) => toggleLens(key, selected)}
              />
            ))}
          </div>
        </div>
        <div>
          <label className="mb-1 block text-mk-small text-mk-muted">这次陪练体验打几星？</label>
          <Rating value={rating} onChange={setRating} />
        </div>
      </div>
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Feedback
// ---------------------------------------------------------------------------

const BADGE_TONES: BadgeTone[] = ["progress", "done", "draft", "pending"];

function FeedbackSection() {
  const [tab, setTab] = useState("reading");
  const [segment, setSegment] = useState("week");

  return (
    <Section title="反馈组件">
      <div className="flex flex-wrap items-center gap-2">
        {BADGE_TONES.map((tone) => (
          <Badge key={tone} tone={tone}>
            {tone}
          </Badge>
        ))}
        <CountBadge n={7} />
      </div>
      <div>
        <Tabs
          tabs={[
            { key: "reading", label: "阅读" },
            { key: "writing", label: "写作" },
            { key: "review", label: "回顾" },
          ]}
          value={tab}
          onChange={setTab}
        />
        <p className="mt-2 text-mk-small text-mk-muted">当前 tab：{tab}</p>
      </div>
      <div>
        <Segmented
          options={[
            { value: "week", label: "本周" },
            { value: "month", label: "本月" },
            { value: "all", label: "全部" },
          ]}
          value={segment}
          onChange={setSegment}
        />
      </div>
      <div className="flex max-w-sm flex-col gap-2">
        <Progress value={25} />
        <Progress value={60} />
        <Progress value={90} />
      </div>
      <Stepper steps={["溯源", "论证", "让步段", "回顾"]} current={2} />
      <div>
        <Tooltip label="悬停查看提示气泡">
          <span className="cursor-help text-mk-body text-mk-accent-700 underline decoration-dotted">
            什么是让步段？
          </span>
        </Tooltip>
      </div>
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Overlays
// ---------------------------------------------------------------------------

function OverlaysSection() {
  const [modalOpen, setModalOpen] = useState(false);
  const [drawerLeftOpen, setDrawerLeftOpen] = useState(false);
  const [drawerRightOpen, setDrawerRightOpen] = useState(false);

  return (
    <Section title="浮层">
      <div className="flex flex-wrap gap-3">
        <Button variant="secondary" onClick={() => setModalOpen(true)}>
          打开 Modal
        </Button>
        <Button variant="secondary" onClick={() => setDrawerLeftOpen(true)}>
          打开 Drawer（左）
        </Button>
        <Button variant="secondary" onClick={() => setDrawerRightOpen(true)}>
          打开 Drawer（右）
        </Button>
        <Menu
          trigger={
            <Button variant="secondary" iconStart={<Icon icon={MenuIcon} size={16} />}>
              打开 Menu
            </Button>
          }
          items={[
            { key: "rename", label: "重命名", onSelect: () => {} },
            { key: "export", label: "导出 .docx", onSelect: () => {} },
            { key: "delete", label: "删除项目", onSelect: () => {}, tone: "danger" },
          ]}
        />
        <Button variant="ghost" onClick={() => toast("草稿已自动保存")}>
          触发 Toast
        </Button>
      </div>

      <Modal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        title="确认完成写作？"
        footer={
          <>
            <Button variant="ghost" onClick={() => setModalOpen(false)}>
              取消
            </Button>
            <Button variant="primary" onClick={() => setModalOpen(false)}>
              完成并导出
            </Button>
          </>
        }
      >
        完成后正文将变为只读，过程树进入回顾阶段。你仍可随时导出 .docx 带走成品。
      </Modal>

      <Drawer open={drawerLeftOpen} onClose={() => setDrawerLeftOpen(false)} side="left">
        <h3 className="text-mk-h3 text-mk-ink">探索图谱</h3>
        <p className="mt-2 text-mk-body text-mk-muted">左侧抽屉：兔子洞地图的节点详情。</p>
      </Drawer>
      <Drawer open={drawerRightOpen} onClose={() => setDrawerRightOpen(false)} side="right">
        <h3 className="text-mk-h3 text-mk-ink">陪练</h3>
        <p className="mt-2 text-mk-body text-mk-muted">右侧抽屉：常驻的印记陪练侧栏。</p>
      </Drawer>

      <ToastHost />
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Skeletons
// ---------------------------------------------------------------------------

function SkeletonsSection() {
  return (
    <Section title="骨架屏">
      <div className="flex flex-wrap items-start gap-8">
        <div className="w-48">
          <h3 className="mb-2 text-mk-h3 text-mk-ink">SkeletonText</h3>
          <SkeletonText lines={3} />
        </div>
        <div className="w-48">
          <h3 className="mb-2 text-mk-h3 text-mk-ink">SkeletonCard</h3>
          <SkeletonCard />
        </div>
        <div className="w-64">
          <h3 className="mb-2 text-mk-h3 text-mk-ink">SkeletonRow</h3>
          <SkeletonRow />
        </div>
        <div>
          <h3 className="mb-2 text-mk-h3 text-mk-ink">Skeleton（原始方块）</h3>
          <Skeleton w={120} h={16} />
        </div>
      </div>
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Pebble
// ---------------------------------------------------------------------------

const PEBBLE_STATES: PebbleState[] = ["idle", "thinking", "generating", "processing", "done"];

function PebbleSection() {
  return (
    <Section title="豆豆 Pebble">
      <div className="flex flex-wrap gap-8">
        {[28, 56].map((size) => (
          <div key={size} className="flex flex-col gap-2">
            <span className="text-mk-small text-mk-faint">{size}px</span>
            <div className="flex items-center gap-4">
              {PEBBLE_STATES.map((state) => (
                <div key={state} className="flex flex-col items-center gap-1">
                  <Pebble state={state} size={size} />
                  <span className="text-mk-small text-mk-muted">{state}</span>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Loaders
// ---------------------------------------------------------------------------

function LoadersSection() {
  return (
    <Section title="加载态">
      <div className="flex max-w-md flex-col gap-4">
        <div>
          <span className="mb-1 block text-mk-small text-mk-faint">PebbleProgress（不确定态，循环）</span>
          <PebbleProgress />
        </div>
        <div>
          <span className="mb-1 block text-mk-small text-mk-faint">PebbleProgress（静态 60%）</span>
          <PebbleProgress value={60} />
        </div>
        <div>
          <span className="mb-1 block text-mk-small text-mk-faint">PebbleProgress（slim，静态 90%）</span>
          <PebbleProgress value={90} slim />
        </div>
        <div className="flex items-center gap-3">
          <span className="text-mk-small text-mk-faint">PebbleInlineSpinner</span>
          <PebbleInlineSpinner />
        </div>
        <div>
          <span className="mb-1 block text-mk-small text-mk-faint">RabbitHoleLoader</span>
          <RabbitHoleLoader />
        </div>
      </div>
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Illustrations
// ---------------------------------------------------------------------------

const ILLUSTRATION_SAMPLE: IllustrationName[] = [
  "bookLover",
  "reading",
  "warren",
  "writing",
  "focus",
  "completed",
  "sent",
  "questions",
];

function IllustrationsSection() {
  return (
    <Section title="插画">
      <div className="grid grid-cols-2 gap-6 sm:grid-cols-4">
        {ILLUSTRATION_SAMPLE.map((name) => (
          <div key={name} className="flex flex-col items-center gap-2">
            <Illustration name={name} className="h-24 w-24" />
            <span className="text-mk-small text-mk-muted">{name}</span>
          </div>
        ))}
      </div>
      <div>
        <h3 className="mb-2 text-mk-h3 text-mk-ink">EmptyState</h3>
        <EmptyState
          illustration="emptyProjects"
          title="快来创建你的第一个写作项目吧！"
          body="带上一篇真实的文章或一个真实的写作任务，印记陪你一起想清楚。"
          action={{ label: "新建项目", onClick: () => {} }}
        />
      </div>
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Root
// ---------------------------------------------------------------------------

function GalleryBody() {
  return (
    <div className="mx-auto flex max-w-5xl flex-col gap-10 px-6 py-10">
      <header>
        <h1 className="text-mk-display text-mk-ink">设计系统</h1>
        <p className="mt-2 text-mk-body text-mk-muted">
          思维印记 UI 库全览 — 仅供开发/QA 使用（<code>?ds</code>），非产品页面。
        </p>
      </header>
      <AccentPickerSection />
      <ColorSwatchesSection />
      <TypographySection />
      <ButtonsSection />
      <CardsSection />
      <FormsSection />
      <FeedbackSection />
      <OverlaysSection />
      <SkeletonsSection />
      <PebbleSection />
      <LoadersSection />
      <IllustrationsSection />
    </div>
  );
}

export function DesignSystemGallery() {
  return (
    <AccentProvider>
      <div className="min-h-screen bg-mk-paper">
        <GalleryBody />
      </div>
    </AccentProvider>
  );
}
