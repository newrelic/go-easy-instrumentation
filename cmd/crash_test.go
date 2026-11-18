package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunStepRecoversPanics(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
		kind string
	}{
		{"nil pointer", func() error { var p *struct{ x int }; _ = p.x; return nil }, "nilPointer"},
		{"index out of range", func() error { s := []int{}; i := 3; _ = s[i]; return nil }, "indexOutOfRange"},
		{"slice bounds", func() error { s := []int{1}; i := 3; _ = s[1:i]; return nil }, "sliceBounds"},
		{"type assertion", func() error { var v any = 1; _ = v.(string); return nil }, "typeAssertion"},
		{"nil map", func() error { var m map[string]int; m["a"] = 1; return nil }, "nilMap"},
		{"explicit panic", func() error { panic("unexpected node") }, "panic"},
		{"explicit panic with error", func() error { panic(errors.New("boom")) }, "panic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runStep("Instrumenting application", tt.fn)

			var crash *crashError
			assert.True(t, errors.As(err, &crash), "a panic must come back as a crashError")
			assert.Equal(t, "Instrumenting application", crash.step)
			assert.NotEmpty(t, crash.stack)
			assert.Equal(t, tt.kind, crashKind(err))
			// Wrapping, as the text path does, must not lose the classification.
			assert.Equal(t, tt.kind, crashKind(fmt.Errorf("Instrumenting application: %w", err)))
		})
	}
}

func TestRunStepPassesThroughOrdinaryResults(t *testing.T) {
	assert.NoError(t, runStep("Writing diff file", func() error { return nil }))

	want := errors.New("disk full")
	err := runStep("Writing diff file", func() error { return want })
	assert.Same(t, want, err)
	assert.Empty(t, crashKind(err), "an ordinary error is not a crash")
	assert.Empty(t, crashKind(nil))
}

func TestPrintCrash(t *testing.T) {
	var out bytes.Buffer
	printCrash(&out, runStep("Tracing package calls", func() error { panic("bad cursor") }))

	assert.Contains(t, out.String(), "crashed while tracing package calls")
	assert.Contains(t, out.String(), issuesURL)
	assert.Contains(t, out.String(), "goroutine", "the stack trace is needed for a bug report")

	out.Reset()
	printCrash(&out, errors.New("ordinary failure"))
	assert.Empty(t, out.String(), "ordinary failures print no crash report")
}
