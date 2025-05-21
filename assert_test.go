package flowtest

import (
	"fmt"
	"testing"
)

type testWrap struct {
	failed  bool
	message string
}

func (t *testWrap) Helper() {
}

func (t *testWrap) Fatal(args ...any) {
	t.failed = true
	t.message = fmt.Sprint(args...)
}

func TestEquals(t *testing.T) {

	for _, tc := range []struct {
		A     any
		B     any
		Equal bool
	}{
		{A: "foo", B: "foo", Equal: true},
		{A: "foo", B: "bar", Equal: false},
		{A: 1, B: 1, Equal: true},
		{A: 1, B: 2, Equal: false},
		{A: nil, B: nil, Equal: true},
		{A: nil, B: 1, Equal: false},
		{A: 1, B: nil, Equal: false},
	} {
		t.Run(fmt.Sprintf("%v == %v", tc.A, tc.B), func(t *testing.T) {
			tw := &testWrap{}
			a := &assertion{
				fatal:  tw.Fatal,
				helper: tw.Helper,
			}
			a.Equal(tc.A, tc.B)
			if tc.Equal {
				if tw.failed {
					t.Errorf("Equals(%#v %T, %#v %T) failed: %s", tc.A, tc.A, tc.B, tc.B, tw.message)
				}
			} else {
				if !tw.failed {
					t.Errorf("Equals(%v, %v) did not fail", tc.A, tc.B)
				}
			}
		})
	}
}

func TestNotNilHappy(t *testing.T) {

	type testStruct struct{}
	notNilVals := []any{
		"foo",
		1,
		0,
		"",
		[]string{"foo"},
		[]string{},
		[]byte{},
		&struct{}{},
		testStruct{},
		&testStruct{},
	}

	for idx, val := range notNilVals {
		t.Run(fmt.Sprintf("happy case %d", idx), func(t *testing.T) {
			if isNil(val) {
				t.Errorf("val (%v) assessed as nil", val)
			}
		})
	}

	nilVals := []func() any{
		func() any { return nil },
		func() any { var a *string; return a },
		func() any { var a *[]byte; return a },
		func() any { var a *struct{}; return a },
		func() any { var a *testStruct; return a },
	}

	for idx, valFunc := range nilVals {
		t.Run(fmt.Sprintf("sad case %d", idx), func(t *testing.T) {
			val := valFunc()
			if !isNil(val) {
				t.Errorf("val (%v) assessed as not nil", val)
			}
		})
	}

}
