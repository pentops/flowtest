package flowtest

import (
	"context"
	"testing"
)

func TestStepperStops(t *testing.T) {

	t.Skip() // Too Meta
	ss := NewStepper[*testing.T](t.Name())
	defer ss.RunSteps(t)

	ss.Step("success", func(ctx context.Context, a Asserter) {
		a.Log("this is fine")
	})

	ss.Step("throw", func(ctx context.Context, a Asserter) {
		a.Log("step 1 ", map[string]interface{}{"foo": "bar"})
		a.Equal(true, true)
		a.Fatal("Test Fatal")
	})

	ss.Step("after", func(ctx context.Context, a Asserter) {
		t.Fatal("I should not be running")
	})

}
