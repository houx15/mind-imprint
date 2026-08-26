package evalbench

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	jsonschema "github.com/google/jsonschema-go/jsonschema"
)

// compiledSchema is private to Evalbench: production does not load or depend
// on experiment output schemas at process startup.
type compiledSchema struct {
	name     string
	resolved *jsonschema.Resolved
}

func mustCompileSchema(name string, raw []byte) *compiledSchema {
	var parsed jsonschema.Schema
	if err := json.Unmarshal(raw, &parsed); err != nil {
		panic(fmt.Sprintf("evalbench schema %s: parse: %v", name, err))
	}
	resolved, err := parsed.Resolve(nil)
	if err != nil {
		panic(fmt.Sprintf("evalbench schema %s: resolve: %v", name, err))
	}
	return &compiledSchema{name: name, resolved: resolved}
}

func validateSchemaJSON(c *compiledSchema, raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var value any
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("%s: invalid JSON: %w", c.name, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("%s: JSON has trailing value", c.name)
		}
		return fmt.Errorf("%s: invalid JSON suffix: %w", c.name, err)
	}
	if err := c.resolved.Validate(value); err != nil {
		return fmt.Errorf("%s: contract validation: %w", c.name, err)
	}
	return nil
}
