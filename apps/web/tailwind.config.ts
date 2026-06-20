import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        mk: {
          primary: "#2A3B7A",
          "primary-hover": "#22305F",
          "primary-tint": "#EDEFF9",
          accent: "#D98263",
          "accent-hover": "#CC7355",
          "accent-tint": "#FBEEE7",
          green: "#4C9A82",
          "green-tint": "#E7F3EE",
          amber: "#E8A33D",
          ink: "#1C2333",
          muted: "#8A92A3",
          "muted-2": "#9AA1B0",
          bg: "#F3F4F8",
          surface: "#FFFFFF",
          border: "#EAECF2",
          "border-2": "#ECEEF3",
          input: "#E1E4ED",
          "input-bg": "#FCFCFD",
        },
      },
      borderRadius: { mk: "14px", "mk-lg": "18px", "mk-sheet": "22px" },
      fontFamily: { sans: ["Plus Jakarta Sans", "Noto Sans SC", "system-ui", "sans-serif"] },
    },
  },
  plugins: [],
} satisfies Config;
