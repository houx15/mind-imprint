# 陪练链接桥 (Coach Link Bridge) · Spec

> 2026-07-30. Fixes the one real gap surfaced by the resource-flow audit: a URL
> dropped into the project coach chat (esp. proposal-formation) is inert — stored
> as plain `chat_message` text, never promoted to a reference, no path to the
> Reading Room. Audit found #1 (lazy fetch) and #4 (activity-log granularity) are
> healthy by design; this bridge also incidentally covers #1's "added but no
> content" case (read-together fetches on enter-reading).

## Behavior (铁律: 触发是自动的，但打开由学生确认)

1. **Detect (server, free, no spend).** On every coach turn, extract the FIRST
   `http(s)` URL in the student's message. If it is NOT already a reference in the
   project's library (exact-URL dedup against `ListReferences`), attach a
   `linkOffer: {url}` to the coach reply JSON. Detection never fails the turn.
2. **Offer (client, 克制).** Under the coach reply, render a gentle, dismissable
   chip: `[加入文献库]` and `[一起读这篇]`. NOTHING fetches or opens on render.
3. **Confirm (student's tap).**
   - `加入文献库` → `createReference({url, title:url})`. The link becomes a
     first-class reference (fixes #3). Chip shows `已加入文献库 ✓`.
   - `一起读这篇` → `createReference` (once) then navigate to the Reading Room
     (`onOpenRoom("reading")`), where the existing `进入阅读室` flow fetches the
     body (fixes #2, and #1's no-content case). No new fetch/entry logic.
   - `跳过` → hide locally. No server record — a per-turn, spend-free, stateless
     detection has nothing to "keep offering" (不操纵).

## Constraints

- Detection is scope-agnostic on the server (any coach scope returns the offer).
- This slice renders the chip only in the **forming** coach (`FormingPhase` in
  `PlanBlock.tsx`) — the described gap. Other rails may adopt it later; because
  the offer costs nothing, a server-returned offer with no renderer is not the
  S4 "invisible paid offer" class.
- `extractFirstURL` is pure (unit-tested without a DB); dedup is exact-URL.
- No prompt change — the coach model is untouched; detection is post-hoc.

## Files

- `apps/api/internal/api/coach_link.go` (new): `extractFirstURL`, `linkOfferDTO`,
  `(*API).detectLinkOffer`.
- `apps/api/internal/api/coach.go`: attach `resp["linkOffer"]` when present.
- `apps/api/internal/api/coach_link_test.go` (new, `package api`): extractor units.
- `apps/api/internal/api/coach_link_integration_test.go` (new, `package api_test`):
  offer present; no offer when URL already a reference.
- `apps/web/src/workspace/api/workspace.ts`: `LinkOfferWire` + `CoachResult.linkOffer`.
- `apps/web/src/workspace/blocks/CoachLinkOffer.tsx` (new): the chip.
- `apps/web/src/workspace/blocks/PlanBlock.tsx`: thread offer through
  `runCoachTurn`; wire `onAddLink` / `onReadTogether` / dismiss into `FormingPhase`.
