/**
 * coverGradient — deterministic macaron-tinted cover for a project/card.
 *
 * Hashes a seed string (project id, title, etc.) to deterministically pick
 * one of the 7 MACARONS and build a soft diagonal gradient from it. Same
 * seed always yields the same macaron + gradient — no Math.random, no
 * external state, safe to call on every render or on the server.
 */
import type { CSSProperties } from "react";
import { MACARONS, type MacaronName } from "./tokens";

const MACARON_NAMES = Object.keys(MACARONS) as MacaronName[];

/** Small stable string hash (djb2-ish). Deterministic across runs/platforms. */
function hashSeed(seed: string): number {
  let hash = 5381;
  for (let i = 0; i < seed.length; i++) {
    hash = (hash * 33) ^ seed.charCodeAt(i);
  }
  // Force unsigned 32-bit so index math is never negative.
  return hash >>> 0;
}

export function coverGradient(seed: string): { background: string; macaron: MacaronName } {
  const index = hashSeed(seed) % MACARON_NAMES.length;
  const macaron = MACARON_NAMES[index] ?? "peach";
  const { base, bg } = MACARONS[macaron];
  const background = `linear-gradient(135deg, ${base}, ${bg})`;
  return { background, macaron };
}

export function coverGradientStyle(seed: string): CSSProperties {
  return { backgroundImage: coverGradient(seed).background };
}
