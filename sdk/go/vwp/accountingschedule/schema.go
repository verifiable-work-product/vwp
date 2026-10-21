package accountingschedule

import (
	"bytes"
	_ "embed"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schema.json
var schemaBytes []byte

var compiledSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes))
	if err != nil {
		return nil, err
	}
	const id = "https://wavie.ai/schemas/accounting-schedule/v1"
	if err := compiler.AddResource(id, doc); err != nil {
		return nil, err
	}
	return compiler.Compile(id)
})

func validateJSONShape(raw []byte) error {
	schema, err := compiledSchema()
	if err != nil {
		return fmt.Errorf("accounting schedule schema: %w", err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	if err := schema.Validate(doc); err != nil {
		return fmt.Errorf("accounting schedule schema: %w", err)
	}
	return nil
}
