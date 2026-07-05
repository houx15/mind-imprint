// Per Aspera — bilingual site content. zh is the source of truth; en mirrors it.
// Each entry is a { zh, en } pair so components can pass it straight to `t()`.

export interface Bilingual {
  zh: string;
  en: string;
}

export const brand: Bilingual = {
  zh: "Per Aspera",
  en: "Per Aspera",
};

export const nav: Record<"programs" | "institute" | "about" | "apply", Bilingual> = {
  programs: { zh: "课程", en: "Programs" },
  institute: { zh: "研究院", en: "Institute" },
  about: { zh: "关于我们", en: "About Us" },
  apply: { zh: "预约说明会", en: "Book an info session" },
};

export const footer: {
  disclaimer: Bilingual;
  contact: Bilingual;
  copyright: Bilingual;
} = {
  disclaimer: {
    zh: "Per Aspera 与 Astra Nova School、Ad Astra School、SpaceX 及 Elon Musk 先生无任何关联、授权或合作关系。本站所引用的关于上述学校的信息均来自公开来源，仅作研究与教育用途。",
    en: "Per Aspera is an independent organization, not affiliated with, endorsed by, or connected to Astra Nova School, Ad Astra School, SpaceX, or Mr. Elon Musk. All information cited comes from public sources, for research and educational purposes only.",
  },
  contact: {
    zh: "联系我们：hello@peraspera.org",
    en: "Contact us: hello@peraspera.org",
  },
  copyright: {
    zh: "© 2026 Per Aspera",
    en: "© 2026 Per Aspera",
  },
};
