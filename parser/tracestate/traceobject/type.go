package traceobject

import (
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// TraceObject is an object that contains New Relic tracing in the form of a transaction.
// Transactions can be injected into various object types that may require different
// methods of retrieval.
//
// This interface defines a standard set of behaviors that all objects containing a transaction
// must implement for the underlying transaction to be usable for tracing.
type TraceObject interface {
	// AddToCall adds a trace object to a call expression, passing it as an argument
	// to the function being invoked in the call.
	//
	// If an import needs to be added to support the changes made, it will be returned as a string.
	AddToCall(pkg *decorator.Package, call *dst.CallExpr, variableName string, async bool) (TraceObject, string)

	// AddToFuncDecl adds a trace object to a function declaration as a parameter, so that
	// trace objects can be passed in calls to this function.
	//
	// Make sure that the package passed is from the same package that the function is defined in.
	//
	// If an import needs to be added to support the changes made, it will be returned as a string.
	AddToFuncDecl(pkg *decorator.Package, decl *dst.FuncDecl) (TraceObject, string)

	// AddToFuncLit adds a trace object to a function literal definition as a parameter, so that
	// trace objects can be passed in calls to this function literal.
	//
	// Make sure that the package passed is from the same package that the function literal is defined in.
	//
	// If an import needs to be added to support the changes made, it will be returned as a string.
	AddToFuncLit(pkg *decorator.Package, lit *dst.FuncLit) (TraceObject, string)

	// AssignTransactionVariable fetches the transaction from the trace object and assigns it to a variable
	//
	// If an import needs to be added to support the changes made, it will be returned as a string.
	//
	// If an import needs to be added to support the changes made, it will be returned as a string.
	AssignTransactionVariable(variableName string) (dst.Stmt, string)
}
