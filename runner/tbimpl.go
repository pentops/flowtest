package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"

	"github.com/fatih/color"
	"github.com/pentops/flowtest"
	"github.com/pentops/flowtest/runner/testclient"
)

type TBImpl struct {
	failed  bool
	context context.Context
}

func (t *TBImpl) Helper() {}

var logColors = map[flowtest.LogLevel]color.Attribute{
	flowtest.LogLevelFatal:   color.FgRed,
	flowtest.LogLevelError:   color.FgRed,
	flowtest.LogLevelDefault: color.FgWhite,
}

func (t *TBImpl) LevelLog(level flowtest.LogLevel, args ...any) {
	var cc *color.Color
	if logColor, ok := logColors[level]; ok {
		cc = color.New(logColor)
	} else {
		cc = color.New()
	}
	if level != flowtest.LogLevelDefault {
		cc.Printf("%s: ", level)
	}
	if len(args) == 1 {
		switch arg := args[0].(type) {
		case *testclient.RequestLog:
			formatAPIResponse(arg)
			return
		}
	}
	fmt.Println(args...)
}

func (t *TBImpl) Context() context.Context {
	return t.context
}

func (t *TBImpl) Log(args ...any) {
	t.LevelLog(flowtest.LogLevelDefault, args...)
}

func (t *TBImpl) Fail() {
	t.failed = true
}

func (t *TBImpl) FailNow() {
	t.failed = true
	runtime.Goexit()
}

func (t *TBImpl) Run(name string, f func(*TBImpl)) bool {
	blue := color.New(color.FgBlue).PrintfFunc()
	child := &TBImpl{}

	blue("== STEP %s\n", name)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		f(child)
	}()
	wg.Wait()
	t.failed = t.failed || child.failed

	return !child.failed
}

func formatAPIResponse(ee *testclient.RequestLog) {
	fmt.Printf("  %s %s\n", ee.Method, ee.Path)
	for key, vals := range ee.RequestHeaders {
		for _, val := range vals {
			fmt.Printf("  | %s: %s\n", key, val)
		}
	}
	if ee.RequestBody != nil {
		formatted, _ := json.MarshalIndent(ee.RequestBody, "  | ", "  ")
		fmt.Printf("  | %s\n", formatted)
	}
	if ee.Error != nil {
		fmt.Printf("  ERR: %s\n", ee.Error)
	}
	if ee.ResponseStatus != 0 {
		fmt.Printf("  %d: %s\n", ee.ResponseStatus, http.StatusText(ee.ResponseStatus))
	}
	for key, vals := range ee.ResponseHeader {
		for _, val := range vals {
			fmt.Printf("  | %s: %s\n", key, val)
		}
	}
	if ee.ResponseBody != nil {
		rawBody, ok := ee.ResponseBody.([]byte)
		if ok {
			indentedBodyBytes := bytes.ReplaceAll(rawBody, []byte("\n"), []byte("\n  | "))
			fmt.Printf("  | %s\n", indentedBodyBytes)
		} else {
			formatted, _ := json.MarshalIndent(ee.ResponseBody, "  | ", "  ")
			fmt.Printf("  | %s\n", formatted)
		}
	}
}
