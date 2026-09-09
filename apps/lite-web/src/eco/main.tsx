import React from "react";
import ReactDOM from "react-dom/client";
import "../index.css";
import "./web-layout/web-layout.css";
import { WebLayoutDemo } from "./web-layout/WebLayoutDemo";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <WebLayoutDemo />
  </React.StrictMode>,
);
