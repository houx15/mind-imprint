import React from "react";
import { createRoot } from "react-dom/client";
import { Harness } from "./dev/Harness";
import "./index.css";

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <Harness />
  </React.StrictMode>,
);
