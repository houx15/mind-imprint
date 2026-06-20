import React from "react";
import { createRoot } from "react-dom/client";
import { DevApp } from "./dev/DevApp";
import "./index.css";

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <DevApp />
  </React.StrictMode>,
);
