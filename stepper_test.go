package flowtest

import "testing"

func TestStepperStops(t *testing.T) {

	ss := NewStepper[*testing.T](t.Name())
	defer ss.RunSteps(t)

	ss.Step("success", func(a Asserter) {
		a.Log("this is fine")
	})

	ss.Step("throw", func(a Asserter) {
		a.Log("step 1 ", map[string]interface{}{"foo": "bar"})
		a.Equal(true, true)
		a.Fatal("Test Fatal")
	})

	ss.Step("after", func(a Asserter) {
		t.Fatal("I should not be running")
	})

}
