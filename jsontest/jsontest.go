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
	Fatalf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Log(args ...interface{})
	Helper()
}

type TestAsserter struct {
	asserter *Asserter
	t        TB
}

func NewTestAsserter(t TB, v interface{}) *TestAsserter {
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

func (ta *TestAsserter) Get(path string) (interface{}, bool) {
	return ta.asserter.Get(path)
}

func (ta *TestAsserter) AssertEqual(path string, value interface{}) {
	ta.asserter.AssertEqual(ta.t, path, value)
}

func (ta *TestAsserter) AssertNotSet(path string) {
	ta.asserter.AssertNotSet(ta.t, path)
}

func (ta *TestAsserter) AssertEqualSet(path string, expected map[string]interface{}) {
	ta.asserter.AssertEqualSet(ta.t, path, expected)
}

type Asserter struct {
	JSON string
}

func NewAsserter(v interface{}) (*Asserter, error) {
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

func (d *Asserter) Get(path string) (interface{}, bool) {
	val := gjson.Get(d.JSON, path)
	if val.Exists() {
		return val.Value(), true
	}
	return nil, false
}

type LenEqual int

func (d *Asserter) AssertEqual(t TB, path string, value interface{}) {
	t.Helper()
	actual, ok := d.Get(path)
	if !ok {
		t.Errorf("path %q not found", path)
		return
	}

	switch value.(type) {
	case LenEqual:
		actualSlice, ok := actual.([]interface{})
		if ok {
			if len(actualSlice) != int(value.(LenEqual)) {
				t.Errorf("expected %d, got %d", value, len(actualSlice))
			}
			return
		}
		actualMap, ok := actual.(map[string]interface{})
		if ok {
			if len(actualMap) != int(value.(LenEqual)) {
				t.Errorf("expected %d, got %d", value, len(actualMap))
			}
			return
		}
		t.Errorf("expected len(%d), got non len object %T", value, actual)
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

func (d *Asserter) AssertEqualSet(t TB, path string, expected map[string]interface{}) {
	t.Helper()
	for key, expectSet := range expected {
		pathKey := key
		if path != "" {
			pathKey = fmt.Sprintf("%s.%s", path, key)
		}

		d.AssertEqual(t, pathKey, expectSet)
	}
}
