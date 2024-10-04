package be

import (
	"fmt"

	"golang.org/x/exp/constraints"
)

type Outcome string

func failf(format string, args ...interface{}) *Outcome {
	str := fmt.Sprintf(format, args...)
	return (*Outcome)(&str)
}

func Equal[T comparable](want, got T) *Outcome {
	if want == got {
		return nil
	}
	return failf("got %v, want %v", got, want)
}

func GreaterThan[T constraints.Ordered](a, b T) *Outcome {
	if a > b {
		return nil
	}
	return failf("%v is not greater than %v", a, b)
}

func LessThan[T constraints.Ordered](a, b T) *Outcome {
	if a < b {
		return nil
	}
	return failf("%v is not less than %v", a, b)
}

func GreaterThanOrEqual[T constraints.Ordered](a, b T) *Outcome {
	if a >= b {
		return nil
	}
	return failf("%v is not greater than or equal to %v", a, b)
}

func LessThanOrEqual[T constraints.Ordered](a, b T) *Outcome {
	if a <= b {
		return nil
	}
	return failf("%v is not less than or equal to %v", a, b)
}
