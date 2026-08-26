import React from "react";
import { createRoot } from "react-dom/client";
import { LiteApp } from "./LiteApp";
import "./index.css";

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <LiteApp />
  </React.StrictMode>,
);
