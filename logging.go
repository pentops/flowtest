package flowtest

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fatih/color"
)

type logLine struct {
	level   string
	message string
	fields  map[string]interface{}
}

func dumpLogLines(t RequiresTB, logLines []logLine) {
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
