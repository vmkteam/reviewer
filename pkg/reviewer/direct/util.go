package direct

import "fmt"

const defaultClip = 100_000

// JSON Schema keywords and primitive type names, centralised so the schema
// builders don't repeat string literals.
const (
	jsType     = "type"
	jsObject   = "object"
	jsString   = "string"
	jsArray    = "array"
	jsBoolean  = "boolean"
	jsNumber   = "number"
	jsInteger  = "integer"
	jsProps    = "properties"
	jsItems    = "items"
	jsEnum     = "enum"
	jsRequired = "required"
	jsAddProps = "additionalProperties"
	jsDesc     = "description"
)

// clip truncates s to the default output cap with a marker.
func clip(s string) string { return clipN(s, defaultClip) }

// clipN truncates s to at most n bytes, appending a truncation note when cut.
func clipN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf("\n... [truncated %d bytes]", len(s)-n)
}

// objSchema builds a JSON Schema object with the given properties and required
// keys. The required key is omitted when empty — a null/empty value is rejected
// by strict validators (e.g. DeepSeek).
func objSchema(props map[string]any, required ...string) map[string]any {
	s := map[string]any{
		jsType:     jsObject,
		jsProps:    props,
		jsAddProps: false,
	}
	if len(required) > 0 {
		s[jsRequired] = required
	}
	return s
}

func strProp(desc string) map[string]any { return primProp(jsString, desc) }
func intProp(desc string) map[string]any { return primProp(jsInteger, desc) }
func boolProp() map[string]any           { return map[string]any{jsType: jsBoolean} }
func numberProp() map[string]any         { return map[string]any{jsType: jsNumber} }

func primProp(typ, desc string) map[string]any {
	p := map[string]any{jsType: typ}
	if desc != "" {
		p[jsDesc] = desc
	}
	return p
}

// arrayOf builds an array schema with the given item schema.
func arrayOf(item map[string]any) map[string]any {
	return map[string]any{jsType: jsArray, jsItems: item}
}

// arrayOfDesc builds an array schema with an item schema and a description.
func arrayOfDesc(item map[string]any, desc string) map[string]any {
	a := arrayOf(item)
	if desc != "" {
		a[jsDesc] = desc
	}
	return a
}

// enumProp builds a string property constrained to the given values.
func enumProp(desc string, values ...string) map[string]any {
	vals := make([]any, len(values))
	for i, v := range values {
		vals[i] = v
	}
	p := map[string]any{jsType: jsString, jsEnum: vals}
	if desc != "" {
		p[jsDesc] = desc
	}
	return p
}

// objectProp builds a nested object property with fixed properties.
func objectProp(props map[string]any, required ...string) map[string]any {
	return objSchema(props, required...)
}

// freeObject builds an open object schema that accepts any properties — used
// where the keys are dynamic (e.g. a map of review-type to markdown body).
func freeObject() map[string]any { return map[string]any{jsType: jsObject} }
