package tracecontext

import "fmt"

// PassError is an error that occurs when a TraceContext object fails to pass its transaction to function call.
type PassError struct {
	tracecontextType string // the string representation of the TraceContext object type
	err              string // the error message
}

func NewPassError(traceContext TraceContext, err string) *PassError {
	return &PassError{
		tracecontextType: traceContext.Type(),
		err:              err,
	}
}

func (e *PassError) Error() string {
	return fmt.Sprintf("failed to pass tracing from %s: %s", e.tracecontextType, e.err)
}
