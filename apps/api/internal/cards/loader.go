// Package cards loads the embedded card-spec catalog. The specs are a generated
// mirror of the canonical packages/contracts/cards; never hand-edit specs/.
package cards

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
)

//go:embed specs/*.json
var specFS embed.FS

// Spec is the view of a card the backend needs (catalog/prompt/refeed).
// The deep card shape stays owned by the TS Zod contract; we parse only what the
// agent brain consumes: identity, the prompt fields, and steps/fields for refeed.
type Spec struct {
	ID               string `json:"id"`
	Category         string `json:"category"`
	Name             string `json:"name"`
	NameEN           string `json:"name_en"`
	Purpose          string `json:"purpose"`
	TriggerCondition string `json:"trigger_condition"`
	InteractionType  string `json:"interaction_type"`
	Mode             string `json:"mode"`
	Steps            []Step `json:"steps"`

	// Gallery metadata (工具卡图鉴): AssetID is the cover-art set key (T-number),
	// empty when no design exists; Example is a short worked example. Both
	// additive/optional. Stage lists the workspace phases the card suits.
	AssetID string   `json:"asset_id"`
	Example string   `json:"example"`
	Stage   []string `json:"stage"`

	// C2 card format evolution (agent-spec §3): the primitive binding + the
	// runtime's typed view of params/completion/graph_effects/observe. All
	// additive and optional — legacy cards (no C2 block) parse unchanged.
	Primitive        string                `json:"primitive"`
	TargetType       string                `json:"target_type"`
	Params           Params                `json:"params"`
	Completion       []CompletionPredicate `json:"completion"`
	GraphEffects     []GraphEffect         `json:"graph_effects"`
	Observe          []ObserveRule         `json:"observe"`
	Consolidation    string                `json:"consolidation"`
	IntrusivenessCap string                `json:"intrusiveness_cap"`

	// ReadingLens is present ONLY on the reading-room disciplinary lenses
	// (category 学科透镜). Additive/optional — nil on every writing tool card.
	// It carries the pick-one-sentence fields the read-together summon prompt
	// needs; buildCardExamplePrompt uses them when non-nil, else falls back to
	// Purpose/TriggerCondition.
	ReadingLens *ReadingLens `json:"reading_lens"`
}

// ReadingLens mirrors the TS contract's optional reading_lens block. A lens
// answers "从什么角度看"; MethodIDs point at the writing tool cards that
// answer "具体做什么" (the methods layer, surfaced later).
type ReadingLens struct {
	Family        string   `json:"family"` // reasoning | institutions | context
	TaskPrompt    string   `json:"task_prompt"`
	SelectionHint string   `json:"selection_hint"`
	ExampleFocus  string   `json:"example_focus"`
	MethodIDs     []string `json:"method_ids"`
}

// Params is the card's C2 params block. CRAAP uses tags (the annotate
// dimensions) + tag_prompts (per-dimension guiding question); other C2
// cards may leave both empty.
type Params struct {
	Tags       []string          `json:"tags"`
	TagPrompts map[string]string `json:"tag_prompts"`

	// LateralDimension names the dimension whose anchor carries a DIFFERENT
	// material than the card's own (SIFT's lateral source). Empty for every
	// single-material card, which is all of them except compare cards.
	LateralDimension string `json:"lateral_dimension"`

	// Slots is the graph primitive's typed-slot config (Slice 7 Toulmin card):
	// one entry per argument role the student authors. Empty for every
	// non-graph card.
	Slots []Slot `json:"slots"`

	// Buckets is the sort/scale primitive's target vocabulary. For a sort card
	// the order is presentational; for a scale card it is the axis order and
	// is meaningful (left-to-right = the continuum). Empty for every
	// non-sort/scale card.
	Buckets []Bucket `json:"buckets"`

	// Cols is the matrix primitive's FIXED column axis. Matrix rows are
	// student-authored (identifying whose view is missing is the thinking),
	// so there is deliberately no Rows counterpart. Empty for non-matrix cards.
	Cols []Axis `json:"cols"`

	// MinItems is the minimum number of bucketed items (sort/scale) or
	// complete rows (matrix) the card requires. 0 means no minimum.
	MinItems int `json:"min_items"`
}

