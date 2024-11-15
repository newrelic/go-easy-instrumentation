package tracestate

import (
	"fmt"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate/traceobject"
)

// State stores the current state of the tracing process in the scope of a current function. Each function should
// be passed its own unique state object in oreder to preserve indepence between functions, and correctness.
// The state object will keep track of the current transaction variable, agent variable, and other stateful information
// about how instrumentation is being added and utilized in the scope of the current function.
//
// When creating a state, a TraceObject should be passed. This object identifies the way the transaction is being passed
// to the function, and takes care of how to correctly handle it.
type State struct {
	main             bool                    // main indicates that the current state is for a main function.
	txnUsed          bool                    // txnUsed indicates that the transaction variable has been used in the current scope.
	definedTxn       bool                    // definedTxn indicates that a transaction has been defined from an agent application in the current scope.
	async            bool                    // async indicates that the current function is an async function.
	needsSegment     bool                    // needsSegment indicates that a segment should be created for the current function.
	addTracingParam  bool                    // addTracingParam indicates that a tracing parameter should be added to the current function.
	agentVariable    string                  // agentVariable is the name of the agent variable in the main function.
	txnVariable      string                  // txnVariable is the name of the transaction variable in the current scope.
	object           traceobject.TraceObject // object is the object that contains the transaction, along with helper functions for how to utilize it.
	funcLitVariables map[string]*dst.FuncLit // funcLitVariables is a map of function literals that have been created in the current scope.
}

// Main creates a new State object for tracing a main function.
// We know the agent must be initialized in the main function.
//
// The agentVariable is the name of the agent variable in the main function.
// The trace object will always be a transaction in this case, since we have to
//
//	start a transaction in the main function.
func Main(agentVariable string) *State {
	return &State{
		main:             true,
		needsSegment:     false,
		agentVariable:    agentVariable,
		txnVariable:      codegen.DefaultTransactionVariable,
		object:           traceobject.NewTransaction(),
		funcLitVariables: make(map[string]*dst.FuncLit),
	}
}

// FunctionBody creates a trace state for tracing a function body.
func FunctionBody(transactionVariable string, obj ...traceobject.TraceObject) *State {
	var object traceobject.TraceObject
	if len(obj) > 0 {
		object = obj[0]
	} else {
		object = traceobject.NewTransaction()
	}

	return &State{
		txnVariable:      transactionVariable,
		object:           object,
		funcLitVariables: make(map[string]*dst.FuncLit),
	}
}

// functionCall creates a trace state for tracing a function call
// that is being made from the scope of the current function.
func (tc *State) functionCall(obj traceobject.TraceObject) *State {
	return &State{
		txnVariable:     tc.txnVariable,
		object:          obj,
		main:            false,
		needsSegment:    true,
		addTracingParam: true,
	}
}

// FunctionCall creates a trace state for tracing an async function call
// that is being made from the scope of the current function.
func (tc *State) goroutine(obj traceobject.TraceObject) *State {
	return &State{
		txnVariable:     tc.txnVariable,
		object:          obj,
		needsSegment:    true,
		addTracingParam: true,
		async:           true,
	}
}

