import type { Config } from "tailwindcss";
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        mk: {
          // accent (swappable)
          accent: "var(--mk-accent)",
          "accent-50":"var(--mk-accent-50)","accent-100":"var(--mk-accent-100)",
          "accent-200":"var(--mk-accent-200)","accent-300":"var(--mk-accent-300)",
          "accent-400":"var(--mk-accent-400)","accent-500":"var(--mk-accent-500)",
          "accent-600":"var(--mk-accent-600)","accent-700":"var(--mk-accent-700)",
          "accent-800":"var(--mk-accent-800)",
          // neutrals
          paper:"var(--mk-paper)", surface:"var(--mk-surface)", border:"var(--mk-border)",
          ink:"var(--mk-ink)", secondary:"var(--mk-secondary)", muted:"var(--mk-muted)",
          faint:"var(--mk-faint)", "input-border":"var(--mk-input-border)",
          // macaron
          peach:"var(--mk-peach)","peach-bg":"var(--mk-peach-bg)","peach-fg":"var(--mk-peach-fg)",
          butter:"var(--mk-butter)","butter-bg":"var(--mk-butter-bg)","butter-fg":"var(--mk-butter-fg)",
          matcha:"var(--mk-matcha)","matcha-bg":"var(--mk-matcha-bg)","matcha-fg":"var(--mk-matcha-fg)",
          lake:"var(--mk-lake)","lake-bg":"var(--mk-lake-bg)","lake-fg":"var(--mk-lake-fg)",
          mist:"var(--mk-mist)","mist-bg":"var(--mk-mist-bg)","mist-fg":"var(--mk-mist-fg)",
          taro:"var(--mk-taro)","taro-bg":"var(--mk-taro-bg)","taro-fg":"var(--mk-taro-fg)",
          berry:"var(--mk-berry)","berry-bg":"var(--mk-berry-bg)","berry-fg":"var(--mk-berry-fg)",
          // semantic
          success:"var(--mk-success)","success-bg":"var(--mk-success-bg)",
          warning:"var(--mk-warning)","warning-bg":"var(--mk-warning-bg)",
          danger:"var(--mk-danger)","danger-bg":"var(--mk-danger-bg)",
          info:"var(--mk-info)","info-bg":"var(--mk-info-bg)",
          // LEGACY ALIASES (warm the 40 old-token files automatically; Parts 2/3 replace with intent)
          primary:"var(--mk-accent)","primary-hover":"var(--mk-accent-600)","primary-tint":"var(--mk-accent-50)",
          "accent-hover":"var(--mk-accent-600)","accent-tint":"var(--mk-accent-50)",
          green:"var(--mk-success)","green-tint":"var(--mk-success-bg)",
          amber:"var(--mk-warning)",
          bg:"var(--mk-paper)","border-2":"var(--mk-border)",
          input:"var(--mk-input-border)","input-bg":"var(--mk-surface)",
          "muted-2":"var(--mk-faint)",
        },
      },
      borderRadius: {
        "mk-xs":"var(--mk-radius-xs)","mk-sm":"var(--mk-radius-sm)",
        "mk-md":"var(--mk-radius-md)","mk-lg":"var(--mk-radius-lg)","mk-full":"var(--mk-radius-full)",
        mk:"10px", "mk-sheet":"12px", // legacy: mk 14→10, mk-sheet 22→12
      },
      boxShadow: {
        "mk-xs":"var(--mk-shadow-xs)","mk-sm":"var(--mk-shadow-sm)",
        "mk-md":"var(--mk-shadow-md)","mk-lg":"var(--mk-shadow-lg)",
      },
      fontFamily: {
        sans: ["-apple-system","BlinkMacSystemFont","PingFang SC","Segoe UI","system-ui","sans-serif"],
        mono: ["ui-monospace","SF Mono","PingFang SC","monospace"],
      },
      fontSize: {
        "mk-display":["32px",{lineHeight:"1.2",fontWeight:"700"}],
        "mk-h1":["24px",{lineHeight:"1.25",fontWeight:"700"}],
        "mk-h2":["18px",{lineHeight:"1.3",fontWeight:"600"}],
        "mk-h3":["16px",{lineHeight:"1.35",fontWeight:"600"}],
        "mk-body-lg":["16px",{lineHeight:"1.75"}],
        // Composition surfaces only — the writing page, and the mirrored layer
        // that highlights inside it. 14px mk-body is chrome type; a page needs
        // page type. 1.9 leading matters more for Chinese than for Latin: the
        // glyphs are dense and full-height.
        "mk-prose":["17px",{lineHeight:"1.9"}],
        "mk-body":["14px",{lineHeight:"1.6"}],
        "mk-small":["12px",{lineHeight:"1.5"}],
        "mk-caption":["12px",{lineHeight:"1.45",fontWeight:"500"}],
        "mk-label":["11px",{lineHeight:"1.4",fontWeight:"700",letterSpacing:"0.1em"}],
      },
      transitionTimingFunction: { mk: "cubic-bezier(.2,0,0,1)" },
    },
  },
  plugins: [],
} satisfies Config;
