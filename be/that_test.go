package be

import (
	"testing"
)

func TestHappyFailFuncs(t *testing.T) {

	for _, tc := range []*Outcome{
		Equal(1, 1),
		Equal(1, int(1)),
		Equal(int32(1), 1),
		Equal("a", "a"),
		GreaterThan(2, 1),
		GreaterThan("b", "a"),
		LessThan(1, 2),
	} {
		if tc != nil {
			t.Errorf("failed: %s", *tc)
		}
	}

}
func TestSadFailFuncs(t *testing.T) {
	for _, tc := range []*Outcome{
		Equal(1, 2),
		Equal(1, int(2)),
		Equal(int32(1), 2),
		Equal("a", "2"),
	} {
		if tc == nil {
			t.Errorf("expected failure, but got none")
		}
	}

}
