package tracecontext

import (
	"github.com/dave/dst"
)

// TraceContext is an object that carries a transaction. This can either be
// an object that contains the transaction in a context, a context, or a transaction variable.
type TraceContext interface {

	// Pass checks the function declaration and passes a trace context
	// to the expression calling that function based on its arguments and parameters.
	// It will update the call expression in place, and add any necessary
	// statments before the call in order to pass the transaction using the cursor.
	// It will return the TraceContext object that should  be passed to the next function.
	Pass(decl *dst.FuncDecl, call *dst.CallExpr, async bool) TraceContext

	// Transaction returns the name of the transaction variable, and a statment that assigns it to a variable if needed.
	//
	// If the transaction does not neeed to be assigned to a variable, it will return nil.
	// This function will store whether or not an agent variable has been assigned in the scope of a function.
	// Proper usage looks like this:
	// 		transaction, assign := tc.Transaction()
	// 		if assign != nil && cursor.Index() >= 0 {
	// 			cursor.InsertBefore(assign)
	// 		}
	Transaction() (string, dst.Stmt)

	// Agent returns the name of the agent variable, and a statment that assigns it to a variable if needed.
	//
	// If the agent does not neeed to be assigned to a variable, it will return nil.
	// This function will store whether or not an agent variable has been assigned in the scope of a function.
	// Proper usage looks like this:
	// 		agent, assign := tc.Agent()
	// 		if assign != nil && cursor.Index() >= 0 {
	// 			cursor.InsertBefore(assign)
	// 		}
	Agent() (string, dst.Stmt)

	// Type returns a string value representing the type of the TraceContext object
	Type() string
}