// Slot is one typed argument role in a graph-primitive card. ID is the node
// type it mints (claim/warrant/evidence/counter/concession); Role is the
// verbatim design label; NeedSrc requires ≥1 cited source material; Q is the
// coach's guiding question for the slot.
type Slot struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	NeedSrc bool   `json:"needSrc"`
	Q       string `json:"q"`
}

// Bucket is one target of a sort/scale primitive: ID is the value persisted
// as Anchor.Dimension, Label is the verbatim design label, Hint is the short
// disambiguating gloss shown under the label.
type Bucket struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Hint  string `json:"hint"`
}

// Axis is one column of a matrix primitive: ID is the value persisted as
// Anchor.Dimension, Label is the column header, Q is the guiding question
// shown with the header.
type Axis struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Q     string `json:"q"`
}

// CompletionPredicate is one closed-set completion check (agent-spec §3):
// "every_tag_present" (Tags) or "field_written_by" (Field + Author).
type CompletionPredicate struct {
	Kind   string   `json:"kind"`
	Tags   []string `json:"tags"`
	Field  string   `json:"field"`
	Author string   `json:"author"`

	// Min is the threshold for count-based predicates (items_bucketed). 0 for
	// every other predicate kind.
	Min int `json:"min"`
}

// GraphEffect is one closed-set graph mutation applied on card completion
// (agent-spec §3): "promote" mints a node of type To from a target of kind
// From, carrying the field named by With.
type GraphEffect struct {
	Kind string `json:"kind"`
	From string `json:"from"`
	To   string `json:"to"`
	With string `json:"with"`
}

// ObserveRule is one closed-set trigger (agent-spec §3) evaluated over the
// card's live primitive state; a match yields a coach Candidate. The JSON
// shape nests the resulting move (mirroring the TS CardSpec contract's
// `{when, move}` shape); ObserveRule flattens it for the Go runtime.
type ObserveRule struct {
	When  string `json:"when"`
	Verb  string `json:"-"`
	Level string `json:"-"`
}

// observeMove is the wire shape of ObserveRule.Move ({verb, level}).
type observeMove struct {
	Verb  string `json:"verb"`
	Level string `json:"level"`
}

// UnmarshalJSON adapts the wire shape `{"when":..., "move":{"verb":...,
// "level":...}}` (the TS CardSpec contract's observe entry) into the flat
// ObserveRule the Go runtime reads.
func (o *ObserveRule) UnmarshalJSON(data []byte) error {
	var wire struct {
		When string      `json:"when"`
		Move observeMove `json:"move"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	o.When = wire.When
	o.Verb = wire.Move.Verb
	o.Level = wire.Move.Level
	return nil
}

// Step is one phase of a card; Key/Title come from the JSON, Fields are the
// inputs the human fills.
type Step struct {
	Key    string  `json:"key"`
	Title  string  `json:"title"`
	Fields []Field `json:"fields"`
}

// Field is one input. Type is the field primitive (text/textarea/single_choice/
// multi_choice/rating/repeatable_group/link_check). ItemFields is populated only
// for repeatable_group.
type Field struct {
	Key        string      `json:"key"`
	Type       string      `json:"type"`
	Label      string      `json:"label"`
	ItemFields []ItemField `json:"item_fields"`
}

// ItemField is one column of a repeatable_group row.
type ItemField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// Catalog reads and parses every embedded spec, sorted by id for determinism.
func Catalog() ([]Spec, error) {
	entries, err := specFS.ReadDir("specs")
	if err != nil {
		return nil, fmt.Errorf("read embedded specs: %w", err)
	}
	specs := make([]Spec, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := specFS.ReadFile("specs/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		var s Spec
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		specs = append(specs, s)
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].ID < specs[j].ID })
	return specs, nil
}

// ByID returns the spec with the given id, if present.
func ByID(id string) (Spec, bool) {
	specs, err := Catalog()
	if err != nil {
		return Spec{}, false
	}
	for _, s := range specs {
		if s.ID == id {
			return s, true
		}
	}
	return Spec{}, false
}
