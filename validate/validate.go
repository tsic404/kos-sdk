// Package validate carries the issue model shared by the manifest and
// declaration checks.
//
// A failed check yields issues and never a result: callers must not build an
// installable artifact from a failed check.
package validate

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// Code classifies a rejected declaration site.
type Code string

const (
	CodeMissingField       Code = "missing-field"
	CodeUnknownField       Code = "unknown-field"
	CodeInvalidType        Code = "invalid-type"
	CodeInvalidValue       Code = "invalid-value"
	CodePermissionNotInSet Code = "permission-not-in-set"
)

// Issue is one rejected site; Path is a JSON path rooted at "$".
type Issue struct {
	Path    string
	Code    Code
	Message string
}

func (i Issue) String() string {
	return fmt.Sprintf("%s: %s (%s)", i.Path, i.Code, i.Message)
}

// Result is the outcome of a manifest check. A failed result carries no manifest.
type Result[T any] struct {
	Manifest *T
	Issues   []Issue
}

// OK reports that the manifest was accepted and no issue was recorded.
func (r Result[T]) OK() bool {
	return r.Manifest != nil && len(r.Issues) == 0
}

// InvalidError reports a declaration rejected before an author can hold it.
type InvalidError struct {
	What   string
	Issues []Issue
}

func (e *InvalidError) Error() string {
	return fmt.Sprintf("%s is invalid: %v", e.What, e.Issues)
}

// Checker accumulates the issues of one check run.
type Checker struct {
	Issues []Issue
}

// Add records one issue at path.
func (c *Checker) Add(path string, code Code, format string, args ...any) {
	c.Issues = append(c.Issues, Issue{Path: path, Code: code, Message: fmt.Sprintf(format, args...)})
}

// Join extends a JSON path with an object key.
func Join(path, key string) string {
	if path == "$" {
		return "$." + key
	}
	return path + "." + key
}

// Index extends a JSON path with an array index.
func Index(path string, index int) string {
	return fmt.Sprintf("%s[%d]", path, index)
}

// Field returns a required field value; an absent key is reported. A present
// null is returned as a value so the caller's type check reports invalid-type.
func (c *Checker) Field(record map[string]any, key, path string) (any, bool) {
	value, present := record[key]
	if !present {
		c.Add(Join(path, key), CodeMissingField, "missing required field %q", key)
		return nil, false
	}
	return value, true
}

// Optional returns a field value when the key is present; a present null is
// not absence.
func (c *Checker) Optional(record map[string]any, key string) (any, bool) {
	value, present := record[key]
	return value, present
}

// RejectUnknown reports fields the declaration does not define.
func (c *Checker) RejectUnknown(record map[string]any, allowed []string, path string) {
	known := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		known[key] = struct{}{}
	}
	var unknown []string
	for key := range record {
		if _, ok := known[key]; !ok {
			unknown = append(unknown, key)
		}
	}
	sort.Strings(unknown)
	for _, key := range unknown {
		c.Add(Join(path, key), CodeUnknownField, "field %q is not defined by the declaration", key)
	}
}

// Record checks that value is a JSON object.
func (c *Checker) Record(value any, path string) (map[string]any, bool) {
	record, ok := value.(map[string]any)
	if !ok {
		c.Add(path, CodeInvalidType, "expected an object")
		return nil, false
	}
	return record, true
}

// Array checks that value is a JSON array.
func (c *Checker) Array(value any, path string) ([]any, bool) {
	items, ok := value.([]any)
	if !ok {
		c.Add(path, CodeInvalidType, "expected an array")
		return nil, false
	}
	return items, true
}

// String checks that value is a non-empty string.
func (c *Checker) String(value any, path string) (string, bool) {
	text, ok := value.(string)
	if !ok || len(text) == 0 {
		c.Add(path, CodeInvalidType, "expected a non-empty string")
		return "", false
	}
	return text, true
}

