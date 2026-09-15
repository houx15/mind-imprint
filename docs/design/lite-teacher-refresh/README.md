# Lite teacher studio refresh

Branch: `codex/lite-teacher-visual`. Latest local main `10f065d6` was merged in `783c208b`.

## Presentation

- Reuse the student Lite design tokens, eight accent presets, illustrations and bookmark companion. Theme controls remain in Settings.
- Compact top navigation replaces the sparse sidebar. The class directory previews student counts, weekly active students, cumulative completed readings/writings/projects, and the three most recently active students.
- Preview data comes from the existing teacher roster endpoint. Class cards fetch when near the viewport; no invented metrics or new API contract.
- Class roster gains name search, keyboard-accessible sort controls, copy feedback and a removal confirmation outside the table.
- Student records use a continuous summary and reading/writing/project sections. Reports retain source distinctions, original student output and the existing single prose follow-up.
- Retain main's new copy and `OutputRecord` summaries, including expandable complete records.
- Teacher settings omit student-only local demonstration toggles. Pro defaults are unchanged; shared components receive optional Lite presentation props.

## Verification

- Lite: 698 tests in 72 files passed after the main merge.
- Lite and Pro TypeScript checks passed.
- Lite production build passed. Existing bundle-size warning remains.
- Chromium checked 1440 and 1280 desktop layouts, light/dark appearance, empty states, populated interest tree, report content and Settings.
- Mock API interaction checks passed: search, sorting, class creation, rename, clipboard, invitation code replacement, removal and refresh, error retry, theme selection, and exactly one pending-report prose follow-up. Administrator routes also rendered without JavaScript errors.
- Screenshots and browser checks use fixture accounts and records, not production student data. Backend authorization and production deployment were not exercised in this UI task.

Open `review.html` for the visual review. `verification.json` records browser checks.

## Data visualization iteration

Class cards and class pages now visualize weekly active share and completed/total learning counts. The class activity histogram groups students by 0–7 active days and filters the roster when clicked. Student profiles visualize active-day count and reading/writing/project completion, replacing duplicated numeric tiles. No date-by-date activity or trend is inferred from aggregate data. Charts expose text descriptions and keyboard-operable filters.

Type checking, 698 existing tests, production build and browser activity-filter checks passed. Screenshots use varied fixture classes.

## Shared report and project visuals

`ReportVisualSummary` now serves student reports, their public view and exported posters, and teacher reports. It separates time/words/dialogue metrics from activity-count bars and keeps source attribution intact. Both poster types grow with content and retain all nonzero statistics. `ProjectProgressVisual` serves teacher project records and the student plan; the student retains its interactive step rows and approval actions. No API or process-state changes.

697 tests in 72 files passed. One obsolete CSS-class layout test was removed; real Chromium screenshots cover the layout instead. Type checking and the production build passed. Browser checks rendered both student reports, expanded a project step, and downloaded and visually inspected actual reading/writing PNGs. Teacher navigation and chart-filter checks were repeated.
