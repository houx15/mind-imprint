import { useEffect, useRef } from "react";

/**
 * Answers "is this component still on screen?" in a way that survives
 * React StrictMode's double-invoked effects.
 *
 * THE TRAP THIS EXISTS FOR — read before deleting it, it has already cost
 * one red e2e run (`writing-walk.spec.ts`, 2026-08-28):
 *
 * `main.tsx` renders the app inside `<React.StrictMode>`, so in dev React
 * mounts every effect, runs its cleanup, then mounts it AGAIN. The obvious
 * hand-rolled shape for a fetch-on-arrival effect combines two guards:
 *
 *   const tried = useRef(false);            // fire the metered call once
 *   useEffect(() => {
 *     if (tried.current) return;
 *     tried.current = true;
 *     let cancelled = false;                // don't setState after unmount
 *     void call().finally(() => { if (!cancelled) setLoading(false); });
 *     return () => { cancelled = true; };
 *   }, [...]);
 *
 * Under StrictMode those two guards disagree:
 *   pass 1   → latch set, request fired
 *   cleanup  → THAT closure's `cancelled` becomes true
 *   pass 2   → skipped, because the latch is already set
 * The one real in-flight request then resolves into the only closure
 * watching it — the one that was told to drop everything. Every `setState`
 * is discarded, the loading flag never clears, and the room hangs forever on
 * a request the server answered perfectly. The bug is invisible to any test
 * that renders without StrictMode, which is why `test/snippetsStage.test.tsx`
 * now renders through it on purpose.
 *
 * `alive` does not have that disagreement, because it is restored to `true`
 * by StrictMode's remount and only goes `false` on a real unmount. Pair it
 * with the once-latch and both properties hold at the same time: the call
 * fires exactly once, and its answer is always applied unless the component
 * is genuinely gone.
 *
 *   const alive = useAlive();
 *   ... .finally(() => { if (alive.current) setLoading(false); });
 *
 * A plain `let cancelled` is still correct for an effect with NO once-latch
 * (a cheap GET that StrictMode simply re-fires, second answer wins). The
 * trap is only the combination.
 */
export function useAlive() {
  const alive = useRef(true);
  useEffect(() => {
    // Set on every mount, not just the first: StrictMode's synthetic
    // unmount runs the cleanup below before remounting, and without this
    // line the ref would stay false for the rest of the component's life.
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  return alive;
}
