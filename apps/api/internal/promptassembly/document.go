// Package promptassembly records ordered prompt sections without changing the
// bytes sent to a model. It has no gateway, storage, or template dependencies.
package promptassembly

import "strings"

// Section describes a byte range in Text. Kind distinguishes instructions,
// facts and legacy mixed sections; metadata is never part of model input.
type Section struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
}

// Selection explains input selection before rendering. Counts are not tokens.
// IDs refer to context categories, never to private database identities.
type Selection struct {
	ID       string `json:"id"`
	Total    int    `json:"total"`
	Included int    `json:"included"`
	Unit     string `json:"unit"`
	Reason   string `json:"reason"`
}

type Document struct {
	Text       string      `json:"text"`
	Sections   []Section   `json:"sections"`
	Selections []Selection `json:"selections,omitempty"`
}

// Builder is local to one call. Mark does not insert delimiters or whitespace.
// Zero-length sections are omitted. Do not copy a Builder after the first write.
type Builder struct {
	strings.Builder
	sections []Section
	current  *Section
}

func (b *Builder) Mark(id, kind, source string) {
	b.finish()
	b.current = &Section{ID: id, Kind: kind, Source: source, Start: b.Len()}
}

func (b *Builder) finish() {
	if b.current == nil {
		return
	}
	s := *b.current
	s.End = b.Len()
	if s.End > s.Start {
		b.sections = append(b.sections, s)
	}
	b.current = nil
}

func (b *Builder) Document(selections ...Selection) Document {
	b.finish()
	return Document{Text: b.String(), Sections: append([]Section(nil), b.sections...), Selections: append([]Selection(nil), selections...)}
}
