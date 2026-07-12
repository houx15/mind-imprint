import { AppShell } from "./shell/AppShell";
import { Harness } from "./dev/Harness";
import { StudioContainer } from "./studio/StudioContainer";

// `?demo` renders the deterministic card gallery (Harness): flip through every
// card — proposal → active sheet → completed — with no LLM and zero latency.
// This is the reliable surface for demoing cards (the live summon path is, by
// design, restrained and not guaranteed per opening line). `?studio` renders
// the live Studio read path (Slice 5b) against a real seeded project.
// Everything else renders the real app.
export function Root() {
  const params = new URLSearchParams(window.location.search);
  if (params.has("demo")) return <Harness />;
  if (params.has("studio")) return <StudioContainer />;
  return <AppShell />;
}
