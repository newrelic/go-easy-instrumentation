package tracecontext

import (
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
)

// addParamTest is a test case for the AddParam method.
// this should work for any tracecontext
type addParamTest struct {
	name     string
	tc       TraceContext
	funcDecl *dst.FuncDecl
	expect   *dst.Field
}

type passTestArgs struct {
	decl  *dst.FuncDecl // the function declaration of the call that is passed to the Pass method
	call  *dst.CallExpr // the call expression that is passed to the Pass method as an argument
	async bool          // a bool argument reporesenting if the call is async
}

// passTest is a test case for the Pass method.
type passTest struct {
	name       string       // the name of the test
	tc         TraceContext // the trace context to test
	args       passTestArgs // the arguments for the call expression that is passed to the Pass method
	wantArgs   []dst.Expr   // the arguments of the call function after the Pass method is called
	wantParams []*dst.Field // the parameters of the function declaration after the Pass method is called
	wantTc     TraceContext // the trace context that should be returned after the Pass method is called
}

func testPass(t *testing.T, tests []passTest) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTraceContext := tt.tc.Pass(tt.args.decl, tt.args.call, tt.args.async)

			assert.Equal(t, tt.wantArgs, tt.args.call.Args, "Pass() should add arguments to the call expression")
			assert.Equal(t, tt.wantTc, gotTraceContext)

			if tt.wantParams != nil {
				assert.Equal(t, tt.wantParams, tt.args.decl.Type.Params.List, "Pass() should add parameters to the function declaration if it can not pass to a valid type")
			}
		})
	}
}
