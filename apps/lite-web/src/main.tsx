import React from "react";
import { createRoot } from "react-dom/client";
import { rootElementFor } from "./rootElementFor";
import { bootTheme } from "./shared/theme";
import "./index.css";

// Before the first render, so a dark-theme student never sees a white flash.
// A no-op unless she has actually chosen dark — the attribute is absent by
// default and the bare `:root` palette applies, which is why adding the theme
// changes nothing for anyone who has not asked for it.
bootTheme();

// `rootElementFor` decides, per path, between the public report viewer
// (`/s/:token`) and the authenticated `LiteApp` shell — see its own doc
// comment (`./rootElementFor.tsx`) for why that choice belongs here, at the
// composition root, rather than inside `LiteApp`. It lives in its own module
// (not inlined below) so a test can call it directly without triggering this
// file's own `createRoot(...).render(...)` call, which runs at module scope
// the instant this file is imported.
createRoot(document.getElementById("root")!).render(
  <React.StrictMode>{rootElementFor(window.location.pathname)}</React.StrictMode>,
);
