package tracestate

import (
	"fmt"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate/traceobject"
)

// State stores the current state of the tracing process.
type State struct {
	main              bool                    // main indicates that the current state is for a main function.
	definedTxn        bool                    // definedTxn indicates that a transaction has been defined from an agent application in the current scope.
	async             bool                    // async indicates that the current function is an async function.
	needsSegment      bool                    // needsSegment indicates that a segment should be created for the current function.
	assignTxnVariable bool                    // assignTxnVariable indicates that the transaction variable should be assigned to a new variable name from the passed object
	agentVariable     string                  // agentVariable is the name of the agent variable in the main function.
	txnVariable       string                  // txnVariable is the name of the transaction variable in the current scope.
	object            traceobject.TraceObject // object is the object that contains the transaction, along with helper functions for how to utilize it.
}

// Main creates a new State object for tracing a main function.
// We know the agent must be initialized in the main function.
//
// The agentVariable is the name of the agent variable in the main function.
func Main(agentVariable string) *State {
	return &State{
		main:          true,
		agentVariable: agentVariable,
		txnVariable:   codegen.DefaultTransactionVariable,
		object:        traceobject.NewTransaction(),
	}
}

// FunctionCall creates a trace state for tracing a function call.
func (tc *State) FunctionCall() *State {
	return &State{
		txnVariable:  tc.txnVariable,
		object:       tc.object,
		needsSegment: true,
	}
}

// FunctionCall creates a trace state for tracing a function call.
func (tc *State) Goroutine() *State {
	return &State{
		txnVariable:  tc.txnVariable,
		object:       tc.object,
		needsSegment: true,
		async:        true,
	}
}

// FunctionBody creates a trace state for tracing a function body.
func FunctionBody(transactionVariable string) *State {
	return &State{
		txnVariable: transactionVariable,
		object:      traceobject.NewTransaction(),
	}
}

// CreateSegment creates a segment for the current function if needed.
// Calling this will add a defer statement to the function declaration that will create a segment as the first
// statement in the function.
func (tc *State) CreateSegment(decl *dst.FuncDecl) (string, bool) {
	if !tc.needsSegment || tc.main {
		return "", false
	}

	name := decl.Name.Name
	if tc.async {
		name = fmt.Sprintf("async %s", name)
	}

	codegen.PrependStatementToFunctionDecl(decl, codegen.DeferSegment(name, tc.TransactionVariable()))
	return codegen.NewRelicAgentImportPath, true
}

// WrapWithTransaction creates a transaction in the line before the current cursor position if all of these contidions are met:
//  1. The agent variable is in scope
//  2. The cursor is in a function body
//  3. We are in the main method
//
// The transaction created will always be assigned to a variable with the default transaction variable name.
func (tc *State) WrapWithTransaction(c *dstutil.Cursor, functionName, transactionVariable string) {
	if tc.main && tc.agentVariable != "" && c.Index() >= 0 {
		tc.txnVariable = transactionVariable
		start := codegen.StartTransaction(tc.agentVariable, tc.txnVariable, functionName, tc.definedTxn)
		tc.definedTxn = true
		end := codegen.EndTransaction(tc.txnVariable)
		codegen.WrapStatements(start, c.Node().(dst.Stmt), end)
		c.InsertBefore(start)
		c.InsertAfter(end)
	}
}

// IsMain returns true if the current state is for a main function.
func (tc *State) IsMain() bool {
	return tc.main
}

// TransactionVariable returns the name of the transaction variable.
func (tc *State) TransactionVariable() string {
	if !tc.main && tc.txnVariable == "" {
		tc.assignTxnVariable = true
		tc.txnVariable = codegen.DefaultTransactionVariable
	}
	return tc.txnVariable
}

// AgentVariable returns the name of the agent variable.
// This may return an empty string if no agent variable is in scope.
func (tc *State) AgentVariable() string {
	return tc.agentVariable
}

// AddToCall passes a New Relic transaction, or an object that contains one, to a function call.
// It MUST be passed the decorator package for the package the function call is being made in.
// If the function call is a goroutine, async should be true.
//
// This function returns a string for any library that needs to be imported with go get before
// the code will compile.
//
// This function will update the call expression in place. The object containing the transaction
// that is passed to the function will depend on what parameters the function takes, and what is
// being passed. In general, the rules for what is passed are:
//  1. If the function takes an argument of the same type as the tracing object, we will pass as that type.
//  2. If the function takes an argument of type context.Context, we will inject the transaction into the passed context.
//  3. If neither case 1 or case 2 is met, a *newrelic.Transaction will be passed as the last argument of the function.
func (tc *State) AddToCall(pkg *decorator.Package, call *dst.CallExpr, async bool) string {
	return tc.object.AddToCall(pkg, call, tc.txnVariable, async)
}

// AddToFunctionDecl adds a parameter to pass a New Relic transaction to a function declaration if needed.
// It MUST be passed the decorator package for the package the function call is being made in.
//
// This function returns a string for any library that needs to be imported with go get before
// the code will compile.
//
// This function will update the call expression in place. The object containing the transaction
// that is passed to the function will depend on what parameters the function takes, and what is
// being passed. In general, the rules for what is passed are:
//  1. If the function takes an argument of the same type as the tracing object, we will pass as that type.
//  2. If the function takes an argument of type context.Context, we will inject the transaction into the passed context.
//  3. If neither case 1 or case 2 is met, a *newrelic.Transaction will be passed as the last argument of the function.
func (tc *State) AddToFunctionDecl(pkg *decorator.Package, decl *dst.FuncDecl) string {
	return tc.object.AddToFuncDecl(pkg, decl)
}

// AddToFunctionDecl adds a parameter to pass a New Relic transaction to a function declaration if needed.
// It MUST be passed the decorator package for the package the function call is being made in.
//
// This function returns a string for any library that needs to be imported with go get before
// the code will compile.
//
// This function will update the call expression in place. The object containing the transaction
// that is passed to the function will depend on what parameters the function takes, and what is
// being passed. In general, the rules for what is passed are:
//  1. If the function takes an argument of the same type as the tracing object, we will pass as that type.
//  2. If the function takes an argument of type context.Context, we will inject the transaction into the passed context.
//  3. If neither case 1 or case 2 is met, a *newrelic.Transaction will be passed as the last argument of the function.
func (tc *State) AddToFunctionLiteral(pkg *decorator.Package, lit *dst.FuncLit) string {
	return tc.object.AddToFuncLit(pkg, lit)
}

// AssignTransactionVariable assigns the transaction variable to a new variable that will always be the default transaction variable name.
// It will handle all the conditional checking for you, and will only add a transaction assignment if needed.
// In some cases, this may require a library to be installed, and it will return the import path for that library.
func (tc *State) AssignTransactionVariable(decl *dst.FuncDecl) string {
	if tc.assignTxnVariable {
		stmt, imp := tc.object.AssignTransactionVariable(codegen.DefaultTransactionVariable)
		if stmt != nil {
			tc.txnVariable = codegen.DefaultTransactionVariable
			codegen.PrependStatementToFunctionDecl(decl, stmt)
			return imp
		}
	}

	return ""
}
