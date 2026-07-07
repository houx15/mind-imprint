# Slice 5 · My-Evaluation + Settings Polish — Design Spec

> 2026-07-07 · Part of `docs/2026-07-06-refactor-roadmap.md` (Slice 5 of 5, the final polish). Stacked on `slice-4-courses`.
> Binding design: `思维印记 工作区.dc.html` — `RECORDS` (我的评估) + `SETTINGS` sections.

## Finding: the reskin was already complete

Slices 1–4 built `RecordsView` and `SettingsView` faithfully to the binding design. On review against the `.dc.html`:

- **`SettingsView`** already matches the design exactly — three sections (个人 / AI 形象 / 其他), the profile card (avatar + name + email), the AI-avatar color picker, the three toggles (自动触发工具卡 / 过程记录 / 使用统计), and 退出登录. The design's SETTINGS section has no sections the current view lacks (verified: no 账号/安全/通知/隐私/关于 sections).
- **`RecordsView`** already matches the 我的评估 design — the 3-tab segmented control (学习记录 / 工具卡 / 能力素养), the activity-calendar heatmap + legend, 成长回顾, the card groups by branch, and the radar + ability list, plus the exact subtitle "你走过的思考，安静地留下印记。"

## The one gap

`RecordsView`'s page **header** still read "记录"; the design (and the Slice-1 nav label) call it **"我的评估"**. That is the only change this slice makes.

## Change

- `apps/web/src/shell/records/RecordsView.tsx`: header text "记录" → "我的评估".
- `apps/web/src/shell/records/RecordsView.test.tsx`: assert the "我的评估" header renders.

Frontend-only, no backend/contract change. This completes the 5-slice refactor.

## Out of scope / deferred (carried forward from earlier slices)
3d polish (methodology "怎么用" modal, close-out abstract framework, question↔span click-sync); 4-voice (TTS/STT); 4c-ask (course "问印记" ask-panel); course-level SOLO eval + note-export; CoursePlayer render-error state.
