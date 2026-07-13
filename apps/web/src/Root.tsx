import { AppShell } from "./shell/AppShell";
import { Harness } from "./dev/Harness";

// `?demo` renders the deterministic card gallery (Harness): flip through every
// card — proposal → active sheet → completed — with no LLM and zero latency.
// This is the reliable surface for demoing cards (the live summon path is, by
// design, restrained and not guaranteed per opening line).
// Everything else renders the real app — which, since Slice 5d, IS the Studio.
export function Root() {
  const params = new URLSearchParams(window.location.search);
  if (params.has("demo")) return <Harness />;
  return <AppShell />;
}
