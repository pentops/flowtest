package flowtest

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"

	"github.com/fatih/color"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type Stepper struct {
	steps    []*step
	asserter *asserter
	name     string
}

func (ss *Stepper) Log(level, message string, fields map[string]interface{}) {
	if ss.asserter == nil {
		panic("Log called on stepper without a current step")
	}

	ss.asserter.logLines = append(ss.asserter.logLines, logLine{
		level:   level,
		message: message,
		fields:  fields,
	})
}

func dumpLogLines(t TB, logLines []logLine) {
	for _, logLine := range logLines {
		fieldsString := make([]string, 0, len(logLine.fields))
		for k, v := range logLine.fields {
			if err, ok := v.(error); ok {
				v = err.Error()
			} else if stringer, ok := v.(fmt.Stringer); ok {
				v = stringer.String()
			}

			vStr, err := json.MarshalIndent(v, "  ", "  ")
			if err != nil {
				vStr = []byte(fmt.Sprintf("ERROR: %s", err))
			}

			fieldsString = append(fieldsString, fmt.Sprintf("  %s: %s", k, vStr))
		}
		levelColor, ok := levelColors[logLine.level]
		if !ok {
			levelColor = color.FgRed
		}

		levelColorPrint := color.New(levelColor).SprintFunc()
		fmt.Printf("%s: %s\n  %s\n", levelColorPrint(logLine.level), logLine.message, strings.Join(fieldsString, "\n  "))
	}
}

var levelColors = map[string]color.Attribute{
	"DEBUG": color.FgHiWhite,
	"INFO":  color.FgGreen,
	"WARN":  color.FgYellow,
	"ERROR": color.FgRed,
	"FATAL": color.FgMagenta,
}

func NewStepper(name string) *Stepper {
	return &Stepper{
		name: name,
	}
}

type logLine struct {
	level   string
	message string
	fields  map[string]interface{}
}

type step struct {
	desc     string
	asserter *asserter
	fn       func(context.Context, Asserter)
}

func (ss *Stepper) Step(desc string, fn func(t Asserter)) {
	wrapped := func(_ context.Context, a Asserter) {
		fn(a)
	}
	ss.steps = append(ss.steps, &step{
		desc: desc,
		fn:   wrapped,
	})
}

func (ss *Stepper) StepC(desc string, fn func(context.Context, Asserter)) {
	ss.steps = append(ss.steps, &step{
		desc: desc,
		fn:   fn,
	})
}

func (ss *Stepper) RunSteps(t TB) {
	ss.RunStepsC(context.Background(), t)
}
func (ss *Stepper) RunStepsC(ctx context.Context, t TB) {
	t.Helper()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	color.NoColor = false
	color.New(color.FgCyan).PrintfFunc()("\n>= %s\n", ss.name)
	blue := color.New(color.FgBlue).PrintfFunc()
	red := color.New(color.FgRed).PrintfFunc()
	for idx, step := range ss.steps {
		asserter := &asserter{
			t:      t,
			cancel: cancel,
		}
		ss.asserter = asserter
		step.asserter = asserter
		blue("STEP %s\n", step.desc)
		func() {
			defer func() {
				if r := recover(); r != nil {
					if r == earlyTestExit {
						return
					}

					// Can't use Fatal because that causes a panic loop
					asserter.log("FATAL", fmt.Sprintf("Test Step Panic: %v", r))
					asserter.failed = true
					fullStack := strings.Split(string(debug.Stack()), "\n")
					filteredStack := make([]string, 0, len(fullStack))
					filteredStack = append(filteredStack, fullStack[0])
					collect := false
					for _, line := range fullStack[1:] {
						if collect {
							filteredStack = append(filteredStack, line)
						} else if strings.Contains(line, "panic.go") {
							collect = true
							continue
						}
					}

					asserter.failStack = filteredStack
				}
			}()

			step.fn(ctx, asserter)
		}()
		if asserter.failed {
			for _, previous := range ss.steps[0:idx] {
				blue("STEP %s - OK\n", previous.desc)
				dumpLogLines(t, previous.asserter.logLines)
			}

			red("STEP %s FAILED\n", step.desc)
			dumpLogLines(t, asserter.logLines)
			if len(asserter.failStack) > 0 {
				fmt.Printf("Stack: %s\n", strings.Join(asserter.failStack, "\n"))
			}
			t.FailNow()
		}
	}
}

type TB interface {

	//Cleanup(func())
	Error(args ...any)
	Errorf(format string, args ...any)
	//Fail()
	FailNow()
	//Failed() bool
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
type Asserter interface {
	TB

	NoError(err error)
	Equal(want, got interface{})
	CodeError(err error, code codes.Code)
}

type RequiresTB interface {
	Helper()
	Log(args ...interface{})
}

type asserter struct {
	t         RequiresTB
	logLines  []logLine
	failed    bool
	failStack []string
	cancel    func()
}

func (t *asserter) NoError(err error) {
	t.t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func (t *asserter) Equal(want, got interface{}) {
	t.t.Helper()
	if got == nil || want == nil {
		if got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
		return
	}

	if aProto, ok := got.(proto.Message); ok {
		bProto, ok := want.(proto.Message)
		if !ok {
			t.Fatalf("want was a proto, got was not (%T)", got)
			return
		}
		if !proto.Equal(aProto, bProto) {
			t.Fatalf("got %v, want %v", got, want)
		}
		return
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func (t *asserter) CodeError(err error, code codes.Code) {
	if err == nil {
		t.Fatalf("got no error, want code %s", code)
		return
	}

	if s, ok := status.FromError(err); !ok {
		t.Fatalf("got error %s (%T), want code %s", err, err, code)
	} else {
		if s.Code() != code {
			t.Fatalf("got code %s, want %s", s.Code(), code)
		}
		return
	}
}

func (t *asserter) log(level, message string) {
	t.logLines = append(t.logLines, logLine{
		level:   level,
		message: message,
	})
}

func (t *asserter) Helper() {
	t.t.Helper()
}

func (t *asserter) Log(args ...interface{}) {
	t.t.Helper()
	t.log("DEBUG", fmt.Sprint(args...))
}

func (t *asserter) Logf(format string, args ...interface{}) {
	t.t.Helper()
	t.t.Log(fmt.Sprintf(format, args...))
}

func (t *asserter) Fatal(args ...interface{}) {
	t.t.Helper()
	t.log("FATAL", fmt.Sprint(args...))
	t.FailNow()
}

func (t *asserter) Fatalf(format string, args ...interface{}) {
	t.t.Helper()
	t.log("FATAL", fmt.Sprintf(format, args...))
	t.FailNow()
}

func (t *asserter) FailNow() {
	t.t.Helper()
	//t.failStack = debug.Stack()
	t.failed = true
	// Panic exits the caller, and is caught later. This is strange but not
	// clear if there is any other mechanism in go
	t.cancel()
	panic(earlyTestExit)
}

var earlyTestExit = "EARLY EXIT" //struct{}{}

func (t *asserter) Error(args ...interface{}) {
	t.t.Helper()
	t.Log("ERROR", fmt.Sprint(args...))
	t.failed = true
}

func (t *asserter) Errorf(format string, args ...interface{}) {
	t.t.Helper()
	t.Log("ERROR", fmt.Sprintf(format, args...))
	t.failed = true
}
