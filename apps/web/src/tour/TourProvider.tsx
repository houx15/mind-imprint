import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import type { TourController, TourJourney, TourNavContext, TourSegment } from "./types";

const noop = () => {};
const TourContext = createContext<TourController>({
  running: false, segment: null, step: null, segmentIndex: 0, stepIndex: 0, progress: 0,
  play: noop, next: noop, prev: noop, skipSegment: noop, stop: noop,
});

function asJourney(target: TourSegment | TourJourney): TourJourney {
  return Array.isArray(target) ? target : [target];
}

export function TourProvider({ nav, onComplete, children }: {
  nav: TourNavContext;
  onComplete?: () => void;
  children: ReactNode;
}) {
  const [journey, setJourney] = useState<TourJourney | null>(null);
  const [segIdx, setSegIdx] = useState(0);
  const [stepIdx, setStepIdx] = useState(0);
  // Keep the latest nav/onComplete without re-subscribing effects.
  const navRef = useRef(nav); navRef.current = nav;
  const doneRef = useRef(onComplete); doneRef.current = onComplete;

  const running = journey != null;
  const segment = running ? journey![segIdx] ?? null : null;
  const step = segment ? segment.steps[stepIdx] ?? null : null;

  const finish = useCallback(() => {
    setJourney(null); setSegIdx(0); setStepIdx(0);
    doneRef.current?.();
  }, []);

  const play = useCallback((target: TourSegment | TourJourney) => {
    setJourney(asJourney(target)); setSegIdx(0); setStepIdx(0);
  }, []);

  const advanceTo = useCallback((nextSeg: number, nextStep: number) => {
    setSegIdx(nextSeg); setStepIdx(nextStep);
  }, []);

  const nextStable = useCallback(() => {
    if (!journey) return;
    const seg = journey[segIdx];
    if (!seg) return;
    if (stepIdx + 1 < seg.steps.length) { advanceTo(segIdx, stepIdx + 1); return; }
    if (segIdx + 1 < journey.length) { advanceTo(segIdx + 1, 0); return; }
    finish();
  }, [journey, segIdx, stepIdx, advanceTo, finish]);

  const prevStable = useCallback(() => {
    if (!journey) return;
    if (stepIdx > 0) { advanceTo(segIdx, stepIdx - 1); return; }
    if (segIdx > 0) { const ps = journey[segIdx - 1]; advanceTo(segIdx - 1, Math.max(0, (ps?.steps.length ?? 1) - 1)); }
  }, [journey, segIdx, stepIdx, advanceTo]);

  const skipSegment = useCallback(() => {
    if (!journey) return;
    if (segIdx + 1 < journey.length) advanceTo(segIdx + 1, 0);
    else finish();
  }, [journey, segIdx, advanceTo, finish]);

  // onEnter fires whenever the active step changes while running.
  useEffect(() => {
    if (!running || !step) return;
    void step.onEnter?.(navRef.current);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [running, segIdx, stepIdx]);

  const progress = useMemo(() => {
    if (!journey) return 0;
    const total = journey.reduce((n, s) => n + s.steps.length, 0);
    const done = journey.slice(0, segIdx).reduce((n, s) => n + s.steps.length, 0) + stepIdx;
    return total ? done / total : 0;
  }, [journey, segIdx, stepIdx]);

  const value: TourController = {
    running, segment, step, segmentIndex: segIdx, stepIndex: stepIdx, progress,
    play, next: nextStable, prev: prevStable, skipSegment, stop: finish,
  };
  return <TourContext.Provider value={value}>{children}</TourContext.Provider>;
}

export const useTour = (): TourController => useContext(TourContext);
