package flowtest

import (
	"fmt"
	"reflect"

	"slices"

	"github.com/pentops/flowtest/be"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Assertion interface {
	// Sub creates a named sub-assertion
	Sub(name string, args ...any) Assertion

	// T accepts the assertion types like Equal which use generics and therefore
	// can't be a method of Assertion directly.
	T(outcome *be.Outcome)

	// NoError asserts be the error is nil, and fails the test if not
	NoError(err error)

	// MustMessage is used to assert topic requests work
	MustMessage(*emptypb.Empty, error)

	// Equal asserts be want == got. If extraLog is set, and the first
	// argument is a string it is used as a format string for the rest of the
	// arguments. If the first argument is not a string, everything is just
	// logged
	Equal(want, got any)

	// CodeError asserts be the error returned was non-nil and a Status error
	// with the given code
	CodeError(err error, code codes.Code)

	// NotEmpty asserts be the given values are not nil or zero values (zero
	// as in reflect.Value.IsZero)
	NotEmpty(got ...any)

	// NotNil asserts be the given values are not nil, assessing in order and
	// stopping at the first nil value (i.e. you can pass thing, thing.field,
	// thing.field.subfield)
	NotNil(gots ...any)

	// Nil asserts be the given values are nil
	Nil(gots ...any)

	// Fatal fails the test with the given message
	Fatal(args ...any)

	// Fatalf fails the test with the given format string
	Fatalf(format string, args ...any)

	// Error fails the test with the given message
	Error(args ...any)

	// Errorf fails the test with the given format string
	Errorf(format string, args ...any)

	Helper()
}

type assertionParent interface {
	Helper()
	Fatal(args ...any)
	Error(args ...any)
}

type assertion struct {
	name   string
	fatal  func(args ...any)
	error  func(args ...any)
	helper func()

	assertionParent
}

func (a *assertion) T(outcome *be.Outcome) {
	a.helper()
	if outcome != nil {
		a.fail(string(*outcome))
	}
}

func (a *assertion) Sub(name string, args ...any) Assertion {
	return &assertion{
		name:            fmt.Sprintf(name, args...),
		helper:          a.helper,
		fatal:           a.fatal,
		error:           a.error,
		assertionParent: a,
	}
}

func (a *assertion) fail(format string, args ...any) {
	a.helper()
	if a.name != "" {
		format = fmt.Sprintf("%s: %s", a.name, format)
	}
	a.fatal(fmt.Sprintf(format, args...))
}

func (a *assertion) Fatal(args ...any) {
	a.helper()
	a.fail(fmt.Sprint(args...))
}

func (a *assertion) Fatalf(format string, args ...any) {
	a.helper()
	a.fail(format, args...)
}

func (a *assertion) Error(args ...any) {
	a.helper()
	if a.name != "" {
		args = append([]any{a.name + ": "}, args...)
	}
	a.error(fmt.Sprint(args...))
}

func (a *assertion) Errorf(format string, args ...any) {
	a.helper()
	if a.name != "" {
		format = fmt.Sprintf("%s: %s", a.name, format)
	}
	a.error(fmt.Sprintf(format, args...))
}

func (a *assertion) NoError(err error) {
	a.helper()
	if err != nil {
		statErr, ok := status.FromError(err)
		if ok {
			a.fail("unexpected error %s\n  %s\n", err.Error(), prototext.Format(statErr.Proto()))
		} else {
			a.fail("got error %s (%T), want no error", err, err)
		}
	}
}

func (a *assertion) MustMessage(m *emptypb.Empty, err error) {
	a.helper()
	a.NoError(err)
	a.NotNil(m)
}

func (a *assertion) Equal(want, got any) {
	a.helper()
	if got == nil || want == nil {
		if got != want {
			a.fail("got %v, want %v", got, want)
		}
		return
	}

	if wantProto, ok := want.(proto.Message); ok {
		gotProto, ok := got.(proto.Message)
		if !ok {
			a.fail("want was a proto, got was (%T)", got)
			return
		}
		if !proto.Equal(wantProto, gotProto) {
			a.fail("got %v, want %v", got, want)
		}
		return
	}

	if !reflect.DeepEqual(got, want) {
		a.fail("got %v, want %v", got, want)
	}

}

func (a *assertion) NotEmpty(gots ...any) {
	a.helper()
	for _, got := range gots {
		if got == nil {
			a.fail("got nil, want non-nil")
			return
		}

		rv := reflect.ValueOf(got)
		if rv.IsZero() {
			a.fail("got zero value, want non-zero")
		}
	}

}

func isNil(got any) bool {
	if got == nil {
		return true
	}
	rv := reflect.ValueOf(got)
	if rv.Kind() != reflect.Pointer {
		return false
	}
	return rv.IsNil()
}

func (a *assertion) NotNil(gots ...any) {
	a.helper()
	if slices.ContainsFunc(gots, isNil) {
		a.fail("value was nil")
		return
	}
}

func (a *assertion) Nil(gots ...any) {
	a.helper()
	for _, got := range gots {
		if !isNil(got) {
			a.fail("value was not nil")
			return
		}
	}
}

func (a *assertion) CodeError(err error, code codes.Code) {
	if err == nil {
		a.fail("got no error, want code %s", code)
		return
	}

	if s, ok := status.FromError(err); !ok {
		a.fail("got error %s (%T), want code %s", err, err, code)
	} else {
		if s.Code() != code {
			a.fail("got code %s, want %s", s.Code(), code)
		}
		return
	}
}
