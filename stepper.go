package flowtest

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
)

type Asserter interface {
	TB
	Assertion
}

type Logger interface {
	Log(args ...any)
}

type shiftingLogger struct {
	Logger
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
	backgroundHooks   []callbackErr
	preStepHooks      []callbackErr
	preVariationHooks []callbackErr
	postStepHooks     []callbackErr

	shiftingLogger *shiftingLogger
}

func NewStepper[T RequiresTB](name string) *Stepper[T] {
	return &Stepper[T]{
		name: name,
	}
}

// Setup steps run at the start of each RunSteps call, or the start of each
// Variation. The context passed to the setup will be canceled after all steps
// are completed, or after any fatal error.
func (ss *Stepper[T]) Setup(fn callbackErr) {
	ss.setup = append(ss.setup, fn)
}

// BackgroundHook runs at the start and end of each RunSteps call, or the start of each
// Variation. The context passed to the setup will be canceled after all steps
// are completed, or after any fatal error.
// The hook need not return until the context is canceled.
func (ss *Stepper[T]) Background(fn callbackErr) {
	ss.backgroundHooks = append(ss.backgroundHooks, fn)
}

// PreStepHook runs before every step, after any variations.
func (ss *Stepper[T]) PreStepHook(fn callbackErr) {
	ss.preStepHooks = append(ss.preStepHooks, fn)
}

// PreVariationHook runs before every Variation, before any steps.
func (ss *Stepper[T]) PreVariationHook(fn callbackErr) {
	ss.preVariationHooks = append(ss.preVariationHooks, fn)
}

// PostStepHook runs after every step.
func (ss *Stepper[T]) PostStepHook(fn callbackErr) {
	ss.postStepHooks = append(ss.postStepHooks, fn)
}

// Step registers a function to make assertions on the running code, this is the
// main assertion set.
func (ss *Stepper[_]) Step(desc string, fn callback) {
	ss.steps = append(ss.steps, &step{
		desc: desc,
		fn:   fn,
	})
}

// Adds a variation to the stepper. Each Variation causes the Setup hooks,
// followed by the Variation, then every registered Step (and hooks), allowing
// one call to RunSteps to run multiple variations of the same test.
func (ss *Stepper[_]) Variation(desc string, fn callback) {
	ss.variations = append(ss.variations, &step{
		desc: desc,
		fn:   fn,
	})
}

// FollowAsserter returns an asserter which always points to the current test.
// It is valid only as long as the stepper is valid.
func (ss *Stepper[T]) ShiftingLogger() Logger {
	if ss.shiftingLogger == nil {
		ss.shiftingLogger = &shiftingLogger{
			Logger: ss,
		}
	}
	return ss.shiftingLogger
}

var _ StepSetter = &Stepper[RequiresTB]{}

// StepSetter is a minimal interface to configure the steps and hooks for a test.
type StepSetter interface {
	// Setup steps run at the start of each RunSteps call, or the start of each
	// Variation. The context passed to the setup will be canceled after all steps
	// are completed, or after any fatal error.
	Setup(fn callbackErr)

	// PreStepHook runs before every step, after any variations.
	PreStepHook(fn callbackErr)

	// PreVariationHook runs before every Variation, before any steps.
	PreVariationHook(fn callbackErr)

	// PostStepHook runs after every step.
	PostStepHook(fn callbackErr)

	// Step registers a function to make assertions on the running code, this is the
	// main assertion set.
	Step(desc string, fn callback)

	// Adds a variation to the stepper. Each Variation causes the Setup hooks,
	// followed by the Variation, then every registered Step (and hooks), allowing
	// one call to RunSteps to run multiple variations of the same test.
	Variation(desc string, fn callback)

	// LevelLog implements a global logger compatible with pentops/log.go/log.
	// Log lines will be captured into the currently running test step.
	LevelLog(level, message string, fields map[string]interface{})

	// Log logs any object, it can be used within test callbacks.
	// Log lines will be captured into the currently running test step.
	Log(...interface{})
}

func (ss *Stepper[T]) Log(args ...interface{}) {
	if ss.asserter == nil {
		fmt.Printf("WARNING: Log called on stepper without a current step. %s", fmt.Sprint(args...))
		return
	}
	ss.asserter.helper()
	ss.asserter.Log(args...)
}

// LevelLog implements a global logger compatible with pentops/log.go/log
// DefaultLogger, and others, to capture log lines from within the handlers
// into the test output
func (ss *Stepper[T]) LevelLog(level, message string, fields map[string]interface{}) {
	if ss.asserter != nil {
		ss.asserter.helper()
	}

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
		fmt.Printf("WARNING: Log called on stepper without a current step (level: %s and message: %s)\n   |%s\n", level, message, strings.Join(fieldStrings, "\n   |"))
		return
	}
	ss.Log(strings.Join(fieldStrings, "\n"))

}

const maxStackLen = 50

func (ss *Stepper[T]) LogQuery(ctx context.Context, statement string, params ...interface{}) {
	if ss.asserter != nil {
		ss.asserter.helper()
	}
	var pc [maxStackLen]uintptr
	n := runtime.Callers(4, pc[:])
	if n == 0 {
		ss.Log("No stack available")
		return
	}
	frames := runtime.CallersFrames(pc[:])
	var frame runtime.Frame
	var hasMore bool
	for {
		frame, hasMore = frames.Next()
		if !hasMore {
			break
		}
		if strings.HasPrefix(frame.Function, "github.com/pentops/sqrlx.go") {
			continue
		}
		break
	}

	lines := make([]string, 0, len(params)+1)
	lines = append(lines, fmt.Sprintf("QUERY (%s:%d)\n%s", frame.File, frame.Line, statement))
	for i, param := range params {
		switch param := param.(type) {
		case []byte:
			if len(param) > 1 && param[0] == '{' && param[len(param)-1] == '}' {
				lines = append(lines, fmt.Sprintf("  $%d %s", i+1, string(param)))
				continue
			}
		}
		lines = append(lines, fmt.Sprintf("  $%d %#v", i+1, param))
	}
	ss.Log(strings.Join(lines, "\n"))
}

// RunSteps is the main entry point of the stepper. For each Variation, or just
// once if no variation is registered, the Setup hooks are run, followed by the
// Variation, then every registered Step with pre and post hooks.
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

	backgroundCtx, backgroundCancel := context.WithCancel(ctx)
	defer backgroundCancel()
	chBackgroundErr := make(chan error)
	go func() {
		err := ss.runHooks(backgroundCtx, cancel, t, ss.backgroundHooks...)
		chBackgroundErr <- err
	}()

	for idx, step := range ss.steps {
		success := ss.runStep(ctx, t, fmt.Sprintf("%d %s", idx, step.desc), step, ss.preStepHooks, ss.postStepHooks)
		if !success {
			return
		}
	}

	backgroundCancel()
	if err := <-chBackgroundErr; err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		t.Log("Background hook failed", err)
		t.FailNow()
	}
}

func (ss *Stepper[T]) buildAsserter(t RequiresTB, cancel func()) *stepRun {
	asserter := &stepRun{
		RequiresTB: t,
		cancel:     cancel,
	}
	asserter.assertion = asserter.anon()
	ss.asserter = asserter
	return asserter
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
	asserter := ss.buildAsserter(t, cancel)
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

		asserter := ss.buildAsserter(t, cancel)
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
