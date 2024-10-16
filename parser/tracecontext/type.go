package tracecontext

import (
	"github.com/dave/dst"
)

// TraceContext is an object that carries a transaction. This can either be
// an object that contains the transaction in a context, a context, or a transaction variable.
type TraceContext interface {
	// AssignTransactionVariable returns a dst.Stmt that
	// assigns a transaction to a variable from the object
	// that is carrying it.
	//
	// If the object is a transaction variable, then it will return nil
	AssignTransactionVariable(variableName string) dst.Stmt

	// Pass checks the function declaration and passes a trace context
	// to the expression calling that function based on its arguments and parameters.
	// It will update the call expression in place, and add any necessary
	// statments before the call in order to pass the transaction using the cursor.
	// It will return the TraceContext object that should  be passed to the next function.
	Pass(decl *dst.FuncDecl, call *dst.CallExpr, async bool) (TraceContext, error)

	// TransactionVariableName returns the name of the transaction variable
	TransactionVariableName() string

	// Type returns a string value representing the type of the TraceContext object
	Type() string
}
