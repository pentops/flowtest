package flowtest

import (
	"context"
	"fmt"
	"strings"
)

type Asserter interface {
	TB
	Assertion
}

type step struct {
	desc     string
	asserter *stepRun
	fn       callback
}

type callback func(context.Context, Asserter)
type callbackErr func(context.Context, Asserter) error

type Stepper[T RequiresTB] struct {
	steps      []*step
	variations []*step
	asserter   *stepRun
	name       string

	setup             []callbackErr
	preStepHooks      []callbackErr
	preVariationHooks []callbackErr
	postStepHooks     []callbackErr
}

// Setup steps run at the start of each RunSteps call, or the start of each
// Variation. The context passed to the setup will be canceled after all steps
// are completed, or after any fatal error.
func (ss *Stepper[T]) Setup(fn callbackErr) {
	ss.setup = append(ss.setup, fn)
}

// PreStepHook runs before every step, after any variations.
func (ss *Stepper[T]) PreStepHook(fn func(context.Context, Asserter) error) {
	ss.preStepHooks = append(ss.preStepHooks, fn)
}

// PreVariationHook runs before every Variation, before any steps.
func (ss *Stepper[T]) PreVariationHook(fn func(context.Context, Asserter) error) {
	ss.preVariationHooks = append(ss.preVariationHooks, fn)
}

// PostStepHook runs after every step.
func (ss *Stepper[T]) PostStepHook(fn func(context.Context, Asserter) error) {
	ss.postStepHooks = append(ss.postStepHooks, fn)
}

// StepSetter is a minimal interface to configure the steps and hooks for a test.
type StepSetter interface {
	// Setup steps run at the start of each RunSteps call, or the start of each
	// Variation. The context passed to the setup will be canceled after all steps
	// are completed, or after any fatal error.
	Setup(fn callbackErr)

	// PreStepHook runs before every step, after any variations.
	PreStepHook(fn func(context.Context, Asserter) error)

	// PreVariationHook runs before every Variation, before any steps.
	PreVariationHook(fn func(context.Context, Asserter) error)

	// PostStepHook runs after every step.
	PostStepHook(fn func(context.Context, Asserter) error)

	// Step registers a function to make assertions on the running code, this is the
	// main assertion set.
	Step(desc string, fn func(context.Context, Asserter))

	// Adds a variation to the stepper. Each Variation causes the Setup hooks,
	// followed by the Variation, then every registered Step (and hooks), allowing
	// one call to RunSteps to run multiple variations of the same test.
	Variation(desc string, fn func(context.Context, Asserter))

	// LevelLog implements a global logger compatible with pentops/log.go/log.
	// Log lines will be captured into the currently running test step.
	LevelLog(level, message string, fields map[string]interface{})

	// Log logs any object, it can be used within test callbacks.
	// Log lines will be captured into the currently running test step.
	Log(...interface{})
}

func (ss *Stepper[T]) Log(args ...interface{}) {
	ss.asserter.Log(args...)
}

// LevelLog implements a global logger compatible with pentops/log.go/log
// DefaultLogger, and others, to capture log lines from within the handlers
// into the test output
func (ss *Stepper[T]) LevelLog(level, message string, fields map[string]interface{}) {

	fieldStrings := make([]string, 0, len(fields)+1)
	fieldStrings = append(fieldStrings, fmt.Sprintf("%s: %s", level, message))
	for k, v := range fields {
		if stackLines, ok := v.([]string); ok && k == "stack" {
			fieldStrings = append(fieldStrings, stackLines...)
			continue
		}
		fieldStrings = append(fieldStrings, fmt.Sprintf("%s: %v", k, v))
	}
	if ss.asserter == nil {
		fmt.Printf("WARNING: Log called on stepper without a current step (level: %s and message: %s)\n%s", level, message, strings.Join(fieldStrings, "\n"))
		return
	}
	ss.Log(strings.Join(fieldStrings, "\n"))

}

func NewStepper[T RequiresTB](name string) *Stepper[T] {
	return &Stepper[T]{
		name: name,
	}
}

// Step registers a function to make assertions on the running code, this is the
// main assertion set.
func (ss *Stepper[_]) Step(desc string, fn func(context.Context, Asserter)) {
	ss.steps = append(ss.steps, &step{
		desc: desc,
		fn:   fn,
	})
}

// Adds a variation to the stepper. Each Variation causes the Setup hooks,
// followed by the Variation, then every registered Step (and hooks), allowing
// one call to RunSteps to run multiple variations of the same test.
func (ss *Stepper[_]) Variation(desc string, fn func(context.Context, Asserter)) {
	ss.variations = append(ss.variations, &step{
		desc: desc,
		fn:   fn,
	})
}

// RunSteps is the main entry point of the stepper. For each Variation, or just
// once if no variation is registered, the Setup hooks are run, followed by the
// Variation, then every registered Step with pre and post hoooks.
func (ss *Stepper[T]) RunSteps(t RunnableTB[T]) {
	ctx := context.Background()
	t.Helper()
	ss.RunStepsWithContext(ctx, t)
}

