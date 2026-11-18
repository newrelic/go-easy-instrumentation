package cmd

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	rdebug "runtime/debug"
	"strings"
)

// issuesURL is where users are asked to report a crash.
const issuesURL = "https://github.com/newrelic/go-easy-instrumentation/issues"

// crashError is a panic recovered from a pipeline step.
//
// A panic used to kill the process outright: the stack trace was printed, but no
// usage data was sent, so crashes, the likeliest kind of bug in this tool, were
// invisible. Recovering turns them into an ordinary failed run that is reported
// like any other, while keeping the stack trace for the user's bug report.
type crashError struct {
	step  string
	value any
	stack []byte
}

func (e *crashError) Error() string {
	return fmt.Sprintf("internal error: %v", e.value)
}

// runStep runs one pipeline step, converting a panic into a *crashError.
//
// The pipeline runs on a single goroutine, so this catches every panic a step
// can raise.
func runStep(step string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &crashError{step: step, value: r, stack: rdebug.Stack()}
		}
	}()

	return fn()
}

// crashKind classifies a crash into a fixed vocabulary, or returns "" when err
// is not a crash.
//
// Only the kind is reported, never the panic value or the stack: a panic
// message can carry identifiers from the user's code, such as a type or
// function name.
func crashKind(err error) string {
	var crash *crashError
	if !errors.As(err, &crash) {
		return ""
	}

	rerr, ok := crash.value.(runtime.Error)
	if !ok {
		// An explicit panic(...) in this tool or a dependency.
		return "panic"
	}

	msg := rerr.Error()
	switch {
	case strings.Contains(msg, "nil pointer dereference"):
		return "nilPointer"
	case strings.Contains(msg, "index out of range"):
		return "indexOutOfRange"
	case strings.Contains(msg, "slice bounds out of range"):
		return "sliceBounds"
	case strings.Contains(msg, "interface conversion"):
		return "typeAssertion"
	case strings.Contains(msg, "assignment to entry in nil map"):
		return "nilMap"
	default:
		return "runtime"
	}
}

// printCrash writes the stack trace of a crash and a request to report it. It
// does nothing when err is not a crash.
func printCrash(w io.Writer, err error) {
	var crash *crashError
	if !errors.As(err, &crash) {
		return
	}

	fmt.Fprintf(w, "\ngo-easy-instrumentation crashed while %s. This is a bug in the tool, not in your code.\n", strings.ToLower(crash.step))
	fmt.Fprintf(w, "Please report it at %s and include the trace below.\n\n%s\n", issuesURL, crash.stack)
}
