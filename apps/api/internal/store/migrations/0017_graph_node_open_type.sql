-- +goose Up
-- Slice 4 of the whole-product refactor found that graph_node.type is an OPEN,
-- skill-defined vocabulary — the gate engine's node_present{type} /
-- node_count_at_least{type} predicates match graph_node.type against whatever
-- artifact names a skill's contracts declare (rubric_translation, perspective,
-- preregistration, provisional_answer, concession, reflection, …). Migration
-- 0016 modelled it as a fixed 5-value enum, which was correct for the structural
-- types alone but wrong the moment intake mints skill-declared artifacts.
--
-- Relax the CHECK to a non-empty guard. The structural types 'plan' and
-- 'gate_state' remain exact string values (queried by GetPlanNode /
-- GetGateStateNode) and keep working unchanged; 'claim'/'evidence'/'note' are
-- likewise still valid. Enumerating skill vocabulary at the DB layer would
-- couple the schema to one skill — the wrong layer; a typo'd type simply fails
-- to satisfy its gate (which stays owed — the safe DEC-3 direction).
ALTER TABLE graph_node DROP CONSTRAINT graph_node_type_check;
ALTER TABLE graph_node ADD CONSTRAINT graph_node_type_check CHECK (type <> '');

-- +goose Down
-- NOT VALID, deliberately: by the time this reverses, real rows may carry
-- skill-declared open types (e.g. 'perspective') that predate this rollback
-- and cannot be rewritten or dropped out from under a live project. A
-- VALIDATE-ing ADD CONSTRAINT would abort the whole Down with a check
-- violation (SQLSTATE 23514) the moment such a row exists. NOT VALID
-- restores the pre-0017 structure and enforces the narrow enum for new rows
-- going forward, without demanding the past conform to it.
ALTER TABLE graph_node DROP CONSTRAINT graph_node_type_check;
ALTER TABLE graph_node ADD CONSTRAINT graph_node_type_check
    CHECK (type IN ('claim','evidence','plan','gate_state','note')) NOT VALID;
