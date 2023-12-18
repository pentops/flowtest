package flowtest

import (
	"context"
	"fmt"
)

type Asserter interface {
	TB
	Assertion
}

type Stepper[T RequiresTB] struct {
	steps    []*step
	asserter *stepRun
	name     string
}

// Log implements a global logger compatible with pentops/log.go/log
// DefaultLogger, and others, to capture log lines from within the handlers
// into the test output
func (ss *Stepper[T]) Log(level, message string, fields map[string]interface{}) {
	if ss.asserter == nil {
		panic(fmt.Sprintf("Log called on stepper without a current step (level: %s and message: %s)", level, message))
	}

	ss.asserter.logLines = append(ss.asserter.logLines, logLine{
		level:   level,
		message: message,
		fields:  fields,
	})
}

func NewStepper[T RequiresTB](name string) *Stepper[T] {
	return &Stepper[T]{
		name: name,
	}
}

type step struct {
	desc     string
	asserter *stepRun
	fn       func(context.Context, Asserter)
}

func (ss *Stepper[_]) Step(desc string, fn func(t Asserter)) {
	wrapped := func(_ context.Context, a Asserter) {
		fn(a)
	}
	ss.steps = append(ss.steps, &step{
		desc: desc,
		fn:   wrapped,
	})
}

func (ss *Stepper[_]) StepC(desc string, fn func(context.Context, Asserter)) {
	ss.steps = append(ss.steps, &step{
		desc: desc,
		fn:   fn,
	})
}

func (ss *Stepper[T]) RunSteps(t RunnableTB[T]) {
	ss.RunStepsC(context.Background(), t)
}

func (ss *Stepper[T]) RunStepsC(ctx context.Context, t RunnableTB[T]) {
	t.Helper()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for idx, step := range ss.steps {
		actuallyDidRun := false
		success := t.Run(fmt.Sprintf("%d %s", idx, step.desc), func(t T) {
			actuallyDidRun = true
			asserter := &stepRun{
				cancel: cancel,
			}
			asserter.assertion = asserter.anon()
			ss.asserter = asserter
			step.asserter = asserter
			asserter.t = t
			step.fn(ctx, asserter)
		})
		if !actuallyDidRun {
			// We can't prevent or override this (AFAIK), so we just have to fail
			t.Log(fmt.Sprintf("Step %s did not run - did you call test with a sub-filter?", step.desc))
			t.FailNow()
		}
		if !success {
			// in an ordinary go test, sub tests can fail then the outer test
			// continues, which is the main point here: We need all steps to run
			// in order.
			return
		}
	}
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
}

// RunnableTB is the subset of the testing.TB interface which this library
// requires. Keeping it to a minimum to allow alternate implementations
type RunnableTB[T RequiresTB] interface {
	RequiresTB
	Run(name string, f func(T)) bool
}

type stepRun struct {
	t         RequiresTB
	logLines  []logLine
	failed    bool
	failStack []string
	cancel    func()
	*assertion
}

func (t *stepRun) Failed() bool {
	return t.failed
}

func (t *stepRun) Helper() {
	t.t.Helper()
}

func (t *stepRun) log(level LogLevel, args ...interface{}) {
	t.t.Helper()
	if levelLogger, ok := t.t.(levelLogger); ok {
		levelLogger.LevelLog(level, args...)
	} else {
		if level == LogLevelDefault {
			t.t.Log(args...)
		} else {
			t.t.Log(fmt.Sprintf("%s: %s", level, fmt.Sprint(args...)))
		}
	}
}

func (t *stepRun) Log(args ...interface{}) {
	t.t.Helper()
	t.log(LogLevelDefault, args...)
}

func (t *stepRun) Logf(format string, args ...interface{}) {
	t.t.Helper()
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
	t.t.Helper()
	t.log(LogLevelFatal, fmt.Sprint(args...))
	t.FailNow()
}

func (t *stepRun) Fatalf(format string, args ...interface{}) {
	t.t.Helper()
	t.Fatal(fmt.Sprintf(format, args...))
}

func (t *stepRun) FailNow() {
	t.t.Helper()
	t.failed = true
	t.cancel()
	t.t.FailNow()
}

func (t *stepRun) Error(args ...interface{}) {
	t.t.Helper()
	t.log("ERROR", args...)
	t.failed = true
}

func (t *stepRun) Errorf(format string, args ...interface{}) {
	t.t.Helper()
	t.Error(fmt.Sprintf(format, args...))
	t.failed = true
}

func (t *stepRun) anon() *assertion {
	return &assertion{
		name: "",
		step: t,
	}
}
