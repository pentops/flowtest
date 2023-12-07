package flowtest

/*
// MiniTB implementes the absolute minimum of RunnableTB to allow
// stepper to run without a real testing.T
type MiniTB struct {
}

blue := color.New(color.FgBlue).SprintfFunc()
red := color.New(color.FgRed).SprintfFunc()

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

		t.Log(red("STEP %s FAILED\n", step.desc))
		dumpLogLines(t, asserter.logLines)
		if len(asserter.failStack) > 0 {
			fmt.Printf("Stack: %s\n", strings.Join(asserter.failStack, "\n"))
		}
		t.FailNow()
		return
	}
type testStep struct {
	t RequiresTB
}

func (t *testStep) log(level, message string) {
	t.t.Helper()
	t.Logf("%s: %s", level, message)
	t.logLines = append(t.logLines, logLine{
		level:   level,
		message: message,
	})
}
*/
