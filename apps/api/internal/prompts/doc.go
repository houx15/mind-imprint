// Package prompts owns fixed Lite prompt text as Go constants.
//
// Review entry point: Assemblies lists each feature's purpose, builders,
// context selectors, output contracts, and tests. Catalog lists individual
// text definitions. cmd/promptinspect prints either list and renders synthetic
// examples through the production builders, without loading keys or a database.
//
// Source comments are maintenance metadata, never model input. Go constants
// preserve whitespace, compile-time composition and existing fmt placeholders.
// There is no runtime template parser, remote prompt service, or second model
// configuration. gateway/models.json remains the model routing authority.
//
// Responsibilities after the September 2026 refactor:
//   - this package: fixed teaching text and inspection metadata;
//   - feature *_prompt.go / domain builders: language/condition selection and
//     rendering, including dynamic instructions and retry messages;
//   - feature *_context.go: context selection/projection (reading coach and
//     writing planning have typed selectors; smaller domains keep typed inputs);
//   - handlers: load authorized data, call gateway, validate and persist results.
//
// Existing wire content/order, budgets and truncation are deliberately preserved.
// Legacy mixed fact/instruction sections are marked "mixed", not relabeled as
// pure facts. Reading/writing trace metadata never enters a model request.
// Shared course, reading and project-coach consumers include Pro: changes here
// require their tests too. Structured card/method/rubric registries remain their
// own source of truth rather than being copied into this package.
package prompts
