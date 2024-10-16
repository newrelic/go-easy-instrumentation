package tracecontext

import (
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
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

// testAddParam tests the AddParam method for any tracecontext
func testAddParam(t *testing.T, tests []addParamTest) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalParams := make([]*dst.Field, len(tt.funcDecl.Type.Params.List))
			for i, param := range tt.funcDecl.Type.Params.List {
				copy := *param
				originalParams[i] = &copy
			}

			tt.tc.AddParam(tt.funcDecl)

			if tt.expect == nil {
				for i, param := range tt.funcDecl.Type.Params.List {
					if i >= len(originalParams) {
						t.Errorf("AddParam() added a parameter to the function declaration when it should not have: %+v", param)
					} else {
						if param != originalParams[i] {
							assert.Equal(t, originalParams[i], param, "AddParam() modified a parameter to the function declaration when it should not have")
						}
					}
				}
			} else {
				if len(tt.funcDecl.Type.Params.List) != len(originalParams)+1 {
					t.Errorf("Expected AddParam() to add a new parameter: %+v", tt.expect)
				}
				for i, param := range tt.funcDecl.Type.Params.List {
					if i >= len(originalParams) {
						assert.Equal(t, tt.expect, param, "AddParam() should add a new parameter to the end of the list of parameters")
					} else {
						assert.Equal(t, originalParams[i], param, "AddParam() should not modify existing parameters")
					}
				}
			}
		})
	}
}

type passTestArgs struct {
	decl  *dst.FuncDecl // the function declaration of the call that is passed to the Pass method
	call  *dst.CallExpr // the call expression that is passed to the Pass method as an argument
	async bool          // a bool argument reporesenting if the call is async
}

// passTest is a test case for the Pass method.
type passTest struct {
	name           string       // the name of the test
	tc             TraceContext // the trace context to test
	args           passTestArgs // the arguments for the call expression that is passed to the Pass method
	wantStatements []dst.Stmt   // the statemtns that should be added before the call expression after Pass is called
	wantArgs       []dst.Expr   // the arguments of the call function after the Pass method is called
	wantTc         TraceContext // the trace context that should be returned after the Pass method is called
}

func testPass(t *testing.T, tests []passTest) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// outerFunciton is a function that only contains the call expression we are passing to
			outerFunction := &dst.FuncDecl{
				Name: dst.NewIdent("outerFunction"),
				Body: &dst.BlockStmt{
					List: []dst.Stmt{
						&dst.ExprStmt{
							X: tt.args.call,
						},
					},
				},
			}

			var gotTraceContext TraceContext
			dstutil.Apply(outerFunction, func(cursor *dstutil.Cursor) bool {
				node := cursor.Node()
				switch node.(type) {
				case *dst.ExprStmt:
					// we found the call expression
					gotTraceContext = tt.tc.Pass(tt.args.decl, tt.args.call, tt.args.async)
					return false
				}
				return true
			}, nil)

			if tt.wantStatements != nil {
				if len(outerFunction.Body.List)-1 != len(tt.wantStatements) {
					t.Errorf("Pass() did not add the expected number of statements before the call expression: expected %d, got %d", len(tt.wantStatements), len(outerFunction.Body.List)-1)
				} else {
					for i, stmt := range tt.wantStatements {
						assert.Equal(t, stmt, outerFunction.Body.List[i], "Pass() should add statements before the call expression")
					}
				}
			} else {
				if len(outerFunction.Body.List) != 1 {
					t.Errorf("Pass() should not add any statements before the call expression")
				}
			}

			assert.Equal(t, tt.wantArgs, tt.args.call.Args, "Pass() should add arguments to the call expression")
			assert.Equal(t, tt.wantTc, gotTraceContext)
		})
	}
}
