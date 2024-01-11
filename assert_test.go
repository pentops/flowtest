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

func (t *testWrap) Fatal(args ...interface{}) {
	t.failed = true
	t.message = fmt.Sprint(args...)
}

func TestT(t *testing.T) {

	for _, tc := range []*Failure{
		Equal(1, 1),
		Equal(1, int(1)),
		Equal(int32(1), 1),
		Equal("a", "a"),
		GreaterThan(2, 1),
		GreaterThan("b", "a"),
		LessThan(1, 2),
	} {
		tw := &testWrap{}
		a := &assertion{
			fatal:  tw.Fatal,
			helper: tw.Helper,
		}
		a.T(tc)
		if tw.failed {
			t.Errorf("failed: %s", tw.message)
		}
	}

	for _, tc := range []*Failure{
		Equal(1, 2),
		Equal(1, int(2)),
		Equal(int32(1), 2),
		Equal("a", "2"),
	} {
		tw := &testWrap{}
		a := &assertion{
			fatal:  tw.Fatal,
			helper: tw.Helper,
		}
		a.T(tc)
		if !tw.failed {
			t.Errorf("expected failure, but got none")
		}
	}

}

func TestEquals(t *testing.T) {

	for _, tc := range []struct {
		A     interface{}
		B     interface{}
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
