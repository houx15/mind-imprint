import { AppShell } from "./shell/AppShell";
import { Harness } from "./dev/Harness";
import { StudioPrototype } from "./proto/StudioPrototype";

// `?demo` renders the deterministic card gallery (Harness): flip through every
// card — proposal → active sheet → completed — with no LLM and zero latency.
// This is the reliable surface for demoing cards (the live summon path is, by
// design, restrained and not guaranteed per opening line).
// `?proto` renders the studio-redesign prototype (design-only, mock data — a
// self-contained sketch of the four-room workspace, wired to no backend).
// Everything else renders the real app — which, since Slice 5d, IS the Studio.
export function Root() {
  const params = new URLSearchParams(window.location.search);
  if (params.has("demo")) return <Harness />;
  if (params.has("proto")) return <StudioPrototype />;
  return <AppShell />;
}