// RunStepsWithContext allows the caller to provide a context to the stepper.
func (ss *Stepper[T]) RunStepsWithContext(ctx context.Context, t RunnableTB[T]) {
	t.Helper()

	if len(ss.variations) > 0 {
		for variationIdx, variation := range ss.variations {
			success := ss.runVariation(ctx, t, variationIdx, variation)
			if !success {
				return
			}

		}
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := ss.runHooks(ctx, cancel, t, ss.setup...); err != nil {
		t.Log("Setup failed", err)
		t.FailNow()
	}

	for idx, step := range ss.steps {
		success := ss.runStep(ctx, t, fmt.Sprintf("%d %s", idx, step.desc), step, ss.preStepHooks, ss.postStepHooks)
		if !success {
			return
		}
	}
}

func (ss *Stepper[T]) runVariation(ctx context.Context, t RunnableTB[T], variationIdx int, variation *step) bool {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := ss.runHooks(ctx, cancel, t, ss.setup...); err != nil {
		t.Log("Setup failed", err)
		t.FailNow()
	}

	success := ss.runStep(ctx, t, fmt.Sprintf("vary %d %s", variationIdx, variation.desc), variation, ss.preVariationHooks, nil)
	if !success {
		return false
	}
	for idx, step := range ss.steps {
		success := ss.runStep(ctx, t, fmt.Sprintf("vary %d %d %s", variationIdx, idx, step.desc), step, ss.preStepHooks, ss.postStepHooks)
		if !success {
			return false
		}
	}
	return true
}

func (ss *Stepper[T]) runHooks(ctx context.Context, cancel func(), t RunnableTB[T], fns ...callbackErr) error {
	if len(fns) == 0 {
		return nil
	}
	asserter := &stepRun{
		cancel:     cancel,
		RequiresTB: t,
	}
	asserter.assertion = asserter.anon()
	ss.asserter = asserter
	for _, fn := range fns {
		if err := fn(ctx, asserter); err != nil {
			return err
		}
	}

	if asserter.failed {
		return fmt.Errorf("hook failed")
	}
	return nil
}

func (ss *Stepper[T]) runStep(ctx context.Context, t RunnableTB[T], name string, step *step, preHooks []callbackErr, postHooks []callbackErr) bool {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	actuallyDidRun := false
	success := t.Run(name, func(t T) {
		actuallyDidRun = true
		asserter := &stepRun{
			cancel:     cancel,
			RequiresTB: t,
		}
		asserter.assertion = asserter.anon()
		ss.asserter = asserter
		step.asserter = asserter

		for _, hook := range preHooks {
			err := hook(ctx, ss.asserter)
			if err != nil {
				t.Log("Pre hook failed", err)
				t.FailNow()
			}
		}

		step.fn(ctx, asserter)

		for _, hook := range postHooks {
			err := hook(ctx, ss.asserter)
			if err != nil {
				t.Log("Post hook failed", err)
				t.FailNow()
			}
		}
	})
	if !actuallyDidRun {
		// We can't prevent or override this (AFAIK), so we just have to fail
		t.Log(fmt.Sprintf("Step %s did not run - did you call test with a sub-filter?", step.desc))
		t.FailNow()
	}
	return success
}

// TB is the subset of the testing.TB interface which the stepper's asserter
// implements.
type TB interface {
	//Cleanup(func())
	Error(args ...any)
	Errorf(format string, args ...any)
	//Fail()
	FailNow()
	Failed() bool
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Helper()
	Log(args ...any)
	Logf(format string, args ...any)
	//Name() string
	//Setenv(key, value string)
	//Skip(args ...any)
	//SkipNow()
	//Skipf(format string, args ...any)
	//Skipped() bool
	//TempDir() string
}

type RequiresTB interface {
	Helper()
	Log(args ...interface{})
	FailNow()
	Fail()
}

// RunnableTB is the subset of the testing.TB interface which this library
// requires. Keeping it to a minimum to allow alternate implementations
type RunnableTB[T RequiresTB] interface {
	RequiresTB
	Run(name string, f func(T)) bool
}

type stepRun struct {
	RequiresTB
	failed bool
	cancel func()
	*assertion
}

func (t *stepRun) Failed() bool {
	return t.failed
}

func (t *stepRun) log(level LogLevel, args ...interface{}) {
	t.Helper()
	if levelLogger, ok := t.RequiresTB.(levelLogger); ok {
		levelLogger.LevelLog(level, args...)
	} else {
		if level == LogLevelDefault {
			t.RequiresTB.Log(args...)
		} else {
			t.RequiresTB.Log(fmt.Sprintf("%s: %s", level, fmt.Sprint(args...)))
		}
	}
}

func (t *stepRun) Log(args ...interface{}) {
	t.Helper()
	t.log(LogLevelDefault, args...)
}

func (t *stepRun) Logf(format string, args ...interface{}) {
	t.Helper()
	t.log(LogLevelDefault, fmt.Sprintf(format, args...))
}

type LogLevel string

const (
	LogLevelFatal   LogLevel = "FATAL"
	LogLevelError   LogLevel = "ERROR"
	LogLevelDefault LogLevel = ""
)

type levelLogger interface {
	LevelLog(level LogLevel, args ...interface{})
}

func (t *stepRun) Fatal(args ...interface{}) {
	t.Helper()
	t.log(LogLevelFatal, fmt.Sprint(args...))
	t.FailNow()
}

func (t *stepRun) Fatalf(format string, args ...interface{}) {
	t.Helper()
	t.Fatal(fmt.Sprintf(format, args...))
}

func (t *stepRun) FailNow() {
	t.Helper()
	t.failed = true
	t.cancel()
	t.RequiresTB.FailNow()
}

func (t *stepRun) Error(args ...interface{}) {
	t.Helper()
	t.log("ERROR", args...)
	t.RequiresTB.Fail()
	t.failed = true
}

func (t *stepRun) Errorf(format string, args ...interface{}) {
	t.Helper()
	t.Error(fmt.Sprintf(format, args...))
	t.failed = true
}

func (t *stepRun) anon() *assertion {
	return &assertion{
		name:   "",
		helper: t.Helper,
		fatal:  t.Fatal,
	}
}
