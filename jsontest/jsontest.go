package jsontest

import (
	"encoding/json"
	"fmt"

	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type TB interface {
	Fatalf(format string, args ...any)
	Errorf(format string, args ...any)
	Log(args ...any)
	Helper()
}

type TestAsserter struct {
	asserter *Asserter
	t        TB
}

func NewTestAsserter(t TB, v any) *TestAsserter {
	asserter, err := NewAsserter(v)
	if err != nil {
		t.Fatalf("failed to create asserter: %v", err)
	}
	return &TestAsserter{asserter: asserter, t: t}
}

func (ta *TestAsserter) Print() {
	ta.asserter.Print(ta.t)
}

func (ta *TestAsserter) PrintAt(path string) {
	ta.asserter.PrintAt(ta.t, path)
}

func (ta *TestAsserter) Get(path string) (any, bool) {
	return ta.asserter.Get(path)
}

func (ta *TestAsserter) AssertEqual(path string, value any) {
	ta.asserter.AssertEqual(ta.t, path, value)
}

func (ta *TestAsserter) AssertNotSet(path string) {
	ta.asserter.AssertNotSet(ta.t, path)
}

func (ta *TestAsserter) AssertEqualSet(path string, expected map[string]any) {
	ta.asserter.AssertEqualSet(ta.t, path, expected)
}

type Asserter struct {
	JSON string
}

func NewAsserter(v any) (*Asserter, error) {
	var val string

	switch v := v.(type) {
	case string:
		val = v

	case []byte:
		val = string(v)

	case proto.Message:
		val = protojson.Format(v)

	default:
		bb, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return nil, err
		}
		val = string(bb)
	}
	return &Asserter{JSON: val}, nil
}

func (d *Asserter) Print(t TB) {
	t.Helper()
	t.Log(string(d.JSON))
}

func (d *Asserter) PrintAt(t TB, path string) {
	val := gjson.Get(d.JSON, path)
	if val.Exists() {
		t.Log(val.String())
	} else {
		t.Log("path not found")
	}
}

func (d *Asserter) Get(path string) (any, bool) {
	val := gjson.Get(d.JSON, path)
	if val.Exists() {
		return val.Value(), true
	}
	return nil, false
}

type LenEqual int

type NotSet struct{}

type IsOneofKey string

type Array[T any] []T

func (aa Array[T]) toJSONArray() []any {
	out := make([]any, len(aa))
	for idx, val := range aa {
		out[idx] = val
	}
	return out
}

type isArray interface {
	toJSONArray() []any
}

func (d *Asserter) AssertEqual(t TB, path string, value any) {
	t.Helper()
	if _, ok := value.(NotSet); ok {
		_, ok := d.Get(path)
		if ok {
			t.Errorf("path %q was set", path)
		}
		return
	}
	if _, ok := value.(IsOneofKey); ok {
		atPath, ok := d.Get(path)
		if !ok {
			t.Errorf("path %q not found", path)
		}
		pathObj, ok := atPath.(map[string]any)
		if !ok {
			t.Errorf("path %q invalid for oneof", path)
		}
		keys := make([]string, 0)

		for k := range pathObj {

			if k == "!type" {
				continue // skip type key

			}
			keys = append(keys, k)
		}
		if len(keys) == 0 {
			t.Errorf("no key found at path %q", path)
		} else if len(keys) > 1 {
			t.Errorf("multiple keys found at path %q: %v", path, keys)
		} else if keys[0] != string(value.(IsOneofKey)) {
			t.Errorf("expected key %q, got %q", value, keys[0])
		}
		return
	}

	actual, ok := d.Get(path)
	if !ok {
		t.Errorf("path %q not found", path)
		return
	}

	switch value := value.(type) {
	case LenEqual:
		actualSlice, ok := actual.([]any)
		if ok {
			if len(actualSlice) != int(value) {
				t.Errorf("expected %d, got %d", value, len(actualSlice))
			}
			return
		}
		actualMap, ok := actual.(map[string]any)
		if ok {
			if len(actualMap) != int(value) {
				t.Errorf("expected %d, got %d", value, len(actualMap))
			}
			return
		}
		t.Errorf("expected len(%d), got non len object %T", value, actual)
	case isArray:
		wantVal := value.toJSONArray()
		assert.EqualValues(t, wantVal, actual, "array at path %q", path)

	default:
		assert.EqualValues(t, value, actual, "at path %q", path)
	}
}

func (d *Asserter) AssertNotSet(t TB, path string) {
	_, ok := d.Get(path)
	if ok {
		t.Errorf("path %q was set", path)
	}
}

func (d *Asserter) AssertEqualSet(t TB, path string, expected map[string]any) {
	t.Helper()
	for key, expectSet := range expected {
		pathKey := key
		if path != "" {
			pathKey = fmt.Sprintf("%s.%s", path, key)
		}

		d.AssertEqual(t, pathKey, expectSet)
	}
}