// CreateSegment creates a segment for the current function if needed.
// Calling this will add a defer statement to the function declaration that will create a segment as the first
// statement in the function.
// This function will return the import path for the library that needs to be installed if a segment is created.
// The second return value will be true if a segment was created.
func (tc *State) CreateSegment(node dst.Node) (string, bool) {
	if !tc.needsSegment || tc.main {
		return "", false
	}
	switch decl := node.(type) {
	case *dst.FuncDecl:
		name := decl.Name.Name
		if tc.async {
			name = fmt.Sprintf("async %s", name)
		}

		codegen.PrependStatementToFunctionDecl(decl, codegen.DeferSegment(name, tc.TransactionVariable()))
		return codegen.NewRelicAgentImportPath, true
	case *dst.FuncLit:
		// function lits should alwas get a segment
		name := "function literal"
		if tc.async {
			name = "async " + name
		}

		codegen.PrependStatementToFunctionLit(decl, codegen.DeferSegment(name, tc.TransactionVariable()))
		return codegen.NewRelicAgentImportPath, true
	}

	return "", false
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
func (tc *State) TransactionVariable() dst.Expr {
	if tc.main || tc.txnVariable == "" {
		tc.txnVariable = codegen.DefaultTransactionVariable
	}

	tc.txnUsed = true
	return dst.NewIdent(tc.txnVariable)
}

// AgentVariable returns the name of the agent variable.
// This may return an empty string if no agent variable is in scope.
func (tc *State) AgentVariable() dst.Expr {
	if tc.agentVariable != "" {
		return dst.NewIdent(tc.agentVariable)
	}

	return codegen.GetApplication(tc.TransactionVariable())
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
func (tc *State) AddToCall(pkg *decorator.Package, call *dst.CallExpr, async bool) (*State, string) {
	callReturn := tc.object.AddToCall(pkg, call, tc.txnVariable, async)
	if callReturn.NeedsTx {
		tc.txnUsed = true
	}

	if async {
		return tc.goroutine(callReturn.TraceObject), callReturn.Import
	}
	return tc.functionCall(callReturn.TraceObject), callReturn.Import
}

// FuncDeclaration creates a trace state for a function declaration.
func (tc *State) FuncLiteralDeclaration(pkg *decorator.Package, lit *dst.FuncLit) *State {
	return tc.functionCall(tc.object)
}

// NoticeFuncLiteralAssignment is called when a function literal is assigned to a variable.
func (tc *State) NoticeFuncLiteralAssignment(pkg *decorator.Package, variable dst.Expr, lit *dst.FuncLit) {
	variableString := util.WriteExpr(variable, pkg)
	if variableString == "" {
		return
	}
	tc.funcLitVariables[variableString] = lit
}

// GetFuncLitVariable returns a function literal that was assigned to a variable in the scope of this function.
// TODO: Move this functionality to manager.
func (tc *State) GetFuncLitVariable(pkg *decorator.Package, variable dst.Expr) (*dst.FuncLit, bool) {
	variableString := util.WriteExpr(variable, pkg)
	if variableString == "" {
		return nil, false
	}

	lit, ok := tc.funcLitVariables[variableString]
	return lit, ok
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
func (tc *State) AddParameterToDeclaration(pkg *decorator.Package, node dst.Node) string {
	if tc.addTracingParam {
		switch decl := node.(type) {
		case *dst.FuncDecl:
			obj, goGet := tc.object.AddToFuncDecl(pkg, decl)
			tc.object = obj
			return goGet
		case *dst.FuncLit:
			obj, goGet := tc.object.AddToFuncLit(pkg, decl)
			tc.object = obj
			return goGet
		}
	}

	return ""
}

// AssignTransactionVariable assigns the transaction variable to a new variable that will always be the default transaction variable name.
// It will handle all the conditional checking for you, and will only add a transaction assignment if needed.
// In some cases, this may require a library to be installed, and it will return the import path for that library.
func (tc *State) AssignTransactionVariable(node dst.Node) string {
	// we dont need to assign this if nothing ever invoked the transaction
	if !tc.txnUsed {
		return ""
	}

	stmt, imp := tc.object.AssignTransactionVariable(codegen.DefaultTransactionVariable)
	if stmt != nil {
		tc.txnVariable = codegen.DefaultTransactionVariable

		// check that a segment was added, so we can fix the formatting
		switch decl := node.(type) {
		case *dst.FuncDecl:
			if tc.needsSegment {
				codegen.CreateStatementBlock(false, stmt, decl.Body.List[0])
			}
			codegen.PrependStatementToFunctionDecl(decl, stmt)
		case *dst.FuncLit:
			if tc.needsSegment {
				codegen.CreateStatementBlock(false, stmt, decl.Body.List[0])
			}
			codegen.PrependStatementToFunctionLit(decl, stmt)
		}
		return imp
	}

	return ""
}
