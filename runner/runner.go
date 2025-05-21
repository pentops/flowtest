package runner

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/pentops/flowtest"
	"slices"
)

type TestCallback func(flowtest.StepSetter)

type Test struct {
	Order float64
	Name  string
	Setup TestCallback

	CategoryTags map[string][]string
	Tags         []string
}

func (t *Test) CategoriesMatch(filter map[string][]string) bool {
	for filterKey, filterValues := range filter {
		matched := false
		for _, filterValue := range filterValues {
			if slices.Contains(t.CategoryTags[filterKey], filterValue) {
				matched = true
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

func (t *Test) TagsMatch(filter []string) bool {
	for _, filterTag := range filter {
		matched := slices.Contains(t.Tags, filterTag)
		if !matched {
			return false
		}
	}

	return true
}

type TestSet []Test

func (ts TestSet) Len() int {
	return len(ts)
}

func (ts TestSet) Less(i, j int) bool {
	return ts[i].Order < ts[j].Order
}

func (ts TestSet) Swap(i, j int) {
	ts[i], ts[j] = ts[j], ts[i]
}

func splitTags(tags []string) ([]string, map[string][]string) {
	categoryTags := map[string][]string{}
	flatTags := []string{}
	for _, tag := range tags {
		parts := strings.SplitN(tag, "=", 2)
		if len(parts) == 2 {
			category, ok := categoryTags[parts[0]]
			if !ok {
				category = []string{}
			}
			category = append(category, parts[1])
			categoryTags[parts[0]] = category
		} else {
			flatTags = append(flatTags, tag)
		}
	}
	return flatTags, categoryTags
}

// Register registers a testing callback into the test set.
func (ts *TestSet) Register(order float64, name string, callback TestCallback, tags ...string) {

	flatTags, categoryTags := splitTags(tags)
	*ts = append(*ts, Test{
		Order:        order,
		Name:         name,
		Setup:        callback,
		Tags:         flatTags,
		CategoryTags: categoryTags,
	})
}

var DefaultTestSet TestSet

// Register registers a testing callback into the default test set.
// It is usually called from an init() function.
func Register(order float64, name string, callback TestCallback, tags ...string) {
	DefaultTestSet.Register(order, name, callback, tags...)
}

// Run runs the registered tests, filtered per the filter rules.
func (ts *TestSet) Run(ctx context.Context, filter []string) error {
	sort.Sort(ts)

	flatFilter, catFilter := splitTags(filter)
	toRun := make(TestSet, 0, len(*ts))
	for _, test := range *ts {
		if !test.TagsMatch(flatFilter) {
			fmt.Printf("skipping %s due to tags %v != %v\n", test.Name, filter, test.Tags)
			continue
		}
		if !test.CategoriesMatch(catFilter) {
			fmt.Printf("skipping %s due to tags %v != %v\n", test.Name, filter, test.Tags)
			continue
		}
		toRun = append(toRun, test)
	}

	failures := []string{}

	red := color.New(color.FgRed).PrintfFunc()
	green := color.New(color.FgGreen).PrintfFunc()

	for _, test := range toRun {
		test := test

		testLabel := fmt.Sprintf("%f: %s", test.Order, test.Name)
		stepper := flowtest.NewStepper[*TBImpl](testLabel)

		green("== %s == Running\n", testLabel)
		tb := &TBImpl{}

		test.Setup(stepper)

		stepper.RunSteps(tb)
		if tb.failed {
			failures = append(failures, testLabel)
			red("== Failed %s\n", testLabel)
		}
		color.New(color.FgGreen).Printf("== Finished %s\n", testLabel)
	}

	if len(failures) > 0 {
		red("Tests complete with %d failures:\n", len(failures))
		for _, failure := range failures {
			fmt.Printf(" - %s\n", failure)
		}

		return fmt.Errorf("tests complete with %d failures", len(failures))
	}
	return nil
}
