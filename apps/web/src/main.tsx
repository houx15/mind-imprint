import React from "react";
import { createRoot } from "react-dom/client";
import { Root } from "./Root";
import "./index.css";
import "./shell/report/print/print.css";

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <Root />
  </React.StrictMode>,
);