// Number checks that value is a finite number; positive also excludes zero.
// Input may be any Go numeric type or json.Number, not only a decoded float64.
func (c *Checker) Number(value any, path string, positive bool) (float64, bool) {
	number, ok := toFloat64(value)
	if !ok {
		c.Add(path, CodeInvalidType, "%s, got %T", numberExpectation(positive), value)
		return 0, false
	}
	if math.IsInf(number, 0) || math.IsNaN(number) || (positive && number <= 0) {
		c.Add(path, CodeInvalidType, "%s, got %v", numberExpectation(positive), number)
		return 0, false
	}
	return number, true
}

func numberExpectation(positive bool) string {
	if positive {
		return "expected a positive number"
	}
	return "expected a finite number"
}

func toFloat64(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	case json.Number:
		parsed, err := number.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

// Boolean checks that value is a boolean.
func (c *Checker) Boolean(value any, path string) (bool, bool) {
	flag, ok := value.(bool)
	if !ok {
		c.Add(path, CodeInvalidType, "expected a boolean")
		return false, false
	}
	return flag, true
}

// StringArray checks that value is an array of non-empty strings.
func (c *Checker) StringArray(value any, path string) ([]string, bool) {
	items, ok := c.Array(value, path)
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(items))
	for index, item := range items {
		text, ok := c.String(item, Index(path, index))
		if !ok {
			continue
		}
		result = append(result, text)
	}
	return result, true
}

// RequireString reads a required string field.
func (c *Checker) RequireString(record map[string]any, key, path string) (string, bool) {
	value, ok := c.Field(record, key, path)
	if !ok {
		return "", false
	}
	return c.String(value, Join(path, key))
}

// RequireRecord reads a required object field.
func (c *Checker) RequireRecord(record map[string]any, key, path string) (map[string]any, bool) {
	value, ok := c.Field(record, key, path)
	if !ok {
		return nil, false
	}
	return c.Record(value, Join(path, key))
}

// RequireArray reads a required array field.
func (c *Checker) RequireArray(record map[string]any, key, path string) ([]any, bool) {
	value, ok := c.Field(record, key, path)
	if !ok {
		return nil, false
	}
	return c.Array(value, Join(path, key))
}

// RequireNumber reads a required number field.
func (c *Checker) RequireNumber(record map[string]any, key, path string, positive bool) (float64, bool) {
	value, ok := c.Field(record, key, path)
	if !ok {
		return 0, false
	}
	return c.Number(value, Join(path, key), positive)
}

// RequireBoolean reads a required boolean field.
func (c *Checker) RequireBoolean(record map[string]any, key, path string) (bool, bool) {
	value, ok := c.Field(record, key, path)
	if !ok {
		return false, false
	}
	return c.Boolean(value, Join(path, key))
}

// RequireStringArray reads a required array of strings.
func (c *Checker) RequireStringArray(record map[string]any, key, path string) ([]string, bool) {
	value, ok := c.Field(record, key, path)
	if !ok {
		return nil, false
	}
	return c.StringArray(value, Join(path, key))
}

// OptionalString reads an optional string field.
func (c *Checker) OptionalString(record map[string]any, key, path string) (string, bool) {
	value, ok := c.Optional(record, key)
	if !ok {
		return "", false
	}
	return c.String(value, Join(path, key))
}

// OptionalRecord reads an optional object field.
func (c *Checker) OptionalRecord(record map[string]any, key, path string) (map[string]any, bool) {
	value, ok := c.Optional(record, key)
	if !ok {
		return nil, false
	}
	return c.Record(value, Join(path, key))
}

// OptionalArray reads an optional array field.
func (c *Checker) OptionalArray(record map[string]any, key, path string) ([]any, bool) {
	value, ok := c.Optional(record, key)
	if !ok {
		return nil, false
	}
	return c.Array(value, Join(path, key))
}

// OptionalStringArray reads an optional array of strings.
func (c *Checker) OptionalStringArray(record map[string]any, key, path string) ([]string, bool) {
	value, ok := c.Optional(record, key)
	if !ok {
		return nil, false
	}
	return c.StringArray(value, Join(path, key))
}
